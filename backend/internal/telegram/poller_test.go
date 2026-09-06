package telegram

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeService simula Service con una cola programada de respuestas.
// Cuando la cola se vacia simula un long poll vacio (emptyWait),
// evitando un hot loop en los tests.
type fakeService struct {
	mu        sync.Mutex
	responses []fakeResponse
	offsets   []int
	calls     int
	block     bool          // true: GetUpdates espera ctx.Done (caso cancelacion)
	emptyWait time.Duration // pausa por poll vacio (default 10ms)
}

type fakeResponse struct {
	updates []Update
	err     error
}

// Los otros metodos de Service no se usan en estos tests.
func (f *fakeService) GetMe(ctx context.Context) (BotUser, error) { return BotUser{}, nil }
func (f *fakeService) SetWebhook(ctx context.Context, url, secret string, allowed []string) error {
	return nil
}
func (f *fakeService) DeleteWebhook(ctx context.Context) error { return nil }

func (f *fakeService) GetUpdates(ctx context.Context, offset, timeout int, allowed []string) ([]Update, error) {
	f.mu.Lock()
	f.calls++
	f.offsets = append(f.offsets, offset)

	if f.block {
		f.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	var r fakeResponse
	if len(f.responses) > 0 {
		r = f.responses[0]
		f.responses = f.responses[1:]
		f.mu.Unlock()
		return r.updates, r.err
	}

	wait := f.emptyWait
	if wait == 0 {
		wait = 10 * time.Millisecond
	}
	f.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(wait):
		return nil, nil
	}
}

func TestPoller_AdvancesOffset(t *testing.T) {
	svc := &fakeService{responses: []fakeResponse{
		{updates: []Update{{UpdateID: 10}, {UpdateID: 11}}},
	}}
	poller := NewPoller(svc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var batches [][]Update
	var mu sync.Mutex
	done := make(chan error, 1)
	go func() {
		done <- poller.Run(ctx, func(updates []Update) {
			mu.Lock()
			defer mu.Unlock()
			batches = append(batches, updates)
		})
	}()

	// Esperamos el lote inicial y que el offset avance antes de cancelar.
	time.Sleep(200 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run() = %v, want nil on cancel", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(batches) != 1 {
		t.Fatalf("batches = %d, want 1 (lote inicial)", len(batches))
	}
	if got := batches[0][len(batches[0])-1].UpdateID; got != 11 {
		t.Errorf("last update id = %d, want 11", got)
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if len(svc.offsets) < 2 {
		t.Fatalf("calls = %d, want >= 2 (offset avanzado)", len(svc.offsets))
	}
	if svc.offsets[0] != 0 {
		t.Errorf("first offset = %d, want 0", svc.offsets[0])
	}
	if svc.offsets[1] == 0 {
		t.Errorf("second offset = %d, want > 0 (avanzado tras el lote)", svc.offsets[1])
	}
}

func TestPoller_TransientErrorDoesNotAdvanceOffset(t *testing.T) {
	transient := errors.New("boom")
	svc := &fakeService{responses: []fakeResponse{
		{err: transient},
		{updates: []Update{{UpdateID: 10}}},
	}}
	poller := NewPoller(svc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var batches [][]Update
	done := make(chan error, 1)
	go func() {
		done <- poller.Run(ctx, func(updates []Update) {
			mu.Lock()
			defer mu.Unlock()
			batches = append(batches, updates)
		})
	}()

	// El primer error dispara backoff ~1s; el segundo poll trae el lote.
	time.Sleep(1800 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run() = %v, want nil on cancel", err)
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()
	// offsets 0(transitorio), 0(backoff), 11(tras lote)...
	if len(svc.offsets) < 3 {
		t.Fatalf("calls = %d, want >= 3", len(svc.offsets))
	}
	if svc.offsets[1] != 0 {
		t.Errorf("offset tras error transitorio = %d, want 0 (no avanzado)", svc.offsets[1])
	}
	for _, off := range svc.offsets[2:] {
		if off == 0 {
			t.Errorf("offset %d deberia haber avanzado tras exito", off)
		}
	}
}

func TestPoller_RateLimitRetries(t *testing.T) {
	svc := &fakeService{responses: []fakeResponse{
		{err: &RateLimitError{RetryAfter: time.Second}},
		{updates: []Update{{UpdateID: 10}}},
	}}
	poller := NewPoller(svc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- poller.Run(ctx, func([]Update) {}) }()

	start := time.Now()
	time.Sleep(1500 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run() = %v, want nil on cancel", err)
	}
	if since := time.Since(start); since < time.Second {
		t.Errorf("Run terminó antes del retry_after (duración %v)", since)
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.calls < 2 {
		t.Errorf("calls = %d, want >= 2 (reintento tras rate limit)", svc.calls)
	}
}

func TestPoller_RateLimitExhausted(t *testing.T) {
	rateErr := &RateLimitError{RetryAfter: 10 * time.Millisecond}
	svc := &fakeService{responses: []fakeResponse{
		{err: rateErr},
		{err: rateErr},
		{err: rateErr},
		{err: rateErr}, // 429 persistente: agota reintentos
	}}
	poller := NewPoller(svc)

	err := poller.Run(context.Background(), func([]Update) {})
	if err == nil {
		t.Fatal("Run() = nil, want rate limit error")
	}
}

func TestPoller_ConflictIsFatal(t *testing.T) {
	svc := &fakeService{responses: []fakeResponse{{err: ErrWebhookConflict}}}
	poller := NewPoller(svc)

	err := poller.Run(context.Background(), func([]Update) {})
	if !errors.Is(err, ErrWebhookConflict) {
		t.Fatalf("Run() = %v, want ErrWebhookConflict", err)
	}
}

func TestPoller_InvalidTokenIsFatal(t *testing.T) {
	svc := &fakeService{responses: []fakeResponse{{err: ErrInvalidToken}}}
	poller := NewPoller(svc)

	err := poller.Run(context.Background(), func([]Update) {})
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Run() = %v, want ErrInvalidToken", err)
	}
}

func TestPoller_ContextCancel(t *testing.T) {
	svc := &fakeService{block: true}
	poller := NewPoller(svc)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- poller.Run(ctx, func([]Update) {}) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() = %v, want nil on cancel", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run() no retornó tras cancel")
	}
}
