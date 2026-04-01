package groupbuys

import (
	"context"
	"fmt"
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
	g := r.Group("/group-buys")
	g.Use(middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager, auth.RoleMember))
	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:id", h.get)
	g.PATCH("/:id", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.update)
	g.POST("/:id/publish", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.publish)
	g.POST("/:id/cancel", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.cancel)
	g.POST("/:id/join", middleware.RequireRoles(auth.RoleMember), h.join)
	g.DELETE("/:id/leave", middleware.RequireRoles(auth.RoleMember), h.leave)
	g.GET("/:id/participants", h.participants)
}

// ── Types ─────────────────────────────────────────────────────────────────────

type createRequest struct {
	ItemID       string  `json:"item_id" binding:"required"`
	LocationID   string  `json:"location_id" binding:"required"`
	Title        string  `json:"title" binding:"required"`
	Description  string  `json:"description"`
	MinQuantity  int     `json:"min_quantity" binding:"required,min=1"`
	CutoffAt     string  `json:"cutoff_at" binding:"required"`
	PricePerUnit float64 `json:"price_per_unit" binding:"required"`
	Notes        string  `json:"notes"`
}

type updateRequest struct {
	Title        *string  `json:"title"`
	Description  *string  `json:"description"`
	MinQuantity  *int     `json:"min_quantity"`
	CutoffAt     *string  `json:"cutoff_at"`
	PricePerUnit *float64 `json:"price_per_unit"`
	Notes        *string  `json:"notes"`
}

type joinRequest struct {
	Quantity int `json:"quantity"`
}

