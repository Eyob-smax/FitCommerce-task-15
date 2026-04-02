package reports

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"fitcommerce/backend/internal/auth"
	"fitcommerce/backend/internal/http/response"
	"fitcommerce/backend/internal/middleware"
)

// KPI definitions (documented for reproducibility):
//
// member_growth:     COUNT of members created in the period
// member_churn:      COUNT of members whose status changed to inactive/cancelled/expired in the period
// renewal_rate:      (members whose membership_end > period_end) / (total active members) * 100
// engagement:        COUNT of engagement_events in the period
// class_fill_rate:   AVG(booked_seats / capacity * 100) for classes in the period
// coach_productivity: COUNT of completed classes per coach in the period

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/reports")
	g.GET("/dashboard", middleware.RequireRoles(
		auth.RoleAdministrator, auth.RoleOperationsManager,
		auth.RoleProcurementSpecialist, auth.RoleCoach,
	), h.dashboard)
	g.GET("/member-growth", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.memberGrowth)
	g.GET("/churn", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.churn)
	g.GET("/inventory", middleware.RequireRoles(
		auth.RoleAdministrator, auth.RoleOperationsManager, auth.RoleProcurementSpecialist,
	), h.inventoryReport)
	g.GET("/group-buys", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.groupBuysReport)
	g.GET("/coach/:id", middleware.RequireRoles(
		auth.RoleAdministrator, auth.RoleOperationsManager, auth.RoleCoach,
	), h.coachReport) // coach sees own, admin/ops sees any
}

// ── Time helpers ──────────────────────────────────────────────────────────────

func parsePeriod(c *gin.Context) (start, end time.Time) {
	now := time.Now().UTC()
	granularity := c.DefaultQuery("granularity", "monthly")

	if s := c.Query("start_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			start = t
		}
	}
	if e := c.Query("end_date"); e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			end = t.Add(24*time.Hour - time.Nanosecond) // end of day
		}
	}
	if !start.IsZero() && !end.IsZero() && end.Before(start) {
		start, end = end, start
	}

	if start.IsZero() || end.IsZero() {
		switch granularity {
		case "daily":
			start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			end = start.Add(24*time.Hour - time.Nanosecond)
		case "weekly":
			weekday := int(now.Weekday())
			start = time.Date(now.Year(), now.Month(), now.Day()-weekday, 0, 0, 0, 0, time.UTC)
			end = start.Add(7*24*time.Hour - time.Nanosecond)
		default: // monthly
			start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			end = start.AddDate(0, 1, 0).Add(-time.Nanosecond)
		}
	}
	return
}

// ── KPI: Dashboard ────────────────────────────────────────────────────────────

type dashboardKPI struct {
	MemberGrowth      int     `json:"member_growth"`
	MemberChurn       int     `json:"member_churn"`
	RenewalRate       float64 `json:"renewal_rate"`
	Engagement        int     `json:"engagement"`
	ClassFillRate     float64 `json:"class_fill_rate"`
	CoachProductivity int     `json:"coach_productivity"`
	Period            string  `json:"period"`
	StartDate         string  `json:"start_date"`
	EndDate           string  `json:"end_date"`
}

