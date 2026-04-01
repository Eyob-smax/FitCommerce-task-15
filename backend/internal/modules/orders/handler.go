package orders

import (
	"context"
	"encoding/json"
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
	g := r.Group("/orders")
	g.Use(middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager, auth.RoleMember))
	g.GET("", h.list)
	g.POST("", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.create)
	g.GET("/:id", h.get)
	g.POST("/:id/adjust", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.adjust)
	g.POST("/:id/cancel", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.cancel)
	g.POST("/:id/split", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.split)
	g.POST("/:id/status", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.changeStatus)
	g.GET("/:id/timeline", h.timeline)
	g.POST("/:id/notes", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.addNote)
	g.GET("/:id/notes", h.listNotes)
}

// ── Types ─────────────────────────────────────────────────────────────────────

type createOrderRequest struct {
	MemberID     string      `json:"member_id" binding:"required"`
	LocationID   string      `json:"location_id" binding:"required"`
	GroupBuyID   string      `json:"group_buy_id"`
	TotalAmount  float64     `json:"total_amount" binding:"required"`
	Deposit      float64     `json:"deposit_amount"`
	Notes        string      `json:"notes"`
	Lines        []lineInput `json:"lines" binding:"required,min=1"`
}

type lineInput struct {
	ItemID         string  `json:"item_id" binding:"required"`
	Quantity       int     `json:"quantity" binding:"required,min=1"`
	UnitPrice      float64 `json:"unit_price" binding:"required"`
	DepositPerUnit float64 `json:"deposit_per_unit"`
}

type adjustRequest struct {
	LineID      string  `json:"line_id" binding:"required"`
	NewQuantity int     `json:"new_quantity" binding:"required,min=1"`
	Reason      string  `json:"reason" binding:"required"`
}

type splitRequest struct {
	Lines  []splitLine `json:"lines" binding:"required,min=1"`
	Reason string      `json:"reason" binding:"required"`
}

type splitLine struct {
	LineID   string `json:"line_id" binding:"required"`
	Quantity int    `json:"quantity" binding:"required,min=1"`
}

type statusChangeRequest struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason" binding:"required"`
}

type noteRequest struct {
	Content string `json:"content" binding:"required"`
}

type orderResponse struct {
	ID           string         `json:"id"`
	GroupBuyID   *string        `json:"group_buy_id"`
	MemberID     string         `json:"member_id"`
	LocationID   string         `json:"location_id"`
	Status       string         `json:"status"`
	TotalAmount  float64        `json:"total_amount"`
	Deposit      float64        `json:"deposit_amount"`
	Notes        *string        `json:"notes"`
	CreatedBy    *string        `json:"created_by"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at"`
	Version      int            `json:"version"`
	Lines        []lineResponse `json:"lines,omitempty"`
}

type lineResponse struct {
	ID             string  `json:"id"`
	OrderID        string  `json:"order_id"`
	ItemID         string  `json:"item_id"`
	Quantity       int     `json:"quantity"`
	UnitPrice      float64 `json:"unit_price"`
	DepositPerUnit float64 `json:"deposit_per_unit"`
}