type gbResponse struct {
	ID              string  `json:"id"`
	ItemID          string  `json:"item_id"`
	LocationID      string  `json:"location_id"`
	CreatedBy       *string `json:"created_by"`
	Title           string  `json:"title"`
	Description     *string `json:"description"`
	MinQuantity     int     `json:"min_quantity"`
	CurrentQuantity int     `json:"current_quantity"`
	Status          string  `json:"status"`
	CutoffAt        string  `json:"cutoff_at"`
	PricePerUnit    float64 `json:"price_per_unit"`
	Notes           *string `json:"notes"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	Version         int     `json:"version"`
	Progress        float64 `json:"progress"`
}

type participantResponse struct {
	ID         string `json:"id"`
	GroupBuyID string `json:"group_buy_id"`
	MemberID   string `json:"member_id"`
	Quantity   int    `json:"quantity"`
	JoinedAt   string `json:"joined_at"`
	Status     string `json:"status"`
}

func toGBResponse(id, itemID, locID string, createdBy, desc, notes *string,
	title string, minQty, curQty, version int, status string,
	cutoffAt, createdAt, updatedAt time.Time, pricePerUnit float64) gbResponse {
	progress := 0.0
	if minQty > 0 {
		progress = float64(curQty) / float64(minQty) * 100
		if progress > 100 {
			progress = 100
		}
	}
	return gbResponse{
		ID: id, ItemID: itemID, LocationID: locID, CreatedBy: createdBy,
		Title: title, Description: desc, MinQuantity: minQty, CurrentQuantity: curQty,
		Status: status, CutoffAt: cutoffAt.UTC().Format(time.RFC3339),
		PricePerUnit: pricePerUnit, Notes: notes,
		CreatedAt: createdAt.UTC().Format(time.RFC3339),
		UpdatedAt: updatedAt.UTC().Format(time.RFC3339),
		Version: version, Progress: progress,
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *Handler) list(c *gin.Context) {
	role := c.GetString(middleware.KeyRole)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "25"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	baseQuery := `FROM group_buys WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	// Members only see published/active/succeeded/failed/fulfilled (not draft)
	if role == auth.RoleMember || role == auth.RoleCoach {
		baseQuery += fmt.Sprintf(" AND status IN ($%d,$%d,$%d,$%d,$%d)", argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4)
		args = append(args, "published", "active", "succeeded", "failed", "fulfilled")
		argIdx += 5
	}

	if status := c.Query("status"); status != "" {
		baseQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	if itemID := c.Query("item_id"); itemID != "" {
		baseQuery += fmt.Sprintf(" AND item_id = $%d", argIdx)
		args = append(args, itemID)
		argIdx++
	}

	var total int
	_ = h.db.QueryRow(c.Request.Context(), "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)

	query := `SELECT id, item_id, location_id, created_by, title, description, min_quantity, current_quantity,
		status, cutoff_at, price_per_unit, notes, created_at, updated_at, version ` +
		baseQuery + fmt.Sprintf(" ORDER BY cutoff_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, perPage, (page-1)*perPage)

	rows, err := h.db.Query(c.Request.Context(), query, args...)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	gbs := []gbResponse{}
	for rows.Next() {
		var id, itemID, locID, title, status string
		var createdBy, desc, notes *string
		var minQty, curQty, version int
		var cutoffAt, createdAt, updatedAt time.Time
		var pricePerUnit float64
		if err := rows.Scan(&id, &itemID, &locID, &createdBy, &title, &desc,
			&minQty, &curQty, &status, &cutoffAt, &pricePerUnit, &notes,
			&createdAt, &updatedAt, &version); err != nil {
			response.InternalError(c)
			return
		}
		gbs = append(gbs, toGBResponse(id, itemID, locID, createdBy, desc, notes,
			title, minQty, curQty, version, status, cutoffAt, createdAt, updatedAt, pricePerUnit))
	}

	response.OKPaginated(c, gbs, response.Meta{Page: page, PerPage: perPage, Total: total})
}

func (h *Handler) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	cutoff, err := time.Parse(time.RFC3339, req.CutoffAt)
	if err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", "cutoff_at must be RFC3339 format")
		return
	}
	if cutoff.Before(time.Now()) {
		response.Unprocessable(c, "validation failed", map[string]string{"cutoff_at": "must be in the future"})
		return
	}

	userID := c.GetString(middleware.KeyUserID)

	var notes *string
	if req.Notes != "" {
		notes = &req.Notes
	}
	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	// Members create published group-buys directly; staff create as draft
	role := c.GetString(middleware.KeyRole)
	initStatus := "draft"
	if role == auth.RoleMember {
		initStatus = "published"
	}

	var id string
	var createdAt, updatedAt time.Time
	err = h.db.QueryRow(c.Request.Context(), `
		INSERT INTO group_buys (item_id, location_id, created_by, title, description, min_quantity, status, cutoff_at, price_per_unit, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`, req.ItemID, req.LocationID, userID, req.Title, desc, req.MinQuantity, initStatus, cutoff, req.PricePerUnit, notes,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		response.InternalError(c)
		return
	}

	audit.Log(c.Request.Context(), h.db, &userID, "groupbuy.create", "group_buy", id, nil, req, c.ClientIP())

	response.Created(c, toGBResponse(id, req.ItemID, req.LocationID, &userID, desc, notes,
		req.Title, req.MinQuantity, 0, 1, initStatus, cutoff, createdAt, updatedAt, req.PricePerUnit))
}

func (h *Handler) get(c *gin.Context) {
	id := c.Param("id")
	gb, err := h.fetchGB(c, id)
	if err != nil {
		return // error already sent
	}
	// Non-staff cannot see draft group-buys
	role := c.GetString(middleware.KeyRole)
	if gb.Status == "draft" && role != auth.RoleAdministrator && role != auth.RoleOperationsManager {
		response.NotFound(c, "group buy")
		return
	}
	response.OK(c, gb)
}

func (h *Handler) update(c *gin.Context) {
	id := c.Param("id")
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	// Only allow update on draft
	var currentStatus string
	err := h.db.QueryRow(c.Request.Context(), `SELECT status FROM group_buys WHERE id = $1`, id).Scan(&currentStatus)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "group buy")
		return
	}
	if currentStatus != "draft" {
		response.Conflict(c, "can only edit draft group buys")
		return
	}

	// Simple: just update all provided fields
	_, err = h.db.Exec(c.Request.Context(), `
		UPDATE group_buys SET
			title = COALESCE($1, title),
			description = COALESCE($2, description),
			min_quantity = COALESCE($3, min_quantity),
			cutoff_at = COALESCE($4::timestamptz, cutoff_at),
			price_per_unit = COALESCE($5, price_per_unit),
			notes = COALESCE($6, notes),
			updated_at = NOW(), version = version + 1
		WHERE id = $7
	`, req.Title, req.Description, req.MinQuantity, req.CutoffAt, req.PricePerUnit, req.Notes, id)
	if err != nil {
		response.InternalError(c)
		return
	}

	userID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &userID, "groupbuy.update", "group_buy", id, nil, req, c.ClientIP())

	gb, _ := h.fetchGB(c, id)
	if gb != nil {
		response.OK(c, gb)
	}
}

func (h *Handler) publish(c *gin.Context) {
	id := c.Param("id")
	tag, err := h.db.Exec(c.Request.Context(), `
		UPDATE group_buys SET status = 'published', updated_at = NOW(), version = version + 1
		WHERE id = $1 AND status = 'draft'
	`, id)
	if err != nil {
		response.InternalError(c)
		return
	}
	if tag.RowsAffected() == 0 {
		h.conflictOrNotFound(c, id, "can only publish draft group buys")
		return
	}

	userID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &userID, "groupbuy.publish", "group_buy", id, nil, nil, c.ClientIP())

	gb, _ := h.fetchGB(c, id)
	if gb != nil {
		response.OK(c, gb)
	}
}

