package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/telegram-manager/backend/internal/telegram"
	"github.com/telegram-manager/backend/internal/tenants"
)

// administratorStore es la vista minima de datos que el service
// necesita sobre admins. Declarada donde se consume; *Repository y los
// fakes de test la satisfacen.
type administratorStore interface {
	GetByUsername(ctx context.Context, username string) (Admin, error)
	GetByID(ctx context.Context, id int64) (Admin, error)
	UpdateLastLogin(ctx context.Context, id int64, at time.Time) error
}

// Service contiene la logica de negocio de auth: login, refresh y
// logout. Depende del repositorio (identidad) y del TokenManager
// (emision). No conoce HTTP.
type Service struct {
	repo   administratorStore
	tokens *TokenManager
}

// NewService construye el servicio de auth.
func NewService(repo administratorStore, tokens *TokenManager) *Service {
	return &Service{repo: repo, tokens: tokens}
}

// LoginResult es el resultado de un login exitoso.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	Admin        Admin
}

// Login valida username+password, emite access (15 min) y refresh
// (7 dias), y registra last_login_at. Devuelve ErrCredentialInvalid
// para username inexistente O password incorrecto (mismo error: no
// revela cual fallo).
func (s *Service) Login(ctx context.Context, username, password string) (LoginResult, error) {
	admin, err := s.repo.GetByUsername(ctx, username)
	if errors.Is(err, ErrNotFound) {
		return LoginResult{}, ErrCredentialInvalid
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf("auth: login: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return LoginResult{}, ErrCredentialInvalid
	}

	access, err := s.tokens.IssueAccess(admin)
	if err != nil {
		return LoginResult{}, fmt.Errorf("auth: login: issue access: %w", err)
	}
	refresh, err := s.tokens.IssueRefresh(admin)
	if err != nil {
		return LoginResult{}, fmt.Errorf("auth: login: issue refresh: %w", err)
	}
	if err := s.repo.UpdateLastLogin(ctx, admin.ID, time.Now()); err != nil {
		// No falla el login por un problema de telemetria; solo se
		// propaga el error real. Se loguea en capa superior.
		return LoginResult{}, fmt.Errorf("auth: login: update last login: %w", err)
	}

	return LoginResult{AccessToken: access, RefreshToken: refresh, Admin: admin}, nil
}

// Refresh valida el refresh token stateless y emite un nuevo access
// token. El refresh no rota: sigue valido hasta expirar (7 dias).
func (s *Service) Refresh(ctx context.Context, refreshToken string) (string, error) {
	claims, err := s.tokens.ParseRefresh(refreshToken)
	if err != nil {
		return "", fmt.Errorf("auth: refresh: %w", err)
	}

	// Re-verifica que el admin siga existiendo (por si fue eliminado).
	id, err := parseSubjectID(claims.Subject)
	if err != nil {
		return "", fmt.Errorf("auth: refresh: %w", err)
	}
	admin, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("auth: refresh: %w", err)
	}

	return s.tokens.IssueAccess(admin)
}

// Logout no mantiene estado (tokens stateless): la sesion se cierra en
// el cliente borrando la cookie. Metodo presente para la intencion.
func (s *Service) Logout() {}

// GetAdminByID devuelve un admin por su ID. Lo usan los handlers de
// tenant settings para validar la password antes de rotar el token.
func (s *Service) GetAdminByID(ctx context.Context, id int64) (*Admin, error) {
	admin, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get admin %d: %w", id, err)
	}
	return &admin, nil
}

func parseSubjectID(subject string) (int64, error) {
	var id int64
	if _, err := fmt.Sscanf(subject, "%d", &id); err != nil {
		return 0, fmt.Errorf("auth: invalid subject: %w", err)
	}
	return id, nil
}

// Errores de dominio del signup (el handler los mapea a §18).
var (
	// ErrSignupValidation: campos faltantes/vacios o password bajo el
	// minimo (400 VALIDATION_ERROR, sin TG ni DB).
	ErrSignupValidation = errors.New("auth: invalid signup input")
	// ErrSlugTaken: slug ya registrado (409 CONFLICT, sin TG ni DB).
	ErrSlugTaken = errors.New("auth: slug already taken")
	// ErrUsernameTaken: username en uso en cualquier tenant (Q1-a,
	// UNIQUE global; 409 CONFLICT, sin TG ni DB).
	ErrUsernameTaken = errors.New("auth: username already taken")
	// ErrInvalidBotToken: Telegram rechazo el token en getMe (502
	// TELEGRAM_ERROR, sin persistir nada).
	ErrInvalidBotToken = errors.New("auth: invalid bot token")
	// ErrSignupNotConfigured: sin TENANT_TOKEN_ENC_KEY no se puede
	// cifrar el token (500 INTERNAL_ERROR).
	ErrSignupNotConfigured = errors.New("auth: signup not configured (missing encryption key)")
)

// minSignupPasswordLength es la politica minima de password del signup.
// No existia politica previa en el repo; 8 es el minimo estandar.
const minSignupPasswordLength = 8

// BotValidator verifica un bot token contra Telegram y devuelve el
// username del bot (de getMe). Tipo-funcion para inyectar un fake en
// tests: jamas Bot API real en tests.
type BotValidator func(ctx context.Context, token string) (botUsername string, err error)