type noteResponse struct {
	ID        string `json:"id"`
	OrderID   string `json:"order_id"`
	AuthorID  string `json:"author_id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type timelineEvent struct {
	ID          string          `json:"id"`
	OrderID     string          `json:"order_id"`
	ActorID     *string         `json:"actor_id"`
	EventType   string          `json:"event_type"`
	Description string          `json:"description"`
	Before      json.RawMessage `json:"before_snapshot"`
	After       json.RawMessage `json:"after_snapshot"`
	OccurredAt  string          `json:"occurred_at"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────


// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *Handler) list(c *gin.Context) {
	role := c.GetString(middleware.KeyRole)
	userID := c.GetString(middleware.KeyUserID)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "25"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	baseQuery := `FROM orders WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	// Members see only their own orders
	if role == auth.RoleMember {
		var memberID string
		err := h.db.QueryRow(c.Request.Context(), `SELECT id FROM members WHERE user_id = $1`, userID).Scan(&memberID)
		if err != nil {
			response.OK(c, []interface{}{})
			return
		}
		baseQuery += fmt.Sprintf(" AND member_id = $%d", argIdx)
		args = append(args, memberID)
		argIdx++
	}

	if status := c.Query("status"); status != "" {
		baseQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int
	_ = h.db.QueryRow(c.Request.Context(), "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)

	query := `SELECT id, group_buy_id, member_id, location_id, status, total_amount, deposit_amount, notes, created_by, created_at, updated_at, version ` +
		baseQuery + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, perPage, (page-1)*perPage)

	rows, err := h.db.Query(c.Request.Context(), query, args...)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	orders := []orderResponse{}
	for rows.Next() {
		o := h.scanOrder(rows)
		if o == nil {
			response.InternalError(c)
			return
		}
		orders = append(orders, *o)
	}

	response.OKPaginated(c, orders, response.Meta{Page: page, PerPage: perPage, Total: total})
}

func (h *Handler) create(c *gin.Context) {
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
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

	var gbID *string
	if req.GroupBuyID != "" {
		gbID = &req.GroupBuyID
	}
	var notes *string
	if req.Notes != "" {
		notes = &req.Notes
	}

	var orderID string
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (group_buy_id, member_id, location_id, total_amount, deposit_amount, notes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`, gbID, req.MemberID, req.LocationID, req.TotalAmount, req.Deposit, notes, userID,
	).Scan(&orderID, &createdAt, &updatedAt)
	if err != nil {
		response.InternalError(c)
		return
	}

	lines := []lineResponse{}
	for _, l := range req.Lines {
		var lineID string
		err = tx.QueryRow(ctx, `
			INSERT INTO order_line_items (order_id, item_id, quantity, unit_price, deposit_per_unit)
			VALUES ($1, $2, $3, $4, $5) RETURNING id
		`, orderID, l.ItemID, l.Quantity, l.UnitPrice, l.DepositPerUnit).Scan(&lineID)
		if err != nil {
			response.InternalError(c)
			return
		}
		lines = append(lines, lineResponse{
			ID: lineID, OrderID: orderID, ItemID: l.ItemID,
			Quantity: l.Quantity, UnitPrice: l.UnitPrice, DepositPerUnit: l.DepositPerUnit,
		})
	}

	// Timeline: creation
	_, _ = tx.Exec(ctx, `
		INSERT INTO order_timeline_events (order_id, actor_id, event_type, description)
		VALUES ($1, $2, 'creation', $3)
	`, orderID, userID, fmt.Sprintf("Order created with %d line items, total $%.2f", len(lines), req.TotalAmount))

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c)
		return
	}

	audit.Log(ctx, h.db, &userID, "order.create", "order", orderID, nil, req, c.ClientIP())

	response.Created(c, orderResponse{
		ID: orderID, GroupBuyID: gbID, MemberID: req.MemberID, LocationID: req.LocationID,
		Status: "pending", TotalAmount: req.TotalAmount, Deposit: req.Deposit, Notes: notes,
		CreatedBy: &userID, CreatedAt: createdAt.UTC().Format(time.RFC3339),
		UpdatedAt: updatedAt.UTC().Format(time.RFC3339), Version: 1, Lines: lines,
	})
}

func (h *Handler) get(c *gin.Context) {
	id := c.Param("id")
	role := c.GetString(middleware.KeyRole)
	userID := c.GetString(middleware.KeyUserID)

	o := h.fetchOrder(c, id)
	if o == nil {
		return
	}

	// Member can only see own order
	if role == auth.RoleMember {
		var memberID string
		_ = h.db.QueryRow(c.Request.Context(), `SELECT id FROM members WHERE user_id = $1`, userID).Scan(&memberID)
		if o.MemberID != memberID {
			response.NotFound(c, "order")
			return
		}
	}

	response.OK(c, o)
}

