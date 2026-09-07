package joinrequests

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/telegram-manager/backend/internal/database"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("integration: skipping with -short")
	}

	db, err := database.OpenTestDB(t, "joinrequests")
	if err != nil {
		t.Skipf("integration: no postgres available: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "TRUNCATE join_requests"); err != nil {
		t.Fatalf("truncate join_requests: %v", err)
	}
	return db
}

func TestRepository_UpsertPendingIdempotent(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	// El indice parcial (group_id, user_id) WHERE status='pending'
	// absorbe el segundo upsert: sigue habiendo una sola fila.
	if err := repo.UpsertPending(ctx, -1001, 42); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := repo.UpsertPending(ctx, -1001, 42); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := repo.ListByGroup(ctx, -1001)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("requests = %d, want 1 (sin duplicar)", len(got))
	}
	if got[0].Status != StatusPending || got[0].UserID != 42 {
		t.Errorf("request = %+v, want pending de user 42", got[0])
	}
}

func TestRepository_UpsertAfterResolveCreatesNewRow(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	admin := int64(7)

	if err := repo.UpsertPending(ctx, -1001, 42); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	first, err := repo.ListByGroup(ctx, -1001)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := repo.Resolve(ctx, first[0].ID, StatusRejected, &admin); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// Nueva solicitud del mismo usuario: indice parcial no bloquea.
	if err := repo.UpsertPending(ctx, -1001, 42); err != nil {
		t.Fatalf("upsert tras resolver: %v", err)
	}

	got, err := repo.ListByGroup(ctx, -1001)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("requests = %d, want 2 (rejected + nueva pending)", len(got))
	}
}

func TestRepository_Resolve(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	admin := int64(3)

	if err := repo.UpsertPending(ctx, -1001, 42); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	req, err := repo.ListByGroup(ctx, -1001)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if err := repo.Resolve(ctx, req[0].ID, StatusApproved, &admin); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	got, err := repo.GetByID(ctx, req[0].ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != StatusApproved {
		t.Errorf("status = %s, want approved", got.Status)
	}
	if got.DecidedBy == nil || *got.DecidedBy != admin {
		t.Errorf("decided_by = %v, want 3", got.DecidedBy)
	}
	if got.DecidedAt == nil {
		t.Error("decided_at nil, want fecha")
	}
}

func TestRepository_ResolveNotPending(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	admin := int64(3)

	if err := repo.UpsertPending(ctx, -1001, 42); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	req, _ := repo.ListByGroup(ctx, -1001)
	if err := repo.Resolve(ctx, req[0].ID, StatusApproved, &admin); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	// Segunda resolucion: ya no esta pendiente.
	if err := repo.Resolve(ctx, req[0].ID, StatusRejected, &admin); !errors.Is(err, ErrNotPending) {
		t.Errorf("second resolve error = %v, want ErrNotPending", err)
	}
}

func TestRepository_GetNotFound(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	_, err := repo.GetByID(context.Background(), 999999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestRepository_ListFiltersByGroup(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	if err := repo.UpsertPending(ctx, -1001, 42); err != nil {
		t.Fatalf("upsert group 1: %v", err)
	}
	if err := repo.UpsertPending(ctx, -1002, 43); err != nil {
		t.Fatalf("upsert group 2: %v", err)
	}

	got, err := repo.ListByGroup(ctx, -1001)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].GroupID != -1001 {
		t.Errorf("requests del grupo -1001 = %+v, want solo la de ese grupo", got)
	}
}
