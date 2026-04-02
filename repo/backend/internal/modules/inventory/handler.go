package inventory

import (
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
	g := r.Group("/inventory")
	g.Use(middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager, auth.RoleProcurementSpecialist))
	g.GET("", h.list)
	g.GET("/:id", h.get)
	g.POST("/:id/adjust", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager, auth.RoleProcurementSpecialist), h.adjust)
	g.GET("/:id/adjustments", h.listAdjustments)
}

// ── Types ─────────────────────────────────────────────────────────────────────

type adjustRequest struct {
	QuantityChange int    `json:"quantity_change" binding:"required"`
	ReasonCode     string `json:"reason_code" binding:"required"`
	Notes          string `json:"notes"`
}

type stockResponse struct {
	ID         string `json:"id"`
	ItemID     string `json:"item_id"`
	ItemName   string `json:"item_name"`
	LocationID string `json:"location_id"`
	OnHand     int    `json:"on_hand"`
	Reserved   int    `json:"reserved"`
	Allocated  int    `json:"allocated"`
	InRental   int    `json:"in_rental"`
	Returned   int    `json:"returned"`
	Damaged    int    `json:"damaged"`
	Available  int    `json:"available"`
	UpdatedAt  string `json:"updated_at"`
}

type adjustmentResponse struct {
	ID             string  `json:"id"`
	ItemID         string  `json:"item_id"`
	LocationID     string  `json:"location_id"`
	QuantityChange int     `json:"quantity_change"`
	PreviousOnHand int     `json:"previous_on_hand"`
	NewOnHand      int     `json:"new_on_hand"`
	ReasonCode     string  `json:"reason_code"`
	Notes          *string `json:"notes"`
	AdjustedBy     *string `json:"adjusted_by"`
	CreatedAt      string  `json:"created_at"`
}

var validReasonCodes = map[string]bool{
	"damaged": true, "found": true, "correction": true, "return": true,
	"theft": true, "audit": true, "expired": true, "other": true,
}

// ── Handlers ──────────────────────────────────────────────────────────────────

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
	_ = h.db.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM inventory_stock`).Scan(&total)

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT s.id, s.item_id, i.name, s.location_id, s.on_hand, s.reserved, s.allocated,
			s.in_rental, s.returned, s.damaged, s.updated_at
		FROM inventory_stock s
		JOIN items i ON i.id = s.item_id
		ORDER BY i.name
		LIMIT $1 OFFSET $2
	`, perPage, (page-1)*perPage)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	items := []stockResponse{}
	for rows.Next() {
		var s stockResponse
		var updatedAt time.Time
		if err := rows.Scan(&s.ID, &s.ItemID, &s.ItemName, &s.LocationID, &s.OnHand,
			&s.Reserved, &s.Allocated, &s.InRental, &s.Returned, &s.Damaged, &updatedAt); err != nil {
			response.InternalError(c)
			return
		}
		s.Available = s.OnHand - s.Reserved - s.Allocated
		s.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		items = append(items, s)
	}

	response.OKPaginated(c, items, response.Meta{Page: page, PerPage: perPage, Total: total})
}

func (h *Handler) get(c *gin.Context) {
	itemID := c.Param("id")

	var s stockResponse
	var updatedAt time.Time
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT s.id, s.item_id, i.name, s.location_id, s.on_hand, s.reserved, s.allocated,
			s.in_rental, s.returned, s.damaged, s.updated_at
		FROM inventory_stock s
		JOIN items i ON i.id = s.item_id
		WHERE s.item_id = $1
	`, itemID).Scan(&s.ID, &s.ItemID, &s.ItemName, &s.LocationID, &s.OnHand,
		&s.Reserved, &s.Allocated, &s.InRental, &s.Returned, &s.Damaged, &updatedAt)

	if err == pgx.ErrNoRows {
		response.NotFound(c, "inventory stock")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}
	s.Available = s.OnHand - s.Reserved - s.Allocated
	s.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)

	response.OK(c, s)
}

func (h *Handler) adjust(c *gin.Context) {
	itemID := c.Param("id")
	var req adjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	if !validReasonCodes[req.ReasonCode] {
		response.Unprocessable(c, "validation failed", map[string]string{
			"reason_code": "must be one of: damaged, found, correction, return, theft, audit, expired, other",
		})
		return
	}
	if req.QuantityChange == 0 {
		response.BadRequest(c, "VALIDATION_ERROR", "quantity_change cannot be zero")
		return
	}

	ctx := c.Request.Context()
	userID := c.GetString(middleware.KeyUserID)

	tx, err := h.db.Begin(ctx)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer tx.Rollback(ctx)

	// Lock the stock row
	var stockID, locationID string
	var currentOnHand int
	err = tx.QueryRow(ctx, `
		SELECT id, location_id, on_hand FROM inventory_stock
		WHERE item_id = $1 FOR UPDATE
	`, itemID).Scan(&stockID, &locationID, &currentOnHand)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "inventory stock")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}

	newOnHand := currentOnHand + req.QuantityChange
	if newOnHand < 0 {
		response.Unprocessable(c, "validation failed", map[string]string{
			"quantity_change": fmt.Sprintf("would result in negative on_hand (%d)", newOnHand),
		})
		return
	}

	// Update stock
	_, err = tx.Exec(ctx, `UPDATE inventory_stock SET on_hand = $1, updated_at = NOW() WHERE id = $2`, newOnHand, stockID)
	if err != nil {
		response.InternalError(c)
		return
	}

	// Record adjustment
	var notes *string
	if req.Notes != "" {
		notes = &req.Notes
	}
	var adjID string
	var createdAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO stock_adjustments (item_id, location_id, quantity_change, previous_on_hand, new_on_hand, reason_code, notes, adjusted_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`, itemID, locationID, req.QuantityChange, currentOnHand, newOnHand, req.ReasonCode, notes, userID,
	).Scan(&adjID, &createdAt)
	if err != nil {
		response.InternalError(c)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c)
		return
	}

	audit.Log(ctx, h.db, &userID, "inventory.adjust", "inventory_stock", stockID,
		map[string]int{"on_hand": currentOnHand},
		map[string]interface{}{"on_hand": newOnHand, "reason_code": req.ReasonCode, "change": req.QuantityChange},
		c.ClientIP())

	response.OK(c, adjustmentResponse{
		ID: adjID, ItemID: itemID, LocationID: locationID,
		QuantityChange: req.QuantityChange, PreviousOnHand: currentOnHand, NewOnHand: newOnHand,
		ReasonCode: req.ReasonCode, Notes: notes, AdjustedBy: &userID,
		CreatedAt: createdAt.UTC().Format(time.RFC3339),
	})
}

func (h *Handler) listAdjustments(c *gin.Context) {
	itemID := c.Param("id")

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, item_id, location_id, quantity_change, previous_on_hand, new_on_hand,
			reason_code, notes, adjusted_by, created_at
		FROM stock_adjustments WHERE item_id = $1
		ORDER BY created_at DESC
	`, itemID)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	adjs := []adjustmentResponse{}
	for rows.Next() {
		var a adjustmentResponse
		var createdAt time.Time
		if err := rows.Scan(&a.ID, &a.ItemID, &a.LocationID, &a.QuantityChange,
			&a.PreviousOnHand, &a.NewOnHand, &a.ReasonCode, &a.Notes,
			&a.AdjustedBy, &createdAt); err != nil {
			response.InternalError(c)
			return
		}
		a.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		adjs = append(adjs, a)
	}

	response.OK(c, adjs)
}
