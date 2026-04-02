package roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

var availableRoles = []gin.H{
	{"name": "administrator", "label": "Administrator"},
	{"name": "operations_manager", "label": "Operations Manager"},
	{"name": "procurement_specialist", "label": "Procurement Specialist"},
	{"name": "coach", "label": "Coach"},
	{"name": "member", "label": "Member"},
}

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	r.GET("/roles", h.list)
}

func (h *Handler) list(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": availableRoles})
}
