package automation

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/telegram-manager/backend/internal/telegram"
)

// Rule es la interfaz que toda regla de moderacion automatica debe
// implementar. Name() devuelve un identificador estable que se persiste
// en logs y metadata; Evaluate devuelve *RuleHit (no nil) cuando la
// regla se dispara para el mensaje.
//
// Slice 2: la firma extendida agrega `lists *Lists` entre `ws` y `now`
// para que el Service pueda pre-cargar banned-words y link-allowlist
// una vez por mensaje (evita N queries por rule). El Service puede
// pasar nil si los toggles dependientes estan off — las reglas son
// defensivas (vuelven nil si lists es nil y necesitan una lista).
//
// El contexto no se usa actualmente (las reglas son sincronas y
// locales) pero se mantiene por simetria con el resto del backend y
// para futures reglas que requieran I/O.
type Rule interface {
	Name() string
	Evaluate(ctx context.Context, msg *telegram.Message, s *Settings, ws *WarningState, lists *Lists, now time.Time) *RuleHit
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
func (r *Registry) Evaluate(ctx context.Context, msg *telegram.Message, s *Settings, ws *WarningState, lists *Lists, now time.Time) *RuleHit {
	if msg == nil {
		return nil
	}
	r.mu.RLock()
	rules := make([]Rule, len(r.rules))
	copy(rules, r.rules)
	r.mu.RUnlock()

	for _, rule := range rules {
		if hit := rule.Evaluate(ctx, msg, s, ws, lists, now); hit != nil {
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
//
// Slice 2: acepta `lists *Lists` por la firma extendida pero lo ignora
// (la regla FloodRule es 100% in-mem sobre la ventana deslizante).
func (f *FloodRule) Evaluate(ctx context.Context, msg *telegram.Message, s *Settings, ws *WarningState, lists *Lists, now time.Time) *RuleHit {
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

// --- AntiSpamRule ---

// AntiSpamRule detecta 3 patrones clasicos de spam (spec REQ-9,
// AGENTS §23):
//
//  1. ALL CAPS: mas del 70% de letras son mayusculas en un mensaje de
//     longitud > 10 (el resto se ignora — solo letras cuentan).
//  2. Caracteres repetidos: 5 o mas caracteres iguales consecutivos
//     (ej. "nooooooo", "!!!!!!").
//  3. URLs muy cortas: una URL http(s)://t.me/|telegram.me/ con
//     < 20 caracteres en el cuerpo (despues del esquema/dominio base)
//     suele ser un enlace malicioso disfrazado (t.me/x con 1 char).
//
// Stateless: sin estado mutable, sin mutex, sin I/O. Solo opera sobre
// el texto del mensaje. El Service filtra por anti_spam_enabled y por
// msg.Text no vacio antes de invocar, pero la regla es defensiva
// (vuelve nil si settings nil, toggle off o texto < 10 chars para el
// detector de all-caps).
type AntiSpamRule struct{}

// NewAntiSpamRule construye una instancia. Stateless: las pruebas
// pueden crear multiples instancias sin colisiones.
func NewAntiSpamRule() *AntiSpamRule { return &AntiSpamRule{} }

// Name devuelve el identificador estable para logs/metadata.
func (a *AntiSpamRule) Name() string { return "anti_spam" }

// Evaluate aplica los 3 sub-detectores en orden. Si el settings es
// nil o anti_spam_enabled=false → nil. Si el texto es vacio → nil.
// Si texto < 10 chars → no se evalua all-caps (umbral del detector).
// Cada hit reporta el sub-detector especifico en Reason.
func (a *AntiSpamRule) Evaluate(ctx context.Context, msg *telegram.Message, s *Settings, _ *WarningState, _ *Lists, _ time.Time) *RuleHit {
	if s == nil || !s.AntiSpamEnabled {
		return nil
	}
	if msg == nil || msg.Text == "" {
		return nil
	}

	text := msg.Text

	if hit := detectAllCaps(text); hit != "" {
		return &RuleHit{RuleName: a.Name(), Reason: hit}
	}
	if hit := detectRepeatedChars(text); hit != "" {
		return &RuleHit{RuleName: a.Name(), Reason: hit}
	}
	if hit := detectShortURL(text); hit != "" {
		return &RuleHit{RuleName: a.Name(), Reason: hit}
	}
	return nil
}

// detectAllCaps: >70% letras mayusculas en texto > 10 chars.
// Solo letras cuentan para el ratio (digitos/espacios/simbolos no).
func detectAllCaps(text string) string {
	if len(text) <= 10 {
		return ""
	}
	var letters, upper int
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				upper++
			}
		}
	}
	// Sin letras -> no es spam por all-caps (puede ser numeros/simbolos).
	if letters == 0 {
		return ""
	}
	// Ratio en int (porcentaje sobre 100) para evitar float.
	if upper*100 >= letters*70 {
		return "message is mostly uppercase letters"
	}
	return ""
}

// detectRepeatedChars: 5+ caracteres iguales consecutivos (no
// necesariamente letras). Evita falsos positivos en lenguajes con
// repeticiones legitimas (ej. "jajajaja" en espanol) usando el umbral
// 5 estricto (no 3 ni 4).
func detectRepeatedChars(text string) string {
	if len(text) < 5 {
		return ""
	}
	// Usamos []rune para soportar unicode multi-byte.
	runes := []rune(text)
	for i := 0; i+5 <= len(runes); i++ {
		c := runes[i]
		same := true
		for j := 1; j < 5; j++ {
			if runes[i+j] != c {
				same = false
				break
			}
		}
		if same {
			return "message has 5 or more repeated characters in a row"
		}
	}
	return ""
}

// shortURLRegex: detecta prefijos de URL clasicos. El detector
// "shortURL" mide el CUERPO del enlace (lo que viene despues de
// t.me/ | telegram.me/ | http(s)://). Si el cuerpo tiene < 20 chars,
// suele ser un link malicioso (t.me/x) o un acortador evadiendo el
// detector de longitud.
var shortURLRegex = regexp.MustCompile(`(?:https?://|t\.me/|telegram\.me/)([^\s]+)`)

// detectShortURL: extrae la URL y mide el cuerpo. < 20 chars → hit.
// Si la URL esta sola o rodeada de whitespace, igual matchea.
func detectShortURL(text string) string {
	matches := shortURLRegex.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		body := strings.Trim(m[1], ".,;:!?)")
		if len([]rune(body)) > 0 && len([]rune(body)) < 20 {
			return "message contains a very short URL (likely spam)"
		}
	}
	return ""
}

// --- AntiLinkRule ---

// AntiLinkRule detecta URLs en el texto del mensaje y reporta hit si
// el dominio NO esta en la link_allowlist del grupo (pre-cargada por
// el Service en `lists.LinkAllowlist`).
//
// Dominios aceptados:
//
//   - Dominio exacto: "example.com" matchea example.com.
//   - Subdominio: "example.com" matchea sub.example.com, a.b.example.com.
//   - NO matchea: "notexample.com" (sin punto previo).
//
// Esta politica sigue el helper `domainMatches` (decision D3 del
// design) que exige "." antes del allow domain. El matcher lowercases
// ambos lados para que "Example.COM" y "example.com" matcheen.
type AntiLinkRule struct{}

// NewAntiLinkRule construye una instancia. Stateless.
func NewAntiLinkRule() *AntiLinkRule { return &AntiLinkRule{} }

// Name devuelve el identificador estable para logs/metadata.
func (a *AntiLinkRule) Name() string { return "anti_link" }

// antiLinkURLRegex: URLs http(s) o dominios t.me/telegram.me. El
// grupo de captura 1 es la URL completa; la extraccion del host la
// hace extractHost mas abajo.
var antiLinkURLRegex = regexp.MustCompile(`(?i)\b(?:https?://[^\s]+|t\.me/[^\s]+|telegram\.me/[^\s]+)`)

// Evaluate: si settings es nil o anti_link_enabled=false → nil.
// Si no hay listas o la lista esta vacia → CUALQUIER URL es hit
// (politica permisiva: sin allowlist → todo se considera link no
// autorizado, esperando que el admin configure la allowlist).
// Si la allowlist tiene al menos un dominio → hit solo si NINGUN
// dominio de la URL matchea la allowlist (helper domainMatches).
func (a *AntiLinkRule) Evaluate(ctx context.Context, msg *telegram.Message, s *Settings, _ *WarningState, lists *Lists, _ time.Time) *RuleHit {
	if s == nil || !s.AntiLinkEnabled {
		return nil
	}
	if msg == nil || msg.Text == "" {
		return nil
	}

	matches := antiLinkURLRegex.FindAllString(msg.Text, -1)
	if len(matches) == 0 {
		return nil
	}

	allowlist := extractAllowlist(lists)
	for _, raw := range matches {
		host := extractHost(raw)
		if host == "" {
			continue
		}
		if !domainAllowed(host, allowlist) {
			return &RuleHit{
				RuleName: a.Name(),
				Reason:   "message contains a link to a non-allowlisted domain: " + host,
			}
		}
	}
	return nil
}

// extractAllowlist devuelve la allowlist de lists, defensivo ante nil.
func extractAllowlist(lists *Lists) []string {
	if lists == nil {
		return nil
	}
	return lists.LinkAllowlist
}

// extractHost parsea el host de una URL cruda. Para http(s):// usa
// net/url; para t.me/X devuelve "t.me" como host base (t.me/foo →
// host="t.me"). Si falla el parse, devuelve "" para que el caller
// ignore.
func extractHost(raw string) string {
	trimmed := strings.TrimRight(raw, ".,;:!?)")
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		u, err := url.Parse(trimmed)
		if err == nil && u.Host != "" {
			return strings.ToLower(u.Host)
		}
	}
	for _, prefix := range []string{"t.me/", "telegram.me/"} {
		if strings.HasPrefix(trimmed, prefix) {
			return strings.ToLower(strings.TrimSuffix(prefix, "/"))
		}
	}
	// Bare host (ej. "example.com" sin scheme): tratarlo como host.
	if !strings.Contains(trimmed, " ") && strings.Contains(trimmed, ".") {
		return strings.ToLower(trimmed)
	}
	return ""
}

