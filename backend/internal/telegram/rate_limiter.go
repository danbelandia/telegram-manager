package telegram

import (
	"context"
	"sync"
	"time"
)

// tokenBucket es el rate limiter del adapter (AGENTS.md §18.1). Cada
// request de la Bot API consume un token; los tokens se reponen a rate
// tokens/segundo hasta un maximo de burst. Es un canal interno de Go,
// sin infraestructura externa (sin Redis).
type tokenBucket struct {
	mu      sync.Mutex
	rate    float64 // tokens por segundo
	burst   float64 // capacidad maxima (tokens acumulables)
	tokens  float64 // tokens disponibles
	updated time.Time
}

func newTokenBucket(rate, burst float64) *tokenBucket {
	return &tokenBucket{
		rate:    rate,
		burst:   burst,
		tokens:  burst,
		updated: time.Now(),
	}
}

// wait consume un token, bloqueando hasta que haya uno disponible o el
// ctx se cancele. Es el unico punto donde el adapter espera por rate
// limit; los reintentos ante 429 los maneja doWithRetry.
func (b *tokenBucket) wait(ctx context.Context) error {
	for {
		b.mu.Lock()
		now := time.Now()
		// Reponer tokens segun el tiempo transcurrido.
		b.tokens += now.Sub(b.updated).Seconds() * b.rate
		if b.tokens > b.burst {
			b.tokens = b.burst
		}
		b.updated = now

		if b.tokens >= 1 {
			b.tokens--
			b.mu.Unlock()
			return nil
		}

		// Cuanto falta para el siguiente token.
		need := (1 - b.tokens) / b.rate
		duration := time.Duration(need * float64(time.Second))
		b.mu.Unlock()

		select {
		case <-time.After(duration):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
