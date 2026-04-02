package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	// migrate package (package name: db)
	appdb "fitcommerce/backend/database"
	dbseeds "fitcommerce/backend/database/seeds"

	"fitcommerce/backend/internal/auth"
	"fitcommerce/backend/internal/config"
	"fitcommerce/backend/internal/database"
	approuter "fitcommerce/backend/internal/http/router"
)

func main() {
	// ── Logger ───────────────────────────────────────────────────
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	// ── Config ───────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}
	log.Info().Str("env", cfg.Env).Str("port", cfg.Port).Msg("starting fitcommerce api")

	// ── Migrations ───────────────────────────────────────────────
	log.Info().Msg("running database migrations")
	if err := appdb.RunMigrations(cfg.DB.URL); err != nil {
		log.Fatal().Err(err).Msg("migrations failed")
	}
	log.Info().Msg("migrations complete")

	// ── Database pool ────────────────────────────────────────────
	pool, err := database.Connect(cfg.DB.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer pool.Close()

	// ── Redis ────────────────────────────────────────────────────
	redisOpts, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid redis url")
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("redis connection failed")
	}

	// ── Seed ─────────────────────────────────────────────────────
	log.Info().Msg("running seed (idempotent)")
	if err := dbseeds.Run(ctx, pool); err != nil {
		log.Fatal().Err(err).Msg("seed failed")
	}
	log.Info().Msg("seed complete")

	// ── JWT manager ──────────────────────────────────────────────
	jwtMgr := auth.NewManager(&cfg.JWT)

	// ── Router ───────────────────────────────────────────────────
	r := approuter.New(cfg, pool, rdb, jwtMgr, &log.Logger)

	// ── HTTP server with graceful shutdown ────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("http server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down gracefully")
	shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}
	log.Info().Msg("server stopped")
}
