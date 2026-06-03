// Package db provides the Postgres-backed implementation of store.Store plus
// connection bootstrap and goose migrations. The persistence layer is isolated
// here; disabling it (store.Noop) leaves the MCP core fully functional.
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib" // pgx database/sql driver
	"github.com/pressly/goose/v3"

	"github.com/tergrigoryantc/asc-mcp/migrations"
)

// Open connects to Postgres via the pgx stdlib driver.
func Open(ctx context.Context, dsn string) (*sqlx.DB, error) {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return sqlx.NewDb(sqlDB, "pgx"), nil
}

// Migrate applies all embedded goose migrations.
func Migrate(db *sqlx.DB) error {
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("db: set dialect: %w", err)
	}
	if err := goose.Up(db.DB, "."); err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}
	return nil
}
