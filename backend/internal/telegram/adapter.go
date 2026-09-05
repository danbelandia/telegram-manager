package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const defaultBaseURL = "https://api.telegram.org"

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

// Adapter implementa Service contra la Bot API real. El token NUNCA se
// loguea: solo se usa para construir el path de la request.
type Adapter struct {
	token   string
	baseURL string
	client  *http.Client

	mu        sync.Mutex
	connected bool
	username  string
}

// NewAdapter crea un Adapter apuntando a la Bot API real.
func NewAdapter(token string, opts ...Option) *Adapter {
	a := &Adapter{
		token:   token,
		baseURL: defaultBaseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
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
	url := fmt.Sprintf("%s/bot%s/%s", a.baseURL, a.token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("telegram: build request: %w", err)
	}

	res, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTelegramUnavailable, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("telegram: read response: %w", err)
	}

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
		if envelope.ErrorCode == http.StatusUnauthorized {
			return ErrInvalidToken
		}
		return fmt.Errorf("telegram: api error %d: %s", envelope.ErrorCode, envelope.Description)
	}

	if result != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, result); err != nil {
			return fmt.Errorf("telegram: decode result: %w", err)
		}
	}
	return nil
}
