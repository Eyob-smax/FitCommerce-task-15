package users

import (
	"crypto/sha256"
	"encoding/hex"
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
	db     *pgxpool.Pool
	jwtMgr *auth.Manager
}

func NewHandler(db *pgxpool.Pool, jwtMgr *auth.Manager) *Handler {
	return &Handler{db: db, jwtMgr: jwtMgr}
}

// RegisterRoutes registers protected user-management routes.
// Auth endpoints (login/refresh/logout) are registered separately in the router.
func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/users")
	// /me is accessible to any authenticated user
	g.GET("/me", h.me)

	// All user management routes are administrator-only
	admin := g.Group("", middleware.RequireRoles(auth.RoleAdministrator))
	admin.GET("", h.list)
	admin.POST("", h.create)
	admin.GET("/:id", h.get)
	admin.PATCH("/:id", h.update)
	admin.DELETE("/:id", h.delete)
}

// ── Request / response types ──────────────────────────────────────────────────

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type createUserRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	Role      string `json:"role" binding:"required"`
}

type updateUserRequest struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Role      *string `json:"role"`
	IsActive  *bool   `json:"is_active"`
}

type userResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func validRole(role string) bool {
	switch role {
	case auth.RoleAdministrator, auth.RoleOperationsManager,
		auth.RoleProcurementSpecialist, auth.RoleCoach, auth.RoleMember:
		return true
	}
	return false
}

// ── Public endpoints ──────────────────────────────────────────────────────────

// Login authenticates a user and returns a token pair.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	// Fetch user by email.
	var (
		id           string
		passwordHash string
		firstName    string
		lastName     string
		role         string
		isActive     bool
		createdAt    time.Time
	)
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, password_hash, first_name, last_name, role, is_active, created_at
		FROM users
		WHERE email = $1
	`, req.Email).Scan(&id, &passwordHash, &firstName, &lastName, &role, &isActive, &createdAt)

	if err == pgx.ErrNoRows {
		response.BadRequest(c, "INVALID_CREDENTIALS", "invalid email or password")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}
	if !auth.CheckPassword(req.Password, passwordHash) {
		response.BadRequest(c, "INVALID_CREDENTIALS", "invalid email or password")
		return
	}
	if !isActive {
		response.Forbidden(c)
		return
	}

	// Issue token pair.
	pair, err := h.jwtMgr.Issue(id, req.Email, role)
	if err != nil {
		response.InternalError(c)
		return
	}

	// Persist SHA-256 hash of refresh token.
	expiresAt := time.Now().Add(h.jwtMgr.GetRefreshTTL())
	_, err = h.db.Exec(c.Request.Context(), `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, id, hashToken(pair.RefreshToken), expiresAt)
	if err != nil {
		response.InternalError(c)
		return
	}

	// Fire-and-forget audit event.
	actorID := id
	audit.Log(c.Request.Context(), h.db, &actorID, "user.login", "user", id, nil, nil, c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"access_token":  pair.AccessToken,
			"refresh_token": pair.RefreshToken,
			"expires_in":    pair.ExpiresIn,
			"user": userResponse{
				ID:        id,
				Email:     req.Email,
				FirstName: firstName,
				LastName:  lastName,
				Role:      role,
				IsActive:  isActive,
				CreatedAt: createdAt.UTC().Format(time.RFC3339),
			},
		},
	})
}

// RefreshToken rotates the refresh token.
func (h *Handler) RefreshToken(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	// Validate the JWT signature / expiry.
	claims, err := h.jwtMgr.ValidateRefresh(req.RefreshToken)
	if err != nil {
		response.Unauthorized(c)
		return
	}

	// Look up the token hash in DB — must exist, not revoked, not expired.
	tokenHash := hashToken(req.RefreshToken)
	var tokenID string
	err = h.db.QueryRow(c.Request.Context(), `
		SELECT id FROM refresh_tokens
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
	`, tokenHash).Scan(&tokenID)
	if err == pgx.ErrNoRows {
		response.Unauthorized(c)
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}

	// Revoke the old token (rotation).
	_, err = h.db.Exec(c.Request.Context(), `
		UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1
	`, tokenID)
	if err != nil {
		response.InternalError(c)
		return
	}

	// Fetch user details to embed in new access token.
	var email, role string
	var isActive bool
	err = h.db.QueryRow(c.Request.Context(), `
		SELECT email, role, is_active FROM users WHERE id = $1
	`, claims.UserID).Scan(&email, &role, &isActive)
	if err != nil || !isActive {
		response.Unauthorized(c)
		return
	}

	// Issue new pair.
	pair, err := h.jwtMgr.Issue(claims.UserID, email, role)
	if err != nil {
		response.InternalError(c)
		return
	}

	// Persist new refresh token hash.
	expiresAt := time.Now().Add(h.jwtMgr.GetRefreshTTL())
	_, err = h.db.Exec(c.Request.Context(), `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, claims.UserID, hashToken(pair.RefreshToken), expiresAt)
	if err != nil {
		response.InternalError(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"access_token":  pair.AccessToken,
			"refresh_token": pair.RefreshToken,
			"expires_in":    pair.ExpiresIn,
		},
	})
}

// Logout revokes the refresh token. (protected — requires valid access token)
func (h *Handler) Logout(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}

	userID := c.GetString(middleware.KeyUserID)
	tokenHash := hashToken(req.RefreshToken)

	_, _ = h.db.Exec(c.Request.Context(), `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE token_hash = $1 AND user_id = $2 AND revoked_at IS NULL
	`, tokenHash, userID)

	actorID := userID
	audit.Log(c.Request.Context(), h.db, &actorID, "user.logout", "user", userID, nil, nil, c.ClientIP())

	response.NoContent(c)
}

// ── Protected endpoints ───────────────────────────────────────────────────────

func (h *Handler) me(c *gin.Context) {
	userID := c.GetString(middleware.KeyUserID)

	var u userResponse
	var createdAt time.Time
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, email, first_name, last_name, role, is_active, created_at
		FROM users WHERE id = $1
	`, userID).Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.IsActive, &createdAt)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "user")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}
	u.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	response.OK(c, u)
}

