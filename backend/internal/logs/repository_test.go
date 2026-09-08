package logs

import (
	"context"
	"database/sql"
	"testing"
	"time"

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

// testTenant devuelve el id del tenant `default` (idempotente, slice
// 0): los writes del paquete requieren tenant.
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

func sampleEntry(tenantID, groupID int64, action string, status Status) *Entry {
	actor := int64(1)
	return &Entry{
		TenantID: tenantID,
		ActorID:  &actor,
		GroupID:  groupID,
		Action:   action,
		Status:   status,
	}
}

func TestRepository_CreateAndGet(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	tid := testTenant(t, db)

	ctx := context.Background()
	target := int64(456)
	entry := &Entry{
		TenantID:     tid,
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

	got, err := repo.ListByGroup(ctx, tid, -100123)
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
	tid := testTenant(t, db)

	ctx := context.Background()
	// Logs del grupo 1 (mas reciente al final en insercion) y uno del 2.
	for _, e := range []*Entry{
		sampleEntry(tid, -1001, ActionLockGroup, StatusSuccess),
		sampleEntry(tid, -1002, ActionPinMessage, StatusSuccess),
		sampleEntry(tid, -1001, ActionUnlockGroup, StatusSuccess),
	} {
		if err := repo.Create(ctx, e); err != nil {
			t.Fatalf("create %s: %v", e.Action, err)
		}
	}

	got, err := repo.ListByGroup(ctx, tid, -1001)
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
	tid := testTenant(t, db)

	got, err := repo.ListByGroup(context.Background(), tid, -999)
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
	tid := testTenant(t, db)

	ctx := context.Background()
	msg := "el bot no tiene permisos suficientes"
	entry := sampleEntry(tid, -1001, ActionBanUser, StatusPermissionDenied)
	entry.TargetUserID = ptr64(456)
	entry.ErrorMessage = &msg
	if err := repo.Create(ctx, entry); err != nil {
		t.Fatalf("create failure log: %v", err)
	}

	got, err := repo.ListByGroup(ctx, tid, -1001)
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

// --- Tests de slice 3: CountByActionAndGroup ---

// insertLogAt crea un log con created_at fijo (via SQL directo: la
// columna tiene DEFAULT now(), pero los tests del dashboard necesitan
// logs antiguos para verificar el filtro since). Helper de slice 3.
func insertLogAt(t *testing.T, db *sql.DB, tenantID, groupID int64, action string, createdAt time.Time) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO logs (tenant_id, actor_id, group_id, action, status, created_at) VALUES ($1, NULL, $2, $3, 'SUCCESS', $4)`,
		tenantID, groupID, action, createdAt.UTC(),
	); err != nil {
		t.Fatalf("insert log %s @ %v: %v", action, createdAt, err)
	}
}

// TestRepository_CountByActionAndGroup_HappyPath: inserta logs de los
// 3 actions de auto-moderacion + 1 action manual (BAN_USER, fuera del
// filtro); retorna solo los counts de los 3 actions pedidos.
func TestRepository_CountByActionAndGroup_HappyPath(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	tid := testTenant(t, db)
	groupID := int64(-100301)

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		insertLogAt(t, db, tid, groupID, ActionRuleTriggered, now)
	}
	for i := 0; i < 2; i++ {
		insertLogAt(t, db, tid, groupID, ActionAutomuteUser, now)
	}
	insertLogAt(t, db, tid, groupID, ActionAutobanUser, now)
	for i := 0; i < 3; i++ {
		insertLogAt(t, db, tid, groupID, ActionBanUser, now)
	}

	got, err := 	repo.CountByActionAndGroup(ctx, tid, groupID,
		[]string{ActionRuleTriggered, ActionAutomuteUser, ActionAutobanUser},
		now.Add(-1*time.Hour),
	)
	if err != nil {
		t.Fatalf("CountByActionAndGroup: %v", err)
	}
	want := map[string]int{
		ActionRuleTriggered: 5,
		ActionAutomuteUser:  2,
		ActionAutobanUser:   1,
	}
	if len(got) != len(want) {
		t.Errorf("map len = %d, want %d (got = %v)", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("got[%q] = %d, want %d", k, got[k], v)
		}
	}
	if _, ok := got[ActionBanUser]; ok {
		t.Errorf("BAN_USER no debio contarse (fuera del filtro): %v", got)
	}
}

// TestRepository_CountByActionAndGroup_SinceFilter: logs antiguos
// (fuera de la ventana) NO se cuentan.
func TestRepository_CountByActionAndGroup_SinceFilter(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	tid := testTenant(t, db)
	groupID := int64(-100302)

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		insertLogAt(t, db, tid, groupID, ActionRuleTriggered, now.Add(-48*time.Hour))
	}
	for i := 0; i < 3; i++ {
		insertLogAt(t, db, tid, groupID, ActionRuleTriggered, now.Add(-1*time.Hour))
	}

	since := now.Add(-24 * time.Hour)
	got, err := 	repo.CountByActionAndGroup(ctx, tid, groupID, []string{ActionRuleTriggered}, since)
	if err != nil {
		t.Fatalf("CountByActionAndGroup: %v", err)
	}
	if got[ActionRuleTriggered] != 3 {
		t.Errorf("count = %d, want 3 (solo logs dentro de 24h)", got[ActionRuleTriggered])
	}
}

// TestRepository_CountByActionAndGroup_ActionsSubset: actions fuera
// del array NO aparecen en el map.
func TestRepository_CountByActionAndGroup_ActionsSubset(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	tid := testTenant(t, db)
	groupID := int64(-100303)

	now := time.Now().UTC()
	for i := 0; i < 4; i++ {
		insertLogAt(t, db, tid, groupID, ActionRuleTriggered, now)
	}
	insertLogAt(t, db, tid, groupID, ActionAutomuteUser, now)

	got, err := 	repo.CountByActionAndGroup(ctx, tid, groupID, []string{ActionRuleTriggered}, now.Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("CountByActionAndGroup: %v", err)
	}
	if got[ActionRuleTriggered] != 4 {
		t.Errorf("RULE_TRIGGERED count = %d, want 4", got[ActionRuleTriggered])
	}
	if _, ok := got[ActionAutomuteUser]; ok {
		t.Errorf("AUTOMUTE_USER no debio aparecer (no estaba en actions): %v", got)
	}
	if len(got) != 1 {
		t.Errorf("map len = %d, want 1", len(got))
	}
}

// TestRepository_CountByActionAndGroup_Empty: grupo sin logs retorna
// map vacio (no nil) y sin error.
func TestRepository_CountByActionAndGroup_Empty(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	tid := testTenant(t, db)

	got, err := repo.CountByActionAndGroup(ctx, tid, -100304,
		[]string{ActionRuleTriggered, ActionAutomuteUser, ActionAutobanUser},
		time.Now().Add(-1*time.Hour),
	)
	if err != nil {
		t.Fatalf("CountByActionAndGroup: %v", err)
	}
	if got == nil {
		t.Error("map = nil, want empty map")
	}
	if len(got) != 0 {
		t.Errorf("map len = %d, want 0 (got = %v)", len(got), got)
	}
}

func TestRepository_CrossTenantIsolation(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	tidA := testTenant(t, db)
	var tidB int64
	const q = `
INSERT INTO tenants (slug) VALUES ('logs-other')
ON CONFLICT (slug) DO UPDATE SET slug = EXCLUDED.slug
RETURNING id`
	if err := db.QueryRowContext(ctx, q).Scan(&tidB); err != nil {
		t.Fatalf("ensure tenant: %v", err)
	}

	if err := repo.Create(ctx, sampleEntry(tidA, -1001, ActionBanUser, StatusSuccess)); err != nil {
		t.Fatalf("create A: %v", err)
	}

	gotB, err := repo.ListByGroup(ctx, tidB, -1001)
	if err != nil {
		t.Fatalf("list B: %v", err)
	}
	if len(gotB) != 0 {
		t.Errorf("list B = %+v, want vacio (fuga del tenant A)", gotB)
	}
	gotA, err := repo.ListByGroup(ctx, tidA, -1001)
	if err != nil || len(gotA) != 1 {
		t.Fatalf("list A: %+v, %v", gotA, err)
	}
	if gotA[0].TenantID != tidA {
		t.Errorf("tenant_id = %d, want %d", gotA[0].TenantID, tidA)
	}
}
