package suppliers

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
	g := r.Group("/suppliers")
	g.Use(middleware.RequireRoles(auth.RoleAdministrator, auth.RoleProcurementSpecialist, auth.RoleOperationsManager))
	g.GET("", h.list)
	g.POST("", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleProcurementSpecialist), h.create)
	g.GET("/:id", h.get)
	g.PATCH("/:id", middleware.RequireRoles(auth.RoleAdministrator, auth.RoleProcurementSpecialist), h.update)
}

// ── Types ─────────────────────────────────────────────────────────────────────

type createSupplierRequest struct {
	Name        string `json:"name" binding:"required"`
	ContactName string `json:"contact_name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Address     string `json:"address"`
	IsActive    *bool  `json:"is_active"`
}

type updateSupplierRequest struct {
	Name        *string `json:"name"`
	ContactName *string `json:"contact_name"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
	Address     *string `json:"address"`
	IsActive    *bool   `json:"is_active"`
}

type supplierResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	ContactName *string `json:"contact_name"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
	Address     *string `json:"address"`
	IsActive    bool    `json:"is_active"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
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

	baseQuery := `FROM suppliers WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if search := c.Query("search"); search != "" {
		baseQuery += fmt.Sprintf(" AND name ILIKE $%d", argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if active := c.Query("is_active"); active == "true" || active == "false" {
		baseQuery += fmt.Sprintf(" AND is_active = $%d", argIdx)
		args = append(args, active == "true")
		argIdx++
	}

	var total int
	_ = h.db.QueryRow(c.Request.Context(), "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)

	query := "SELECT id, name, contact_name, email, phone, address, is_active, created_at, updated_at " + baseQuery +
		fmt.Sprintf(" ORDER BY name LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, perPage, (page-1)*perPage)

	rows, err := h.db.Query(c.Request.Context(), query, args...)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	suppliers := []supplierResponse{}
	for rows.Next() {
		var s supplierResponse
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&s.ID, &s.Name, &s.ContactName, &s.Email, &s.Phone, &s.Address,
			&s.IsActive, &createdAt, &updatedAt); err != nil {
			response.InternalError(c)
			return
		}
		s.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		s.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		suppliers = append(suppliers, s)
	}

	response.OKPaginated(c, suppliers, response.Meta{Page: page, PerPage: perPage, Total: total})
}

func (h *Handler) create(c *gin.Context) {
	var req createSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	var s supplierResponse
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(c.Request.Context(), `
		INSERT INTO suppliers (name, contact_name, email, phone, address, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, contact_name, email, phone, address, is_active, created_at, updated_at
	`, req.Name, nilIfEmpty(req.ContactName), nilIfEmpty(req.Email), nilIfEmpty(req.Phone),
		nilIfEmpty(req.Address), isActive,
	).Scan(&s.ID, &s.Name, &s.ContactName, &s.Email, &s.Phone, &s.Address,
		&s.IsActive, &createdAt, &updatedAt)
	if err != nil {
		response.InternalError(c)
		return
	}
	s.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	s.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)

	userID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &userID, "supplier.create", "supplier", s.ID, nil, req, c.ClientIP())

	response.Created(c, s)
}

func (h *Handler) get(c *gin.Context) {
	id := c.Param("id")
	var s supplierResponse
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, name, contact_name, email, phone, address, is_active, created_at, updated_at
		FROM suppliers WHERE id = $1
	`, id).Scan(&s.ID, &s.Name, &s.ContactName, &s.Email, &s.Phone, &s.Address,
		&s.IsActive, &createdAt, &updatedAt)

	if err == pgx.ErrNoRows {
		response.NotFound(c, "supplier")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}
	s.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	s.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)

	response.OK(c, s)
}

func (h *Handler) update(c *gin.Context) {
	id := c.Param("id")
	var req updateSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	sets := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *req.Name)
		argIdx++
	}
	if req.ContactName != nil {
		sets = append(sets, fmt.Sprintf("contact_name = $%d", argIdx))
		args = append(args, *req.ContactName)
		argIdx++
	}
	if req.Email != nil {
		sets = append(sets, fmt.Sprintf("email = $%d", argIdx))
		args = append(args, *req.Email)
		argIdx++
	}
	if req.Phone != nil {
		sets = append(sets, fmt.Sprintf("phone = $%d", argIdx))
		args = append(args, *req.Phone)
		argIdx++
	}
	if req.Address != nil {
		sets = append(sets, fmt.Sprintf("address = $%d", argIdx))
		args = append(args, *req.Address)
		argIdx++
	}
	if req.IsActive != nil {
		sets = append(sets, fmt.Sprintf("is_active = $%d", argIdx))
		args = append(args, *req.IsActive)
		argIdx++
	}

	if len(sets) == 0 {
		response.BadRequest(c, "VALIDATION_ERROR", "no fields to update")
		return
	}
	sets = append(sets, "updated_at = NOW()")

	query := fmt.Sprintf(`UPDATE suppliers SET %s WHERE id = $%d
		RETURNING id, name, contact_name, email, phone, address, is_active, created_at, updated_at`,
		strings.Join(sets, ", "), argIdx)
	args = append(args, id)

	var s supplierResponse
	var createdAt, updatedAt time.Time
	err := h.db.QueryRow(c.Request.Context(), query, args...).Scan(
		&s.ID, &s.Name, &s.ContactName, &s.Email, &s.Phone, &s.Address,
		&s.IsActive, &createdAt, &updatedAt)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "supplier")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}
	s.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	s.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)

	userID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &userID, "supplier.update", "supplier", id, nil, req, c.ClientIP())

	response.OK(c, s)
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

