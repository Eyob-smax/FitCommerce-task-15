package classes

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"fitcommerce/backend/internal/auth"
	"fitcommerce/backend/internal/http/response"
	"fitcommerce/backend/internal/middleware"
)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/classes")
	g.GET("", h.list)
	g.POST("", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager, auth.RoleCoach), h.create)
	g.GET("/:id", h.get)
	g.PATCH("/:id", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager, auth.RoleCoach), h.update)
	g.POST("/:id/cancel", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.cancel)
	g.POST("/:id/book", h.book)
	g.DELETE("/:id/book", h.cancelBooking)
}

type classResponse struct {
	ID              string  `json:"id"`
	CoachID         string  `json:"coach_id"`
	LocationID      string  `json:"location_id"`
	Name            string  `json:"name"`
	Description     *string `json:"description"`
	ScheduledAt     string  `json:"scheduled_at"`
	DurationMinutes int     `json:"duration_minutes"`
	Capacity        int     `json:"capacity"`
	BookedSeats     int     `json:"booked_seats"`
	Status          string  `json:"status"`
	CreatedAt       string  `json:"created_at"`
}

func (h *Handler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "25"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	var total int
	_ = h.db.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM classes`).Scan(&total)

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, coach_id, location_id, name, description, scheduled_at, duration_minutes, capacity, booked_seats, status, created_at
		FROM classes ORDER BY scheduled_at DESC LIMIT $1 OFFSET $2
	`, perPage, (page-1)*perPage)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	classes := []classResponse{}
	for rows.Next() {
		var cl classResponse
		var scheduledAt, createdAt time.Time
		if rows.Scan(&cl.ID, &cl.CoachID, &cl.LocationID, &cl.Name, &cl.Description, &scheduledAt,
			&cl.DurationMinutes, &cl.Capacity, &cl.BookedSeats, &cl.Status, &createdAt) == nil {
			cl.ScheduledAt = scheduledAt.UTC().Format(time.RFC3339)
			cl.CreatedAt = createdAt.UTC().Format(time.RFC3339)
			classes = append(classes, cl)
		}
	}
	response.OKPaginated(c, classes, response.Meta{Page: page, PerPage: perPage, Total: total})
}

func (h *Handler) create(c *gin.Context) {
	var req struct {
		CoachID         string  `json:"coach_id" binding:"required"`
		LocationID      string  `json:"location_id" binding:"required"`
		Name            string  `json:"name" binding:"required"`
		Description     *string `json:"description"`
		ScheduledAt     string  `json:"scheduled_at" binding:"required"`
		DurationMinutes int     `json:"duration_minutes" binding:"required,min=1"`
		Capacity        int     `json:"capacity" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", "scheduled_at must be RFC3339")
		return
	}

	if c.GetString(middleware.KeyRole) == auth.RoleCoach {
		ownCoachID, err := h.getCoachIDForUser(c.Request.Context(), c.GetString(middleware.KeyUserID))
		if err == pgx.ErrNoRows {
			response.Forbidden(c)
			return
		}
		if err != nil {
			response.InternalError(c)
			return
		}
		if req.CoachID != ownCoachID {
			response.Forbidden(c)
			return
		}
	}

	var cl classResponse
	var createdAt time.Time
	err = h.db.QueryRow(c.Request.Context(), `
		INSERT INTO classes (coach_id, location_id, name, description, scheduled_at, duration_minutes, capacity)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, coach_id, location_id, name, description, scheduled_at, duration_minutes, capacity, booked_seats, status, created_at
	`, req.CoachID, req.LocationID, req.Name, req.Description, scheduledAt, req.DurationMinutes, req.Capacity,
	).Scan(&cl.ID, &cl.CoachID, &cl.LocationID, &cl.Name, &cl.Description,
		&scheduledAt, &cl.DurationMinutes, &cl.Capacity, &cl.BookedSeats, &cl.Status, &createdAt)
	if err != nil {
		response.InternalError(c)
		return
	}
	cl.ScheduledAt = scheduledAt.UTC().Format(time.RFC3339)
	cl.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	response.Created(c, cl)
}

func (h *Handler) get(c *gin.Context) {
	id := c.Param("id")
	var cl classResponse
	var scheduledAt, createdAt time.Time
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, coach_id, location_id, name, description, scheduled_at, duration_minutes, capacity, booked_seats, status, created_at
		FROM classes WHERE id = $1
	`, id).Scan(&cl.ID, &cl.CoachID, &cl.LocationID, &cl.Name, &cl.Description, &scheduledAt,
		&cl.DurationMinutes, &cl.Capacity, &cl.BookedSeats, &cl.Status, &createdAt)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "class")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}
	cl.ScheduledAt = scheduledAt.UTC().Format(time.RFC3339)
	cl.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	response.OK(c, cl)
}

