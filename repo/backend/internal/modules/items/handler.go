package items

import (
	"fmt"
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
	g := r.Group("/items")
	g.GET("", h.list)
	g.POST("", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.create)
	g.POST("/batch", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.batchUpdate)
	g.GET("/:id", h.get)
	g.PATCH("/:id", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.update)
	g.DELETE("/:id", middleware.RequireRoles(auth.RoleAdministrator), h.delete)
	g.POST("/:id/publish", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.publish)
	g.POST("/:id/unpublish", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.unpublish)
	g.GET("/:id/availability-windows", h.listWindows)
	g.POST("/:id/availability-windows", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.addWindow)
	g.DELETE("/:id/availability-windows/:wid", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleOperationsManager), h.deleteWindow)
}

// ── Request / response types ──────────────────────────────────────────────────

type createItemRequest struct {
	Name          string   `json:"name" binding:"required"`
	SKU           string   `json:"sku"`
	Category      string   `json:"category" binding:"required"`
	Brand         string   `json:"brand"`
	Condition     string   `json:"condition"`
	Description   string   `json:"description"`
	Images        []string `json:"images"`
	DepositAmount *float64 `json:"deposit_amount"`
	BillingModel  string   `json:"billing_model"`
	Price         float64  `json:"price" binding:"required"`
	LocationID    string   `json:"location_id"`
}

type updateItemRequest struct {
	Name          *string  `json:"name"`
	SKU           *string  `json:"sku"`
	Category      *string  `json:"category"`
	Brand         *string  `json:"brand"`
	Condition     *string  `json:"condition"`
	Description   *string  `json:"description"`
	Images        []string `json:"images"`
	DepositAmount *float64 `json:"deposit_amount"`
	BillingModel  *string  `json:"billing_model"`
	Price         *float64 `json:"price"`
}

type batchUpdateRequest struct {
	ItemIDs             []string                 `json:"item_ids" binding:"required,min=1"`
	Price               *float64                 `json:"price"`
	AvailabilityWindows []batchAvailabilityInput `json:"availability_windows"`
}

type batchAvailabilityInput struct {
	StartsAt string `json:"starts_at" binding:"required"`
	EndsAt   string `json:"ends_at" binding:"required"`
}

type addWindowRequest struct {
	StartsAt string `json:"starts_at" binding:"required"`
	EndsAt   string `json:"ends_at" binding:"required"`
}

