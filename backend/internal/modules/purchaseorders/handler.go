package purchaseorders

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
	g := r.Group("/purchase-orders")
	g.Use(middleware.RequireRoles(auth.RoleAdministrator, auth.RoleProcurementSpecialist))
	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:id", h.get)
	g.PATCH("/:id", h.update)
	g.POST("/:id/issue", h.issue)
	g.POST("/:id/cancel", h.cancel)
	g.POST("/:id/receive", h.receive)
}

// ── Types ─────────────────────────────────────────────────────────────────────

type poLineRequest struct {
	ItemID   string  `json:"item_id" binding:"required"`
	Quantity int     `json:"quantity" binding:"required,min=1"`
	UnitCost float64 `json:"unit_cost" binding:"required"`
}

type createPORequest struct {
	SupplierID string          `json:"supplier_id" binding:"required"`
	LocationID string          `json:"location_id" binding:"required"`
	Notes      string          `json:"notes"`
	ExpectedAt string          `json:"expected_at"`
	Lines      []poLineRequest `json:"lines" binding:"required,min=1"`
}

type updatePORequest struct {
	Notes      *string `json:"notes"`
	ExpectedAt *string `json:"expected_at"`
}

type receiveLineRequest struct {
	POLineItemID     string `json:"po_line_item_id" binding:"required"`
	QuantityReceived int    `json:"quantity_received" binding:"required,min=0"`
	DiscrepancyNotes string `json:"discrepancy_notes"`
}

type receiveRequest struct {
	Notes string               `json:"notes"`
	Lines []receiveLineRequest `json:"lines" binding:"required,min=1"`
}

type poResponse struct {
	ID         string     `json:"id"`
	SupplierID string     `json:"supplier_id"`
	LocationID string     `json:"location_id"`
	Status     string     `json:"status"`
	Notes      *string    `json:"notes"`
	IssuedAt   *string    `json:"issued_at"`
	ExpectedAt *string    `json:"expected_at"`
	CreatedBy  *string    `json:"created_by"`
	CreatedAt  string     `json:"created_at"`
	UpdatedAt  string     `json:"updated_at"`
	Version    int        `json:"version"`
	Lines      []lineResp `json:"lines,omitempty"`
}

type lineResp struct {
	ID               string  `json:"id"`
	POID             string  `json:"po_id"`
	ItemID           string  `json:"item_id"`
	Quantity         int     `json:"quantity"`
	UnitCost         float64 `json:"unit_cost"`
	ReceivedQuantity int     `json:"received_quantity"`
}

type receiptResponse struct {
	ID         string `json:"id"`
	POID       string `json:"po_id"`
	ReceivedBy string `json:"received_by"`
	ReceivedAt string `json:"received_at"`
	Notes      string `json:"notes"`
}

