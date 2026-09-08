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
	if _, err := db.ExecContext(context.Background(), "TRUNCATE users, join_requests, warnings"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

// testTenant devuelve el id del tenant `default` (idempotente, slice
// 0): GetByTenant requiere presencia en tablas hijas del tenant.
func testTenant(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var id int64
	const q = `
INSERT INTO tenants (slug) VALUES ('default')
ON CONFLICT (slug) DO UPDATE SET slug = EXCLUDED.slug
RETURNING id`
	if err := db.QueryRowContext(context.Background(), q).Scan(&id); err != nil {
		t.Fatalf("ensure default tenant: %v", err)
	}
	return id
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

// --- Scope por tenant via join (slice 0, T9/Q3-a) ---

func TestRepository_GetByTenantViaJoinRequest(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	tid := testTenant(t, db)

	u := &User{TelegramID: 2000000001, FirstName: "Ana", Username: ptr("ana")}
	if err := repo.UpsertByTelegramID(ctx, u); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	// Sin presencia en el tenant: NOT_FOUND aunque el usuario exista.
	if _, err := repo.GetByTenant(ctx, tid, u.TelegramID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sin presencia err = %v, want ErrNotFound", err)
	}
	// Presencia via join_request del tenant.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO join_requests (tenant_id, group_id, user_id) VALUES ($1, $2, $3)`,
		tid, -1001, u.TelegramID,
	); err != nil {
		t.Fatalf("insert join request: %v", err)
	}
	got, err := repo.GetByTenant(ctx, tid, u.TelegramID)
	if err != nil {
		t.Fatalf("get by tenant: %v", err)
	}
	if got.FirstName != "Ana" {
		t.Errorf("first_name = %q, want Ana", got.FirstName)
	}
}

func TestRepository_GetByTenantOtherTenantNotFound(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	tid := testTenant(t, db)
	var tidB int64
	const q = `
INSERT INTO tenants (slug) VALUES ('users-other')
ON CONFLICT (slug) DO UPDATE SET slug = EXCLUDED.slug
RETURNING id`
	if err := db.QueryRowContext(ctx, q).Scan(&tidB); err != nil {
		t.Fatalf("ensure tenant: %v", err)
	}

	u := &User{TelegramID: 2000000002, FirstName: "Beto"}
	if err := repo.UpsertByTelegramID(ctx, u); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	// Presencia SOLO en el otro tenant.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO join_requests (tenant_id, group_id, user_id) VALUES ($1, $2, $3)`,
		tidB, -1002, u.TelegramID,
	); err != nil {
		t.Fatalf("insert join request: %v", err)
	}
	// Usuario solo de otro tenant → NOT_FOUND.
	if _, err := repo.GetByTenant(ctx, tid, u.TelegramID); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	// Y visible desde su tenant.
	if _, err := repo.GetByTenant(ctx, tidB, u.TelegramID); err != nil {
		t.Errorf("get by tenant B: %v", err)
	}
}

func TestRepository_GetByTenantViaWarning(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	tid := testTenant(t, db)

	u := &User{TelegramID: 2000000003, FirstName: "Ceci"}
	if err := repo.UpsertByTelegramID(ctx, u); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	// Presencia via warnings del tenant (sin join_request).
	if _, err := db.ExecContext(ctx,
		`INSERT INTO warnings (tenant_id, group_id, user_id) VALUES ($1, $2, $3)`,
		tid, -1003, u.TelegramID,
	); err != nil {
		t.Fatalf("insert warning: %v", err)
	}
	if _, err := repo.GetByTenant(ctx, tid, u.TelegramID); err != nil {
		t.Errorf("get by tenant via warning: %v", err)
	}
}
