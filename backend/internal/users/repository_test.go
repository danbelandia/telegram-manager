package users

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/telegram-manager/backend/internal/database"
)

// testDB abre PostgreSQL real (convencion: no mockear la capa DB) con
// migraciones aplicadas. Base propia por paquete ("users") para que
// `go test ./...` corra suites en paralelo sin pisarse (testdb.go); el
// task 2.8 decia suite "rest", pero la convencion del proyecto es una
// base por paquete de test.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("integration: skipping with -short")
	}

	db, err := database.OpenTestDB(t, "users")
	if err != nil {
		t.Skipf("integration: no postgres available: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "TRUNCATE users"); err != nil {
		t.Fatalf("truncate users: %v", err)
	}
	return db
}

func TestRepository_UpsertIdempotent(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	u := &User{TelegramID: 1000000001, FirstName: "Juan", Username: ptr("juan")}

	if err := repo.UpsertByTelegramID(ctx, u); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	u.FirstName = "Juan Pablo"
	if err := repo.UpsertByTelegramID(ctx, u); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	const countQ = `SELECT count(*) FROM users WHERE telegram_id = $1`
	var n int
	if err := db.QueryRowContext(ctx, countQ, u.TelegramID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("rows for telegram_id %d = %d, want 1", u.TelegramID, n)
	}

	got, err := repo.GetByTelegramID(ctx, u.TelegramID)
	if err != nil {
		t.Fatalf("get after upsert: %v", err)
	}
	if got.FirstName != "Juan Pablo" {
		t.Errorf("first_name = %q, want Juan Pablo", got.FirstName)
	}
	if got.Username == nil || *got.Username != "juan" {
		t.Errorf("username = %v, want juan", got.Username)
	}
}

func TestRepository_GetNotFound(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	_, err := repo.GetByTelegramID(context.Background(), 999999999999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestRepository_UpsertNullUsername(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	u := &User{TelegramID: 1000000002, FirstName: "Sin User"}
	if err := repo.UpsertByTelegramID(ctx, u); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := repo.GetByTelegramID(ctx, u.TelegramID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Username != nil {
		t.Errorf("username = %v, want nil", got.Username)
	}
}

func ptr(s string) *string { return &s }