func (h *Handler) dashboard(c *gin.Context) {
	start, end := parsePeriod(c)
	ctx := c.Request.Context()
	role := c.GetString(middleware.KeyRole)
	itemCategory := c.Query("item_category")

	kpi := dashboardKPI{
		Period:    c.DefaultQuery("granularity", "monthly"),
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
	}

	locationFilter := ""
	args := []interface{}{start, end}
	argIdx := 3
	if locID := c.Query("location_id"); locID != "" {
		locationFilter = fmt.Sprintf(" AND location_id = $%d", argIdx)
		args = append(args, locID)
		argIdx++
	}

	// Member growth
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM members WHERE created_at BETWEEN $1 AND $2`+
		locationFilter, args...).Scan(&kpi.MemberGrowth)

	// Member churn
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM members WHERE status IN ('inactive','cancelled','expired')
		AND updated_at BETWEEN $1 AND $2`+locationFilter, args...).Scan(&kpi.MemberChurn)

	// Renewal rate
	var totalActive, renewed int
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM members WHERE status = 'active'`).Scan(&totalActive)
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM members WHERE status = 'active' AND membership_end > $1`, end).Scan(&renewed)
	if totalActive > 0 {
		kpi.RenewalRate = float64(renewed) / float64(totalActive) * 100
	}

	// Engagement
	if itemCategory == "" {
		_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM engagement_events WHERE occurred_at BETWEEN $1 AND $2`,
			start, end).Scan(&kpi.Engagement)
	} else {
		_ = h.db.QueryRow(ctx, `
			SELECT COUNT(DISTINCT e.id)
			FROM engagement_events e
			LEFT JOIN orders o ON e.event_type = 'order' AND e.entity_id = o.id
			LEFT JOIN order_line_items oli ON o.id = oli.order_id
			LEFT JOIN items oi ON oi.id = oli.item_id
			LEFT JOIN group_buys gb ON e.event_type = 'group_buy_join' AND e.entity_id = gb.id
			LEFT JOIN items gi ON gi.id = gb.item_id
			WHERE e.occurred_at BETWEEN $1 AND $2
			AND (
				(e.event_type = 'order' AND oi.category = $3)
				OR (e.event_type = 'group_buy_join' AND gi.category = $3)
			)
		`, start, end, itemCategory).Scan(&kpi.Engagement)
	}

	// Class fill rate — scoped by location when provided
	_ = h.db.QueryRow(ctx, `SELECT COALESCE(AVG(CASE WHEN capacity > 0 THEN booked_seats::float / capacity * 100 ELSE 0 END), 0)
		FROM classes WHERE scheduled_at BETWEEN $1 AND $2`+locationFilter, args...).Scan(&kpi.ClassFillRate)

	// Coach productivity (completed classes count)
	// Scope: coach role always sees own data; admin/ops can filter by coach_id query param.
	coachFilter := c.Query("coach_id")
	if role == auth.RoleCoach {
		userID := c.GetString(middleware.KeyUserID)
		var ownCoachID string
		_ = h.db.QueryRow(ctx, `SELECT id FROM coaches WHERE user_id = $1`, userID).Scan(&ownCoachID)
		_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM classes WHERE coach_id = $1 AND status = 'completed'
			AND scheduled_at BETWEEN $2 AND $3`, ownCoachID, start, end).Scan(&kpi.CoachProductivity)
	} else if coachFilter != "" {
		_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM classes WHERE coach_id = $1 AND status = 'completed'
			AND scheduled_at BETWEEN $2 AND $3`, coachFilter, start, end).Scan(&kpi.CoachProductivity)
	} else {
		_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM classes WHERE status = 'completed'
			AND scheduled_at BETWEEN $1 AND $2`, start, end).Scan(&kpi.CoachProductivity)
	}

	response.OK(c, kpi)
}

// ── KPI: Member Growth Detail ─────────────────────────────────────────────────

func (h *Handler) memberGrowth(c *gin.Context) {
	start, end := parsePeriod(c)
	ctx := c.Request.Context()

	var growth, total int
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM members WHERE created_at BETWEEN $1 AND $2`, start, end).Scan(&growth)
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM members`).Scan(&total)

	response.OK(c, gin.H{
		"new_members":   growth,
		"total_members": total,
		"start_date":    start.Format("2006-01-02"),
		"end_date":      end.Format("2006-01-02"),
	})
}

// ── KPI: Churn ────────────────────────────────────────────────────────────────

func (h *Handler) churn(c *gin.Context) {
	start, end := parsePeriod(c)
	ctx := c.Request.Context()

	var churned, total int
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM members WHERE status IN ('inactive','cancelled','expired')
		AND updated_at BETWEEN $1 AND $2`, start, end).Scan(&churned)
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM members`).Scan(&total)

	rate := 0.0
	if total > 0 {
		rate = float64(churned) / float64(total) * 100
	}

	response.OK(c, gin.H{
		"churned_members": churned,
		"total_members":   total,
		"churn_rate":      rate,
		"start_date":      start.Format("2006-01-02"),
		"end_date":        end.Format("2006-01-02"),
	})
}