// ValidateBotToken es el BotValidator de produccion: getMe con un
// adapter efimero (Telegram es fuente de verdad).
func ValidateBotToken(ctx context.Context, token string) (string, error) {
	u, err := telegram.NewAdapter(token).GetMe(ctx)
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

// SignupInput es el alta de un tenant bot-per-tenant.
type SignupInput struct {
	Slug     string
	Username string
	Password string
	BotToken string
}

// isPgUniqueViolation detecta 23505 bajo pgx stdlib (carreras de
// slug/username entre el chequeo previo y el INSERT).
func isPgUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// SignupResult es lo que vuelve al panel: ids y nombres. El token
// NUNCA vuelve en la respuesta ni en logs.
type SignupResult struct {
	TenantID      int64
	TenantSlug    string
	AdminID       int64
	AdminUsername string
}

// SignupService da de alta tenants de forma transaccional (slice 0).
// Orden exacto (§3/D7): ① validar → ② slug libre → ③ username libre
// global → ④ getMe (fuera de la tx: no se retienen locks/conn DB
// durante una llamada de red) → ⑤ BEGIN: INSERT tenants (con token ya
// cifrado + bot_username) → INSERT admins → COMMIT. Cualquier fallo
// → rollback total, cero filas huerfanas.
type SignupService struct {
	db          *sql.DB
	tenants     *tenants.Repository
	admins      *Repository
	crypter     *tenants.Crypter
	validateBot BotValidator
}

// NewSignupService construye el servicio. crypter nil = signup
// deshabilitado (falta TENANT_TOKEN_ENC_KEY): Signup devuelve
// ErrSignupNotConfigured sin tocar TG ni DB.
func NewSignupService(db *sql.DB, tenantRepo *tenants.Repository, adminRepo *Repository, crypter *tenants.Crypter, validateBot BotValidator) *SignupService {
	return &SignupService{db: db, tenants: tenantRepo, admins: adminRepo, crypter: crypter, validateBot: validateBot}
}

// Signup ejecuta el alta. Los errores son los sentinels de arriba
// (comparar con errors.Is); cualquier otro es interno (5xx).
func (s *SignupService) Signup(ctx context.Context, in SignupInput) (SignupResult, error) {
	slug := strings.TrimSpace(in.Slug)
	username := strings.TrimSpace(in.Username)
	if slug == "" || len(slug) > 63 || username == "" ||
		len(in.Password) < minSignupPasswordLength || strings.TrimSpace(in.BotToken) == "" {
		return SignupResult{}, ErrSignupValidation
	}
	if s.crypter == nil {
		return SignupResult{}, ErrSignupNotConfigured
	}

	// ② Slug libre (previo a TG: no se valida token para un slug
	// ocupado).
	if _, err := s.tenants.GetBySlug(ctx, slug); err == nil {
		return SignupResult{}, ErrSlugTaken
	} else if !errors.Is(err, tenants.ErrNotFound) {
		return SignupResult{}, fmt.Errorf("auth: signup: check slug: %w", err)
	}

	// ③ Username libre global (Q1-a).
	if _, err := s.admins.GetByUsername(ctx, username); err == nil {
		return SignupResult{}, ErrUsernameTaken
	} else if !errors.Is(err, ErrNotFound) {
		return SignupResult{}, fmt.Errorf("auth: signup: check username: %w", err)
	}

	// ④ getMe ANTES de persistir: rechazo → 502 sin filas.
	botUsername, err := s.validateBot(ctx, in.BotToken)
	if err != nil {
		return SignupResult{}, fmt.Errorf("%w: %v", ErrInvalidBotToken, err)
	}

	// ⑤ Transaccion unica: tenants + admins o nada.
	enc, err := s.crypter.Encrypt([]byte(in.BotToken))
	if err != nil {
		return SignupResult{}, fmt.Errorf("auth: signup: encrypt token: %w", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcryptCost)
	if err != nil {
		return SignupResult{}, fmt.Errorf("auth: signup: hash password: %w", err)
	}
	var botUsernameParam *string
	if botUsername != "" {
		botUsernameParam = &botUsername
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SignupResult{}, fmt.Errorf("auth: signup: begin: %w", err)
	}
	// Rollback si el commit no ocurre (incluye panics: el driver lo
	// aborta al cerrar la tx abierta).
	defer func() { _ = tx.Rollback() }()

	var tenantID int64
	const insertTenant = `
INSERT INTO tenants (slug, tier, bot_token_encrypted, bot_username)
VALUES ($1, 'pro', $2, $3)
RETURNING id`
	if err := tx.QueryRowContext(ctx, insertTenant, slug, enc, botUsernameParam).Scan(&tenantID); err != nil {
		if isPgUniqueViolation(err) {
			// Carrera: otro signup tomo el slug entre ② y ahora.
			return SignupResult{}, ErrSlugTaken
		}
		return SignupResult{}, fmt.Errorf("auth: signup: insert tenant: %w", err)
	}

	var adminID int64
	const insertAdmin = `
INSERT INTO admins (username, password_hash, tenant_id)
VALUES ($1, $2, $3)
RETURNING id`
	if err := tx.QueryRowContext(ctx, insertAdmin, username, string(hash), tenantID).Scan(&adminID); err != nil {
		if isPgUniqueViolation(err) {
			// Carrera sobre username global.
			return SignupResult{}, ErrUsernameTaken
		}
		return SignupResult{}, fmt.Errorf("auth: signup: insert admin: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return SignupResult{}, fmt.Errorf("auth: signup: commit: %w", err)
	}
	return SignupResult{
		TenantID:      tenantID,
		TenantSlug:    slug,
		AdminID:       adminID,
		AdminUsername: username,
	}, nil
}