type itemResponse struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	SKU           *string  `json:"sku"`
	Category      string   `json:"category"`
	Brand         *string  `json:"brand"`
	Condition     string   `json:"condition"`
	Description   *string  `json:"description"`
	Images        []string `json:"images"`
	DepositAmount float64  `json:"deposit_amount"`
	BillingModel  string   `json:"billing_model"`
	Price         float64  `json:"price"`
	Status        string   `json:"status"`
	LocationID    *string  `json:"location_id"`
	CreatedBy     *string  `json:"created_by"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	Version       int      `json:"version"`
}

type windowResponse struct {
	ID        string `json:"id"`
	ItemID    string `json:"item_id"`
	StartsAt  string `json:"starts_at"`
	EndsAt    string `json:"ends_at"`
	CreatedAt string `json:"created_at"`
}

// ── Validation helpers ────────────────────────────────────────────────────────

var validConditions = map[string]bool{"new": true, "open-box": true, "used": true}
var validBillingModels = map[string]bool{"one-time": true, "monthly-rental": true}

func validateItemFields(cond, billing string, price float64, deposit *float64) map[string]string {
	errs := map[string]string{}
	if cond != "" && !validConditions[cond] {
		errs["condition"] = "must be new, open-box, or used"
	}
	if billing != "" && !validBillingModels[billing] {
		errs["billing_model"] = "must be one-time or monthly-rental"
	}
	if price < 0 {
		errs["price"] = "must be >= 0"
	}
	if deposit != nil && *deposit < 0 {
		errs["deposit_amount"] = "must be >= 0"
	}
	return errs
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *Handler) list(c *gin.Context) {
	role := c.GetString(middleware.KeyRole)

	baseQuery := `SELECT id, name, sku, category, brand, condition, description, images,
		deposit_amount, billing_model, price, status, location_id, created_by, created_at, updated_at, version
		FROM items WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	// Role-based filtering
	switch role {
	case auth.RoleMember:
		baseQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, "published")
		argIdx++
	case auth.RoleCoach:
		// Coach sees published items only (readiness subset)
		baseQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, "published")
		argIdx++
	}

	// Optional filters
	if cat := c.Query("category"); cat != "" {
		baseQuery += fmt.Sprintf(" AND category = $%d", argIdx)
		args = append(args, cat)
		argIdx++
	}
	if status := c.Query("status"); status != "" && role != auth.RoleMember && role != auth.RoleCoach {
		baseQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	if search := c.Query("search"); search != "" {
		baseQuery += fmt.Sprintf(" AND (name ILIKE $%d OR sku ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "25"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	// Count
	countQuery := strings.Replace(baseQuery, "SELECT id, name, sku, category, brand, condition, description, images,\n\t\tdeposit_amount, billing_model, price, status, location_id, created_by, created_at, updated_at, version",
		"SELECT COUNT(*)", 1)
	var total int
	_ = h.db.QueryRow(c.Request.Context(), countQuery, args...).Scan(&total)

	baseQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, perPage, (page-1)*perPage)

	rows, err := h.db.Query(c.Request.Context(), baseQuery, args...)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	items := []itemResponse{}
	for rows.Next() {
		var it itemResponse
		var sku, brand, desc, locID, createdBy *string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&it.ID, &it.Name, &sku, &it.Category, &brand, &it.Condition, &desc,
			&it.Images, &it.DepositAmount, &it.BillingModel, &it.Price, &it.Status,
			&locID, &createdBy, &createdAt, &updatedAt, &it.Version); err != nil {
			response.InternalError(c)
			return
		}
		it.SKU = sku
		it.Brand = brand
		it.Description = desc
		it.LocationID = locID
		it.CreatedBy = createdBy
		it.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		it.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		if it.Images == nil {
			it.Images = []string{}
		}
		items = append(items, it)
	}

	response.OKPaginated(c, items, response.Meta{Page: page, PerPage: perPage, Total: total})
}

func (h *Handler) create(c *gin.Context) {
	var req createItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	cond := req.Condition
	if cond == "" {
		cond = "new"
	}
	billing := req.BillingModel
	if billing == "" {
		billing = "one-time"
	}
	deposit := 50.0
	if req.DepositAmount != nil {
		deposit = *req.DepositAmount
	}

	if errs := validateItemFields(cond, billing, req.Price, req.DepositAmount); len(errs) > 0 {
		response.Unprocessable(c, "validation failed", errs)
		return
	}

	images := req.Images
	if images == nil {
		images = []string{}
	}

	userID := c.GetString(middleware.KeyUserID)
	var locID *string
	if req.LocationID != "" {
		locID = &req.LocationID
	}
	var sku *string
	if req.SKU != "" {
		sku = &req.SKU
	}

	var id string
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(c.Request.Context(), `
		INSERT INTO items (name, sku, category, brand, condition, description, images, deposit_amount, billing_model, price, location_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at
	`, req.Name, sku, req.Category, req.Brand, cond, req.Description, images, deposit, billing, req.Price, locID, userID,
	).Scan(&id, &createdAt, &updatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "items_sku_key") {
			response.Conflict(c, "SKU already exists")
			return
		}
		response.InternalError(c)
		return
	}

	actorID := userID
	audit.Log(c.Request.Context(), h.db, &actorID, "item.create", "item", id, nil, req, c.ClientIP())

	response.Created(c, itemResponse{
		ID: id, Name: req.Name, SKU: sku, Category: req.Category, Brand: &req.Brand,
		Condition: cond, Description: &req.Description, Images: images,
		DepositAmount: deposit, BillingModel: billing, Price: req.Price, Status: "draft",
		LocationID: locID, CreatedBy: &userID,
		CreatedAt: createdAt.UTC().Format(time.RFC3339),
		UpdatedAt: updatedAt.UTC().Format(time.RFC3339),
		Version:   1,
	})
}

