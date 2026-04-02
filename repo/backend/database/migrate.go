package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"

	// pgx stdlib driver for goose
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var Migrations embed.FS

func RunMigrations(dsn string) error {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db for migrations: %w", err)
	}
	defer sqlDB.Close()

	goose.SetBaseFS(Migrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
