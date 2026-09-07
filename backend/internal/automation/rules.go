package automation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/telegram-manager/backend/internal/telegram"
)

// Rule es la interfaz que toda regla de moderacion automatica debe
// implementar. Name() devuelve un identificador estable que se persiste
// en logs y metadata; Evaluate devuelve *RuleHit (no nil) cuando la
// regla se dispara para el mensaje.
//
// El contexto no se usa actualmente (las reglas son sincronas y
// locales) pero se mantiene por simetria con el resto del backend y
// para futures reglas que requieran I/O (ej. banned-words contra
// lista persistida).
type Rule interface {
	Name() string
	Evaluate(ctx context.Context, msg *telegram.Message, s *Settings, ws *WarningState, now time.Time) *RuleHit
}

// Registry es el set ordenado de reglas registradas. Short-circuit:
// Evaluate itera en orden de registro y retorna el PRIMER hit. Si
// ninguna regla dispara, retorna nil.
//
// El Registry es thread-safe: Register se llama en startup (una vez)
// y Evaluate se invoca desde el handler del bus (multiples goroutines
// del poller/webhook). La lista se congela en startup con freeze().
// En la practica las goroutines llaman Evaluate despues del freeze,
// asi que el read es data-race free.
type Registry struct {
	mu    sync.RWMutex
	rules []Rule
}

// NewRegistry crea un Registry vacio.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register agrega una regla al final del orden. Si la regla ya estaba
// registrada (por nombre) NO se duplica — queda la primera instancia.
// Llamar despues de la primera Evaluate es valido pero se imprime un
// warning en tests; en produccion se espera Register en startup.
func (r *Registry) Register(rule Rule) {
	if rule == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.rules {
		if existing.Name() == rule.Name() {
			return
		}
	}
	r.rules = append(r.rules, rule)
}

// Rules devuelve una copia de la lista actual (para tests / debug).
func (r *Registry) Rules() []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Rule, len(r.rules))
	copy(out, r.rules)
	return out
}

// Evaluate itera las reglas en orden y retorna el primer hit. Si
// ninguna dispara, retorna nil. Short-circuit: una vez que una regla
// devuelve *RuleHit, las siguientes no se evaluan.
//
// Si el mensaje es nil, retorna nil (no panic).
func (r *Registry) Evaluate(ctx context.Context, msg *telegram.Message, s *Settings, ws *WarningState, now time.Time) *RuleHit {
	if msg == nil {
		return nil
	}
	r.mu.RLock()
	rules := make([]Rule, len(r.rules))
	copy(rules, r.rules)
	r.mu.RUnlock()

	for _, rule := range rules {
		if hit := rule.Evaluate(ctx, msg, s, ws, now); hit != nil {
			return hit
		}
	}
	return nil
}

// --- FloodRule ---

// FloodRule detecta cuando un mismo usuario envia al menos
// `flood_messages` mensajes dentro de `flood_seconds` segundos en el
// mismo grupo.
//
// Estado: map[groupID]map[userID][]time.Time — ventana deslizante en
// memoria. NO se persiste entre restarts (acceptable per spec REQ-6:
// "reset on process start"). Cada grupo tiene su propio mutex para
// que la concurrencia entre grupos no se serialice; dentro de un
// grupo, un solo evaluate a la vez para ese grupo.
//
// Mecanica de pruning: ANTES de append, Evaluate descarta los
// timestamps fuera de la ventana (now - flood_seconds). Asi el buffer
// se mantiene acotado al ancho de la ventana +1. Cuando un usuario
// deja de estar activo, su []time.Time queda vacio pero NO se borra
// del map (costo de lookup vs costo de alloc). Esto es intencional:
// evita allocaciones por mensaje; el map crece solo con usuarios
// activos.
type FloodRule struct {
	mu       sync.Mutex
	counters map[int64]map[int64][]time.Time // groupID -> userID -> ventana
}

// NewFloodRule construye una FloodRule con su estado interno vacio.
func NewFloodRule() *FloodRule {
	return &FloodRule{counters: make(map[int64]map[int64][]time.Time)}
}

// Name devuelve el identificador estable para logs/metadata.
func (f *FloodRule) Name() string { return "flood" }

// Evaluate registra el mensaje actual en la ventana del (grupo, usuario)
// y retorna hit=true si el tamano de la ventana post-append es >=
// flood_messages.
//
// Pre-condicion: msg.From != nil (el caller Service.HandleMessage ya
// filtro mensajes sin From antes de invocar la regla).
//
// Si settings es nil o flood_enabled=false, retorna nil — el caller
// (Service.HandleMessage) ya filtra por enabled, pero la regla es
// defensiva para uso standalone.
func (f *FloodRule) Evaluate(ctx context.Context, msg *telegram.Message, s *Settings, ws *WarningState, now time.Time) *RuleHit {
	if s == nil || !s.FloodEnabled {
		return nil
	}
	if msg == nil || msg.From == nil || msg.From.ID == 0 {
		return nil
	}

	windowSec := time.Duration(s.FloodSeconds) * time.Second
	cutoff := now.Add(-windowSec)

	f.mu.Lock()
	defer f.mu.Unlock()

	groupBucket, ok := f.counters[msg.Chat.ID]
	if !ok {
		groupBucket = make(map[int64][]time.Time)
		f.counters[msg.Chat.ID] = groupBucket
	}
	window := groupBucket[msg.From.ID]

	// Pruning: descartar timestamps fuera de la ventana. La ventana es
	// ordenada por orden de insercion (cada Evaluate es un append), asi
	// que alcanza con avanzar el indice hasta el primer timestamp dentro
	// de la ventana.
	pruned := window[:0]
	for _, ts := range window {
		if ts.After(cutoff) || ts.Equal(cutoff) {
			pruned = append(pruned, ts)
		}
	}
	window = pruned

	// Append del mensaje actual.
	window = append(window, now)
	groupBucket[msg.From.ID] = window

	if int16(len(window)) >= s.FloodMessages {
		reason := fmt.Sprintf("user sent %d messages in %d seconds",
			len(window), s.FloodSeconds)
		return &RuleHit{RuleName: f.Name(), Reason: reason}
	}
	return nil
}

// resetForTest limpia toda la ventana. Solo se usa en tests; no se
// exporta del paquete en runtime.
func (f *FloodRule) resetForTest() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counters = make(map[int64]map[int64][]time.Time)
}
