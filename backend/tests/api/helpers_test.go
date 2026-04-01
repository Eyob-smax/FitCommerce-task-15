package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"fitcommerce/backend/internal/auth"
	"fitcommerce/backend/internal/config"
	appRouter "fitcommerce/backend/internal/http/router"
)

// testEnv bundles the dependencies a test needs to drive the API.
type testEnv struct {
	Router *gin.Engine
	DB     *pgxpool.Pool
	JWT    *auth.Manager
}

// setupRouter builds a real Gin engine backed by the test database.
// When DATABASE_URL env var is set (CI / Docker) failures are hard errors.
// When it is absent (local dev without test DB) integration tests are skipped.
func setupRouter(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := testConfig()
	inCI := os.Getenv("DATABASE_URL") != ""

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(cfg.DB.URL)
	if err != nil {
		if inCI {
			t.Fatalf("cannot parse DB DSN: %v", err)
		}
		t.Skipf("cannot parse DB DSN (skipping integration test): %v", err)
	}
	poolCfg.MaxConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		if inCI {
			t.Fatalf("cannot connect to test DB: %v", err)
		}
		t.Skipf("cannot connect to test DB (skipping integration test): %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		if inCI {
			t.Fatalf("cannot ping test DB: %v", err)
		}
		t.Skipf("cannot ping test DB (skipping integration test): %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	rdbOpts, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		rdbOpts = &redis.Options{Addr: "localhost:6379"}
	}
	rdb := redis.NewClient(rdbOpts)
	t.Cleanup(func() { rdb.Close() })

	jwtMgr := auth.NewManager(&cfg.JWT)
	log := zerolog.Nop()

	r := appRouter.New(cfg, pool, rdb, jwtMgr, &log)

	return &testEnv{Router: r, DB: pool, JWT: jwtMgr}
}

func testConfig() *config.Config {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://fitcommerce:fitcommerce@localhost:5432/fitcommerce_test?sslmode=disable"
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}
	jwtCfg := auth.TestJWTConfig()
	return &config.Config{
		Env:       "test",
		Port:      "0",
		ExportDir: "/tmp/exports",
		DB:        config.DBConfig{URL: dbURL},
		Redis:     config.RedisConfig{URL: redisURL},
		JWT:       jwtCfg,
		CORS:      config.CORSConfig{AllowedOrigins: []string{"*"}},
	}
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func doPost(router *gin.Engine, path string, body interface{}, token ...string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if len(token) > 0 && token[0] != "" {
		req.Header.Set("Authorization", "Bearer "+token[0])
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func doGet(router *gin.Engine, path string, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func parseBody(w *httptest.ResponseRecorder) map[string]interface{} {
	var result map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	return result
}

// redisAddrFromURL extracts host:port from a redis:// URL. Used only in tests
// that need a raw redis.Options.Addr without the go-redis URL parser.
func redisAddrFromURL(u string) string {
	u = strings.TrimPrefix(u, "redis://")
	if idx := strings.Index(u, "/"); idx >= 0 {
		u = u[:idx]
	}
	return u
}
