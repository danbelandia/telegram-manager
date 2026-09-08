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
	db           pinger
	botStatus    botStatusProvider
	auth         authenticator
	authSvc      authService
	cookieSecure bool
	mux          *http.ServeMux

	// Alta de tenants bot-per-tenant (slice 0). signup ejecuta la
	// transaccion; onTenantReady levanta el poller en caliente tras el
	// commit (lo provee main con el registry). Nils = signup apagado.
	signup        signupService
	onTenantReady func(ctx context.Context, tenantID int64, tokenPlain string) error

	// Modulos del paso 10 (moderacion). Se inyectan via options; cada
	// handler verifica que su modulo este habilitado antes de actuar.
	groups       groupStore
	groupUsers   groupUsersLookup
	moderation   moderationActions
	joinRequests joinRequestStore
	logStore     logStore

	// Modulo de publicaciones (Fase 2, slice 1).
	publications publicationStore

	// Modulo de moderacion automatica (Fase 3, slice 2): settings +
	// listas. Los handlers reciben el Service (que expone los metodos
	// de settings + listas) y el logs.Repository para auditoria manual.
	automation       automationService
	automationLogs   automationLogWriter
	automationGroups automationGroupChecker
	// Slice 3 — dashboard de moderacion. NO pasa por Service para
	// mantener la arquitectura de slices 1+2+2.1 intacta: el handler
	// consume el repo directamente. Ver automationDashboardRepo en
	// automation_handlers.go.
	automationDashboard automationDashboardRepo
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

// WithAuth monta las rutas de autenticacion del panel (login, refresh,
// logout, me) con el servicio y flags de cookie. Sin esto, el Server no
// sabe autenticar: solo health/webhook.
func WithAuth(svc authService, verifier authenticator, cookieSecure bool) Option {
	return func(s *Server) {
		s.authSvc = svc
		s.auth = verifier
		s.cookieSecure = cookieSecure
		s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
		s.mux.HandleFunc("POST /api/auth/refresh", s.handleRefresh)
		s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
		s.mux.HandleFunc("GET /api/auth/me", s.requireAuth(s.handleMe))
	}
}

// WithSignup monta POST /api/auth/signup (slice 0). onReady puede ser
// nil en tests (sin poller en caliente).
func WithSignup(svc signupService, onReady func(ctx context.Context, tenantID int64, tokenPlain string) error) Option {
	return func(s *Server) {
		s.signup = svc
		s.onTenantReady = onReady
		s.mux.HandleFunc("POST /api/auth/signup", s.handleSignup)
	}
}

// WithWebhook monta POST /api/telegram/webhook, la entrada de eventos
// en modo webhook. Solo debe usarse cuando TELEGRAM_MODE=webhook: en
// polling el endpoint no existe.
func WithWebhook(publisher updatePublisher, secret string) Option {
	return func(s *Server) {
		s.mux.HandleFunc("POST /api/telegram/webhook", telegramWebhookHandler(publisher, secret))
	}
}

// WithGroups monta las rutas de consulta de grupos (paso 10): listado,
// detalle y usuarios (admins/lookup). groups los provee el repositorio;
// groupUsers las llamadas de lectura del adapter de Telegram.
func WithGroups(groups groupStore, groupUsers groupUsersLookup) Option {
	return func(s *Server) {
		s.groups = groups
		s.groupUsers = groupUsers
		s.mux.HandleFunc("GET /api/groups", s.requireAuth(s.handleListGroups))
		s.mux.HandleFunc("GET /api/groups/{id}", s.requireAuth(s.handleGetGroup))
		s.mux.HandleFunc("GET /api/groups/{id}/users", s.requireAuth(s.handleListGroupUsers))
	}
}

// WithModeration monta las acciones de moderacion (paso 10): ban/unban/
// mute/unmute, delete/pin, lock/unlock y approve/reject de solicitudes.
// El servicio orquesta Grupo→Permiso→Telegram→Log; el handler solo
// valida inputs, extrae actor y mapea errores.
func WithModeration(mod moderationActions) Option {
	return func(s *Server) {
		s.moderation = mod
		s.mux.HandleFunc("POST /api/groups/{id}/users/{userId}/ban", s.requireAuth(s.handleBan))
		s.mux.HandleFunc("POST /api/groups/{id}/users/{userId}/unban", s.requireAuth(s.handleUnban))
		s.mux.HandleFunc("POST /api/groups/{id}/users/{userId}/mute", s.requireAuth(s.handleMute))
		s.mux.HandleFunc("POST /api/groups/{id}/users/{userId}/unmute", s.requireAuth(s.handleUnmute))
		s.mux.HandleFunc("POST /api/groups/{id}/messages/{messageId}/delete", s.requireAuth(s.handleDeleteMessage))
		s.mux.HandleFunc("POST /api/groups/{id}/messages/{messageId}/pin", s.requireAuth(s.handlePinMessage))
		s.mux.HandleFunc("POST /api/groups/{id}/lock", s.requireAuth(s.handleLock))
		s.mux.HandleFunc("POST /api/groups/{id}/unlock", s.requireAuth(s.handleUnlock))
	}
}

