package logs

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/telegram-manager/backend/internal/database"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("integration: skipping with -short")
	}

	db, err := database.OpenTestDB(t, "logs")
	if err != nil {
		t.Skipf("integration: no postgres available: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), "TRUNCATE logs"); err != nil {
		t.Fatalf("truncate logs: %v", err)
	}
	return db
}

func sampleEntry(groupID int64, action string, status Status) *Entry {
	actor := int64(1)
	return &Entry{
		ActorID: &actor,
		GroupID: groupID,
		Action:  action,
		Status:  status,
	}
}

func TestRepository_CreateAndGet(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	target := int64(456)
	entry := &Entry{
		ActorID:      ptr64(7),
		GroupID:      -100123,
		Action:       ActionBanUser,
		TargetUserID: &target,
		Metadata:     map[string]any{"hasta": "indefinido"},
		Status:       StatusSuccess,
	}
	if err := repo.Create(ctx, entry); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.ListByGroup(ctx, -100123)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("logs = %d, want 1", len(got))
	}
	g := got[0]
	if g.ActorID == nil || *g.ActorID != 7 {
		t.Errorf("actor_id = %v, want 7", g.ActorID)
	}
	if g.Action != ActionBanUser || g.Status != StatusSuccess {
		t.Errorf("action/status = %s/%s", g.Action, g.Status)
	}
	if g.TargetUserID == nil || *g.TargetUserID != 456 {
		t.Errorf("target_user_id = %v, want 456", g.TargetUserID)
	}
	if g.Metadata["hasta"] != "indefinido" {
		t.Errorf("metadata = %v, want hasta=indefinido", g.Metadata)
	}
	if g.CreatedAt.IsZero() {
		t.Error("created_at no poblado")
	}
}

func TestRepository_ListByGroupFiltersAndOrders(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	// Logs del grupo 1 (mas reciente al final en insercion) y uno del 2.
	for _, e := range []*Entry{
		sampleEntry(-1001, ActionLockGroup, StatusSuccess),
		sampleEntry(-1002, ActionPinMessage, StatusSuccess),
		sampleEntry(-1001, ActionUnlockGroup, StatusSuccess),
	} {
		if err := repo.Create(ctx, e); err != nil {
			t.Fatalf("create %s: %v", e.Action, err)
		}
	}

	got, err := repo.ListByGroup(ctx, -1001)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("logs del grupo -1001 = %d, want 2", len(got))
	}
	// El segundo insertado es mas reciente: debe venir primero.
	if got[0].Action != ActionUnlockGroup || got[1].Action != ActionLockGroup {
		t.Errorf("orden = %s, %s; want UnlockGroup, LockGroup", got[0].Action, got[1].Action)
	}
}

func TestRepository_ListByGroupEmpty(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	got, err := repo.ListByGroup(context.Background(), -999)
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("logs = %d, want 0", len(got))
	}
}

func TestRepository_CreateFailureLog(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	msg := "el bot no tiene permisos suficientes"
	entry := sampleEntry(-1001, ActionBanUser, StatusPermissionDenied)
	entry.TargetUserID = ptr64(456)
	entry.ErrorMessage = &msg
	if err := repo.Create(ctx, entry); err != nil {
		t.Fatalf("create failure log: %v", err)
	}

	got, err := repo.ListByGroup(ctx, -1001)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got[0].Status != StatusPermissionDenied {
		t.Errorf("status = %s, want PERMISSION_DENIED", got[0].Status)
	}
	if got[0].ErrorMessage == nil || *got[0].ErrorMessage != msg {
		t.Errorf("error_message = %v, want %q", got[0].ErrorMessage, msg)
	}
}

func ptr64(n int64) *int64 { return &n }
