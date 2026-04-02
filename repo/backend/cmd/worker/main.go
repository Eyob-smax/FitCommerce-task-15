package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"fitcommerce/backend/internal/config"
	"fitcommerce/backend/internal/database"
	"fitcommerce/backend/internal/modules/exports"
	"fitcommerce/backend/internal/modules/groupbuys"
)

func main() {
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Str("service", "worker").Logger()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}
	log.Info().Str("env", cfg.Env).Msg("starting fitcommerce worker")

	pool, err := database.Connect(cfg.DB.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("database connection failed")
	}
	defer pool.Close()

	redisOpts, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid redis url")
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()

	ctx, cancel := context.WithCancel(context.Background())

	go runGroupBuyCutoffLoop(ctx, pool)
	go runExportJobLoop(ctx, pool, rdb, cfg.ExportDir)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down worker")
	cancel()
	time.Sleep(2 * time.Second)
	log.Info().Msg("worker stopped")
}

// runGroupBuyCutoffLoop polls for active group-buys past their cutoff and evaluates them.
func runGroupBuyCutoffLoop(ctx context.Context, pool *pgxpool.Pool) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Run once immediately on startup
	evaluateCutoffs(ctx, pool)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			evaluateCutoffs(ctx, pool)
		}
	}
}

func evaluateCutoffs(ctx context.Context, pool *pgxpool.Pool) {
	processed, err := groupbuys.EvaluateCutoffs(ctx, pool)
	if err != nil {
		log.Error().Err(err).Msg("group-buy cutoff evaluation failed")
		return
	}
	if processed > 0 {
		log.Info().Int("processed", processed).Msg("group-buy cutoffs evaluated")
	}
}

// runExportJobLoop drains the export job queue.
func runExportJobLoop(ctx context.Context, pool *pgxpool.Pool, rdb *redis.Client, exportDir string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processed, err := exports.ProcessQueuedExports(ctx, pool, rdb, exportDir)
			if err != nil {
				log.Error().Err(err).Msg("export job processing failed")
			}
			if processed > 0 {
				log.Info().Int("processed", processed).Msg("export jobs completed")
			}
		}
	}
}
