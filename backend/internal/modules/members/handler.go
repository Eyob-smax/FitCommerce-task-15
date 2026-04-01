package members

import (
	"strings"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"fitcommerce/backend/internal/auth"
	"fitcommerce/backend/internal/http/response"
	"fitcommerce/backend/internal/middleware"
	"fitcommerce/backend/internal/modules/audit"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/members")
	// List is restricted to staff — members cannot enumerate all member records
	g.GET("", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.list)
	g.POST("", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.create)
	// Read access is limited to admin/ops/member; member role is further constrained to own record.
	g.GET("/:id", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager, auth.RoleMember), h.get)
	g.PATCH("/:id", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.update)
	g.GET("/:id/orders", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager, auth.RoleMember), h.orders)
	g.GET("/:id/group-buys", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager, auth.RoleMember), h.groupBuys)
}

type createMemberRequest struct {
	UserID          string  `json:"user_id" binding:"required"`
	LocationID      *string `json:"location_id"`
	MembershipType  string  `json:"membership_type" binding:"required"`
	MembershipStart *string `json:"membership_start"`
	MembershipEnd   *string `json:"membership_end"`
	Status          string  `json:"status" binding:"required"`
}

type updateMemberRequest struct {
	LocationID      *string `json:"location_id"`
	MembershipType  *string `json:"membership_type"`
	MembershipStart *string `json:"membership_start"`
	MembershipEnd   *string `json:"membership_end"`
	Status          *string `json:"status"`
}

var validStatuses = map[string]bool{
	"active": true, "inactive": true, "expired": true, "cancelled": true,
}

type memberResponse struct {
	ID             string  `json:"id"`
	UserID         string  `json:"user_id"`
	LocationID     *string `json:"location_id"`
	MembershipType string  `json:"membership_type"`
	MembershipStart *string `json:"membership_start"`
	MembershipEnd   *string `json:"membership_end"`
	Status         string  `json:"status"`
	CreatedAt      string  `json:"created_at"`
}

func (h *Handler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "25"))
	if page < 1 { page = 1 }
	if perPage < 1 || perPage > 100 { perPage = 25 }

	var total int
	_ = h.db.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM members`).Scan(&total)

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, user_id, location_id, membership_type, membership_start::text, membership_end::text, status, created_at
		FROM members ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, perPage, (page-1)*perPage)
	if err != nil { response.InternalError(c); return }
	defer rows.Close()

	members := []memberResponse{}
	for rows.Next() {
		var m memberResponse
		var createdAt time.Time
		if rows.Scan(&m.ID, &m.UserID, &m.LocationID, &m.MembershipType, &m.MembershipStart, &m.MembershipEnd, &m.Status, &createdAt) == nil {
			m.CreatedAt = createdAt.UTC().Format(time.RFC3339)
			members = append(members, m)
		}
	}
	response.OKPaginated(c, members, response.Meta{Page: page, PerPage: perPage, Total: total})
}

func (h *Handler) create(c *gin.Context) {
	var req createMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	if !validStatuses[req.Status] {
		response.BadRequest(c, "VALIDATION_ERROR", "invalid status")
		return
	}
	if strings.TrimSpace(req.MembershipType) == "" {
		response.BadRequest(c, "VALIDATION_ERROR", "membership_type is required")
		return
	}

	var member memberResponse
	var createdAt time.Time
	err := h.db.QueryRow(c.Request.Context(), `
		INSERT INTO members (user_id, location_id, membership_type, membership_start, membership_end, status)
		VALUES ($1, $2, $3, $4::date, $5::date, $6)
		RETURNING id, user_id, location_id, membership_type, membership_start::text, membership_end::text, status, created_at
	`, req.UserID, req.LocationID, req.MembershipType, req.MembershipStart, req.MembershipEnd, req.Status,
	).Scan(&member.ID, &member.UserID, &member.LocationID, &member.MembershipType,
		&member.MembershipStart, &member.MembershipEnd, &member.Status, &createdAt)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			response.Conflict(c, "member already exists for user")
			return
		}
		response.InternalError(c)
		return
	}

	member.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	actorID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &actorID, "member.create", "member", member.ID, nil, req, c.ClientIP())
	response.Created(c, member)
}

func (h *Handler) get(c *gin.Context) {
	id := c.Param("id")

	// Member role may only view their own record
	role := c.GetString(middleware.KeyRole)
	if role == auth.RoleMember {
		userID := c.GetString(middleware.KeyUserID)
		var ownMemberID string
		err := h.db.QueryRow(c.Request.Context(), `SELECT id FROM members WHERE user_id = $1`, userID).Scan(&ownMemberID)
		if err != nil || ownMemberID != id {
			response.NotFound(c, "member")
			return
		}
	}

	var m memberResponse
	var createdAt time.Time
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, user_id, location_id, membership_type, membership_start::text, membership_end::text, status, created_at
		FROM members WHERE id = $1
	`, id).Scan(&m.ID, &m.UserID, &m.LocationID, &m.MembershipType, &m.MembershipStart, &m.MembershipEnd, &m.Status, &createdAt)
	if err == pgx.ErrNoRows { response.NotFound(c, "member"); return }
	if err != nil { response.InternalError(c); return }
	m.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	response.OK(c, m)
}

