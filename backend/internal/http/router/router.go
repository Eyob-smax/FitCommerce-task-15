package router

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"fitcommerce/backend/internal/auth"
	"fitcommerce/backend/internal/config"
	"fitcommerce/backend/internal/middleware"

	"fitcommerce/backend/internal/modules/audit"
	"fitcommerce/backend/internal/modules/classes"
	"fitcommerce/backend/internal/modules/exports"
	"fitcommerce/backend/internal/modules/groupbuys"
	"fitcommerce/backend/internal/modules/health"
	"fitcommerce/backend/internal/modules/inventory"
	"fitcommerce/backend/internal/modules/items"
	"fitcommerce/backend/internal/modules/members"
	"fitcommerce/backend/internal/modules/orders"
	"fitcommerce/backend/internal/modules/purchaseorders"
	"fitcommerce/backend/internal/modules/reports"
	"fitcommerce/backend/internal/modules/roles"
	"fitcommerce/backend/internal/modules/suppliers"
	syncsvc "fitcommerce/backend/internal/modules/sync"
	"fitcommerce/backend/internal/modules/users"
)

func New(
	cfg *config.Config,
	db *pgxpool.Pool,
	rdb *redis.Client,
	jwtMgr *auth.Manager,
	log *zerolog.Logger,
) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.SecureHeaders())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger(log))
	r.Use(middleware.CORS(cfg.CORS.AllowedOrigins))
	r.Use(middleware.RateLimit(100))

	// ── Health (no auth) ─────────────────────────────────────────
	healthH := health.NewHandler(db, rdb)
	r.GET("/health", healthH.Health)
	r.GET("/health/ready", healthH.Ready)
	r.GET("/health/info", healthH.Info)

	// ── Public auth endpoints ─────────────────────────────────────
	usersH := users.NewHandler(db, jwtMgr)
	r.POST("/api/v1/auth/login", usersH.Login)
	r.POST("/api/v1/auth/refresh", usersH.RefreshToken)

	// ── Protected API v1 ─────────────────────────────────────────
	v1 := r.Group("/api/v1")
	v1.Use(middleware.Auth(jwtMgr))

	// Auth (protected)
	v1.POST("/auth/logout", usersH.Logout)

	// Module routes
	usersH.RegisterRoutes(v1)
	roles.NewHandler(db).RegisterRoutes(v1)
	items.NewHandler(db).RegisterRoutes(v1)
	inventory.NewHandler(db).RegisterRoutes(v1)
	groupbuys.NewHandler(db).RegisterRoutes(v1)
	orders.NewHandler(db).RegisterRoutes(v1)
	reports.NewHandler(db).RegisterRoutes(v1)
	suppliers.NewHandler(db).RegisterRoutes(v1)
	purchaseorders.NewHandler(db).RegisterRoutes(v1)
	classes.NewHandler(db).RegisterRoutes(v1)
	members.NewHandler(db).RegisterRoutes(v1)
	exports.NewHandler(db, rdb, cfg.ExportDir).RegisterRoutes(v1)
	syncsvc.NewHandler(db, rdb).RegisterRoutes(v1)
	audit.NewHandler(db).RegisterRoutes(v1)

	return r
}
