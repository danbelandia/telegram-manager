package api

import (
	"context"
	"net/http"
)

// pinger es la minima vista de la base de datos que el API necesita.
// Declarada donde se consume; *sql.DB la satisface.
type pinger interface {
	PingContext(ctx context.Context) error
}

// Server agrupa las dependencias del API y su router.
type Server struct {
	db        pinger
	botStatus botStatusProvider
	mux       *http.ServeMux
}

// NewServer construye el handler HTTP del API.
// botStatus expone el estado cacheado del bot de Telegram; se declara
// aqui (lado consumidor), como pide la convencion de interfaces de Go.
func NewServer(db pinger, botStatus botStatusProvider, opts ...Option) *Server {
	s := &Server{
		db:        db,
		botStatus: botStatus,
		mux:       http.NewServeMux(),
	}
	s.routes()
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Option configura Server al construirse.
type Option func(*Server)

// WithWebhook monta POST /api/telegram/webhook, la entrada de eventos
// en modo webhook. Solo debe usarse cuando TELEGRAM_MODE=webhook: en
// polling el endpoint no existe.
func WithWebhook(publisher updatePublisher, secret string) Option {
	return func(s *Server) {
		s.mux.HandleFunc("POST /api/telegram/webhook", telegramWebhookHandler(publisher, secret))
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
}

// ServeHTTP hace que *Server sea un http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
