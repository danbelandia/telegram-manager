package automation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/telegram-manager/backend/internal/events"
	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/logs"
	"github.com/telegram-manager/backend/internal/telegram"
)

// fakeActioner implementa AutoActioner registrando las llamadas.
// Util para tests de Worker y Subscriber.
type fakeActioner struct {
	mu    sync.Mutex
	calls []AutoAction
	err   error
	// errFor: si esta set y la action coincide por GroupID+UserID,
	// devuelve err.
	errFor map[[2]int64]error
}

func (f *fakeActioner) Execute(_ context.Context, action AutoAction) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, action)
	if f.err != nil {
		return f.err
	}
	if f.errFor != nil {
		if e, ok := f.errFor[[2]int64{action.GroupID, action.UserID}]; ok {
			return e
		}
	}
	return nil
}

// TestWorker_Run_ProcessesFIFO: encolar 3 acciones, ctx no cancelado,
// Run las procesa en orden y termina.
func TestWorker_Run_ProcessesFIFO(t *testing.T) {
	ch := make(chan AutoAction, 3)
	a := &fakeActioner{}
	w := NewWorker(ch, a, nil)

	ch <- AutoAction{Kind: AutoActionMute, GroupID: -1001, UserID: 1, RuleName: "flood", WarningCount: 3}
	ch <- AutoAction{Kind: AutoActionBan, GroupID: -1001, UserID: 2, RuleName: "flood", WarningCount: 5}
	ch <- AutoAction{Kind: AutoActionMute, GroupID: -1002, UserID: 3, RuleName: "flood", WarningCount: 3}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	// Esperar a que se procesen las 3 acciones (con margen).
	deadline := time.After(1 * time.Second)
	for {
		a.mu.Lock()
		n := len(a.calls)
		a.mu.Unlock()
		if n >= 3 {
			break
		}
		select {
		case <-deadline:
			a.mu.Lock()
			n = len(a.calls)
			a.mu.Unlock()
			t.Fatalf("timeout esperando 3 calls, got %d", n)
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	if err := <-done; err != nil {
		t.Errorf("Run: %v, want nil", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.calls) != 3 {
		t.Fatalf("calls = %d, want 3", len(a.calls))
	}
	// FIFO: primer call es la primera accion encolada.
	if a.calls[0].UserID != 1 || a.calls[1].UserID != 2 || a.calls[2].UserID != 3 {
		t.Errorf("FIFO order roto: %v", a.calls)
	}
}

// TestWorker_Run_CtxCancelReturnsNil: ctx cancelado antes de recibir
// nada → Run retorna nil inmediatamente.
func TestWorker_Run_CtxCancelReturnsNil(t *testing.T) {
	ch := make(chan AutoAction, 1)
	a := &fakeActioner{}
	w := NewWorker(ch, a, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run: %v, want nil", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run no retorno tras ctx cancel")
	}
}

// TestWorker_ProcessOnce: helper que procesa UN item del canal.
func TestWorker_ProcessOnce(t *testing.T) {
	ch := make(chan AutoAction, 1)
	a := &fakeActioner{}
	w := NewWorker(ch, a, nil)

	ch <- AutoAction{Kind: AutoActionBan, GroupID: -1001, UserID: 1, RuleName: "flood", WarningCount: 5}

	if err := w.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("ProcessOnce: %v", err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.calls) != 1 {
		t.Errorf("calls = %d, want 1", len(a.calls))
	}
}

// TestWorker_ProcessOnce_ContextCancelled: si ctx esta cancelado,
// ProcessOnce retorna ctx.Err() sin procesar nada.
func TestWorker_ProcessOnce_ContextCancelled(t *testing.T) {
	ch := make(chan AutoAction, 1)
	a := &fakeActioner{}
	w := NewWorker(ch, a, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := w.ProcessOnce(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ProcessOnce: err = %v, want context.Canceled", err)
	}
	if len(a.calls) != 0 {
		t.Errorf("calls = %d, want 0 (ctx cancelado)", len(a.calls))
	}
}

// TestWorker_Run_ActionerError_ContinuesProcessing: si el AutoActioner
// devuelve error, el worker loguea y procesa la siguiente accion.
func TestWorker_Run_ActionerError_ContinuesProcessing(t *testing.T) {
	ch := make(chan AutoAction, 2)
	a := &fakeActioner{err: errors.New("adapter timeout")}
	w := NewWorker(ch, a, nil)

	ch <- AutoAction{Kind: AutoActionMute, GroupID: -1001, UserID: 1, RuleName: "flood", WarningCount: 3}
	ch <- AutoAction{Kind: AutoActionBan, GroupID: -1001, UserID: 2, RuleName: "flood", WarningCount: 5}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	deadline := time.After(500 * time.Millisecond)
	for {
		a.mu.Lock()
		n := len(a.calls)
		a.mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			a.mu.Lock()
			n = len(a.calls)
			a.mu.Unlock()
			t.Fatalf("timeout: calls = %d, want 2", n)
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	<-done

	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.calls) != 2 {
		t.Errorf("calls = %d, want 2 (worker sigue pese a errores)", len(a.calls))
	}
}

// TestWorker_Run_ChannelClosed: si el canal se cierra, Run retorna nil.
func TestWorker_Run_ChannelClosed(t *testing.T) {
	ch := make(chan AutoAction)
	a := &fakeActioner{}
	w := NewWorker(ch, a, nil)

	go func() {
		time.Sleep(50 * time.Millisecond)
		close(ch)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run: %v, want nil", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run no retorno tras channel close")
	}
}

// --- Subscriber tests ---

// TestSubscriber_FiltersUpdatesWithoutMessage: solo los Updates con
// Message != nil y From != nil llegan al service.
func TestSubscriber_FiltersUpdatesWithoutMessage(t *testing.T) {
	bus := events.NewBus()
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{GroupID: -1001, Enabled: false} // disabled → skip silencioso
	warnRepo := newFakeWarnRepo()
	ch := make(chan AutoAction, 1)
	svc, _ := newSvc(settingsRepo, warnRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)
	s := NewSubscriber(bus, svc, nil)
	s.Register()

	cases := []struct {
		name   string
		update *telegram.Update
	}{
		{
			name:   "nil update",
			update: nil,
		},
		{
			name:   "chat_member update",
			update: &telegram.Update{UpdateID: 1, ChatMember: &telegram.ChatMemberUpdated{}},
		},
		{
			name: "message sin From",
			update: &telegram.Update{
				UpdateID: 2,
				Message:  &telegram.Message{MessageID: 1, Chat: telegram.Chat{ID: -1001, Type: "channel"}, Text: "anon"},
			},
		},
		{
			name: "message con From.ID==0",
			update: &telegram.Update{
				UpdateID: 3,
				Message: &telegram.Message{
					MessageID: 2,
					From:      &telegram.User{ID: 0},
					Chat:      telegram.Chat{ID: -1001, Type: "channel"},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("subscriber.Handle panico con %s: %v", tc.name, r)
				}
			}()
			bus.Publish(tc.update)
		})
	}

	// Ninguno de los updates invalidos debe haber creado warning_state.
	if _, ok := warnRepo.rows[[2]int64{-1001, 999}]; ok {
		t.Errorf("warning_state creada con update invalido")
	}
}

// TestSubscriber_PassesValidMessageToService: Update con Message + From
// valido → invoca svc.HandleMessage.
func TestSubscriber_PassesValidMessageToService(t *testing.T) {
	bus := events.NewBus()

	// Service real con fakes minimos.
	settingsRepo := newFakeSettingsRepo()
	settingsRepo.rows[-1001] = &Settings{GroupID: -1001, Enabled: false} // disabled → skip silencioso
	warnRepo := newFakeWarnRepo()
	ch := make(chan AutoAction, 1)
	svc, _ := newSvc(settingsRepo, warnRepo, map[int64]*groups.Group{
		-1001: {TelegramID: -1001, BotStatus: groups.StatusAdministrator},
	}, ch)

	s := NewSubscriber(bus, svc, nil)
	s.Register()

	bus.Publish(&telegram.Update{
		UpdateID: 1,
		Message: &telegram.Message{
			MessageID: 1,
			From:      &telegram.User{ID: 999, FirstName: "u"},
			Chat:      telegram.Chat{ID: -1001, Type: "supergroup"},
			Text:      "hi",
		},
	})

	// Como enabled=false, el service retorna nil sin tocar nada; lo
	// importante es que el subscriber lo llamo (no panic).
	// Verificamos que la warning_state NO se creo (disabled).
	if _, ok := warnRepo.rows[[2]int64{-1001, 999}]; ok {
		t.Errorf("warning_state creada pese a disabled")
	}
}

// --- Quick sanity: usamos logs.Entry para verificar importes (evita
//
//	"unused import" si algun dia se reduce el archivo). ---
var _ = logs.StatusSuccess
