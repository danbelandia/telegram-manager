package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/telegram-manager/backend/internal/database"
	"github.com/telegram-manager/backend/internal/tenants"
)

// tenantsRepo expone *tenants.Repository como tenantEnsurer del seed.
// Requiere la migracion 00009 aplicada (testDB migra todo).
func tenantsRepo(t *testing.T, db *sql.DB) *tenants.Repository {
	t.Helper()
	return tenants.NewRepository(db)
}

// ensureDefaultTenant crea el tenant `default` si falta (idempotente).
func ensureDefaultTenant(t *testing.T, db *sql.DB) (int64, error) {
	t.Helper()
	return tenants.NewRepository(db).EnsureDefault(context.Background())
}

// testDB abre una PostgreSQL real (convencion del proyecto: no mockear
// la capa DB) y aplica las migraciones. Se saltea en -short o si no
// hay base disponible: son tests de integracion. Cada suite usa su
// propia base (telegram_manager_auth) para no pisarse con otros
// paquetes en `go test ./...`.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("integration: skipping with -short")
	}

	db, err := database.OpenTestDB(t, "auth")
	if err != nil {
		t.Skipf("integration: no postgres available: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	// Los tests comparten la base: cada test arranca sin admins ni
	// tenants. Los tests de signup usan slugs fijos; sin truncar
	// tenants, un re-run colisionaria con la corrida anterior (409
	// fantasma). CASCADE por admins.tenant_id → tenants.
	if _, err := db.ExecContext(context.Background(), "TRUNCATE admins, tenants CASCADE"); err != nil {
		t.Fatalf("truncate admins, tenants: %v", err)
	}
	return db
}

func seedAdmin(t *testing.T, db *sql.DB, username, password string) Admin {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	tenantID, err := ensureDefaultTenant(t, db)
	if err != nil {
		t.Fatalf("ensure default tenant: %v", err)
	}
	repo := NewRepository(db)
	id, err := repo.Create(context.Background(), username, string(hash), tenantID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	admin, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	return admin
}

func TestRepository_GetByUsername_Found(t *testing.T) {
	db := testDB(t)
	seedAdmin(t, db, "admin", "secret123")
	repo := NewRepository(db)

	got, err := repo.GetByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("get by username: %v", err)
	}
	if got.Username != "admin" {
		t.Errorf("username = %q, want admin", got.Username)
	}
}

func TestRepository_GetByUsername_NotFound(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	if _, err := repo.GetByUsername(context.Background(), "nadie"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestRepository_GetByID(t *testing.T) {
	db := testDB(t)
	admin := seedAdmin(t, db, "admin", "secret123")
	repo := NewRepository(db)

	got, err := repo.GetByID(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Username != "admin" || got.ID != admin.ID {
		t.Errorf("got %#v, want admin id=%d", got, admin.ID)
	}
}

func TestRepository_UpdateLastLogin(t *testing.T) {
	db := testDB(t)
	admin := seedAdmin(t, db, "admin", "secret123")
	repo := NewRepository(db)

	if admin.LastLoginAt != nil {
		t.Fatalf("precondition: last_login should be nil, got %v", admin.LastLoginAt)
	}

	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	if err := repo.UpdateLastLogin(context.Background(), admin.ID, at); err != nil {
		t.Fatalf("update last login: %v", err)
	}

	got, err := repo.GetByID(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.LastLoginAt == nil || !got.LastLoginAt.Equal(at) {
		t.Errorf("last_login_at = %v, want %v", got.LastLoginAt, at)
	}
}

func TestEnsureInitialAdmin_EmptyTableCreates(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	if err := EnsureInitialAdmin(context.Background(), repo, tenantsRepo(t, db), "admin", "secret123"); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	// Debe existir y autenticar (hash bcrypt válido).
	got, err := repo.GetByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("get after bootstrap: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("secret123")); err != nil {
		t.Errorf("password no valida tras bootstrap: %v", err)
	}
	if got.TenantID == 0 {
		t.Error("admin bootstrap sin tenant: TenantID == 0, want tenant default")
	}
}

func TestEnsureInitialAdmin_DoesNotDuplicate(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	seedAdmin(t, db, "admin", "secret123")

	if err := EnsureInitialAdmin(context.Background(), repo, tenantsRepo(t, db), "admin", "otro-password"); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	got, err := repo.GetByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	// La password original sigue valida: no fue pisada.
	if err := bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("secret123")); err != nil {
		t.Errorf("password original pisada por bootstrap: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("otro-password")); err == nil {
		t.Error("password nueva aplicada sin querer")
	}
}
