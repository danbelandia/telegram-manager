// Integration tests del Repository (slice 3) contra PostgreSQL real.
// Convencion AGENTS.md §21 / backend-go-skill §8: SQL real, no mocks
// para el repo. Cada test arranca con la tabla publications vacia
// (TRUNCATE ... CASCADE) para no depender del estado que dejo el
// anterior.
//
// Se salta con -short o si no hay Postgres disponible.
package publications

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/telegram-manager/backend/internal/database"
)

// setupRepoDB abre una base dedicada "publications", aplica migraciones,
// trunca tablas y devuelve el *sql.DB concreto.
func setupRepoDB(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("integration: skipping with -short")
	}
	db, err := database.OpenTestDB(t, "publications")
	if err != nil {
		t.Skipf("integration: no postgres available: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// CASCADE porque publications tiene FK a groups.telegram_id.
	if _, err := db.ExecContext(context.Background(), "TRUNCATE publications, groups CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

// insertPublication inserta una fila minima via SQL directo y devuelve
// el id. Usamos INSERT directo (bypaseando Create) para evitar que los
// tests dependan de actor_id NULL / MarshalButtons.
func insertPublication(t *testing.T, db *sql.DB, telegramID int64, status Status, scheduledAt *time.Time) int64 {
	t.Helper()
	// Asegurar que el grupo existe (FK telegram_id -> groups.telegram_id).
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO groups (telegram_id, title, type, bot_status) VALUES ($1, $2, 'supergroup', 'administrator') ON CONFLICT (telegram_id) DO NOTHING`,
		telegramID, "test",
	); err != nil {
		t.Fatalf("insert group: %v", err)
	}
	scheduledArg := any(nil)
	if scheduledAt != nil {
		scheduledArg = *scheduledAt
	}
	var id int64
	if err := db.QueryRowContext(context.Background(),
		`INSERT INTO publications (telegram_id, text, status, scheduled_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		telegramID, "x", string(status), scheduledArg,
	).Scan(&id); err != nil {
		t.Fatalf("insert row: %v", err)
	}
	return id
}

// TestRepository_ClaimScheduledDue_BatchSize: N due rows ->
// ClaimScheduledDue retorna N y todas quedan sending.
func TestRepository_ClaimScheduledDue_BatchSize(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	past := time.Now().Add(-1 * time.Minute)
	for i := 0; i < 3; i++ {
		insertPublication(t, db, -1000000000-int64(i), StatusScheduled, &past)
	}

	rows, err := repo.ClaimScheduledDue(ctx, 25)
	if err != nil {
		t.Fatalf("ClaimScheduledDue: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("rows = %d, want 3", len(rows))
	}
	for _, r := range rows {
		if r.Status != StatusSending {
			t.Errorf("row %d status = %s, want sending", r.ID, r.Status)
		}
	}
}

// TestRepository_ClaimScheduledDue_SkipsLockedByAnotherTxn: dos
// transacciones concurrentes; SKIP LOCKED garantiza que la segunda no
// ve las filas que la primera tiene lockeadas.
func TestRepository_ClaimScheduledDue_SkipsLockedByAnotherTxn(t *testing.T) {
	db := setupRepoDB(t)

	ctx := context.Background()
	past := time.Now().Add(-1 * time.Minute)
	for i := 0; i < 2; i++ {
		insertPublication(t, db, -1000000000-int64(i), StatusScheduled, &past)
	}

	// T1 abre BeginTx + SELECT FOR UPDATE SKIP LOCKED y NO commitea.
	// Subquery + ORDER BY + LIMIT para emular el patron del repo.
	tx1, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx1: %v", err)
	}
	defer tx1.Rollback()
	rows1, err := tx1.QueryContext(ctx,
		`SELECT id FROM publications WHERE status='scheduled' AND scheduled_at <= now() ORDER BY scheduled_at ASC LIMIT 25 FOR UPDATE SKIP LOCKED`)
	if err != nil {
		t.Fatalf("t1 query: %v", err)
	}
	var got1 []int64
	for rows1.Next() {
		var id int64
		if err := rows1.Scan(&id); err != nil {
			rows1.Close()
			t.Fatalf("t1 scan: %v", err)
		}
		got1 = append(got1, id)
	}
	rows1.Close()
	if len(got1) != 2 {
		t.Errorf("t1 rows = %d, want 2 (sin concurrencia)", len(got1))
	}

	// T2 abre BeginTx + SELECT FOR UPDATE SKIP LOCKED con T1 aun
	// abierto: SKIP LOCKED hace que T2 NO vea ninguna fila.
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	defer tx2.Rollback()
	rows2, err := tx2.QueryContext(ctx,
		`SELECT id FROM publications WHERE status='scheduled' AND scheduled_at <= now() ORDER BY scheduled_at ASC LIMIT 25 FOR UPDATE SKIP LOCKED`)
	if err != nil {
		t.Fatalf("t2 query: %v", err)
	}
	var got2 []int64
	for rows2.Next() {
		var id int64
		if err := rows2.Scan(&id); err != nil {
			rows2.Close()
			t.Fatalf("t2 scan: %v", err)
		}
		got2 = append(got2, id)
	}
	rows2.Close()
	if len(got2) != 0 {
		t.Errorf("t2 rows = %d, want 0 (SKIP LOCKED)", len(got2))
	}
}

// TestRepository_List_LimitOffset_Pagina: 75 filas, limit=10&offset=20
// retorna 10 filas correctas en orden DESC.
func TestRepository_List_LimitOffset_Pagina(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	for i := 0; i < 75; i++ {
		insertPublication(t, db, -1000000000-int64(i), StatusSent, nil)
	}

	list, err := repo.List(ctx, 10, 20)
	if err != nil {
		t.Fatalf("List(10,20): %v", err)
	}
	if len(list) != 10 {
		t.Errorf("len = %d, want 10", len(list))
	}
	// created_at DESC: IDs decrecientes.
	for i := 0; i < len(list)-1; i++ {
		if list[i].ID < list[i+1].ID {
			t.Errorf("list[%d].ID=%d < list[%d].ID=%d (deberia ser DESC)",
				i, list[i].ID, i+1, list[i+1].ID)
		}
	}
}

// TestRepository_Cancel_DeleteRow: fila scheduled -> DELETE; subsiguiente
// GetByID -> ErrNotFound.
func TestRepository_Cancel_DeleteRow(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	future := time.Now().Add(1 * time.Hour)
	id := insertPublication(t, db, -1000000000, StatusScheduled, &future)

	if err := repo.Cancel(ctx, id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if _, err := repo.GetByID(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("post-cancel GetByID = %v, want ErrNotFound", err)
	}
}

// TestRepository_Cancel_SentReturnsErrCancelNotAllowed: fila sent -> 409.
func TestRepository_Cancel_SentReturnsErrCancelNotAllowed(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)

	ctx := context.Background()
	id := insertPublication(t, db, -1000000000, StatusSent, nil)

	if err := repo.Cancel(ctx, id); !errors.Is(err, ErrCancelNotAllowed) {
		t.Errorf("Cancel(sent) = %v, want ErrCancelNotAllowed", err)
	}
	// Fila intacta.
	if _, err := repo.GetByID(ctx, id); err != nil {
		t.Errorf("post-cancel GetByID = %v, want fila intacta", err)
	}
}

// TestRepository_Cancel_NotFoundReturnsErrNotFound: id inexistente.
func TestRepository_Cancel_NotFoundReturnsErrNotFound(t *testing.T) {
	db := setupRepoDB(t)
	repo := NewRepository(db)

	err := repo.Cancel(context.Background(), 999999999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Cancel(inexistente) = %v, want ErrNotFound", err)
	}
}