func (h *Handler) get(c *gin.Context) {
	id := c.Param("id")
	role := c.GetString(middleware.KeyRole)

	var it itemResponse
	var sku, brand, desc, locID, createdBy *string
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, name, sku, category, brand, condition, description, images,
			deposit_amount, billing_model, price, status, location_id, created_by, created_at, updated_at, version
		FROM items WHERE id = $1
	`, id).Scan(&it.ID, &it.Name, &sku, &it.Category, &brand, &it.Condition, &desc,
		&it.Images, &it.DepositAmount, &it.BillingModel, &it.Price, &it.Status,
		&locID, &createdBy, &createdAt, &updatedAt, &it.Version)

	if err == pgx.ErrNoRows {
		response.NotFound(c, "item")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}

	// Role-based visibility
	if (role == auth.RoleMember || role == auth.RoleCoach) && it.Status != "published" {
		response.NotFound(c, "item")
		return
	}

	it.SKU = sku
	it.Brand = brand
	it.Description = desc
	it.LocationID = locID
	it.CreatedBy = createdBy
	it.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	it.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	if it.Images == nil {
		it.Images = []string{}
	}

	response.OK(c, it)
}

func (h *Handler) update(c *gin.Context) {
	id := c.Param("id")
	var req updateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	// Validate provided fields
	cond := ""
	if req.Condition != nil {
		cond = *req.Condition
	}
	billing := ""
	if req.BillingModel != nil {
		billing = *req.BillingModel
	}
	price := 0.0
	if req.Price != nil {
		price = *req.Price
	}
	if errs := validateItemFields(cond, billing, price, req.DepositAmount); len(errs) > 0 {
		response.Unprocessable(c, "validation failed", errs)
		return
	}

	// Build dynamic UPDATE
	sets := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *req.Name)
		argIdx++
	}
	if req.SKU != nil {
		sets = append(sets, fmt.Sprintf("sku = $%d", argIdx))
		args = append(args, *req.SKU)
		argIdx++
	}
	if req.Category != nil {
		sets = append(sets, fmt.Sprintf("category = $%d", argIdx))
		args = append(args, *req.Category)
		argIdx++
	}
	if req.Brand != nil {
		sets = append(sets, fmt.Sprintf("brand = $%d", argIdx))
		args = append(args, *req.Brand)
		argIdx++
	}
	if req.Condition != nil {
		sets = append(sets, fmt.Sprintf("condition = $%d", argIdx))
		args = append(args, *req.Condition)
		argIdx++
	}
	if req.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *req.Description)
		argIdx++
	}
	if req.Images != nil {
		sets = append(sets, fmt.Sprintf("images = $%d", argIdx))
		args = append(args, req.Images)
		argIdx++
	}
	if req.DepositAmount != nil {
		sets = append(sets, fmt.Sprintf("deposit_amount = $%d", argIdx))
		args = append(args, *req.DepositAmount)
		argIdx++
	}
	if req.BillingModel != nil {
		sets = append(sets, fmt.Sprintf("billing_model = $%d", argIdx))
		args = append(args, *req.BillingModel)
		argIdx++
	}
	if req.Price != nil {
		sets = append(sets, fmt.Sprintf("price = $%d", argIdx))
		args = append(args, *req.Price)
		argIdx++
	}

	if len(sets) == 0 {
		response.BadRequest(c, "VALIDATION_ERROR", "no fields to update")
		return
	}

	sets = append(sets, "updated_at = NOW()")
	sets = append(sets, fmt.Sprintf("version = version + 1"))

	query := fmt.Sprintf(`UPDATE items SET %s WHERE id = $%d
		RETURNING id, name, sku, category, brand, condition, description, images,
			deposit_amount, billing_model, price, status, location_id, created_by, created_at, updated_at, version`,
		strings.Join(sets, ", "), argIdx)
	args = append(args, id)

	var it itemResponse
	var sku, brand, desc, locID, createdBy *string
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(c.Request.Context(), query, args...).Scan(
		&it.ID, &it.Name, &sku, &it.Category, &brand, &it.Condition, &desc,
		&it.Images, &it.DepositAmount, &it.BillingModel, &it.Price, &it.Status,
		&locID, &createdBy, &createdAt, &updatedAt, &it.Version)

	if err == pgx.ErrNoRows {
		response.NotFound(c, "item")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "items_sku_key") {
			response.Conflict(c, "SKU already exists")
			return
		}
		response.InternalError(c)
		return
	}

	it.SKU = sku
	it.Brand = brand
	it.Description = desc
	it.LocationID = locID
	it.CreatedBy = createdBy
	it.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	it.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	if it.Images == nil {
		it.Images = []string{}
	}

	userID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &userID, "item.update", "item", id, nil, req, c.ClientIP())

	response.OK(c, it)
}

func (h *Handler) delete(c *gin.Context) {
	id := c.Param("id")
	tag, err := h.db.Exec(c.Request.Context(), `DELETE FROM items WHERE id = $1`, id)
	if err != nil {
		response.InternalError(c)
		return
	}
	if tag.RowsAffected() == 0 {
		response.NotFound(c, "item")
		return
	}
	userID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &userID, "item.delete", "item", id, nil, nil, c.ClientIP())
	response.NoContent(c)
}

func (h *Handler) publish(c *gin.Context) {
	h.changeStatus(c, c.Param("id"), "published", []string{"draft", "unpublished"})
}

func (h *Handler) unpublish(c *gin.Context) {
	h.changeStatus(c, c.Param("id"), "unpublished", []string{"published"})
}

func (h *Handler) changeStatus(c *gin.Context, id, newStatus string, allowedFrom []string) {
	placeholders := make([]string, len(allowedFrom))
	args := []interface{}{newStatus, id}
	for i, s := range allowedFrom {
		placeholders[i] = fmt.Sprintf("$%d", i+3)
		args = append(args, s)
	}

	query := fmt.Sprintf(`UPDATE items SET status = $1, updated_at = NOW(), version = version + 1
		WHERE id = $2 AND status IN (%s)
		RETURNING id, name, sku, category, brand, condition, description, images,
			deposit_amount, billing_model, price, status, location_id, created_by, created_at, updated_at, version`,
		strings.Join(placeholders, ","))

	var it itemResponse
	var sku, brand, desc, locID, createdBy *string
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(c.Request.Context(), query, args...).Scan(
		&it.ID, &it.Name, &sku, &it.Category, &brand, &it.Condition, &desc,
		&it.Images, &it.DepositAmount, &it.BillingModel, &it.Price, &it.Status,
		&locID, &createdBy, &createdAt, &updatedAt, &it.Version)

	if err == pgx.ErrNoRows {
		// Check if item exists at all
		var exists bool
		_ = h.db.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM items WHERE id = $1)`, id).Scan(&exists)
		if !exists {
			response.NotFound(c, "item")
		} else {
			response.Conflict(c, fmt.Sprintf("item cannot transition to %s from current status", newStatus))
		}
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}

	it.SKU = sku
	it.Brand = brand
	it.Description = desc
	it.LocationID = locID
	it.CreatedBy = createdBy
	it.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	it.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	if it.Images == nil {
		it.Images = []string{}
	}

	userID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &userID, "item."+newStatus, "item", id, nil, nil, c.ClientIP())

	response.OK(c, it)
}