func (h *Handler) adjust(c *gin.Context) {
	orderID := c.Param("id")
	var req adjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	ctx := c.Request.Context()
	userID := c.GetString(middleware.KeyUserID)

	// Verify order is adjustable
	var status string
	err := h.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&status)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "order")
		return
	}
	if status == "cancelled" || status == "refunded" {
		response.Conflict(c, "cannot adjust a "+status+" order")
		return
	}

	// Get current line quantity
	var currentQty int
	var unitPrice float64
	err = h.db.QueryRow(ctx, `SELECT quantity, unit_price FROM order_line_items WHERE id = $1 AND order_id = $2`,
		req.LineID, orderID).Scan(&currentQty, &unitPrice)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "order line")
		return
	}

	// Update
	_, err = h.db.Exec(ctx, `UPDATE order_line_items SET quantity = $1 WHERE id = $2`, req.NewQuantity, req.LineID)
	if err != nil {
		response.InternalError(c)
		return
	}

	// Recalculate total
	_, err = h.db.Exec(ctx, `
		UPDATE orders SET
			total_amount = (SELECT COALESCE(SUM(quantity * unit_price), 0) FROM order_line_items WHERE order_id = $1),
			updated_at = NOW(), version = version + 1
		WHERE id = $1
	`, orderID)
	if err != nil {
		response.InternalError(c)
		return
	}

	// Timeline
	_, _ = h.db.Exec(ctx, `
		INSERT INTO order_timeline_events (order_id, actor_id, event_type, description, before_snapshot, after_snapshot)
		VALUES ($1, $2, 'adjustment', $3, $4, $5)
	`, orderID, userID, req.Reason,
		mustJSON(map[string]interface{}{"line_id": req.LineID, "quantity": currentQty}),
		mustJSON(map[string]interface{}{"line_id": req.LineID, "quantity": req.NewQuantity}))

	audit.Log(ctx, h.db, &userID, "order.adjust", "order", orderID, nil, req, c.ClientIP())

	o := h.fetchOrder(c, orderID)
	if o != nil {
		response.OK(c, o)
	}
}

func (h *Handler) cancel(c *gin.Context) {
	orderID := c.Param("id")
	ctx := c.Request.Context()
	userID := c.GetString(middleware.KeyUserID)

	var reason struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&reason)
	if reason.Reason == "" {
		reason.Reason = "Cancelled by staff"
	}

	var currentStatus string
	err := h.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&currentStatus)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "order")
		return
	}
	if currentStatus == "cancelled" || currentStatus == "refunded" || currentStatus == "fulfilled" {
		response.Conflict(c, "cannot cancel a "+currentStatus+" order")
		return
	}

	_, err = h.db.Exec(ctx, `
		UPDATE orders SET status = 'cancelled', updated_at = NOW(), version = version + 1 WHERE id = $1
	`, orderID)
	if err != nil {
		response.InternalError(c)
		return
	}

	_, _ = h.db.Exec(ctx, `
		INSERT INTO order_timeline_events (order_id, actor_id, event_type, description, before_snapshot, after_snapshot)
		VALUES ($1, $2, 'cancellation', $3, $4, $5)
	`, orderID, userID, reason.Reason,
		mustJSON(map[string]string{"status": currentStatus}),
		mustJSON(map[string]string{"status": "cancelled"}))

	audit.Log(ctx, h.db, &userID, "order.cancel", "order", orderID, nil, nil, c.ClientIP())
	response.NoContent(c)
}

