package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"fitcommerce/backend/internal/auth"
)

const (
	KeyUserID    = "user_id"
	KeyEmail     = "email"
	KeyRole      = "role"
	KeyRequestID = "request_id"
)

// RequestID injects a unique request ID into every request.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		c.Set(KeyRequestID, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

// Logger logs structured request info via zerolog.
func Logger(log *zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		log.Info().
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", c.Writer.Status()).
			Dur("latency", time.Since(start)).
			Str("request_id", c.GetString(KeyRequestID)).
			Str("ip", c.ClientIP()).
			Msg("request")
	}
}

// Auth validates the Bearer JWT and sets user context keys.
func Auth(jwtManager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "UNAUTHORIZED", "message": "missing bearer token"},
			})
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := jwtManager.Validate(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "INVALID_TOKEN", "message": "invalid or expired token"},
			})
			return
		}
		c.Set(KeyUserID, claims.UserID)
		c.Set(KeyEmail, claims.Email)
		c.Set(KeyRole, claims.Role)
		c.Next()
	}
}

// RequireRoles enforces RBAC — only listed roles may proceed.
func RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		role := c.GetString(KeyRole)
		if !allowed[role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{"code": "FORBIDDEN", "message": "insufficient permissions"},
			})
			return
		}
		c.Next()
	}
}

// SecureHeaders adds standard security headers.
func SecureHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

// RateLimit implements a simple per-IP token bucket rate limiter.
func RateLimit(rps int) gin.HandlerFunc {
	type bucket struct {
		tokens  int
		lastAdd time.Time
	}
	var mu sync.Mutex
	buckets := map[string]*bucket{}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		b, ok := buckets[ip]
		if !ok {
			b = &bucket{tokens: rps, lastAdd: time.Now()}
			buckets[ip] = b
		}
		// Refill tokens
		elapsed := time.Since(b.lastAdd)
		refill := int(elapsed.Seconds()) * rps
		if refill > 0 {
			b.tokens += refill
			if b.tokens > rps*10 {
				b.tokens = rps * 10
			}
			b.lastAdd = time.Now()
		}
		if b.tokens <= 0 {
			mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{"code": "RATE_LIMITED", "message": "too many requests"},
			})
			return
		}
		b.tokens--
		mu.Unlock()
		c.Next()
	}
}

// CORS sets cross-origin headers for listed origins.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	origins := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		origins[o] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origins[origin] || origins["*"] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-ID")
			c.Header("Access-Control-Expose-Headers", "X-Request-ID,Content-Disposition")
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
