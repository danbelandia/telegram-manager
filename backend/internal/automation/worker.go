package automation

import (
	"context"
	"log/slog"

	"github.com/telegram-manager/backend/internal/events"
	"github.com/telegram-manager/backend/internal/telegram"
)

// Worker drena el canal `autoActionCh` en orden FIFO y delega cada
// AutoAction al AutoActioner (que re-checkea permissionOkAdmin y
// llama tg.MuteUser/BanUser con token bucket del adapter, §18.1).
//
// El Worker NO salta al adapter directamente: lo hace via el
// AutoActioner para que toda la logica de re-check + log + mapeo de
// errores viva en un solo lugar.
type Worker struct {
	ch       <-chan AutoAction
	actioner AutoActioner
	logger   *slog.Logger
}

// NewWorker construye el worker con sus dependencias. logger puede
// ser nil (usa slog.Default()).
func NewWorker(ch <-chan AutoAction, actioner AutoActioner, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{ch: ch, actioner: actioner, logger: logger}
}

// Run ejecuta el loop hasta que ctx.Done() se dispara. Retorna nil
// siempre (cancelacion limpia); errores del AutoActioner se loguean y
// el siguiente item se procesa (spec REQ-7: errores no rompen el
// worker).
//
// Mientras ctx no este cancelado, el worker:
//   - lee del canal
//   - llama actioner.Execute(ctx, action)
//   - loguea si hubo error
//
// Si el canal se cierra antes de ctx.Done(), Run retorna nil igual
// (no es un error; es una senal de shutdown).
func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("automation worker started")
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("automation worker stopping")
			return nil
		case action, ok := <-w.ch:
			if !ok {
				// Canal cerrado (shutdown limpio del main). El siguiente
				// select ya no recibira mas items; ctx.Done() gatilla
				// despues. Por las dudas retornamos nil.
				w.logger.Info("automation worker: channel closed")
				return nil
			}
			if err := w.actioner.Execute(ctx, action); err != nil {
				w.logger.Warn("automation worker: dispatch failed",
					"kind", string(action.Kind),
					"group_id", action.GroupID,
					"user_id", action.UserID,
					"rule", action.RuleName,
					"error", err)
			}
		}
	}
}

// ProcessOnce es un helper para tests: procesa UN item del canal
// respetando ctx. Util cuando un test quiere inyectar un item y
// verificar la ejecucion sin tener que correr el goroutine.
func (w *Worker) ProcessOnce(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case action, ok := <-w.ch:
		if !ok {
			return nil
		}
		return w.actioner.Execute(ctx, action)
	}
}

// --- Events Subscriber ---

// Subscriber conecta el modulo automation al events.Bus. Filtra
// updates que NO son Message (chat_member, my_chat_member,
// chat_join_request) y updates con Message.From nil (sistema, canal).
// Para los updates validos, delega a Service.HandleMessage.
//
// Es el UNICO consumer de automation en el bus; se registra con
// bus.Handle(subscriber.Handle) en cmd/server/main.go.
type Subscriber struct {
	bus    *events.Bus
	svc    *Service
	logger *slog.Logger
}

// NewSubscriber construye el subscriber. logger puede ser nil.
func NewSubscriber(bus *events.Bus, svc *Service, logger *slog.Logger) *Subscriber {
	if logger == nil {
		logger = slog.Default()
	}
	return &Subscriber{bus: bus, svc: svc, logger: logger}
}

// Register conecta Handle al bus. Llamar UNA vez en startup,
// despues de los handlers existentes (logging, groups, joinrequests).
func (s *Subscriber) Register() {
	s.bus.Handle(s.Handle)
}

// Handle es el handler que el bus invoca por cada Update.
// Filtra:
//   - u == nil
//   - u.Message == nil (no es un mensaje: chat_member, etc.)
//   - u.Message.From == nil (canal sin autor)
//
// Para los updates validos, llama svc.HandleMessage. Los errores se
// loguean y NO se propagan al bus (los demas handlers deben seguir
// recibiendo el update).
func (s *Subscriber) Handle(u *telegram.Update) {
	if u == nil || u.Message == nil || u.Message.From == nil || u.Message.From.ID == 0 {
		return
	}
	ctx := contextFromUpdate(u)
	if err := s.svc.HandleMessage(ctx, u.Message); err != nil {
		s.logger.Error("automation: subscriber.Handle failed",
			"update_id", u.UpdateID,
			"group_id", u.Message.Chat.ID,
			"user_id", u.Message.From.ID,
			"error", err)
	}
}

// contextFromUpdate devuelve un context.Background(). El contexto real
// del bus (con cancelation del main) deberia pasarse en slice 2; por
// ahora slice 1 lo trata como fire-and-forget (las llamadas a la DB
// respetan sus propios timeouts via el pool de pgx).
func contextFromUpdate(_ *telegram.Update) context.Context {
	return context.Background()
}