func (h *Handler) batchUpdate(c *gin.Context) {
	var req batchUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	if req.Price != nil && *req.Price < 0 {
		response.Unprocessable(c, "validation failed", map[string]string{"price": "must be >= 0"})
		return
	}

	if req.Price == nil && len(req.AvailabilityWindows) == 0 {
		response.BadRequest(c, "VALIDATION_ERROR", "at least one field (price or availability_windows) must be provided")
		return
	}

	type parsedWindow struct {
		start time.Time
		end   time.Time
	}
	parsedWindows := make([]parsedWindow, 0, len(req.AvailabilityWindows))
	for i, w := range req.AvailabilityWindows {
		start, err := time.Parse(time.RFC3339, w.StartsAt)
		if err != nil {
			response.Unprocessable(c, "validation failed", map[string]string{
				fmt.Sprintf("availability_windows[%d].starts_at", i): "must be RFC3339 format",
			})
			return
		}
		end, err := time.Parse(time.RFC3339, w.EndsAt)
		if err != nil {
			response.Unprocessable(c, "validation failed", map[string]string{
				fmt.Sprintf("availability_windows[%d].ends_at", i): "must be RFC3339 format",
			})
			return
		}
		if !end.After(start) {
			response.Unprocessable(c, "validation failed", map[string]string{
				fmt.Sprintf("availability_windows[%d].ends_at", i): "must be after starts_at",
			})
			return
		}
		parsedWindows = append(parsedWindows, parsedWindow{start: start, end: end})
	}

	ctx := c.Request.Context()
	tx, err := h.db.Begin(ctx)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer tx.Rollback(ctx)

	updatedRows := int64(0)
	if req.Price != nil {
		// Build IN clause for price updates.
		placeholders := make([]string, len(req.ItemIDs))
		args := []interface{}{*req.Price}
		for i, id := range req.ItemIDs {
			placeholders[i] = fmt.Sprintf("$%d", i+2)
			args = append(args, id)
		}

		query := fmt.Sprintf(`UPDATE items SET price = $1, updated_at = NOW(), version = version + 1
			WHERE id IN (%s)`, strings.Join(placeholders, ","))

		tag, err := tx.Exec(ctx, query, args...)
		if err != nil {
			response.InternalError(c)
			return
		}
		updatedRows = tag.RowsAffected()
	}

	if len(parsedWindows) > 0 {
		for _, itemID := range req.ItemIDs {
			if _, err := tx.Exec(ctx, `DELETE FROM item_availability_windows WHERE item_id = $1`, itemID); err != nil {
				response.InternalError(c)
				return
			}
			for _, w := range parsedWindows {
				if _, err := tx.Exec(ctx, `
					INSERT INTO item_availability_windows (item_id, starts_at, ends_at)
					VALUES ($1, $2, $3)
				`, itemID, w.start, w.end); err != nil {
					response.InternalError(c)
					return
				}
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c)
		return
	}

	userID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &userID, "item.batch_update", "item", "batch", nil, req, c.ClientIP())

	response.OK(c, gin.H{"updated": updatedRows, "availability_updated": len(parsedWindows) > 0})
}

