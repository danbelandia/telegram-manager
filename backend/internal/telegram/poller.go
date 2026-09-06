package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// MVPAllowedUpdates son los tipos de update que el MVP procesa
// (AGENTS.md §§6-10). El resto (callback_query, poll, etc.) no llegan.
var MVPAllowedUpdates = []string{"message", "chat_member", "my_chat_member", "chat_join_request"}

const (
	// defaultPollTimeout es el long poll de getUpdates (TG recomienda 30).
	defaultPollTimeout = 30
	// maxRateLimitRetries limita los reintentos ante 429.
	maxRateLimitRetries = 3
)

// Poller hace long polling contra la Bot API y entrega lotes de
// updates. Es una capa delgada sobre Service; el manejo de rate limits
// (429) vive en el adapter, aqui solo se reacciona a RateLimitError.
type Poller struct {
	svc     Service
	timeout int
	logger  *slog.Logger
}

// PollerOption configura un Poller.
type PollerOption func(*Poller)

// WithPollTimeout reemplaza el long poll timeout (default 30).
func WithPollTimeout(timeout int) PollerOption {
	return func(p *Poller) { p.timeout = timeout }
}

// WithPollerLogger establece logger (default slog.Default()).
func WithPollerLogger(logger *slog.Logger) PollerOption {
	return func(p *Poller) { p.logger = logger }
}

// NewPoller crea un Poller contra la Service dada.
func NewPoller(svc Service, opts ...PollerOption) *Poller {
	p := &Poller{
		svc:     svc,
		timeout: defaultPollTimeout,
		logger:  slog.Default(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Run ejecuta el loop de polling hasta que ctx se cancela o ocurre un
// error fatal. Cada lote se entrega a onBatch (el Bus del backend) y
// el offset avanza recien despues de una respuesta exitosa.
func (p *Poller) Run(ctx context.Context, onBatch func([]Update)) error {
	offset := 0
	backoff := time.Second

	for {
		updates, err := p.svc.GetUpdates(ctx, offset, p.timeout, MVPAllowedUpdates)
		if err != nil {
			var rateErr *RateLimitError
			switch {
			case errors.Is(err, ErrInvalidToken), errors.Is(err, ErrWebhookConflict):
				// Errores fatales: reintentar no tiene sentido.
				return err
			case errors.As(err, &rateErr):
				updates, err = p.retryRateLimited(ctx, offset, rateErr)
				if err != nil {
					if ctx.Err() != nil {
						return nil // cancelado durante la espera
					}
					return err
				}
			case ctx.Err() != nil:
				return nil
			default:
				// Error transitorio: backoff sin avanzar el offset.
				p.logger.Warn("telegram poll error; backing off", "err", err, "backoff", backoff)
				if !sleepCtx(ctx, backoff) {
					return nil
				}
				backoff = min(backoff*2, 5*time.Second)
				continue
			}
		}

		backoff = time.Second // exito: reset del backoff

		if len(updates) > 0 {
			onBatch(updates)
			offset = int(updates[len(updates)-1].UpdateID + 1)
		}
	}
}

// retryRateLimited reintenta GetUpdates hasta maxRateLimitRetries,
// esperando el retry_after de cada 429. Devuelve el error fatal que
// corte la cadena, o un error de reintentos agotados.
func (p *Poller) retryRateLimited(ctx context.Context, offset int, first *RateLimitError) ([]Update, error) {
	rateErr := first
	for attempt := 1; attempt <= maxRateLimitRetries; attempt++ {
		p.logger.Warn("telegram rate limited; waiting", "retry_after", rateErr.RetryAfter, "attempt", attempt)
		if !sleepCtx(ctx, rateErr.RetryAfter) {
			return nil, ctx.Err()
		}

		updates, err := p.svc.GetUpdates(ctx, offset, p.timeout, MVPAllowedUpdates)
		if next := asRateLimit(err); next != nil {
			rateErr = next
			continue
		}
		return updates, err // exito, o error no-rate-limit (lo decide Run/upstream)
	}
	return nil, fmt.Errorf("telegram: rate limited after %d attempts: %w", maxRateLimitRetries, rateErr)
}

// asRateLimit devuelve el *RateLimitError envuelto en err, o nil.
func asRateLimit(err error) *RateLimitError {
	var rateErr *RateLimitError
	if errors.As(err, &rateErr) {
		return rateErr
	}
	return nil
}

// sleepCtx espera d o hasta que ctx se cancela; devuelve false si ctx
// se cancelo.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