// ── Admin-only user management ────────────────────────────────────────────────

func (h *Handler) list(c *gin.Context) {
	page := 1
	perPage := 25
	if p := c.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	if pp := c.Query("per_page"); pp != "" {
		if n, err := strconv.Atoi(pp); err == nil && n > 0 && n <= 100 {
			perPage = n
		}
	}

	var total int
	_ = h.db.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM users`).Scan(&total)

	rows, err := h.db.Query(c.Request.Context(), `
		SELECT id, email, first_name, last_name, role, is_active, created_at
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, perPage, (page-1)*perPage)
	if err != nil {
		response.InternalError(c)
		return
	}
	defer rows.Close()

	users := []userResponse{}
	for rows.Next() {
		var u userResponse
		var createdAt time.Time
		if rows.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.IsActive, &createdAt) == nil {
			u.CreatedAt = createdAt.UTC().Format(time.RFC3339)
			users = append(users, u)
		}
	}
	response.OKPaginated(c, users, response.Meta{Page: page, PerPage: perPage, Total: total})
}

func (h *Handler) create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	if !validRole(req.Role) {
		response.BadRequest(c, "VALIDATION_ERROR", "invalid role")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		response.InternalError(c)
		return
	}

	var u userResponse
	var createdAt time.Time
	err = h.db.QueryRow(c.Request.Context(), `
		INSERT INTO users (email, password_hash, first_name, last_name, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, first_name, last_name, role, is_active, created_at
	`, req.Email, hash, req.FirstName, req.LastName, req.Role,
	).Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.IsActive, &createdAt)
	if err != nil {
		// Check for duplicate email (unique constraint violation)
		if isDuplicateKey(err) {
			response.Conflict(c, "email already in use")
			return
		}
		response.InternalError(c)
		return
	}
	u.CreatedAt = createdAt.UTC().Format(time.RFC3339)

	actorID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &actorID, "user.create", "user", u.ID, nil, req, c.ClientIP())

	response.Created(c, u)
}

func (h *Handler) get(c *gin.Context) {
	id := c.Param("id")
	var u userResponse
	var createdAt time.Time
	err := h.db.QueryRow(c.Request.Context(), `
		SELECT id, email, first_name, last_name, role, is_active, created_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.IsActive, &createdAt)
	if err == pgx.ErrNoRows {
		response.NotFound(c, "user")
		return
	}
	if err != nil {
		response.InternalError(c)
		return
	}
	u.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	response.OK(c, u)
}

func (h *Handler) update(c *gin.Context) {
	id := c.Param("id")
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "VALIDATION_ERROR", err.Error())
		return
	}
	if req.Role != nil && !validRole(*req.Role) {
		response.BadRequest(c, "VALIDATION_ERROR", "invalid role")
		return
	}

	// Verify user exists
	var exists bool
	_ = h.db.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, id).Scan(&exists)
	if !exists {
		response.NotFound(c, "user")
		return
	}

	_, err := h.db.Exec(c.Request.Context(), `
		UPDATE users SET
			first_name = COALESCE($1, first_name),
			last_name  = COALESCE($2, last_name),
			role       = COALESCE($3, role),
			is_active  = COALESCE($4, is_active),
			updated_at = NOW(), version = version + 1
		WHERE id = $5
	`, req.FirstName, req.LastName, req.Role, req.IsActive, id)
	if err != nil {
		response.InternalError(c)
		return
	}

	var u userResponse
	var createdAt time.Time
	_ = h.db.QueryRow(c.Request.Context(), `
		SELECT id, email, first_name, last_name, role, is_active, created_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.Role, &u.IsActive, &createdAt)
	u.CreatedAt = createdAt.UTC().Format(time.RFC3339)

	actorID := c.GetString(middleware.KeyUserID)
	audit.Log(c.Request.Context(), h.db, &actorID, "user.update", "user", id, nil, req, c.ClientIP())

	response.OK(c, u)
}

func (h *Handler) delete(c *gin.Context) {
	id := c.Param("id")

	// Prevent self-deletion
	actorID := c.GetString(middleware.KeyUserID)
	if actorID == id {
		response.Conflict(c, "cannot deactivate your own account")
		return
	}

	tag, err := h.db.Exec(c.Request.Context(), `
		UPDATE users SET is_active = false, updated_at = NOW() WHERE id = $1 AND is_active = true
	`, id)
	if err != nil {
		response.InternalError(c)
		return
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		_ = h.db.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, id).Scan(&exists)
		if !exists {
			response.NotFound(c, "user")
			return
		}
		// Already inactive — idempotent
	}

	audit.Log(c.Request.Context(), h.db, &actorID, "user.deactivate", "user", id, nil, nil, c.ClientIP())
	response.NoContent(c)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func isDuplicateKey(err error) bool {
	// PostgreSQL unique-violation SQLSTATE is 23505; pgx includes it in the error string.
	return err != nil && strings.Contains(err.Error(), "23505")
}
