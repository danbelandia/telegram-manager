// Package events centraliza la entrega de updates de Telegram. El
// Poller (modo polling) y el handler webhook publican en un Bus; los
// consumidores de negocio se registran con Handle. El Bus es sincrono:
// Publish entrega en orden y solo retorna cuando todos los handlers
// terminaron.
package events

import (
	"sync"

	"github.com/telegram-manager/backend/internal/telegram"
)

// Handler procesa un update de Telegram.
type Handler func(*telegram.Update)

// Bus despacha updates a los handlers registrados.
type Bus struct {
	mu       sync.Mutex
	handlers []Handler
}

// NewBus crea un Bus vacio.
func NewBus() *Bus {
	return &Bus{}
}

// Handle registra un handler. No se puede registrar handlers mientras
// Publish esta en curso; en la practica los handlers se registran en
// startup, antes de arrancar el Poller o el servidor webhook.
func (b *Bus) Handle(h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, h)
}

// Publish entrega el update a todos los handlers, en orden de
// registro. Un handler que paniquee corta la entrega (panic propagado).
func (b *Bus) Publish(update *telegram.Update) {
	b.mu.Lock()
	handlers := append([]Handler(nil), b.handlers...)
	b.mu.Unlock()

	for _, h := range handlers {
		h(update)
	}
}