// ── Availability windows ──────────────────────────────────────────────────────

func (h *Handler) listWindows(c *gin.Context) {
	itemID := c.Param("id")

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, item_id, starts_at, ends_at, created_at
		FROM item_availability_windows WHERE item_id = $1
		ORDER BY starts_at
	`, itemID)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	windows := []windowResponse{}
	for rows.Next() {
		var w windowResponse
		var startsAt, endsAt, createdAt time.Time
		if err := rows.Scan(&w.ID, &w.ItemID, &startsAt, &endsAt, &createdAt); err != nil {
			response.InternalError(c)
			return
		}
		w.StartsAt = startsAt.UTC().Format(time.RFC3339)
		w.EndsAt = endsAt.UTC().Format(time.RFC3339)
		w.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		windows = append(windows, w)
	}

	response.OK(c, windows)
}

func (h *Handler) addWindow(c *gin.Context) {
	itemID := c.Param("id")
	var req addWindowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", "starts_at must be RFC3339 format")
		return
	}
	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", "ends_at must be RFC3339 format")
		return
	}
	if !endsAt.After(startsAt) {
		response.Unprocessable(c, "validation failed", map[string]string{"ends_at": "must be after starts_at"})
		return
	}

	// Verify item exists
	var exists bool
	_ = h.db.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM items WHERE id = $1)`, itemID).Scan(&exists)
	if !exists {
		response.NotFound(c, "item")
		return
	}

	var w windowResponse
	var sAt, eAt, cAt time.Time
	err = h.db.QueryRow(c.Request.Context(), `
		INSERT INTO item_availability_windows (item_id, starts_at, ends_at)
		VALUES ($1, $2, $3) RETURNING id, item_id, starts_at, ends_at, created_at
	`, itemID, startsAt, endsAt).Scan(&w.ID, &w.ItemID, &sAt, &eAt, &cAt)
	if err != nil {
		response.InternalError(c)
		return
	}
	w.StartsAt = sAt.UTC().Format(time.RFC3339)
	w.EndsAt = eAt.UTC().Format(time.RFC3339)
	w.CreatedAt = cAt.UTC().Format(time.RFC3339)

	response.Created(c, w)
}

func (h *Handler) deleteWindow(c *gin.Context) {
	wid := c.Param("wid")
	tag, err := h.db.Exec(c.Request.Context(), `DELETE FROM item_availability_windows WHERE id = $1`, wid)
	if err != nil {
		response.InternalError(c)
		return
	}
	if tag.RowsAffected() == 0 {
		response.NotFound(c, "availability window")
		return
	}
	response.NoContent(c)
}