// ── Status transitions ────────────────────────────────────────────────────────
// draft → issued → partially_received → received
//                                     → closed
//       → cancelled (from draft or issued only)

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

	baseQuery := `FROM purchase_orders WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if status := c.Query("status"); status != "" {
		baseQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	if supplierID := c.Query("supplier_id"); supplierID != "" {
		baseQuery += fmt.Sprintf(" AND supplier_id = $%d", argIdx)
		args = append(args, supplierID)
		argIdx++
	}

	var total int
	_ = h.db.QueryRow(c.Request.Context(), "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)

	query := `SELECT id, supplier_id, location_id, status, notes, issued_at, expected_at, created_by, created_at, updated_at, version ` +
		baseQuery + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, perPage, (page-1)*perPage)

	rows, err := h.db.Query(c.Request.Context(), query, args...)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	orders := []poResponse{}
	for rows.Next() {
		po := h.scanPO(rows)
		if po == nil {
			response.InternalError(c)
			return
		}
		orders = append(orders, *po)
	}

	response.OKPaginated(c, orders, response.Meta{Page: page, PerPage: perPage, Total: total})
}

func (h *Handler) create(c *gin.Context) {
	var req createPORequest
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

	var expectedAt *string
	if req.ExpectedAt != "" {
		expectedAt = &req.ExpectedAt
	}
	var notes *string
	if req.Notes != "" {
		notes = &req.Notes
	}

	var poID string
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO purchase_orders (supplier_id, location_id, notes, expected_at, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`, req.SupplierID, req.LocationID, notes, expectedAt, userID,
	).Scan(&poID, &createdAt, &updatedAt)
	if err != nil {
		response.InternalError(c)
		return
	}

	lines := []lineResp{}
	for _, l := range req.Lines {
		var lineID string
		err = tx.QueryRow(ctx, `
			INSERT INTO po_line_items (po_id, item_id, quantity, unit_cost)
			VALUES ($1, $2, $3, $4) RETURNING id
		`, poID, l.ItemID, l.Quantity, l.UnitCost).Scan(&lineID)
		if err != nil {
			response.InternalError(c)
			return
		}
		lines = append(lines, lineResp{
			ID: lineID, POID: poID, ItemID: l.ItemID,
			Quantity: l.Quantity, UnitCost: l.UnitCost, ReceivedQuantity: 0,
		})
	}

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c)
		return
	}

	audit.Log(ctx, h.db, &userID, "po.create", "purchase_order", poID, nil, req, c.ClientIP())

	var issuedAt *string
	var expAt *string
	if req.ExpectedAt != "" {
		expAt = &req.ExpectedAt
	}

	response.Created(c, poResponse{
		ID: poID, SupplierID: req.SupplierID, LocationID: req.LocationID,
		Status: "draft", Notes: notes, IssuedAt: issuedAt, ExpectedAt: expAt,
		CreatedBy: &userID,
		CreatedAt: createdAt.UTC().Format(time.RFC3339),
		UpdatedAt: updatedAt.UTC().Format(time.RFC3339),
		Version:   1, Lines: lines,
	})
}

func (h *Handler) get(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	var po poResponse
	var notes, issuedAt, expectedAt, createdBy *string
	var createdAtT, updatedAtT time.Time
	var issuedAtT *time.Time

	err := h.db.QueryRow(ctx, `
		SELECT id, supplier_id, location_id, status, notes, issued_at, expected_at::text, created_by,
			created_at, updated_at, version
		FROM purchase_orders WHERE id = $1
	`, id).Scan(&po.ID, &po.SupplierID, &po.LocationID, &po.Status, &notes,
		&issuedAtT, &expectedAt, &createdBy, &createdAtT, &updatedAtT, &po.Version)

	if err == pgx.ErrNoRows {
		response.NotFound(c, "purchase order")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}

	po.Notes = notes
	po.CreatedBy = createdBy
	po.ExpectedAt = expectedAt
	po.CreatedAt = createdAtT.UTC().Format(time.RFC3339)
	po.UpdatedAt = updatedAtT.UTC().Format(time.RFC3339)
	if issuedAtT != nil {
		s := issuedAtT.UTC().Format(time.RFC3339)
		po.IssuedAt = &s
	}

	// Fetch lines
	rows, err := h.db.Query(ctx, `
		SELECT id, po_id, item_id, quantity, unit_cost, received_quantity
		FROM po_line_items WHERE po_id = $1
	`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var l lineResp
			if rows.Scan(&l.ID, &l.POID, &l.ItemID, &l.Quantity, &l.UnitCost, &l.ReceivedQuantity) == nil {
				po.Lines = append(po.Lines, l)
			}
		}
	}

	response.OK(c, po)
}