// ── KPI: Inventory Report ─────────────────────────────────────────────────────

func (h *Handler) inventoryReport(c *gin.Context) {
	ctx := c.Request.Context()

	type stockSummary struct {
		TotalItems    int `json:"total_items"`
		TotalOnHand   int `json:"total_on_hand"`
		TotalReserved int `json:"total_reserved"`
		LowStock      int `json:"low_stock_count"`
	}
	var s stockSummary
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM items`).Scan(&s.TotalItems)
	_ = h.db.QueryRow(ctx, `SELECT COALESCE(SUM(on_hand), 0), COALESCE(SUM(reserved), 0) FROM inventory_stock`).Scan(&s.TotalOnHand, &s.TotalReserved)
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM inventory_stock WHERE on_hand < 5`).Scan(&s.LowStock)

	response.OK(c, s)
}

// ── KPI: Group Buys Report ────────────────────────────────────────────────────

func (h *Handler) groupBuysReport(c *gin.Context) {
	ctx := c.Request.Context()
	start, end := parsePeriod(c)

	type gbSummary struct {
		Total     int `json:"total"`
		Succeeded int `json:"succeeded"`
		Failed    int `json:"failed"`
		Active    int `json:"active"`
		Cancelled int `json:"cancelled"`
	}
	var s gbSummary
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM group_buys WHERE created_at BETWEEN $1 AND $2`, start, end).Scan(&s.Total)
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM group_buys WHERE status = 'succeeded' AND updated_at BETWEEN $1 AND $2`, start, end).Scan(&s.Succeeded)
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM group_buys WHERE status = 'failed' AND updated_at BETWEEN $1 AND $2`, start, end).Scan(&s.Failed)
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM group_buys WHERE status = 'active'`).Scan(&s.Active)
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM group_buys WHERE status = 'cancelled' AND updated_at BETWEEN $1 AND $2`, start, end).Scan(&s.Cancelled)

	response.OK(c, s)
}

// ── KPI: Coach Report ─────────────────────────────────────────────────────────

func (h *Handler) coachReport(c *gin.Context) {
	coachID := c.Param("id")
	role := c.GetString(middleware.KeyRole)
	userID := c.GetString(middleware.KeyUserID)

	if role != auth.RoleAdministrator && role != auth.RoleOperationsManager && role != auth.RoleCoach {
		response.Forbidden(c)
		return
	}

	// Coach can only see their own report
	if role == auth.RoleCoach {
		var ownCoachID string
		_ = h.db.QueryRow(c.Request.Context(), `SELECT id FROM coaches WHERE user_id = $1`, userID).Scan(&ownCoachID)
		if ownCoachID != coachID {
			response.Forbidden(c)
			return
		}
	}

	start, end := parsePeriod(c)
	ctx := c.Request.Context()

	var totalClasses, completedClasses, totalAttendees int
	var avgFillRate float64
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM classes WHERE coach_id = $1 AND scheduled_at BETWEEN $2 AND $3`,
		coachID, start, end).Scan(&totalClasses)
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM classes WHERE coach_id = $1 AND status = 'completed' AND scheduled_at BETWEEN $2 AND $3`,
		coachID, start, end).Scan(&completedClasses)
	_ = h.db.QueryRow(ctx, `SELECT COALESCE(SUM(booked_seats), 0) FROM classes WHERE coach_id = $1 AND scheduled_at BETWEEN $2 AND $3`,
		coachID, start, end).Scan(&totalAttendees)
	_ = h.db.QueryRow(ctx, `SELECT COALESCE(AVG(CASE WHEN capacity > 0 THEN booked_seats::float / capacity * 100 ELSE 0 END), 0)
		FROM classes WHERE coach_id = $1 AND scheduled_at BETWEEN $2 AND $3`,
		coachID, start, end).Scan(&avgFillRate)

	response.OK(c, gin.H{
		"coach_id":          coachID,
		"total_classes":     totalClasses,
		"completed_classes": completedClasses,
		"total_attendees":   totalAttendees,
		"avg_fill_rate":     avgFillRate,
		"start_date":        start.Format("2006-01-02"),
		"end_date":          end.Format("2006-01-02"),
	})
}
