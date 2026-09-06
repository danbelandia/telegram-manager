package groups

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/telegram-manager/backend/internal/database"
)

// testDB abre una PostgreSQL real (convencion del proyecto: no mockear
// la capa DB) y aplica las migraciones. Se saltea en -short o si no
// hay base disponible: los tests de repositorio son de integracion.
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
	// Los tests comparten la misma base: cada test arranca con la tabla
	// vacia para no depender del estado que dejo el anterior.
	if _, err := db.ExecContext(context.Background(), "TRUNCATE groups"); err != nil {
		t.Fatalf("truncate groups: %v", err)
	}
	return db
}

func sampleGroup(telegramID int64, title string) *Group {
	return &Group{
		TelegramID: telegramID,
		Title:      title,
		Type:       "supergroup",
		BotStatus:  StatusMember,
	}
}

func TestRepository_UpsertIdempotent(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	g := sampleGroup(1000000001, "Beta")

	if err := repo.UpsertByTelegramID(ctx, g); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	g.Title = "Beta Renombrado"
	g.BotStatus = StatusAdministrator
	if err := repo.UpsertByTelegramID(ctx, g); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	// Unicidad: segunda escritura sobre el mismo telegram_id no duplica.
	const countQ = `SELECT count(*) FROM groups WHERE telegram_id = $1`
	var n int
	if err := db.QueryRowContext(ctx, countQ, g.TelegramID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("rows for telegram_id %d = %d, want 1", g.TelegramID, n)
	}

	got, err := repo.GetByTelegramID(ctx, g.TelegramID)
	if err != nil {
		t.Fatalf("get after upsert: %v", err)
	}
	if got.Title != "Beta Renombrado" {
		t.Errorf("title = %q, want %q", got.Title, "Beta Renombrado")
	}
	if got.BotStatus != StatusAdministrator {
		t.Errorf("bot_status = %q, want %q", got.BotStatus, StatusAdministrator)
	}
}

func TestRepository_UpsertUpdatesUpdatedAt(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	g := sampleGroup(1000000002, "Gamma")
	if err := repo.UpsertByTelegramID(ctx, g); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	before, err := repo.GetByTelegramID(ctx, g.TelegramID)
	if err != nil {
		t.Fatalf("get before: %v", err)
	}
	// updated_at cambia por funcion now(): esperar para que el reloj avance.
	time.Sleep(1100 * time.Millisecond)

	if err := repo.UpsertByTelegramID(ctx, g); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	after, err := repo.GetByTelegramID(ctx, g.TelegramID)
	if err != nil {
		t.Fatalf("get after: %v", err)
	}

	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Errorf("updated_at did not advance: %v -> %v", before.UpdatedAt, after.UpdatedAt)
	}
	if !before.CreatedAt.Equal(after.CreatedAt) {
		t.Errorf("created_at changed: %v -> %v", before.CreatedAt, after.CreatedAt)
	}
}

func TestRepository_ListOrderedByTitle(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	for _, g := range []*Group{
		sampleGroup(1000000101, "Zeta"),
		sampleGroup(1000000102, "Alpha"),
		sampleGroup(1000000103, "Milo"),
	} {
		if err := repo.UpsertByTelegramID(ctx, g); err != nil {
			t.Fatalf("upsert %s: %v", g.Title, err)
		}
	}

	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) < 3 {
		t.Fatalf("list returned %d groups, want >= 3", len(got))
	}

	titles := []string{"Alpha", "Milo", "Zeta"}
	for i, want := range titles {
		if got[i].Title != want {
			t.Errorf("list[%d].title = %q, want %q", i, got[i].Title, want)
		}
	}
}

func TestRepository_GetNotFound(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	_, err := repo.GetByTelegramID(context.Background(), 999999999999)
	if err == nil {
		t.Fatal("get inexistente returned nil error, want ErrNotFound")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestRepository_UpsertPermissionsJSONB(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	g := sampleGroup(1000000201, "Permisos")
	g.BotStatus = StatusAdministrator
	g.BotPermissions = map[string]bool{
		"can_delete_messages":  true,
		"can_restrict_members": true,
		"can_pin_messages":     false,
	}
	g.Username = ptr("grupo-permisos")
	g.MemberCount = ptrInt64(42)
	if err := repo.UpsertByTelegramID(ctx, g); err != nil {
		t.Fatalf("upsert with permissions: %v", err)
	}

	got, err := repo.GetByTelegramID(ctx, g.TelegramID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.BotPermissions["can_delete_messages"] != true || got.BotPermissions["can_restrict_members"] != true {
		t.Errorf("permissions mismatch: %#v", got.BotPermissions)
	}
	if got.BotPermissions["can_pin_messages"] != false {
		t.Errorf("can_pin_messages = %v, want false", got.BotPermissions["can_pin_messages"])
	}
	if got.Username == nil || *got.Username != "grupo-permisos" {
		t.Errorf("username = %v, want grupo-permisos", got.Username)
	}
	if got.MemberCount == nil || *got.MemberCount != 42 {
		t.Errorf("member_count = %v, want 42", got.MemberCount)
	}
}

func ptr(s string) *string    { return &s }
func ptrInt64(n int64) *int64 { return &n }