func (h *Handler) split(c *gin.Context) {
	orderID := c.Param("id")
	var req splitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
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

	// Verify original order
	var memberID, locationID, status string
	var gbID *string
	err = tx.QueryRow(ctx, `SELECT member_id, location_id, group_buy_id, status FROM orders WHERE id = $1 FOR UPDATE`, orderID).
		Scan(&memberID, &locationID, &gbID, &status)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "order")
		return
	}
	if status == "cancelled" || status == "refunded" {
		response.Conflict(c, "cannot split a "+status+" order")
		return
	}

	// Create new split order
	var newOrderID string
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (group_buy_id, member_id, location_id, total_amount, deposit_amount, notes, created_by, status)
		VALUES ($1, $2, $3, 0, 0, $4, $5, $6) RETURNING id, created_at, updated_at
	`, gbID, memberID, locationID, "Split from order "+orderID[:8], userID, status,
	).Scan(&newOrderID, &createdAt, &updatedAt)
	if err != nil {
		response.InternalError(c)
		return
	}

	// Move specified quantities to new order
	newTotal := 0.0
	for _, sl := range req.Lines {
		var currentQty int
		var itemID string
		var unitPrice, depositPerUnit float64
		err = tx.QueryRow(ctx, `SELECT item_id, quantity, unit_price, deposit_per_unit FROM order_line_items WHERE id = $1 AND order_id = $2`,
			sl.LineID, orderID).Scan(&itemID, &currentQty, &unitPrice, &depositPerUnit)
		if err == pgx.ErrNoRows {
			response.BadRequest(c, "INVALID_LINE", fmt.Sprintf("line %s not found in order", sl.LineID))
			return
		}
		if sl.Quantity > currentQty {
			response.Unprocessable(c, "split quantity exceeds available", map[string]string{
				sl.LineID: fmt.Sprintf("requested %d but only %d available", sl.Quantity, currentQty),
			})
			return
		}

		// Reduce original line
		remaining := currentQty - sl.Quantity
		if remaining == 0 {
			_, _ = tx.Exec(ctx, `DELETE FROM order_line_items WHERE id = $1`, sl.LineID)
		} else {
			_, _ = tx.Exec(ctx, `UPDATE order_line_items SET quantity = $1 WHERE id = $2`, remaining, sl.LineID)
		}

		// Add to new order
		_, _ = tx.Exec(ctx, `
			INSERT INTO order_line_items (order_id, item_id, quantity, unit_price, deposit_per_unit)
			VALUES ($1, $2, $3, $4, $5)
		`, newOrderID, itemID, sl.Quantity, unitPrice, depositPerUnit)
		newTotal += float64(sl.Quantity) * unitPrice
	}

	// Recalculate both orders' totals
	_, _ = tx.Exec(ctx, `
		UPDATE orders SET total_amount = (SELECT COALESCE(SUM(quantity * unit_price), 0) FROM order_line_items WHERE order_id = $1),
			updated_at = NOW(), version = version + 1
		WHERE id = $1
	`, orderID)
	_, _ = tx.Exec(ctx, `UPDATE orders SET total_amount = $1, updated_at = NOW() WHERE id = $2`, newTotal, newOrderID)

	// Timeline on original
	_, _ = tx.Exec(ctx, `
		INSERT INTO order_timeline_events (order_id, actor_id, event_type, description, after_snapshot)
		VALUES ($1, $2, 'split', $3, $4)
	`, orderID, userID, req.Reason,
		mustJSON(map[string]string{"new_order_id": newOrderID}))

	// Timeline on new order
	_, _ = tx.Exec(ctx, `
		INSERT INTO order_timeline_events (order_id, actor_id, event_type, description, before_snapshot)
		VALUES ($1, $2, 'creation', $3, $4)
	`, newOrderID, userID, "Split from order "+orderID[:8],
		mustJSON(map[string]string{"original_order_id": orderID}))

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c)
		return
	}

	audit.Log(ctx, h.db, &userID, "order.split", "order", orderID, nil,
		map[string]string{"new_order_id": newOrderID, "reason": req.Reason}, c.ClientIP())

	response.Created(c, gin.H{
		"original_order_id": orderID,
		"new_order_id":      newOrderID,
		"split_total":       newTotal,
	})
}

func (h *Handler) changeStatus(c *gin.Context) {
	orderID := c.Param("id")
	var req statusChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	validStatuses := map[string]bool{
		"pending": true, "confirmed": true, "processing": true,
		"fulfilled": true, "cancelled": true, "refunded": true,
	}
	if !validStatuses[req.Status] {
		response.BadRequest(c, "VALIDATION_ERROR", "invalid status")
		return
	}

	ctx := c.Request.Context()
	userID := c.GetString(middleware.KeyUserID)

	var currentStatus string
	err := h.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&currentStatus)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "order")
		return
	}

	_, err = h.db.Exec(ctx, `
		UPDATE orders SET status = $1, updated_at = NOW(), version = version + 1 WHERE id = $2
	`, req.Status, orderID)
	if err != nil {
		response.InternalError(c)
		return
	}

	_, _ = h.db.Exec(ctx, `
		INSERT INTO order_timeline_events (order_id, actor_id, event_type, description, before_snapshot, after_snapshot)
		VALUES ($1, $2, 'status_change', $3, $4, $5)
	`, orderID, userID, req.Reason,
		mustJSON(map[string]string{"status": currentStatus}),
		mustJSON(map[string]string{"status": req.Status}))

	audit.Log(ctx, h.db, &userID, "order.status_change", "order", orderID, nil, req, c.ClientIP())

	o := h.fetchOrder(c, orderID)
	if o != nil {
		response.OK(c, o)
	}
}

func (h *Handler) timeline(c *gin.Context) {
	orderID := c.Param("id")

	// Member role may only view timeline for their own orders
	if role := c.GetString(middleware.KeyRole); role == auth.RoleMember {
		userID := c.GetString(middleware.KeyUserID)
		var memberID, orderMemberID string
		_ = h.db.QueryRow(c.Request.Context(), `SELECT id FROM members WHERE user_id = $1`, userID).Scan(&memberID)
		_ = h.db.QueryRow(c.Request.Context(), `SELECT member_id FROM orders WHERE id = $1`, orderID).Scan(&orderMemberID)
		if memberID == "" || memberID != orderMemberID {
			response.NotFound(c, "order")
			return
		}
	}

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, order_id, actor_id, event_type, description, before_snapshot, after_snapshot, occurred_at
		FROM order_timeline_events WHERE order_id = $1
		ORDER BY occurred_at ASC
	`, orderID)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	events := []timelineEvent{}
	for rows.Next() {
		var e timelineEvent
		var occurredAt time.Time
		var before, after []byte
		if err := rows.Scan(&e.ID, &e.OrderID, &e.ActorID, &e.EventType, &e.Description, &before, &after, &occurredAt); err != nil {
			response.InternalError(c)
			return
		}
		e.Before = before
		e.After = after
		e.OccurredAt = occurredAt.UTC().Format(time.RFC3339)
		events = append(events, e)
	}

	response.OK(c, events)
}