// domainAllowed: helper. Acepta dominio exacto y cualquier subdominio
// (suffix con "."). notexample.com NO matchea example.com.
func domainAllowed(host string, allowlist []string) bool {
	for _, a := range allowlist {
		if domainMatches(a, host) {
			return true
		}
	}
	return false
}

// domainMatches: si `allow` es vacio o `msgHost` es vacio → false.
// Coincidencia exacta (case-insensitive) o subdominio (suffix ".<allow>").
func domainMatches(allow, msgHost string) bool {
	allow = strings.ToLower(strings.TrimSpace(allow))
	msgHost = strings.ToLower(strings.TrimSpace(msgHost))
	if allow == "" || msgHost == "" {
		return false
	}
	if msgHost == allow {
		return true
	}
	return strings.HasSuffix(msgHost, "."+allow)
}

// --- BannedWordsRule ---

// BannedWordsRule detecta si el texto del mensaje contiene alguna de
// las palabras en `lists.BannedWords` (case-insensitive substring
// match, spec REQ-11). El Service pre-carga la lista por mensaje y la
// pasa via `lists.BannedWords`; si la lista es nil o vacia, la regla
// vuelve nil sin evaluar (optimizacion: no se llama a Contains).
//
// IMPORTANTE: como banned_words se persiste con LOWER() (migracion +
// handler), el matcher lowercase el texto y compara contra la lista
// ya normalizada. Primer match determina el Reason (palabra que
// matcheo + tipo "banned_word").
type BannedWordsRule struct{}

