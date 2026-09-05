package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	"github.com/telegram-manager/backend/migrations"
)

// Migrate aplica las migraciones pendientes con goose. El SQL plano
// vive en backend/migrations/ y se embebe en el binario via embed.FS.
func Migrate(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("database: set dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "."); err != nil {
		return fmt.Errorf("database: migrate up: %w", err)
	}
	return nil
}
