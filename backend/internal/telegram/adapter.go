package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://api.telegram.org"

// defaultRequestTimeout aplica a llamadas sin deadline propio (getMe,
// setWebhook...). El long poll NO usa este valor: GetUpdates deriva su
// propio deadline de timeout+5s (ver abajo).
const defaultRequestTimeout = 10 * time.Second

// Limites del rate limiter (AGENTS.md §18.1): ~25 req/seg globales
// dejan margen sobre el limite real de la Bot API (~30 req/seg).
const (
	defaultRateLimitPerSecond = 25.0
	defaultRateLimitBurst     = 25.0
)

// Option configura la creacion de un Adapter (usado en tests).
type Option func(*Adapter)

// WithBaseURL reemplaza la URL base de la Bot API (para httptest).
func WithBaseURL(url string) Option {
	return func(a *Adapter) { a.baseURL = url }
}

// WithHTTPClient reemplaza el cliente HTTP (para httptest).
func WithHTTPClient(c *http.Client) Option {
	return func(a *Adapter) { a.client = c }
}

// WithRateLimiter ajusta el token bucket. En tests se usa un rate alto
// para no bloquear; en produccion quedan los defaults de §18.1.
func WithRateLimiter(rate, burst float64) Option {
	return func(a *Adapter) { a.limit = newTokenBucket(rate, burst) }
}

// Adapter implementa Service contra la Bot API real. El token NUNCA se
// loguea: solo se usa para construir el path de la request.
type Adapter struct {
	token   string
	baseURL string
	client  *http.Client

	// limit es el token bucket global (§18.1); el resto del backend no
	// debe preocuparse por limites de la Bot API (seccion 15 del spec).
	limit *tokenBucket

	mu        sync.Mutex
	connected bool
	username  string
}

// NewAdapter crea un Adapter apuntando a la Bot API real.
// El client NO tiene Timeout global: los cortes se manejan por contexto
// (defaultRequestTimeout para llamadas normales, timeout+5s para el
// long poll), porque un Timeout fijo truncaria el long poll de 30s.
func NewAdapter(token string, opts ...Option) *Adapter {
	a := &Adapter{
		token:   token,
		baseURL: defaultBaseURL,
		client:  &http.Client{},
		limit:   newTokenBucket(defaultRateLimitPerSecond, defaultRateLimitBurst),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Status devuelve el estado cacheado de la conexion del bot, seteado
// por la ultima llamada GetMe exitosa. Es la vista que usa /api/health.
func (a *Adapter) Status() (connected bool, username string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.connected, a.username
}

// GetMe valida el token contra Telegram y cachea el estado del bot.
func (a *Adapter) GetMe(ctx context.Context) (BotUser, error) {
	var result struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
	}
	if err := a.doGet(ctx, "getMe", &result); err != nil {
		return BotUser{}, err
	}

	user := BotUser{ID: result.ID, Username: result.Username, FirstName: result.FirstName}

	a.mu.Lock()
	a.connected = true
	a.username = user.Username
	a.mu.Unlock()

	return user, nil
}

// doGet ejecuta un GET contra un metodo de la Bot API y decodifica el
// envelope estandar {ok, description, error_code, result}.
func (a *Adapter) doGet(ctx context.Context, method string, result any) error {
	return a.doGetQuery(ctx, method, nil, result)
}

// doGetQuery es doGet con query string. La Bot API admite parametros
// por query en GET; se usa para getUpdates/setWebhook/deleteWebhook.
func (a *Adapter) doGetQuery(ctx context.Context, method string, q url.Values, result any) error {
	if err := a.limit.wait(ctx); err != nil {
		return err
	}

	// Deadlines por llamada, no en el client: si el ctx no trae uno de
	// su propio (long poll), aplica el default para no colgarse.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultRequestTimeout)
		defer cancel()
	}

	u := fmt.Sprintf("%s/bot%s/%s", a.baseURL, a.token, method)
	if len(q) > 0 {
		u += "?" + q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("telegram: build request: %w", err)
	}

	body, err := a.doRequest(req)
	if err != nil {
		return err
	}
	return a.handleEnvelope(body, result)
}