func (h *Handler) addNote(c *gin.Context) {
	orderID := c.Param("id")
	var req noteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	ctx := c.Request.Context()
	userID := c.GetString(middleware.KeyUserID)

	// Member role may only add notes to their own orders
	if role := c.GetString(middleware.KeyRole); role == auth.RoleMember {
		var memberID, orderMemberID string
		_ = h.db.QueryRow(ctx, `SELECT id FROM members WHERE user_id = $1`, userID).Scan(&memberID)
		_ = h.db.QueryRow(ctx, `SELECT member_id FROM orders WHERE id = $1`, orderID).Scan(&orderMemberID)
		if memberID == "" || memberID != orderMemberID {
			response.NotFound(c, "order")
			return
		}
	}

	// Verify order exists
	var exists bool
	_ = h.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM orders WHERE id = $1)`, orderID).Scan(&exists)
	if !exists {
		response.NotFound(c, "order")
		return
	}

	var noteID string
	var createdAt time.Time
	err := h.db.QueryRow(ctx, `
		INSERT INTO order_notes (order_id, author_id, content) VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, orderID, userID, req.Content).Scan(&noteID, &createdAt)
	if err != nil {
		response.InternalError(c)
		return
	}

	// Timeline event
	_, _ = h.db.Exec(ctx, `
		INSERT INTO order_timeline_events (order_id, actor_id, event_type, description)
		VALUES ($1, $2, 'note', $3)
	`, orderID, userID, "Note added")

	response.Created(c, noteResponse{
		ID: noteID, OrderID: orderID, AuthorID: userID,
		Content: req.Content, CreatedAt: createdAt.UTC().Format(time.RFC3339),
	})
}