func (h *Handler) update(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Name            *string `json:"name"`
		Description     *string `json:"description"`
		ScheduledAt     *string `json:"scheduled_at"`
		DurationMinutes *int    `json:"duration_minutes"`
		Capacity        *int    `json:"capacity"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	var status, classCoachID string
	err := h.db.QueryRow(c.Request.Context(), `SELECT status, coach_id FROM classes WHERE id = $1`, id).Scan(&status, &classCoachID)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "class")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}

	if c.GetString(middleware.KeyRole) == auth.RoleCoach {
		ownCoachID, err := h.getCoachIDForUser(c.Request.Context(), c.GetString(middleware.KeyUserID))
		if err == pgx.ErrNoRows {
			response.Forbidden(c)
			return
		}
		if err != nil {
			response.InternalError(c)
			return
		}
		if classCoachID != ownCoachID {
			response.Forbidden(c)
			return
		}
	}

	if status == "cancelled" || status == "completed" {
		response.Conflict(c, "cannot update a "+status+" class")
		return
	}

	_, err = h.db.Exec(c.Request.Context(), `
		UPDATE classes SET
			name = COALESCE($1, name),
			description = COALESCE($2, description),
			scheduled_at = COALESCE($3::timestamptz, scheduled_at),
			duration_minutes = COALESCE($4, duration_minutes),
			capacity = COALESCE($5, capacity),
			updated_at = NOW()
		WHERE id = $6
	`, req.Name, req.Description, req.ScheduledAt, req.DurationMinutes, req.Capacity, id)
	if err != nil {
		response.InternalError(c)
		return
	}

	var cl classResponse
	var scheduledAt, createdAt time.Time
	err = h.db.QueryRow(c.Request.Context(), `
		SELECT id, coach_id, location_id, name, description, scheduled_at, duration_minutes, capacity, booked_seats, status, created_at
		FROM classes WHERE id = $1
	`, id).Scan(&cl.ID, &cl.CoachID, &cl.LocationID, &cl.Name, &cl.Description, &scheduledAt,
		&cl.DurationMinutes, &cl.Capacity, &cl.BookedSeats, &cl.Status, &createdAt)
	if err != nil {
		response.InternalError(c)
		return
	}
	cl.ScheduledAt = scheduledAt.UTC().Format(time.RFC3339)
	cl.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	response.OK(c, cl)
}

func (h *Handler) cancel(c *gin.Context) {
	id := c.Param("id")
	tag, err := h.db.Exec(c.Request.Context(), `
		UPDATE classes SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1 AND status NOT IN ('cancelled', 'completed')
	`, id)
	if err != nil {
		response.InternalError(c)
		return
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		_ = h.db.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM classes WHERE id = $1)`, id).Scan(&exists)
		if !exists {
			response.NotFound(c, "class")
		} else {
			response.Conflict(c, "class is already in a terminal state")
		}
		return
	}
	response.NoContent(c)
}

func (h *Handler) book(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()
	userID := c.GetString(middleware.KeyUserID)

	// Only active members may book
	var memberID string
	err := h.db.QueryRow(ctx, `SELECT id FROM members WHERE user_id = $1 AND status = 'active'`, userID).Scan(&memberID)
	if err == pgx.ErrNoRows {
		response.Forbidden(c)
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer tx.Rollback(ctx)

	var capacity, bookedSeats int
	var classStatus string
	err = tx.QueryRow(ctx, `
		SELECT capacity, booked_seats, status FROM classes WHERE id = $1 FOR UPDATE
	`, id).Scan(&capacity, &bookedSeats, &classStatus)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "class")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}
	if classStatus != "scheduled" {
		response.Conflict(c, "class is not open for booking (status: "+classStatus+")")
		return
	}
	if bookedSeats >= capacity {
		response.Conflict(c, "class is fully booked")
		return
	}

	// Duplicate booking check
	var existingID string
	err = tx.QueryRow(ctx, `
		SELECT id FROM class_bookings WHERE class_id = $1 AND member_id = $2 AND status = 'confirmed'
	`, id, memberID).Scan(&existingID)
	if err == nil {
		response.Conflict(c, "you have already booked this class")
		return
	}
	if err != pgx.ErrNoRows {
		response.InternalError(c)
		return
	}

	var bookingID string
	var bookedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO class_bookings (class_id, member_id) VALUES ($1, $2)
		ON CONFLICT (class_id, member_id) DO UPDATE SET status = 'confirmed'
		RETURNING id, booked_at
	`, id, memberID).Scan(&bookingID, &bookedAt)
	if err != nil {
		response.InternalError(c)
		return
	}

	_, err = tx.Exec(ctx, `
		UPDATE classes SET booked_seats = booked_seats + 1, updated_at = NOW() WHERE id = $1
	`, id)
	if err != nil {
		response.InternalError(c)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c)
		return
	}

	response.Created(c, gin.H{
		"booking_id": bookingID,
		"class_id":   id,
		"member_id":  memberID,
		"status":     "confirmed",
		"booked_at":  bookedAt.UTC().Format(time.RFC3339),
	})
}

func (h *Handler) getCoachIDForUser(ctx context.Context, userID string) (string, error) {
	var coachID string
	err := h.db.QueryRow(ctx, `SELECT id FROM coaches WHERE user_id = $1`, userID).Scan(&coachID)
	if err != nil {
		return "", err
	}
	return coachID, nil
}

func (h *Handler) cancelBooking(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()
	userID := c.GetString(middleware.KeyUserID)

	var memberID string
	err := h.db.QueryRow(ctx, `SELECT id FROM members WHERE user_id = $1`, userID).Scan(&memberID)
	if err != nil {
		response.NotFound(c, "member")
		return
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer tx.Rollback(ctx)

	var classStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM classes WHERE id = $1 FOR UPDATE`, id).Scan(&classStatus)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "class")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}
	if classStatus == "cancelled" || classStatus == "completed" {
		response.Conflict(c, "cannot cancel booking for a "+classStatus+" class")
		return
	}

	tag, err := tx.Exec(ctx, `
		UPDATE class_bookings SET status = 'cancelled'
		WHERE class_id = $1 AND member_id = $2 AND status = 'confirmed'
	`, id, memberID)
	if err != nil {
		response.InternalError(c)
		return
	}
	if tag.RowsAffected() == 0 {
		response.NotFound(c, "booking")
		return
	}

	_, _ = tx.Exec(ctx, `
		UPDATE classes SET booked_seats = GREATEST(0, booked_seats - 1), updated_at = NOW() WHERE id = $1
	`, id)

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c)
		return
	}
	response.NoContent(c)
}
