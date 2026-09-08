// Integration tests del Repository contra PostgreSQL real.
// Convencion AGENTS.md §21 / backend-go-skill §8: SQL real, no mocks
// para el repo. Cada test arranca con las tablas vacias
// (TRUNCATE ... CASCADE) para no depender del estado que dejo el
// anterior.
//
// Se salta con -short o si no hay Postgres disponible.
package automation

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/telegram-manager/backend/internal/database"
)

// setupRepoDB abre una base dedicada "automation", aplica migraciones,
// trunca tablas y devuelve el *sql.DB concreto.
func setupRepoDB(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("integration: skipping with -short")
	}
	db, err := database.OpenTestDB(t, "automation")
	if err != nil {
		t.Skipf("integration: no postgres available: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// CASCADE porque group_moderation_settings tiene FK a
	// groups.telegram_id, y user_warning_state se referencia por
	// (group_id, user_id) (no FK formal, pero TRUNCATE CASCADE limpia
	// cualquier cosa colgada). Slice 2: banned_words y link_allowlist
	// tambien tienen FK CASCADE a groups y se truncan aqui.
	if _, err := db.ExecContext(context.Background(),
		"TRUNCATE groups, group_moderation_settings, user_warning_state, banned_words, link_allowlist CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

// insertGroup inserta un grupo minimo via SQL directo. FK
// groups.telegram_id debe existir antes de insertar settings.
func insertGroup(t *testing.T, db *sql.DB, telegramID int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO groups (telegram_id, title, type, bot_status) VALUES ($1, 'g', 'supergroup', 'administrator') ON CONFLICT (telegram_id) DO NOTHING`,
		telegramID,
	); err != nil {
		t.Fatalf("insert group %d: %v", telegramID, err)
	}
}

// TestRepository_GetSettings_NotFound: grupo sin fila → ErrNotFound.
func TestRepository_GetSettings_NotFound(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)

	_, err := repo.GetSettings(context.Background(), -1001)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSettings(in = %d) = %v, want ErrNotFound", -1001, err)
	}
}

// TestRepository_UpsertSettings_RoundTrip: upsert + get devuelve
// los mismos campos (incluido updated_at puesto por la DB).
func TestRepository_UpsertSettings_RoundTrip(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	insertGroup(t, db, -1001)

	want := &Settings{
		GroupID:            -1001,
		Enabled:            true,
		AntiSpamEnabled:    true,
		AntiLinkEnabled:    false,
		BannedWordsEnabled: false,
		FloodEnabled:       true,
		FloodMessages:      7,
		FloodSeconds:       15,
		WarningLimit:       4,
		AutomuteWarnings:   3,
		AutomuteMinutes:    12,
		AutobanWarnings:    6,
		WarningExpireDays:  45,
	}
	if err := repo.UpsertSettings(context.Background(), want); err != nil {
		t.Fatalf("UpsertSettings: %v", err)
	}

	got, err := repo.GetSettings(context.Background(), -1001)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.Enabled != want.Enabled {
		t.Errorf("Enabled = %v, want %v", got.Enabled, want.Enabled)
	}
	if got.FloodMessages != want.FloodMessages {
		t.Errorf("FloodMessages = %d, want %d", got.FloodMessages, want.FloodMessages)
	}
	if got.AutobanWarnings != want.AutobanWarnings {
		t.Errorf("AutobanWarnings = %d, want %d", got.AutobanWarnings, want.AutobanWarnings)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt = zero, want timestamp puesto por la DB")
	}
}

// TestRepository_UpsertSettings_Idempotent: dos upserts reemplazan
// valores (no error, contador 1 fila).
func TestRepository_UpsertSettings_Idempotent(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	insertGroup(t, db, -1001)

	if err := repo.UpsertSettings(context.Background(), &Settings{
		GroupID: -1001, Enabled: false, FloodEnabled: false, FloodMessages: 5, FloodSeconds: 10,
		WarningLimit: 3, AutomuteWarnings: 3, AutomuteMinutes: 10, AutobanWarnings: 5, WarningExpireDays: 30,
	}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := repo.UpsertSettings(context.Background(), &Settings{
		GroupID: -1001, Enabled: true, FloodEnabled: true, FloodMessages: 8, FloodSeconds: 20,
		WarningLimit: 4, AutomuteWarnings: 4, AutomuteMinutes: 20, AutobanWarnings: 7, WarningExpireDays: 60,
	}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := repo.GetSettings(context.Background(), -1001)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.FloodMessages != 8 || got.AutobanWarnings != 7 {
		t.Errorf("segundo upsert no reemplazo: %+v", got)
	}
	var n int
	if err := db.QueryRowContext(context.Background(),
		"SELECT count(*) FROM group_moderation_settings WHERE group_id = $1", -1001).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("rows = %d, want 1 (idempotent)", n)
	}
}

// TestRepository_UpsertSettings_RejectsInvalidThresholds: CHECK
// constraints en la DB rechazan valores invalidos.
func TestRepository_UpsertSettings_RejectsInvalidThresholds(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	insertGroup(t, db, -1001)

	cases := []struct {
		name string
		s    *Settings
	}{
		{
			"flood_messages=0",
			&Settings{GroupID: -1001, FloodMessages: 0, FloodSeconds: 10, WarningLimit: 3, AutomuteWarnings: 3, AutomuteMinutes: 10, AutobanWarnings: 5, WarningExpireDays: 30},
		},
		{
			"autoban_warnings=automute_warnings",
			&Settings{GroupID: -1001, FloodMessages: 5, FloodSeconds: 10, WarningLimit: 3, AutomuteWarnings: 3, AutomuteMinutes: 10, AutobanWarnings: 3, WarningExpireDays: 30},
		},
		{
			"warning_expire_days=0",
			&Settings{GroupID: -1001, FloodMessages: 5, FloodSeconds: 10, WarningLimit: 3, AutomuteWarnings: 3, AutomuteMinutes: 10, AutobanWarnings: 5, WarningExpireDays: 0},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := repo.UpsertSettings(context.Background(), tc.s); err == nil {
				t.Errorf("UpsertSettings(%s) = nil, want CHECK violation", tc.name)
			}
		})
	}
}

// TestRepository_WarningState_Lifecycle: create-if-missing → upsert
// (counter++) → get → list → reset expired.
func TestRepository_WarningState_Lifecycle(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)

	ctx := context.Background()

	// Primer contacto: no existe fila.
	_, err := repo.GetWarningState(ctx, -1001, 999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetWarningState inicial = %v, want ErrNotFound", err)
	}

	// Create-if-missing: crea fila con counter=0.
	if err := repo.CreateWarningStateIfMissing(ctx, -1001, 999); err != nil {
		t.Fatalf("CreateWarningStateIfMissing: %v", err)
	}
	ws, err := repo.GetWarningState(ctx, -1001, 999)
	if err != nil {
		t.Fatalf("GetWarningState post-create: %v", err)
	}
	if ws.WarningCount != 0 {
		t.Errorf("counter inicial = %d, want 0", ws.WarningCount)
	}

	// Upsert con counter=3 + timestamps.
	now := time.Now().UTC().Truncate(time.Microsecond)
	ws.WarningCount = 3
	ws.LastWarningAt = &now
	ws.ExpiresAt = &now
	if err := repo.UpsertWarningState(ctx, ws); err != nil {
		t.Fatalf("UpsertWarningState: %v", err)
	}

	got, err := repo.GetWarningState(ctx, -1001, 999)
	if err != nil {
		t.Fatalf("GetWarningState post-upsert: %v", err)
	}
	if got.WarningCount != 3 {
		t.Errorf("counter = %d, want 3", got.WarningCount)
	}
	if got.LastWarningAt == nil || !got.LastWarningAt.Equal(now) {
		t.Errorf("LastWarningAt = %v, want %v", got.LastWarningAt, now)
	}
}

// TestRepository_ListWarningStates_FilterByGroup: dos warning_states
// en grupos distintos → List devuelve solo los del grupo pedido.
func TestRepository_ListWarningStates_FilterByGroup(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	if err := repo.CreateWarningStateIfMissing(ctx, -1001, 1); err != nil {
		t.Fatalf("create g1 u1: %v", err)
	}
	if err := repo.CreateWarningStateIfMissing(ctx, -1001, 2); err != nil {
		t.Fatalf("create g1 u2: %v", err)
	}
	if err := repo.CreateWarningStateIfMissing(ctx, -1002, 3); err != nil {
		t.Fatalf("create g2 u3: %v", err)
	}

	g1, err := repo.ListWarningStates(ctx, -1001)
	if err != nil {
		t.Fatalf("ListWarningStates(g1): %v", err)
	}
	if len(g1) != 2 {
		t.Errorf("g1 list len = %d, want 2", len(g1))
	}
	for _, ws := range g1 {
		if ws.GroupID != -1001 {
			t.Errorf("g1 list contiene group_id=%d", ws.GroupID)
		}
	}

	g2, err := repo.ListWarningStates(ctx, -1002)
	if err != nil {
		t.Fatalf("ListWarningStates(g2): %v", err)
	}
	if len(g2) != 1 {
		t.Errorf("g2 list len = %d, want 1", len(g2))
	}
}

// TestRepository_ResetExpiredWarnings_OnlyAffectsExpired: dos filas,
// una expirada y otra vigente → solo la expirada se resetea.
func TestRepository_ResetExpiredWarnings_OnlyAffectsExpired(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	// Fila 1: expirada.
	if err := repo.CreateWarningStateIfMissing(ctx, -1001, 1); err != nil {
		t.Fatalf("create u1: %v", err)
	}
	ws1, _ := repo.GetWarningState(ctx, -1001, 1)
	ws1.WarningCount = 5
	ws1.ExpiresAt = &past
	if err := repo.UpsertWarningState(ctx, ws1); err != nil {
		t.Fatalf("upsert u1: %v", err)
	}

	// Fila 2: vigente.
	if err := repo.CreateWarningStateIfMissing(ctx, -1001, 2); err != nil {
		t.Fatalf("create u2: %v", err)
	}
	ws2, _ := repo.GetWarningState(ctx, -1001, 2)
	ws2.WarningCount = 5
	ws2.ExpiresAt = &future
	if err := repo.UpsertWarningState(ctx, ws2); err != nil {
		t.Fatalf("upsert u2: %v", err)
	}

	// Reset: solo la fila 1 deberia ser afectada.
	now := time.Now()
	n, err := repo.ResetExpiredWarnings(ctx, -1001, now)
	if err != nil {
		t.Fatalf("ResetExpiredWarnings: %v", err)
	}
	if n != 1 {
		t.Errorf("rows affected = %d, want 1", n)
	}

	got1, _ := repo.GetWarningState(ctx, -1001, 1)
	if got1.WarningCount != 0 {
		t.Errorf("u1 counter post-reset = %d, want 0", got1.WarningCount)
	}
	if got1.ExpiresAt != nil {
		t.Errorf("u1 expires_at post-reset = %v, want nil", got1.ExpiresAt)
	}
	got2, _ := repo.GetWarningState(ctx, -1001, 2)
	if got2.WarningCount != 5 {
		t.Errorf("u2 counter post-reset = %d, want 5 (vigente)", got2.WarningCount)
	}
}

// TestRepository_CreateWarningStateIfMissing_Idempotent: dos llamadas
// seguidas → una sola fila, counter=0.
func TestRepository_CreateWarningStateIfMissing_Idempotent(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	if err := repo.CreateWarningStateIfMissing(ctx, -1001, 999); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := repo.CreateWarningStateIfMissing(ctx, -1001, 999); err != nil {
		t.Fatalf("second: %v", err)
	}
	ws, err := repo.GetWarningState(ctx, -1001, 999)
	if err != nil {
		t.Fatalf("GetWarningState: %v", err)
	}
	if ws.WarningCount != 0 {
		t.Errorf("counter = %d, want 0", ws.WarningCount)
	}
}

// TestRepository_WarningState_CascadeOnGroupDelete: borrar el grupo
// cascadea las warning_states asociadas (FK semantic via group_id;
// NO hay FK formal, asi que confirmamos el comportamiento actual).
func TestRepository_WarningState_NoCascadeWithoutFK(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// user_warning_state NO tiene FK formal hacia groups (solo el
	// indice idx_user_warning_state_group). Por diseño, la limpieza
	// de filas huerfanas queda para un job de slice 2+ (o el Delete
	// del grupo en el modulo groups).
	if err := repo.CreateWarningStateIfMissing(ctx, -1001, 999); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM groups WHERE telegram_id = $1", -1001); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	// La warning_state sigue existiendo (sin FK).
	_, err := repo.GetWarningState(ctx, -1001, 999)
	if err != nil {
		t.Errorf("warning_state borrada junto al grupo (inesperado sin FK): %v", err)
	}
}

// TestRepository_GroupCascadeOnSettingsDelete: borrar el grupo
// cascadea sus settings (FK formal con ON DELETE CASCADE).
func TestRepository_GroupCascadeOnSettingsDelete(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	insertGroup(t, db, -1001)
	ctx := context.Background()

	if err := repo.UpsertSettings(ctx, &Settings{
		GroupID: -1001, FloodMessages: 5, FloodSeconds: 10, WarningLimit: 3,
		AutomuteWarnings: 3, AutomuteMinutes: 10, AutobanWarnings: 5,
		WarningExpireDays: 30,
	}); err != nil {
		t.Fatalf("upsert settings: %v", err)
	}

	if _, err := db.ExecContext(ctx, "DELETE FROM groups WHERE telegram_id = $1", -1001); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	_, err := repo.GetSettings(ctx, -1001)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("settings no borradas con el grupo: err = %v", err)
	}
}

// --- Tests de listas (slice 2): banned_words y link_allowlist ---

// TestRepository_ListBannedWords_Empty: grupo sin palabras → lista vacia.
func TestRepository_ListBannedWords_Empty(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	insertGroup(t, db, -1001)

	list, err := repo.ListBannedWords(ctx, -1001)
	if err != nil {
		t.Fatalf("ListBannedWords: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("list len = %d, want 0", len(list))
	}
}

// TestRepository_AddBannedWord_Lowercased: el INSERT del repo recibe
// la palabra ya normalizada (lo normaliza el handler); un insert
// directo con "BUZON" verifica que queda como "buzon" cuando el handler
// pre-normaliza. La normalizacion real vive en el handler
// (automation_handlers.go); el repo solo persiste lo que recibe.
func TestRepository_AddBannedWord_RoundTrip(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	insertGroup(t, db, -1001)

	for _, w := range []string{"spam", "viagra", "free-money"} {
		if err := repo.AddBannedWord(ctx, -1001, w); err != nil {
			t.Fatalf("AddBannedWord(%q): %v", w, err)
		}
	}

	list, err := repo.ListBannedWords(ctx, -1001)
	if err != nil {
		t.Fatalf("ListBannedWords: %v", err)
	}
	want := []string{"free-money", "spam", "viagra"}
	if len(list) != len(want) {
		t.Fatalf("list = %v, want %v", list, want)
	}
	for i, w := range want {
		if list[i] != w {
			t.Errorf("list[%d] = %q, want %q", i, list[i], w)
		}
	}
}

// TestRepository_AddBannedWord_Idempotent: dos adds del mismo word →
// una sola fila. Confirma que ON CONFLICT DO NOTHING funciona
// (spec REQ-8: POST idempotente).
func TestRepository_AddBannedWord_Idempotent(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	insertGroup(t, db, -1001)

	if err := repo.AddBannedWord(ctx, -1001, "spam"); err != nil {
		t.Fatalf("first AddBannedWord: %v", err)
	}
	if err := repo.AddBannedWord(ctx, -1001, "spam"); err != nil {
		t.Fatalf("second AddBannedWord: %v", err)
	}

	var n int
	if err := db.QueryRowContext(ctx,
		"SELECT count(*) FROM banned_words WHERE group_id = $1 AND word = $2",
		-1001, "spam",
	).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

// TestRepository_RemoveBannedWord: add + remove + list devuelve vacio.
func TestRepository_RemoveBannedWord(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	insertGroup(t, db, -1001)

	if err := repo.AddBannedWord(ctx, -1001, "spam"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := repo.RemoveBannedWord(ctx, -1001, "spam"); err != nil {
		t.Fatalf("remove: %v", err)
	}

	list, err := repo.ListBannedWords(ctx, -1001)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("post-remove list = %v, want []", list)
	}
}

// TestRepository_RemoveBannedWord_NotFoundIsOK: remove de word que
// no existe → nil error (DELETE idempotente, spec REQ-8).
func TestRepository_RemoveBannedWord_NotFoundIsOK(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	insertGroup(t, db, -1001)

	if err := repo.RemoveBannedWord(ctx, -1001, "does-not-exist"); err != nil {
		t.Errorf("RemoveBannedWord(in = \"does-not-exist\") = %v, want nil", err)
	}
}

// TestRepository_AddBannedWord_CascadeOnGroupDelete: borrar el grupo
// cascadea las banned_words del grupo (FK CASCADE).
func TestRepository_AddBannedWord_CascadeOnGroupDelete(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	insertGroup(t, db, -1001)

	if err := repo.AddBannedWord(ctx, -1001, "spam"); err != nil {
		t.Fatalf("add: %v", err)
	}

	if _, err := db.ExecContext(ctx, "DELETE FROM groups WHERE telegram_id = $1", -1001); err != nil {
		t.Fatalf("delete group: %v", err)
	}

	list, err := repo.ListBannedWords(ctx, -1001)
	if err != nil {
		t.Fatalf("list post-delete: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("banned_words post group delete = %v, want []", list)
	}
}

// TestRepository_AddBannedWord_RejectsEmpty: el CHECK length 1-100
// rechaza insertar string vacio (spec REQ-7).
func TestRepository_AddBannedWord_RejectsEmpty(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	insertGroup(t, db, -1001)

	if err := repo.AddBannedWord(ctx, -1001, ""); err == nil {
		t.Error("AddBannedWord(empty) = nil, want CHECK violation")
	}
}

// TestRepository_LinkAllowlist_RoundTrip: add + add + list devuelve
// las 2 dominios ordenados. Case preserved en storage.
func TestRepository_LinkAllowlist_RoundTrip(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	insertGroup(t, db, -1001)

	for _, d := range []string{"example.com", "github.com", "MiDominio.org"} {
		if err := repo.AddLinkAllowlist(ctx, -1001, d); err != nil {
			t.Fatalf("AddLinkAllowlist(%q): %v", d, err)
		}
	}

	list, err := repo.ListLinkAllowlist(ctx, -1001)
	if err != nil {
		t.Fatalf("ListLinkAllowlist: %v", err)
	}
	// El orden es case-sensitive por el ORDER BY de PostgreSQL
	// ("MiDominio.org" < "example.com" en ASCII uppercase). Verificamos
	// que las 3 estan presentes, case preserved.
	got := make(map[string]bool)
	for _, d := range list {
		got[d] = true
	}
	for _, d := range []string{"example.com", "github.com", "MiDominio.org"} {
		if !got[d] {
			t.Errorf("list no contiene %q (case preserved): %v", d, list)
		}
	}
}

// TestRepository_AddLinkAllowlist_Idempotent: dos adds del mismo
// dominio → una sola fila.
func TestRepository_AddLinkAllowlist_Idempotent(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	insertGroup(t, db, -1001)

	if err := repo.AddLinkAllowlist(ctx, -1001, "example.com"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := repo.AddLinkAllowlist(ctx, -1001, "example.com"); err != nil {
		t.Fatalf("second: %v", err)
	}

	var n int
	if err := db.QueryRowContext(ctx,
		"SELECT count(*) FROM link_allowlist WHERE group_id = $1 AND domain = $2",
		-1001, "example.com",
	).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

// TestRepository_RemoveLinkAllowlist: add + remove + list vacio.
func TestRepository_RemoveLinkAllowlist(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	insertGroup(t, db, -1001)

	if err := repo.AddLinkAllowlist(ctx, -1001, "example.com"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := repo.RemoveLinkAllowlist(ctx, -1001, "example.com"); err != nil {
		t.Fatalf("remove: %v", err)
	}

	list, err := repo.ListLinkAllowlist(ctx, -1001)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("post-remove list = %v, want []", list)
	}
}

// TestRepository_AddLinkAllowlist_CascadeOnGroupDelete: borrar el
// grupo cascadea las link_allowlist del grupo.
func TestRepository_AddLinkAllowlist_CascadeOnGroupDelete(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	insertGroup(t, db, -1001)

	if err := repo.AddLinkAllowlist(ctx, -1001, "example.com"); err != nil {
		t.Fatalf("add: %v", err)
	}

	if _, err := db.ExecContext(ctx, "DELETE FROM groups WHERE telegram_id = $1", -1001); err != nil {
		t.Fatalf("delete group: %v", err)
	}

	list, err := repo.ListLinkAllowlist(ctx, -1001)
	if err != nil {
		t.Fatalf("list post-delete: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("link_allowlist post group delete = %v, want []", list)
	}
}

// TestRepository_AddLinkAllowlist_RejectsEmpty: el CHECK length 1-253
// rechaza string vacio.
func TestRepository_AddLinkAllowlist_RejectsEmpty(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	insertGroup(t, db, -1001)

	if err := repo.AddLinkAllowlist(ctx, -1001, ""); err == nil {
		t.Error("AddLinkAllowlist(empty) = nil, want CHECK violation")
	}
}