// NewBannedWordsRule construye una instancia. Stateless.
func NewBannedWordsRule() *BannedWordsRule { return &BannedWordsRule{} }

// Name devuelve el identificador estable para logs/metadata.
func (b *BannedWordsRule) Name() string { return "banned_words" }

// Evaluate: si settings nil o banned_words_enabled=false → nil.
// Si no hay listas, lista vacia o texto vacio → nil. Si el texto
// contiene alguna palabra (case-insensitive substring) → hit con
// Reason mencionando la palabra matcheada.
func (b *BannedWordsRule) Evaluate(ctx context.Context, msg *telegram.Message, s *Settings, _ *WarningState, lists *Lists, _ time.Time) *RuleHit {
	if s == nil || !s.BannedWordsEnabled {
		return nil
	}
	if msg == nil || msg.Text == "" || lists == nil || len(lists.BannedWords) == 0 {
		return nil
	}

	textLower := strings.ToLower(msg.Text)
	for _, w := range lists.BannedWords {
		wordLower := strings.ToLower(strings.TrimSpace(w))
		if wordLower == "" {
			continue
		}
		if strings.Contains(textLower, wordLower) {
			return &RuleHit{
				RuleName: b.Name(),
				Reason:   "message contains a banned word: " + wordLower,
			}
		}
	}
	return nil
}