func (h *Handler) cancel(c *gin.Context) {
	id := c.Param("id")
	tag, err := h.db.Exec(c.Request.Context(), `
		UPDATE group_buys SET status = 'cancelled', updated_at = NOW(), version = version + 1
		WHERE id = $1 AND status IN ('draft', 'published', 'active')
	`, id)
	if err != nil {
		response.InternalError(c)
		return
	}
	if tag.RowsAffected() == 0 {
		h.conflictOrNotFound(c, id, "can only cancel draft, published, or active group buys")
		return
	}

	userID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &userID, "groupbuy.cancel", "group_buy", id, nil, nil, c.ClientIP())

	response.NoContent(c)
}

func (h *Handler) join(c *gin.Context) {
	gbID := c.Param("id")
	var req joinRequest
	_ = c.ShouldBindJSON(&req) // optional body
	qty := req.Quantity
	if qty < 1 {
		qty = 1
	}

	ctx := c.Request.Context()
	userID := c.GetString(middleware.KeyUserID)

	tx, err := h.db.Begin(ctx)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer tx.Rollback(ctx)

	// Lock group-buy row
	var status string
	var cutoffAt time.Time
	err = tx.QueryRow(ctx, `SELECT status, cutoff_at FROM group_buys WHERE id = $1 FOR UPDATE`, gbID).Scan(&status, &cutoffAt)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "group buy")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}

	// Must be published or active, and not past cutoff
	if status != "published" && status != "active" {
		response.Conflict(c, "group buy is not open for joining (status: "+status+")")
		return
	}
	if time.Now().After(cutoffAt) {
		response.Conflict(c, "group buy cutoff has passed")
		return
	}

	// Look up member ID for this user
	var memberID string
	err = tx.QueryRow(ctx, `SELECT id FROM members WHERE user_id = $1 AND status = 'active'`, userID).Scan(&memberID)
	if err == pgx.ErrNoRows {
		response.Forbidden(c)
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}

	// Check for duplicate (the UNIQUE constraint will also catch this, but we give a nicer error)
	var existingID string
	err = tx.QueryRow(ctx, `
		SELECT id FROM group_buy_participants
		WHERE group_buy_id = $1 AND member_id = $2 AND status = 'committed'
	`, gbID, memberID).Scan(&existingID)
	if err == nil {
		response.Conflict(c, "you have already joined this group buy")
		return
	}
	if err != pgx.ErrNoRows {
		response.InternalError(c)
		return
	}

	// Insert participant
	var partID string
	var joinedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO group_buy_participants (group_buy_id, member_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (group_buy_id, member_id) DO UPDATE SET status = 'committed', quantity = $3
		RETURNING id, joined_at
	`, gbID, memberID, qty).Scan(&partID, &joinedAt)
	if err != nil {
		response.InternalError(c)
		return
	}

	// Update current_quantity
	_, err = tx.Exec(ctx, `
		UPDATE group_buys SET
			current_quantity = (SELECT COALESCE(SUM(quantity), 0) FROM group_buy_participants WHERE group_buy_id = $1 AND status = 'committed'),
			status = CASE WHEN status = 'published' THEN 'active' ELSE status END,
			updated_at = NOW()
		WHERE id = $1
	`, gbID)
	if err != nil {
		response.InternalError(c)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c)
		return
	}

	audit.Log(ctx, h.db, &userID, "groupbuy.join", "group_buy", gbID, nil,
		map[string]interface{}{"member_id": memberID, "quantity": qty}, c.ClientIP())

	response.Created(c, participantResponse{
		ID: partID, GroupBuyID: gbID, MemberID: memberID,
		Quantity: qty, JoinedAt: joinedAt.UTC().Format(time.RFC3339), Status: "committed",
	})
}

func (h *Handler) leave(c *gin.Context) {
	gbID := c.Param("id")
	ctx := c.Request.Context()
	userID := c.GetString(middleware.KeyUserID)

	tx, err := h.db.Begin(ctx)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer tx.Rollback(ctx)

	// Look up member
	var memberID string
	err = tx.QueryRow(ctx, `SELECT id FROM members WHERE user_id = $1`, userID).Scan(&memberID)
	if err != nil {
		response.NotFound(c, "member")
		return
	}

	// Check group-buy status
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM group_buys WHERE id = $1 FOR UPDATE`, gbID).Scan(&status)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "group buy")
		return
	}
	if status != "published" && status != "active" {
		response.Conflict(c, "cannot leave a group buy in terminal state")
		return
	}

	// Cancel the participation
	tag, err := tx.Exec(ctx, `
		UPDATE group_buy_participants SET status = 'cancelled'
		WHERE group_buy_id = $1 AND member_id = $2 AND status = 'committed'
	`, gbID, memberID)
	if err != nil {
		response.InternalError(c)
		return
	}
	if tag.RowsAffected() == 0 {
		response.NotFound(c, "participation")
		return
	}

	// Update current_quantity
	_, err = tx.Exec(ctx, `
		UPDATE group_buys SET
			current_quantity = (SELECT COALESCE(SUM(quantity), 0) FROM group_buy_participants WHERE group_buy_id = $1 AND status = 'committed'),
			updated_at = NOW()
		WHERE id = $1
	`, gbID)
	if err != nil {
		response.InternalError(c)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c)
		return
	}

	audit.Log(ctx, h.db, &userID, "groupbuy.leave", "group_buy", gbID, nil, nil, c.ClientIP())
	response.NoContent(c)
}