func (h *Handler) update(c *gin.Context) {
	id := c.Param("id")
	var req updatePORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	// Only allow update on draft POs
	var currentStatus string
	err := h.db.QueryRow(c.Request.Context(), `SELECT status FROM purchase_orders WHERE id = $1`, id).Scan(&currentStatus)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "purchase order")
		return
	}
	if currentStatus != "draft" {
		response.Conflict(c, "can only edit draft purchase orders")
		return
	}

	sets := []string{"updated_at = NOW()", "version = version + 1"}
	args := []interface{}{}
	argIdx := 1

	if req.Notes != nil {
		sets = append(sets, fmt.Sprintf("notes = $%d", argIdx))
		args = append(args, *req.Notes)
		argIdx++
	}
	if req.ExpectedAt != nil {
		sets = append(sets, fmt.Sprintf("expected_at = $%d", argIdx))
		args = append(args, *req.ExpectedAt)
		argIdx++
	}

	query := fmt.Sprintf("UPDATE purchase_orders SET %s WHERE id = $%d", strings.Join(sets, ", "), argIdx)
	args = append(args, id)

	_, err = h.db.Exec(c.Request.Context(), query, args...)
	if err != nil {
		response.InternalError(c)
		return
	}

	userID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &userID, "po.update", "purchase_order", id, nil, req, c.ClientIP())

	// Return updated PO via get
	h.get(c)
}

func (h *Handler) issue(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	tag, err := h.db.Exec(ctx, `
		UPDATE purchase_orders SET status = 'issued', issued_at = NOW(), updated_at = NOW(), version = version + 1
		WHERE id = $1 AND status = 'draft'
	`, id)
	if err != nil {
		response.InternalError(c)
		return
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		_ = h.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM purchase_orders WHERE id = $1)`, id).Scan(&exists)
		if !exists {
			response.NotFound(c, "purchase order")
		} else {
			response.Conflict(c, "can only issue draft purchase orders")
		}
		return
	}

	userID := c.GetString(middleware.KeyUserID)
	audit.Log(ctx, h.db, &userID, "po.issue", "purchase_order", id, nil, nil, c.ClientIP())

	h.get(c)
}

func (h *Handler) cancel(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	tag, err := h.db.Exec(ctx, `
		UPDATE purchase_orders SET status = 'cancelled', updated_at = NOW(), version = version + 1
		WHERE id = $1 AND status IN ('draft', 'issued')
	`, id)
	if err != nil {
		response.InternalError(c)
		return
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		_ = h.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM purchase_orders WHERE id = $1)`, id).Scan(&exists)
		if !exists {
			response.NotFound(c, "purchase order")
		} else {
			response.Conflict(c, "can only cancel draft or issued purchase orders")
		}
		return
	}

	userID := c.GetString(middleware.KeyUserID)
	audit.Log(ctx, h.db, &userID, "po.cancel", "purchase_order", id, nil, nil, c.ClientIP())

	response.NoContent(c)
}

