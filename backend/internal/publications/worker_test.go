// Tests del Scheduler in-process (slice 3). Son unit-tests fake-driven:
// usan fakePubStore (extendido en service_test.go con ClaimScheduledDue +
// Cancel) y fakeTelegramPub (SendMessage/SendPhoto spies). Cero llamadas
// a la Bot API real (AGENTS.md §21.1).
package publications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/telegram"
)

// TestScheduler_Tick_NoDue_NoSend: tick con store vacio -> cero
// invocaciones a Telegram y log due=0.
func TestScheduler_Tick_NoDue_NoSend(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 1}
	store := newFakePubStore()
	svc, _ := newPubService(t, tg, store, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	})
	sch := NewScheduler(store, &fakeGroupsPub{groups: map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}}, tg, svc.logs, 100*time.Millisecond, nil)

	sch.tick(context.Background())

	if len(tg.calls) != 0 {
		t.Errorf("tg.calls = %d, want 0", len(tg.calls))
	}
}

// TestScheduler_Tick_ClaimsAndPublishes: 2 filas due -> ambas pasan a
// sending -> sent (con message_id). El worker reusa publishOne asi que
// las dos filas quedan en sent.
func TestScheduler_Tick_ClaimsAndPublishes(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 100}
	store := newFakePubStore()
	groupsMap := map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
		-1002: {TelegramID: -1002, BotStatus: groups.StatusAdministrator},
	}
	svc, _ := newPubService(t, tg, store, groupsMap)

	// Insertar filas ya `due` directamente (Schedule rechaza pasado
	// por diseno; aca necesitamos filas listas para el claim).
	actor := int64(7)
	past := time.Now().Add(-time.Minute)
	for _, gid := range []int64{-1001, -1002} {
		p := &Publication{
			TelegramID:  gid,
			Text:        "hola",
			Status:      StatusScheduled,
			ActorID:     &actor,
			ScheduledAt: &past,
		}
		if err := store.Create(context.Background(), p); err != nil {
			t.Fatalf("Create gid=%d: %v", gid, err)
		}
	}

	sch := NewScheduler(store, &fakeGroupsPub{groups: groupsMap}, tg, svc.logs, 100*time.Millisecond, nil)
	sch.tick(context.Background())

	// Ambas filas deben haber sido enviadas.
	if len(tg.calls) != 2 {
		t.Errorf("tg.calls = %d, want 2 (scheduler reusa publishOne)", len(tg.calls))
	}
	// Y quedaron sent en el store (publishOne llama UpdateStatus).
	for id, p := range store.pubs {
		if p.Status != StatusSent {
			t.Errorf("store.pubs[%d].status = %s, want sent", id, p.Status)
		}
		if p.MessageID == nil || *p.MessageID != 100 {
			t.Errorf("store.pubs[%d].message_id = %v, want 100", id, p.MessageID)
		}
	}
}

// TestScheduler_Tick_TelegramError_StaysFailed: si SendMessage falla,
// la fila queda failed (no retry automatico).
func TestScheduler_Tick_TelegramError_StaysFailed(t *testing.T) {
	tg := &fakeTelegramPub{err: telegram.ErrPermissionDenied}
	store := newFakePubStore()
	groupsMap := map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}
	svc, _ := newPubService(t, tg, store, groupsMap)

	actor := int64(7)
	past := time.Now().Add(-time.Minute)
	p := &Publication{
		TelegramID:  -1001,
		Text:        "x",
		Status:      StatusScheduled,
		ActorID:     &actor,
		ScheduledAt: &past,
	}
	if err := store.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	sch := NewScheduler(store, &fakeGroupsPub{groups: groupsMap}, tg, svc.logs, 100*time.Millisecond, nil)
	sch.tick(context.Background())

	got, ok := store.pubs[p.ID]
	if !ok {
		t.Fatalf("fila %d no existe", p.ID)
	}
	if got.Status != StatusFailed {
		t.Errorf("status = %s, want failed", got.Status)
	}
	if got.ErrorMessage == nil || *got.ErrorMessage == "" {
		t.Error("error_message vacio en fila failed")
	}
}

// TestScheduler_Tick_PermissionDenied_StaysFailed: bot no es admin ->
// publishOne detecta permissionOk y falla la fila con mensaje legible.
func TestScheduler_Tick_PermissionDenied_StaysFailed(t *testing.T) {
	tg := &fakeTelegramPub{messageID: 1}
	store := newFakePubStore()
	// Grupo existe pero bot NO es admin: permissionOk devuelve false.
	groupsMap := map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusMember},
	}
	svc, _ := newPubService(t, tg, store, groupsMap)

	actor := int64(7)
	past := time.Now().Add(-time.Minute)
	p := &Publication{
		TelegramID:  -1001,
		Text:        "x",
		Status:      StatusScheduled,
		ActorID:     &actor,
		ScheduledAt: &past,
	}
	if err := store.Create(context.Background(), p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	sch := NewScheduler(store, &fakeGroupsPub{groups: groupsMap}, tg, svc.logs, 100*time.Millisecond, nil)
	sch.tick(context.Background())

	got, ok := store.pubs[p.ID]
	if !ok {
		t.Fatalf("fila %d no existe", p.ID)
	}
	if got.Status != StatusFailed {
		t.Errorf("status = %s, want failed (bot no admin)", got.Status)
	}
	if got.ErrorMessage == nil {
		t.Fatal("error_message vacio")
	}
	if !contains(*got.ErrorMessage, "administrador") {
		t.Errorf("error_message = %q, want mensaje de admin", *got.ErrorMessage)
	}
	// Cero llamadas a Telegram.
	if len(tg.calls) != 0 {
		t.Errorf("tg.calls = %d, want 0 (permissionOk corto antes de dispatch)", len(tg.calls))
	}
}

// TestScheduler_ContextCancel_ReturnsNil: cancelacion limpia de Run.
func TestScheduler_ContextCancel_ReturnsNil(t *testing.T) {
	tg := &fakeTelegramPub{}
	store := newFakePubStore()
	sch := NewScheduler(store, &fakeGroupsPub{}, tg, &fakeLogsPub{}, 50*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- sch.Run(ctx) }()

	time.Sleep(75 * time.Millisecond) // deja tickear al menos 1 vez
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run no retorno en 2s tras cancel")
	}
}

// contains helper local para evitar importar strings.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexOf(s, substr) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// Errores dummy para asegurar import de errors (no falla si no se usa).
var _ = errors.New