func (h *Handler) participants(c *gin.Context) {
	gbID := c.Param("id")
	role := c.GetString(middleware.KeyRole)
	isStaff := role == auth.RoleAdministrator || role == auth.RoleOperationsManager

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, group_buy_id, member_id, quantity, joined_at, status
		FROM group_buy_participants WHERE group_buy_id = $1
		ORDER BY joined_at
	`, gbID)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	parts := []participantResponse{}
	for rows.Next() {
		var p participantResponse
		var joinedAt time.Time
		if err := rows.Scan(&p.ID, &p.GroupBuyID, &p.MemberID, &p.Quantity, &joinedAt, &p.Status); err != nil {
			response.InternalError(c)
			return
		}
		p.JoinedAt = joinedAt.UTC().Format(time.RFC3339)
		// Hide member identifiers from non-staff views
		if !isStaff {
			p.MemberID = ""
		}
		parts = append(parts, p)
	}

	response.OK(c, parts)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (h *Handler) fetchGB(c *gin.Context, id string) (*gbResponse, error) {
	var gbID, itemID, locID, title, status string
	var createdBy, desc, notes *string
	var minQty, curQty, version int
	var cutoffAt, createdAt, updatedAt time.Time
	var pricePerUnit float64

	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, item_id, location_id, created_by, title, description, min_quantity, current_quantity,
			status, cutoff_at, price_per_unit, notes, created_at, updated_at, version
		FROM group_buys WHERE id = $1
	`, id).Scan(&gbID, &itemID, &locID, &createdBy, &title, &desc, &minQty, &curQty,
		&status, &cutoffAt, &pricePerUnit, &notes, &createdAt, &updatedAt, &version)

	if err == pgx.ErrNoRows {
		response.NotFound(c, "group buy")
		return nil, err
	}
	if err != nil {
		response.InternalError(c)
		return nil, err
	}

	gb := toGBResponse(gbID, itemID, locID, createdBy, desc, notes,
		title, minQty, curQty, version, status, cutoffAt, createdAt, updatedAt, pricePerUnit)
	return &gb, nil
}

func (h *Handler) conflictOrNotFound(c *gin.Context, id, msg string) {
	var exists bool
	_ = h.db.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM group_buys WHERE id = $1)`, id).Scan(&exists)
	if !exists {
		response.NotFound(c, "group buy")
	} else {
		response.Conflict(c, msg)
	}
}

// EvaluateCutoffs is called by the worker to process group-buy cutoffs.
// It is exported so cmd/worker can call it.
func EvaluateCutoffs(ctx context.Context, db *pgxpool.Pool) (int, error) {
	rows, err := db.Query(ctx, `
		SELECT id, min_quantity, current_quantity
		FROM group_buys
		WHERE status IN ('published', 'active') AND cutoff_at <= NOW()
		FOR UPDATE SKIP LOCKED
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	processed := 0
	for rows.Next() {
		var id string
		var minQty, curQty int
		if err := rows.Scan(&id, &minQty, &curQty); err != nil {
			continue
		}

		newStatus := "failed"
		if curQty >= minQty {
			newStatus = "succeeded"
		}

		_, err := db.Exec(ctx, `
			UPDATE group_buys SET status = $1, updated_at = NOW(), version = version + 1
			WHERE id = $2
		`, newStatus, id)
		if err != nil {
			continue
		}
		processed++
	}
	return processed, nil
}
