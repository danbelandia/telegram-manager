package auth

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/telegram-manager/backend/internal/database"
)

// testDB abre PostgreSQL real y limpia la tabla admins. Convencion del
// proyecto: no mockear la capa DB (backend-go-skill §8). Se saltea en
// -short o sin base disponible.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("integration: skipping with -short")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://telegram:telegram@localhost:5432/telegram_manager?sslmode=disable"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("integration: no postgres available: %v", err)
	}
	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	// Los tests comparten la base: cada test arranca sin admins.
	if _, err := db.ExecContext(context.Background(), "TRUNCATE admins"); err != nil {
		t.Fatalf("truncate admins: %v", err)
	}
	return db
}

func seedAdmin(t *testing.T, db *sql.DB, username, password string) Admin {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	repo := NewRepository(db)
	id, err := repo.Create(context.Background(), username, string(hash))
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

	if err := EnsureInitialAdmin(context.Background(), repo, "admin", "secret123"); err != nil {
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
}

func TestEnsureInitialAdmin_DoesNotDuplicate(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	seedAdmin(t, db, "admin", "secret123")

	if err := EnsureInitialAdmin(context.Background(), repo, "admin", "otro-password"); err != nil {
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