func (h *Handler) receive(c *gin.Context) {
	poID := c.Param("id")
	var req receiveRequest
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

	// Lock PO row — only issued or partially_received can be received
	var poStatus, locationID string
	err = tx.QueryRow(ctx, `
		SELECT status, location_id FROM purchase_orders WHERE id = $1 FOR UPDATE
	`, poID).Scan(&poStatus, &locationID)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "purchase order")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}
	if poStatus != "issued" && poStatus != "partially_received" {
		response.Conflict(c, "can only receive issued or partially_received purchase orders")
		return
	}

	// Create goods receipt
	var notes *string
	if req.Notes != "" {
		notes = &req.Notes
	}
	var receiptID string
	var receivedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO goods_receipts (po_id, received_by, notes) VALUES ($1, $2, $3)
		RETURNING id, received_at
	`, poID, userID, notes).Scan(&receiptID, &receivedAt)
	if err != nil {
		response.InternalError(c)
		return
	}

	// Process each line
	for _, rl := range req.Lines {
		// Get PO line details with lock
		var lineItemID string
		var ordered, alreadyReceived int
		err = tx.QueryRow(ctx, `
			SELECT item_id, quantity, received_quantity FROM po_line_items WHERE id = $1 AND po_id = $2 FOR UPDATE
		`, rl.POLineItemID, poID).Scan(&lineItemID, &ordered, &alreadyReceived)
		if err == pgx.ErrNoRows {
			response.BadRequest(c, "INVALID_LINE", fmt.Sprintf("PO line %s not found", rl.POLineItemID))
			return
		}
		if err != nil {
			response.InternalError(c)
			return
		}

		// Prevent over-receiving
		if alreadyReceived+rl.QuantityReceived > ordered {
			response.Unprocessable(c, "over-receiving", map[string]string{
				rl.POLineItemID: fmt.Sprintf("would exceed ordered qty (%d ordered, %d already received, %d new)",
					ordered, alreadyReceived, rl.QuantityReceived),
			})
			return
		}

		// Update received quantity on PO line
		_, err = tx.Exec(ctx, `
			UPDATE po_line_items SET received_quantity = received_quantity + $1 WHERE id = $2
		`, rl.QuantityReceived, rl.POLineItemID)
		if err != nil {
			response.InternalError(c)
			return
		}

		// Record goods receipt line
		var discNotes *string
		if rl.DiscrepancyNotes != "" {
			discNotes = &rl.DiscrepancyNotes
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO goods_receipt_lines (receipt_id, po_line_item_id, quantity_received, discrepancy_notes)
			VALUES ($1, $2, $3, $4)
		`, receiptID, rl.POLineItemID, rl.QuantityReceived, discNotes)
		if err != nil {
			response.InternalError(c)
			return
		}

		// Update inventory stock — upsert
		if rl.QuantityReceived > 0 {
			_, err = tx.Exec(ctx, `
				INSERT INTO inventory_stock (item_id, location_id, on_hand)
				VALUES ($1, $2, $3)
				ON CONFLICT (item_id, location_id)
				DO UPDATE SET on_hand = inventory_stock.on_hand + $3, updated_at = NOW()
			`, lineItemID, locationID, rl.QuantityReceived)
			if err != nil {
				response.InternalError(c)
				return
			}
		}
	}

	// Determine new PO status
	var totalOrdered, totalReceived int
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(quantity), 0), COALESCE(SUM(received_quantity), 0)
		FROM po_line_items WHERE po_id = $1
	`, poID).Scan(&totalOrdered, &totalReceived)
	if err != nil {
		response.InternalError(c)
		return
	}

	newStatus := "partially_received"
	if totalReceived >= totalOrdered {
		newStatus = "received"
	}

	_, err = tx.Exec(ctx, `
		UPDATE purchase_orders SET status = $1, updated_at = NOW(), version = version + 1
		WHERE id = $2
	`, newStatus, poID)
	if err != nil {
		response.InternalError(c)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c)
		return
	}

	audit.Log(ctx, h.db, &userID, "po.receive", "purchase_order", poID, nil,
		map[string]interface{}{"receipt_id": receiptID, "new_status": newStatus}, c.ClientIP())

	response.Created(c, receiptResponse{
		ID:         receiptID,
		POID:       poID,
		ReceivedBy: userID,
		ReceivedAt: receivedAt.UTC().Format(time.RFC3339),
		Notes:      req.Notes,
	})
}

func (h *Handler) scanPO(rows pgx.Rows) *poResponse {
	var po poResponse
	var notes, expectedAt, createdBy *string
	var issuedAt *time.Time
	var createdAt, updatedAt time.Time

	if err := rows.Scan(&po.ID, &po.SupplierID, &po.LocationID, &po.Status, &notes,
		&issuedAt, &expectedAt, &createdBy, &createdAt, &updatedAt, &po.Version); err != nil {
		return nil
	}

	po.Notes = notes
	po.ExpectedAt = expectedAt
	po.CreatedBy = createdBy
	po.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	po.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	if issuedAt != nil {
		s := issuedAt.UTC().Format(time.RFC3339)
		po.IssuedAt = &s
	}

	return &po
}