func (h *Handler) update(c *gin.Context) {
	id := c.Param("id")
	var req updateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	if req.Status != nil && !validStatuses[*req.Status] {
		response.BadRequest(c, "VALIDATION_ERROR", "invalid status")
		return
	}
	if req.MembershipType != nil && strings.TrimSpace(*req.MembershipType) == "" {
		response.BadRequest(c, "VALIDATION_ERROR", "membership_type cannot be empty")
		return
	}

	sets := []string{"updated_at = NOW()", "version = version + 1"}
	args := []interface{}{}
	argIdx := 1

	if req.LocationID != nil {
		var loc interface{} = *req.LocationID
		if strings.TrimSpace(*req.LocationID) == "" {
			loc = nil
		}
		sets = append(sets, "location_id = $"+strconv.Itoa(argIdx))
		args = append(args, loc)
		argIdx++
	}
	if req.MembershipType != nil {
		sets = append(sets, "membership_type = $"+strconv.Itoa(argIdx))
		args = append(args, *req.MembershipType)
		argIdx++
	}
	if req.MembershipStart != nil {
		var start interface{} = *req.MembershipStart
		if strings.TrimSpace(*req.MembershipStart) == "" {
			start = nil
		}
		sets = append(sets, "membership_start = $"+strconv.Itoa(argIdx)+"::date")
		args = append(args, start)
		argIdx++
	}
	if req.MembershipEnd != nil {
		var end interface{} = *req.MembershipEnd
		if strings.TrimSpace(*req.MembershipEnd) == "" {
			end = nil
		}
		sets = append(sets, "membership_end = $"+strconv.Itoa(argIdx)+"::date")
		args = append(args, end)
		argIdx++
	}
	if req.Status != nil {
		sets = append(sets, "status = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Status)
		argIdx++
	}

	if len(sets) == 2 {
		response.BadRequest(c, "VALIDATION_ERROR", "no fields to update")
		return
	}

	query := `UPDATE members SET ` + strings.Join(sets, ", ") + `
		WHERE id = $` + strconv.Itoa(argIdx) + `
		RETURNING id, user_id, location_id, membership_type, membership_start::text, membership_end::text, status, created_at`
	args = append(args, id)

	var member memberResponse
	var createdAt time.Time
	err := h.db.QueryRow(c.Request.Context(), query, args...).Scan(
		&member.ID, &member.UserID, &member.LocationID, &member.MembershipType,
		&member.MembershipStart, &member.MembershipEnd, &member.Status, &createdAt,
	)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "member")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}

	member.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	actorID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &actorID, "member.update", "member", id, nil, req, c.ClientIP())
	response.OK(c, member)
}

func (h *Handler) orders(c *gin.Context) {
	memberID := c.Param("id")

	// Member role may only view orders for their own member record
	role := c.GetString(middleware.KeyRole)
	if role == auth.RoleMember {
		userID := c.GetString(middleware.KeyUserID)
		var ownMemberID string
		err := h.db.QueryRow(c.Request.Context(), `SELECT id FROM members WHERE user_id = $1`, userID).Scan(&ownMemberID)
		if err != nil || ownMemberID != memberID {
			response.NotFound(c, "member")
			return
		}
	}

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, status, total_amount, created_at FROM orders WHERE member_id = $1 ORDER BY created_at DESC
	`, memberID)
	if err != nil { response.InternalError(c); return }
	defer rows.Close()

	type orderSummary struct {
		ID        string  `json:"id"`
		Status    string  `json:"status"`
		Total     float64 `json:"total_amount"`
		CreatedAt string  `json:"created_at"`
	}
	results := []orderSummary{}
	for rows.Next() {
		var o orderSummary
		var createdAt time.Time
		if rows.Scan(&o.ID, &o.Status, &o.Total, &createdAt) == nil {
			o.CreatedAt = createdAt.UTC().Format(time.RFC3339)
			results = append(results, o)
		}
	}
	response.OK(c, results)
}

func (h *Handler) groupBuys(c *gin.Context) {
	memberID := c.Param("id")

	// Member role may only view group-buys for their own member record
	role := c.GetString(middleware.KeyRole)
	if role == auth.RoleMember {
		userID := c.GetString(middleware.KeyUserID)
		var ownMemberID string
		err := h.db.QueryRow(c.Request.Context(), `SELECT id FROM members WHERE user_id = $1`, userID).Scan(&ownMemberID)
		if err != nil || ownMemberID != memberID {
			response.NotFound(c, "member")
			return
		}
	}

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT g.id, g.title, g.status, p.quantity, p.joined_at
		FROM group_buy_participants p
		JOIN group_buys g ON g.id = p.group_buy_id
		WHERE p.member_id = $1
		ORDER BY p.joined_at DESC
	`, memberID)
	if err != nil { response.InternalError(c); return }
	defer rows.Close()

	type gbSummary struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Status   string `json:"status"`
		Quantity int    `json:"quantity"`
		JoinedAt string `json:"joined_at"`
	}
	results := []gbSummary{}
	for rows.Next() {
		var g gbSummary
		var joinedAt time.Time
		if rows.Scan(&g.ID, &g.Title, &g.Status, &g.Quantity, &joinedAt) == nil {
			g.JoinedAt = joinedAt.UTC().Format(time.RFC3339)
			results = append(results, g)
		}
	}
	response.OK(c, results)
}