// doPost ejecuta un POST con body JSON contra un metodo de la Bot API y
// decodifica el envelope estandar. Es el transporte de las acciones de
// moderacion (banChatMember, restrictChatMember, etc.): los parametros
// anidados (ChatPermissions) no son seguros en query string.
func (a *Adapter) doPost(ctx context.Context, method string, body any, result any) error {
	if err := a.limit.wait(ctx); err != nil {
		return err
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultRequestTimeout)
		defer cancel()
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("telegram: encode body: %w", err)
	}

	u := fmt.Sprintf("%s/bot%s/%s", a.baseURL, a.token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("telegram: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	respBody, err := a.doRequest(req)
	if err != nil {
		return err
	}
	return a.handleEnvelope(respBody, result)
}

// doRequest ejecuta la request y devuelve el body (max 1 MB). Separa
// errores de red del manejo del envelope de la Bot API.
func (a *Adapter) doRequest(req *http.Request) ([]byte, error) {
	res, err := a.client.Do(req)
	if err != nil {
		// El *url.Error incluye la URL COMPLETA, con el token dentro
		// (bot<TOKEN>/...). Nunca debe llegar a logs: se extrae solo
		// la causa (context deadline exceeded, connection refused...).
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, fmt.Errorf("%w: %v", ErrTelegramUnavailable, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("telegram: read response: %w", err)
	}
	return body, nil
}

// handleEnvelope parsea el envelope estandar y mapea los errores de la
// Bot API a errores de dominio (docs/telegram_api_reference.md §8).
func (a *Adapter) handleEnvelope(body []byte, result any) error {
	var envelope struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		ErrorCode   int             `json:"error_code"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("telegram: decode response: %w", err)
	}

	if !envelope.OK {
		switch envelope.ErrorCode {
		case http.StatusUnauthorized:
			return ErrInvalidToken
		case http.StatusTooManyRequests:
			var errBody struct {
				Parameters struct {
					RetryAfter int `json:"retry_after"`
				} `json:"parameters"`
			}
			// Si Telegram no mando retry_after, el default es 0 (instantaneo).
			_ = json.Unmarshal(body, &errBody)
			return &RateLimitError{RetryAfter: time.Duration(errBody.Parameters.RetryAfter) * time.Second}
		case http.StatusConflict:
			return ErrWebhookConflict
		case http.StatusForbidden:
			return ErrPermissionDenied
		case http.StatusNotFound:
			return ErrTelegramNotFound
		case http.StatusBadRequest:
			// 400 mezcla validacion y "accion imposible"; el mapeo
			// depende del description (referencia §8).
			desc := strings.ToLower(envelope.Description)
			switch {
			case strings.Contains(desc, "not found"):
				return ErrTelegramNotFound
			case strings.Contains(desc, "rights"), strings.Contains(desc, "permission"):
				return ErrPermissionDenied
			default:
				return &TelegramAPIError{Code: envelope.ErrorCode, Description: envelope.Description}
			}
		}
		return &TelegramAPIError{Code: envelope.ErrorCode, Description: envelope.Description}
	}

	if result != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, result); err != nil {
			return fmt.Errorf("telegram: decode result: %w", err)
		}
	}
	return nil
}

// doWithRetry ejecuta la llamada y, ante un 429 con retry_after, espera
// ese tiempo y reintenta hasta maxRateLimitRetries veces (§18.1). Si el
// retry_after es 0 (Telegram no lo mando) o el error no es de rate
// limit, devuelve sin reintentar: nunca se reintenta a ciegas.
func (a *Adapter) doWithRetry(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 0; attempt <= maxRateLimitRetries; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		var rateErr *RateLimitError
		if !errors.As(err, &rateErr) || rateErr.RetryAfter <= 0 {
			return err
		}
		select {
		case <-time.After(rateErr.RetryAfter):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

// GetUpdates hace long polling contra la Bot API. offset>0 confirma
// updates previos; timeout es el long poll en segundos; allowed son los
// tipos de update a recibir (MVPAllowedUpdates).
//
// El deadline del ctx se deriva de timeout: Telegram mantiene la
// respuesta abierta hasta timeout segundos, asi que el HTTP debe poder
// esperar mas que el defaultRequestTimeout.
func (a *Adapter) GetUpdates(ctx context.Context, offset, timeout int, allowed []string) ([]Update, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout+5)*time.Second)
		defer cancel()
	}

	q := url.Values{}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	if timeout > 0 {
		q.Set("timeout", strconv.Itoa(timeout))
	}
	if len(allowed) > 0 {
		q.Set("allowed_updates", mustJSON(allowed))
	}

	var result []Update
	if err := a.doGetQuery(ctx, "getUpdates", q, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetWebhook registra el webhook de produccion. Con secret_token
// habilitado, Telegram lo envia en X-Telegram-Bot-Api-Secret-Token y el
// backend debe validarlo antes de procesar (ver api/webhook.go).
// drop_pending_updates=true evita re-procesar updates acumulados del
// polling.
func (a *Adapter) SetWebhook(ctx context.Context, webhookURL, secret string, allowed []string) error {
	q := url.Values{}
	q.Set("url", webhookURL)
	if secret != "" {
		q.Set("secret_token", secret)
	}
	if len(allowed) > 0 {
		q.Set("allowed_updates", mustJSON(allowed))
	}
	q.Set("drop_pending_updates", "true")

	return a.doGetQuery(ctx, "setWebhook", q, nil)
}

// DeleteWebhook elimina el webhook actual. Necesario para volver a
// polling (getUpdates responde 409 mientras haya webhook).
func (a *Adapter) DeleteWebhook(ctx context.Context) error {
	return a.doGetQuery(ctx, "deleteWebhook", nil, nil)
}

// mustJSON serializa allowed para la query string. Ante un error nunca
// alcanzable con []string, devuelve "[]".
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}