func (h *Handler) listNotes(c *gin.Context) {
	orderID := c.Param("id")
	role := c.GetString(middleware.KeyRole)

	// Members only see notes on their own orders
	if role == auth.RoleMember {
		userID := c.GetString(middleware.KeyUserID)
		var memberID string
		_ = h.db.QueryRow(c.Request.Context(), `SELECT id FROM members WHERE user_id = $1`, userID).Scan(&memberID)
		var orderMemberID string
		_ = h.db.QueryRow(c.Request.Context(), `SELECT member_id FROM orders WHERE id = $1`, orderID).Scan(&orderMemberID)
		if memberID != orderMemberID {
			response.NotFound(c, "order")
			return
		}
	}

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, order_id, author_id, content, created_at
		FROM order_notes WHERE order_id = $1 ORDER BY created_at ASC
	`, orderID)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	notes := []noteResponse{}
	for rows.Next() {
		var n noteResponse
		var createdAt time.Time
		if err := rows.Scan(&n.ID, &n.OrderID, &n.AuthorID, &n.Content, &createdAt); err != nil {
			response.InternalError(c)
			return
		}
		n.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		notes = append(notes, n)
	}

	response.OK(c, notes)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func (h *Handler) scanOrder(rows pgx.Rows) *orderResponse {
	var o orderResponse
	var gbID, notes, createdBy *string
	var createdAt, updatedAt time.Time

	if err := rows.Scan(&o.ID, &gbID, &o.MemberID, &o.LocationID, &o.Status,
		&o.TotalAmount, &o.Deposit, &notes, &createdBy, &createdAt, &updatedAt, &o.Version); err != nil {
		return nil
	}
	o.GroupBuyID = gbID
	o.Notes = notes
	o.CreatedBy = createdBy
	o.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	o.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return &o
}

func (h *Handler) fetchOrder(c *gin.Context, id string) *orderResponse {
	var o orderResponse
	var gbID, notes, createdBy *string
	var createdAt, updatedAt time.Time

	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, group_buy_id, member_id, location_id, status, total_amount, deposit_amount, notes, created_by, created_at, updated_at, version
		FROM orders WHERE id = $1
	`, id).Scan(&o.ID, &gbID, &o.MemberID, &o.LocationID, &o.Status,
		&o.TotalAmount, &o.Deposit, &notes, &createdBy, &createdAt, &updatedAt, &o.Version)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "order")
		return nil
	}
	if err != nil {
		response.InternalError(c)
		return nil
	}
	o.GroupBuyID = gbID
	o.Notes = notes
	o.CreatedBy = createdBy
	o.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	o.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)

	// Fetch lines
	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, order_id, item_id, quantity, unit_price, deposit_per_unit
		FROM order_line_items WHERE order_id = $1
	`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var l lineResponse
			if rows.Scan(&l.ID, &l.OrderID, &l.ItemID, &l.Quantity, &l.UnitPrice, &l.DepositPerUnit) == nil {
				o.Lines = append(o.Lines, l)
			}
		}
	}

	return &o
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// CreateFromGroupBuy creates orders for all committed participants of a succeeded group-buy.
// Called by the worker or admin after cutoff succeeds.
func CreateFromGroupBuy(ctx context.Context, db *pgxpool.Pool, groupBuyID string) (int, error) {
	rows, err := db.Query(ctx, `
		SELECT p.member_id, p.quantity, g.location_id, g.price_per_unit, g.item_id
		FROM group_buy_participants p
		JOIN group_buys g ON g.id = p.group_buy_id
		WHERE p.group_buy_id = $1 AND p.status = 'committed' AND g.status = 'succeeded'
	`, groupBuyID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	created := 0
	for rows.Next() {
		var memberID, locationID, itemID string
		var quantity int
		var pricePerUnit float64
		if err := rows.Scan(&memberID, &quantity, &locationID, &pricePerUnit, &itemID); err != nil {
			continue
		}
		total := float64(quantity) * pricePerUnit
		var orderID string
		err := db.QueryRow(ctx, `
			INSERT INTO orders (group_buy_id, member_id, location_id, total_amount, status)
			VALUES ($1, $2, $3, $4, 'confirmed')
			RETURNING id
		`, groupBuyID, memberID, locationID, total).Scan(&orderID)
		if err != nil {
			continue
		}
		_, _ = db.Exec(ctx, `
			INSERT INTO order_line_items (order_id, item_id, quantity, unit_price)
			VALUES ($1, $2, $3, $4)
		`, orderID, itemID, quantity, pricePerUnit)
		_, _ = db.Exec(ctx, `
			INSERT INTO order_timeline_events (order_id, event_type, description)
			VALUES ($1, 'creation', $2)
		`, orderID, fmt.Sprintf("Auto-created from group buy %s", groupBuyID[:8]))
		created++
	}
	return created, nil
}
