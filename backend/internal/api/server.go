package api

import (
	"context"
	"net/http"

	"github.com/telegram-manager/backend/internal/license"
	"github.com/telegram-manager/backend/internal/telegram"
	"github.com/telegram-manager/backend/internal/tenants"
)

// pinger es la minima vista de la base de datos que el API necesita.
// Declarada donde se consume; *sql.DB la satisface.
type pinger interface {
	PingContext(ctx context.Context) error
}

// tenantSettingsRepo es la vista minima del repositorio de tenants
// que los handlers de tenant settings necesitan. Declarada donde se
// consume; *tenants.Repository la satisface.
type tenantSettingsRepo interface {
	GetByID(ctx context.Context, id int64) (*tenants.Tenant, error)
	UpdateToken(ctx context.Context, id int64, enc []byte, botUsername *string) error
}

// tenantSettingsRegistry es la vista minima del registry que los
// handlers de tenant settings necesitan: status y registerHot.
// telegram.Registry la satisface.
type tenantSettingsRegistry interface {
	Status(tenantID int64) (string, bool)
	RegisterHot(ctx context.Context, tenantID int64, slug, tokenPlain string, bus telegram.Publisher)
}

// tenantSettingsCrypter es la vista minima del crypter para
// descifrar/cifrar tokens. tenants.Crypter la satisface.
type tenantSettingsCrypter interface {
	Encrypt(plain []byte) ([]byte, error)
	Decrypt(blob []byte) ([]byte, error)
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
	groupUsers   GroupUsersLookup
	moderation   ModerationActions
	joinRequests joinRequestStore
	logStore     logStore

	// Modulo de publicaciones (Fase 2, slice 1).
	publications PublicationStore

	// Slice 0 (multitenancy): resolvers por tenant. Main los conecta al
	// registry (una instancia por bot); nil = path legacy de un solo
	// tenant (los singletons de arriba). Los handlers siempre prefieren
	// el resolver y caen al singleton cuando no hay routing.
	moderationFor   func(tenantID int64) (ModerationActions, bool)
	groupUsersFor   func(tenantID int64) (GroupUsersLookup, bool)
	publicationsFor func(tenantID int64) (PublicationStore, bool)
	automationFor   func(tenantID int64) (AutomationService, bool)

	// Tenant settings (slice tenant-settings): repo + registry + crypter
	// para los handlers de GET /tenants/me, PUT /tenants/me/bot-token,
	// GET /tenants/me/status y la ampliacion de /auth/me con tenant_slug.
	tenantsRepo    tenantSettingsRepo
	tenantRegistry tenantSettingsRegistry
	tenantCrypter  tenantSettingsCrypter
	tenantBusFor   func(tenantID int64) telegram.Publisher

	// Lookup de usuarios con scope de tenant (slice 0, Q3-a: users
	// global, alcance via join). Lo usa el ?userId= de grupos.
	tenantUsers tenantUserLookup

	// Modulo de moderacion automatica (Fase 3, slice 2): settings +
	// listas. Los handlers reciben el Service (que expone los metodos
	// de settings + listas) y el logs.Repository para auditoria manual.
	automation       AutomationService
	automationLogs   automationLogWriter
	automationGroups automationGroupChecker
	// Slice 3 — dashboard de moderacion. NO pasa por Service para
	// mantener la arquitectura de slices 1+2+2.1 intacta: el handler
	// consume el repo directamente. Ver automationDashboardRepo en
	// automation_handlers.go.
	automationDashboard automationDashboardRepo

	// License enforcement (license-system change).
	licenseSvc *license.Service

	// Super-admin panel (license-system change).
	adminTenantsRepo    adminTenantsLister
	adminTenantsGetter  adminTenantsGetter
	adminTenantsUpdater adminTenantsUpdater
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

// WithTenantResolvers conecta el routing por tenant (slice 0): cada
// resolver devuelve la instancia del tenant (con SU adapter de
// Telegram) o false si el tenant no tiene runtime. Main los construye
// desde el registry; en modo webhook/legacy quedan nil y los handlers
// usan los singletons. Cada resolver puede ser nil por separado.
func WithTenantResolvers(
	moderation func(tenantID int64) (ModerationActions, bool),
	groupUsers func(tenantID int64) (GroupUsersLookup, bool),
	publications func(tenantID int64) (PublicationStore, bool),
	automation func(tenantID int64) (AutomationService, bool),
) Option {
	return func(s *Server) {
		s.moderationFor = moderation
		s.groupUsersFor = groupUsers
		s.publicationsFor = publications
		s.automationFor = automation
	}
}

// WithTenants monta las rutas de configuracion del tenant
// (GET /tenants/me, PUT /tenants/me/bot-token, GET /tenants/me/status)
// e inyecta las dependencias necesarias: repo, registry, crypter y
// busFor para el reinicio del poller en caliente.
// GET /tenants/me/license es exento de requireLicense para que tenants
// suspendidos/expirados puedan ver su propio estado.
func WithTenants(
	repo tenantSettingsRepo,
	registry tenantSettingsRegistry,
	crypter tenantSettingsCrypter,
	busFor func(tenantID int64) telegram.Publisher,
) Option {
	return func(s *Server) {
		s.tenantsRepo = repo
		s.tenantRegistry = registry
		s.tenantCrypter = crypter
		s.tenantBusFor = busFor
		s.mux.HandleFunc("GET /api/tenants/me", s.requireAuth(s.requireLicense(s.handleGetTenantMe)))
		s.mux.HandleFunc("GET /api/tenants/me/license", s.requireAuth(s.handleGetTenantLicense))
		s.mux.HandleFunc("PUT /api/tenants/me/bot-token", s.requireAuth(s.requireLicense(s.handleRotateBotToken)))
		s.mux.HandleFunc("GET /api/tenants/me/status", s.requireAuth(s.requireLicense(s.handleGetTenantStatus)))
	}
}

// WithUsers monta el lookup de usuarios con scope de tenant (slice 0,
// T9). Sin esto, el ?userId= responde con datos de Telegram sin
// verificar presencia en el tenant.
func WithUsers(users tenantUserLookup) Option {
	return func(s *Server) {
		s.tenantUsers = users
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
func WithGroups(groups groupStore, groupUsers GroupUsersLookup) Option {
	return func(s *Server) {
		s.groups = groups
		s.groupUsers = groupUsers
		s.mux.HandleFunc("GET /api/groups", s.requireAuth(s.requireLicense(s.handleListGroups)))
		s.mux.HandleFunc("GET /api/groups/{id}", s.requireAuth(s.requireLicense(s.handleGetGroup)))
		s.mux.HandleFunc("DELETE /api/groups/{id}", s.requireAuth(s.requireLicense(s.handleDeleteGroup)))
		s.mux.HandleFunc("GET /api/groups/{id}/users", s.requireAuth(s.requireLicense(s.handleListGroupUsers)))
	}
}

// WithModeration monta las acciones de moderacion (paso 10): ban/unban/
// mute/unmute, delete/pin, lock/unlock y approve/reject de solicitudes.
// El servicio orquesta Grupo→Permiso→Telegram→Log; el handler solo
// valida inputs, extrae actor y mapea errores.
func WithModeration(mod ModerationActions) Option {
	return func(s *Server) {
		s.moderation = mod
		s.mux.HandleFunc("POST /api/groups/{id}/users/{userId}/ban", s.requireAuth(s.requireLicense(s.handleBan)))
		s.mux.HandleFunc("POST /api/groups/{id}/users/{userId}/unban", s.requireAuth(s.requireLicense(s.handleUnban)))
		s.mux.HandleFunc("POST /api/groups/{id}/users/{userId}/mute", s.requireAuth(s.requireLicense(s.handleMute)))
		s.mux.HandleFunc("POST /api/groups/{id}/users/{userId}/unmute", s.requireAuth(s.requireLicense(s.handleUnmute)))
		s.mux.HandleFunc("POST /api/groups/{id}/messages/{messageId}/delete", s.requireAuth(s.requireLicense(s.handleDeleteMessage)))
		s.mux.HandleFunc("POST /api/groups/{id}/messages/{messageId}/pin", s.requireAuth(s.requireLicense(s.handlePinMessage)))
		s.mux.HandleFunc("POST /api/groups/{id}/lock", s.requireAuth(s.requireLicense(s.handleLock)))
		s.mux.HandleFunc("POST /api/groups/{id}/unlock", s.requireAuth(s.requireLicense(s.handleUnlock)))
	}
}

// WithJoinRequests monta las rutas de solicitudes de ingreso (paso 10):
// listado y approve/reject (estas dos ultimas delegan en el Service de
// moderacion, que orquesta la decision).
func WithJoinRequests(store joinRequestStore, mod ModerationActions) Option {
	return func(s *Server) {
		s.joinRequests = store
		if s.moderation == nil {
			s.moderation = mod
		}
		s.mux.HandleFunc("GET /api/groups/{id}/join-requests", s.requireAuth(s.requireLicense(s.handleListJoinRequests)))
		s.mux.HandleFunc("POST /api/groups/{id}/join-requests/{requestId}/approve", s.requireAuth(s.requireLicense(s.handleApproveJoinRequest)))
		s.mux.HandleFunc("POST /api/groups/{id}/join-requests/{requestId}/reject", s.requireAuth(s.requireLicense(s.handleRejectJoinRequest)))
	}
}

// WithLogs monta la lectura de auditoria (paso 10): GET .../logs.
func WithLogs(store logStore) Option {
	return func(s *Server) {
		s.logStore = store
		s.mux.HandleFunc("GET /api/groups/{id}/logs", s.requireAuth(s.requireLicense(s.handleListGroupLogs)))
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
func WithPublications(pubs PublicationStore) Option {
	return func(s *Server) {
		s.publications = pubs
		s.mux.HandleFunc("POST /api/publications", s.requireAuth(s.requireLicense(s.handleCreatePublication)))
		s.mux.HandleFunc("POST /api/publications/batch", s.requireAuth(s.requireLicense(s.handleCreatePublicationBatch)))
		s.mux.HandleFunc("GET /api/publications", s.requireAuth(s.requireLicense(s.handleListPublications)))
		s.mux.HandleFunc("GET /api/publications/{id}", s.requireAuth(s.requireLicense(s.handleGetPublication)))
		s.mux.HandleFunc("DELETE /api/publications/{id}", s.requireAuth(s.requireLicense(s.handleDeletePublication)))
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
	auto AutomationService,
	logs automationLogWriter,
	groups automationGroupChecker,
	dashboard automationDashboardRepo,
) Option {
	return func(s *Server) {
		s.automation = auto
		s.automationLogs = logs
		s.automationGroups = groups
		s.automationDashboard = dashboard
		s.mux.HandleFunc("GET /api/groups/{id}/automation/settings", s.requireAuth(s.requireLicense(s.handleGetAutomationSettings)))
		s.mux.HandleFunc("PUT /api/groups/{id}/automation/settings", s.requireAuth(s.requireLicense(s.handlePutAutomationSettings)))
		s.mux.HandleFunc("GET /api/groups/{id}/automation/banned-words", s.requireAuth(s.requireLicense(s.handleListBannedWords)))
		s.mux.HandleFunc("POST /api/groups/{id}/automation/banned-words", s.requireAuth(s.requireLicense(s.handleAddBannedWord)))
		s.mux.HandleFunc("DELETE /api/groups/{id}/automation/banned-words/{word}", s.requireAuth(s.requireLicense(s.handleRemoveBannedWord)))
		s.mux.HandleFunc("GET /api/groups/{id}/automation/link-allowlist", s.requireAuth(s.requireLicense(s.handleListLinkAllowlist)))
		s.mux.HandleFunc("POST /api/groups/{id}/automation/link-allowlist", s.requireAuth(s.requireLicense(s.handleAddLinkAllowlist)))
		s.mux.HandleFunc("DELETE /api/groups/{id}/automation/link-allowlist/{domain}", s.requireAuth(s.requireLicense(s.handleRemoveLinkAllowlist)))
		// Slice 3 — Warnings Dashboard.
		s.mux.HandleFunc("GET /api/groups/{id}/automation/warnings", s.requireAuth(s.requireLicense(s.handleListWarnings)))
		s.mux.HandleFunc("POST /api/groups/{id}/automation/warnings/{user_id}/reset", s.requireAuth(s.requireLicense(s.handleResetWarning)))
		s.mux.HandleFunc("GET /api/groups/{id}/automation/stats", s.requireAuth(s.requireLicense(s.handleGetStats)))
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
}

// WithLicense inyecta el servicio de licencias para enforcement.
func WithLicense(svc *license.Service) Option {
	return func(s *Server) {
		s.licenseSvc = svc
	}
}

// WithAdmin monta las rutas del panel super-admin (requireAuth +
// requireSuperAdmin) e inyecta las dependencias del repositorio.
func WithAdmin(
	lister adminTenantsLister,
	getter adminTenantsGetter,
	updater adminTenantsUpdater,
) Option {
	return func(s *Server) {
		s.adminTenantsRepo = lister
		s.adminTenantsGetter = getter
		s.adminTenantsUpdater = updater
		adminRoutes := func(next http.HandlerFunc) http.HandlerFunc {
			return s.requireAuth(s.requireSuperAdmin(next))
		}
		s.mux.HandleFunc("GET /api/admin/tenants", adminRoutes(s.handleAdminListTenants))
		s.mux.HandleFunc("GET /api/admin/tenants/{id}", adminRoutes(s.handleAdminGetTenant))
		s.mux.HandleFunc("PUT /api/admin/tenants/{id}", adminRoutes(s.handleAdminUpdateTenant))
	}
}

// ServeHTTP hace que *Server sea un http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// tenantIDFromClaims extrae el tenant de los claims inyectados por
// requireAuth. requireAuth ya rechaza claims legacy (TenantID==0) con
// 401, asi que 0 aca es defensivo (identidad invalida).
func tenantIDFromClaims(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := claimsFromContext(r.Context())
	if claims == nil || claims.TenantID == 0 {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "identidad invalida: volve a iniciar sesion")
		return 0, false
	}
	return claims.TenantID, true
}

// resolveModeration devuelve el servicio de moderacion del tenant (con
// su adapter). Fallback al singleton legacy cuando no hay routing
// (tests, modo webhook, tenant default sin token propio).
func (s *Server) resolveModeration(tenantID int64) (ModerationActions, bool) {
	if s.moderationFor != nil {
		if mod, ok := s.moderationFor(tenantID); ok && mod != nil {
			return mod, true
		}
	}
	if s.moderation != nil {
		return s.moderation, true
	}
	return nil, false
}

// resolveGroupUsers devuelve el lookup de usuarios de Telegram del
// tenant (su adapter). Fallback al singleton legacy.
func (s *Server) resolveGroupUsers(tenantID int64) (GroupUsersLookup, bool) {
	if s.groupUsersFor != nil {
		if gu, ok := s.groupUsersFor(tenantID); ok && gu != nil {
			return gu, true
		}
	}
	if s.groupUsers != nil {
		return s.groupUsers, true
	}
	return nil, false
}

// resolvePublications devuelve el servicio de publicaciones del tenant
// (con su adapter). Fallback al singleton legacy.
func (s *Server) resolvePublications(tenantID int64) (PublicationStore, bool) {
	if s.publicationsFor != nil {
		if p, ok := s.publicationsFor(tenantID); ok && p != nil {
			return p, true
		}
	}
	if s.publications != nil {
		return s.publications, true
	}
	return nil, false
}

// resolveAutomation devuelve el servicio de automation del tenant.
// Fallback al singleton legacy.
func (s *Server) resolveAutomation(tenantID int64) (AutomationService, bool) {
	if s.automationFor != nil {
		if a, ok := s.automationFor(tenantID); ok && a != nil {
			return a, true
		}
	}
	if s.automation != nil {
		return s.automation, true
	}
	return nil, false
}