// WithJoinRequests monta las rutas de solicitudes de ingreso (paso 10):
// listado y approve/reject (estas dos ultimas delegan en el Service de
// moderacion, que orquesta la decision).
func WithJoinRequests(store joinRequestStore, mod moderationActions) Option {
	return func(s *Server) {
		s.joinRequests = store
		if s.moderation == nil {
			s.moderation = mod
		}
		s.mux.HandleFunc("GET /api/groups/{id}/join-requests", s.requireAuth(s.handleListJoinRequests))
		s.mux.HandleFunc("POST /api/groups/{id}/join-requests/{requestId}/approve", s.requireAuth(s.handleApproveJoinRequest))
		s.mux.HandleFunc("POST /api/groups/{id}/join-requests/{requestId}/reject", s.requireAuth(s.handleRejectJoinRequest))
	}
}

// WithLogs monta la lectura de auditoria (paso 10): GET .../logs.
func WithLogs(store logStore) Option {
	return func(s *Server) {
		s.logStore = store
		s.mux.HandleFunc("GET /api/groups/{id}/logs", s.requireAuth(s.handleListGroupLogs))
	}
}

// WithPublications monta las rutas de publicaciones (Fase 2, slice 1):
// crear + publicar, listado y detalle. El servicio orquesta
// Grupo→Permiso→Telegram→Log; el handler valida input, extrae actor y
// mapea errores. Slice 3 agrega DELETE /api/publications/{id} para
// cancelar publicaciones `scheduled`.
//
// publications-batch: agrega POST /api/publications/batch (cap 10,
// failure isolation per-item, status 200 OK con envelope valida). El
// handler vive en batch_handlers.go; comparte el store publicationStore
// y reusa Service.PublishMany / Service.Schedule sin nuevos metodos.
func WithPublications(pubs publicationStore) Option {
	return func(s *Server) {
		s.publications = pubs
		s.mux.HandleFunc("POST /api/publications", s.requireAuth(s.handleCreatePublication))
		s.mux.HandleFunc("POST /api/publications/batch", s.requireAuth(s.handleCreatePublicationBatch))
		s.mux.HandleFunc("GET /api/publications", s.requireAuth(s.handleListPublications))
		s.mux.HandleFunc("GET /api/publications/{id}", s.requireAuth(s.handleGetPublication))
		s.mux.HandleFunc("DELETE /api/publications/{id}", s.requireAuth(s.handleDeletePublication))
	}
}

// WithAutomation monta las rutas del modulo de moderacion automatica
// (Fase 3, slice 2 + slice 3): 12 handlers bajo
// /api/groups/{id}/automation/... El service expone settings + listas;
// el handler escribe en logs los cambios manuales (ActorID != nil) —
// distinto del patron slice 1 donde los auto-actions del pipeline
// llevan ActorID=nil.
//
// `groups` permite al handler validar que el grupo existe antes de
// aceptar cambios (404 NOT_FOUND). Puede omitirse (nil) en tests que
// solo verifican auth/validacion, pero el main.go siempre lo pasa.
//
// `dashboard` (slice 3) es la vista minima del repository que cubre
// los 2 endpoints del dashboard de observacion (warnings + reset). NO
// pasa por Service: el Service no se toca (slices 1+2+2.1 intactos).
// Puede ser nil en tests que solo cubren settings + listas, pero
// main.go siempre lo pasa.
//
// Slice 3 agrega los 3 endpoints del dashboard: GET /warnings,
// POST /warnings/{user_id}/reset, GET /stats?period=24h|7d. Los
// handlers usan `logs` (automationLogWriter) tambien para
// CountByActionAndGroup (stats).
func WithAutomation(
	auto automationService,
	logs automationLogWriter,
	groups automationGroupChecker,
	dashboard automationDashboardRepo,
) Option {
	return func(s *Server) {
		s.automation = auto
		s.automationLogs = logs
		s.automationGroups = groups
		s.automationDashboard = dashboard
		s.mux.HandleFunc("GET /api/groups/{id}/automation/settings", s.requireAuth(s.handleGetAutomationSettings))
		s.mux.HandleFunc("PUT /api/groups/{id}/automation/settings", s.requireAuth(s.handlePutAutomationSettings))
		s.mux.HandleFunc("GET /api/groups/{id}/automation/banned-words", s.requireAuth(s.handleListBannedWords))
		s.mux.HandleFunc("POST /api/groups/{id}/automation/banned-words", s.requireAuth(s.handleAddBannedWord))
		s.mux.HandleFunc("DELETE /api/groups/{id}/automation/banned-words/{word}", s.requireAuth(s.handleRemoveBannedWord))
		s.mux.HandleFunc("GET /api/groups/{id}/automation/link-allowlist", s.requireAuth(s.handleListLinkAllowlist))
		s.mux.HandleFunc("POST /api/groups/{id}/automation/link-allowlist", s.requireAuth(s.handleAddLinkAllowlist))
		s.mux.HandleFunc("DELETE /api/groups/{id}/automation/link-allowlist/{domain}", s.requireAuth(s.handleRemoveLinkAllowlist))
		// Slice 3 — Warnings Dashboard.
		s.mux.HandleFunc("GET /api/groups/{id}/automation/warnings", s.requireAuth(s.handleListWarnings))
		s.mux.HandleFunc("POST /api/groups/{id}/automation/warnings/{user_id}/reset", s.requireAuth(s.handleResetWarning))
		s.mux.HandleFunc("GET /api/groups/{id}/automation/stats", s.requireAuth(s.handleGetStats))
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
}

// ServeHTTP hace que *Server sea un http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
