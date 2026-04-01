package audit

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
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
	g := r.Group("/audit")
	g.Use(middleware.RequireRoles(auth.RoleAdministrator))
	g.GET("", h.list)
	g.GET("/:entity/:id", h.entityLog)
}

type auditEntry struct {
	ID         string      `json:"id"`
	ActorID    *string     `json:"actor_id"`
	Action     string      `json:"action"`
	EntityType string      `json:"entity_type"`
	EntityID   string      `json:"entity_id"`
	Before     interface{} `json:"before_snapshot"`
	After      interface{} `json:"after_snapshot"`
	IPAddress  *string     `json:"ip_address"`
	OccurredAt string      `json:"occurred_at"`
}

func (h *Handler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "50"))
	if page < 1 { page = 1 }
	if perPage < 1 || perPage > 100 { perPage = 50 }

	var total int
	_ = h.db.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM audit_log`).Scan(&total)

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, actor_id, action, entity_type, entity_id, before_snapshot, after_snapshot, ip_address, occurred_at
		FROM audit_log ORDER BY occurred_at DESC LIMIT $1 OFFSET $2
	`, perPage, (page-1)*perPage)
	if err != nil { response.InternalError(c); return }
	defer rows.Close()

	entries := []auditEntry{}
	for rows.Next() {
		var e auditEntry
		var before, after []byte
		var occurredAt time.Time
		if rows.Scan(&e.ID, &e.ActorID, &e.Action, &e.EntityType, &e.EntityID, &before, &after, &e.IPAddress, &occurredAt) == nil {
			e.Before = jsonOrNil(before)
			e.After = jsonOrNil(after)
			e.OccurredAt = occurredAt.UTC().Format(time.RFC3339)
			entries = append(entries, e)
		}
	}
	response.OKPaginated(c, entries, response.Meta{Page: page, PerPage: perPage, Total: total})
}

func (h *Handler) entityLog(c *gin.Context) {
	entityType := c.Param("entity")
	entityID := c.Param("id")

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, actor_id, action, entity_type, entity_id, before_snapshot, after_snapshot, ip_address, occurred_at
		FROM audit_log WHERE entity_type = $1 AND entity_id = $2
		ORDER BY occurred_at ASC
	`, entityType, entityID)
	if err != nil { response.InternalError(c); return }
	defer rows.Close()

	entries := []auditEntry{}
	for rows.Next() {
		var e auditEntry
		var before, after []byte
		var occurredAt time.Time
		if rows.Scan(&e.ID, &e.ActorID, &e.Action, &e.EntityType, &e.EntityID, &before, &after, &e.IPAddress, &occurredAt) == nil {
			e.Before = jsonOrNil(before)
			e.After = jsonOrNil(after)
			e.OccurredAt = occurredAt.UTC().Format(time.RFC3339)
			entries = append(entries, e)
		}
	}
	response.OK(c, entries)
}

func jsonOrNil(b []byte) interface{} {
	if len(b) == 0 { return nil }
	return string(b)
}
