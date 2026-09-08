// Command server arranca el backend de Telegram Group Manager:
// configuración, base de datos, migraciones, conexión con Telegram
// (validación del token) y el API HTTP.
//
// Slice 0 (multitenancy bot-per-tenant): un poller+bus+stack de
// servicios por tenant. El path legacy (TELEGRAM_BOT_TOKEN + ADMIN_*)
// sigue vivo como tenant `default`.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/telegram-manager/backend/internal/api"
	"github.com/telegram-manager/backend/internal/auth"
	"github.com/telegram-manager/backend/internal/automation"
	"github.com/telegram-manager/backend/internal/config"
	"github.com/telegram-manager/backend/internal/database"
	"github.com/telegram-manager/backend/internal/events"
	"github.com/telegram-manager/backend/internal/groups"
	"github.com/telegram-manager/backend/internal/joinrequests"
	"github.com/telegram-manager/backend/internal/logs"
	"github.com/telegram-manager/backend/internal/moderation"
	"github.com/telegram-manager/backend/internal/publications"
	"github.com/telegram-manager/backend/internal/telegram"
	"github.com/telegram-manager/backend/internal/tenants"
	"github.com/telegram-manager/backend/internal/users"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("server exited", "error", err)
		os.Exit(1)
	}
}

// tenantStack agrupa el bus y los servicios de un tenant construidos
// sobre SU adapter de Telegram (bot-per-tenant). Los repos (groups,
// requests, logs, pubs, automation, users) son COMPARTIDOS: reciben el
// tenant por parametro en cada llamada (slice 0, tenantID primer
// predicado del WHERE).
type tenantStack struct {
	tenantID   int64
	slug       string
	bus        *events.Bus
	adapter    *telegram.Adapter
	moderation *moderation.Service
	pubs       *publications.Service
	automation *automation.Service
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer db.Close()
	slog.Info("database connected")

	if cfg.RunMigrations {
		if err := database.Migrate(ctx, db); err != nil {
			return fmt.Errorf("run migrations: %w", err)
		}
		slog.Info("migrations applied")
	}

	// Repos compartidos (tenant por parametro).
	groupsRepo := groups.NewRepository(db)
	joinRequestsRepo := joinrequests.NewRepository(db)
	logsRepo := logs.NewRepository(db)
	pubsRepo := publications.NewRepository(db)
	automationRepo := automation.NewRepository(db)
	usersRepo := users.NewRepository(db)
	tenantsRepo := tenants.NewRepository(db)

	// Autenticacion del panel: seed del primer admin (si tabla vacia),
	// adoptando el tenant `default` (slice 0, compat legacy).
	authRepo := auth.NewRepository(db)
	if err := auth.EnsureInitialAdmin(ctx, authRepo, tenantsRepo, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		return fmt.Errorf("startup: seed admin: %w", err)
	}
	slog.Info("auth: admin bootstrap ok")
	defaultTenantID, err := tenantsRepo.EnsureDefault(ctx)
	if err != nil {
		return fmt.Errorf("startup: ensure default tenant: %w", err)
	}
	tokenManager := auth.NewTokenManager(cfg.JWTSecret)
	authService := auth.NewService(authRepo, tokenManager)

	// Cifrado de tokens por tenant (slice 0, signup). Nil = signup
	// deshabilitado (path legacy sin TENANT_TOKEN_ENC_KEY intacto, D10).
	var crypter *tenants.Crypter
	if cfg.TenantTokenEncKey != "" {
		key, err := tenants.ParseKey(cfg.TenantTokenEncKey)
		if err != nil {
			return fmt.Errorf("startup: parse tenant enc key: %w", err)
		}
		crypter, err = tenants.NewCrypter(key)
		if err != nil {
			return fmt.Errorf("startup: tenant crypter: %w", err)
		}
	}

	// Adapter legacy (TELEGRAM_BOT_TOKEN): valida fail-fast como
	// siempre y sirve al tenant `default` salvo que tenga token propio.
	legacyBot := telegram.NewAdapter(cfg.TelegramBotToken)
	botUser, err := legacyBot.GetMe(ctx)
	if err != nil {
		if errors.Is(err, telegram.ErrInvalidToken) {
			return errors.New("startup: telegram rejected the bot token (invalid TELEGRAM_BOT_TOKEN)")
		}
		return fmt.Errorf("startup: validate bot token: %w", err)
	}
	slog.Info("bot connected", "bot_id", botUser.ID, "bot_username", botUser.Username)

	// Canales de errores de background (buffer holgado: N tenants x
	// workers/schedulers; el select toma el primero).
	automationWorkerErrs := make(chan error, 64)
	schedulerErrCh := make(chan error, 16)

	// Registry multi-bot (slice 0, solo polling; en webhook queda sin
	// uso: multiplexado = futuro). onStatus persiste degraded/active.
	tgRegistry := telegram.NewRegistry(slog.Default(),
		func(_ context.Context, tenantID int64, status string) {
			cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := tenantsRepo.SetStatus(cctx, tenantID, status); err != nil {
				slog.Warn("registry: persist status failed", "tenant_id", tenantID, "status", status, "error", err)
			}
		})

	stacks := make(map[int64]*tenantStack)

	// buildBus crea el bus propio del tenant y registra los handlers de
	// eventos que NO necesitan adapter: logging, deteccion de grupos
	// (upsert scopeado al tenant) y solicitudes de ingreso (usuario
	// global + pendiente del tenant).
	buildBus := func(tenantID int64) *events.Bus {
		bus := events.NewBus()
		bus.Handle(func(u *telegram.Update) {
			slog.Info("telegram update", "tenant_id", tenantID, "update_id", u.UpdateID, "kind", u.Kind())
		})

		// Deteccion de grupos: cada my_chat_member registra (o
		// actualiza) el grupo EN EL TENANT del bus. Fallos de
		// persistencia se loguean y el bus continua.
		bus.Handle(func(u *telegram.Update) {
			if u.MyChatMember == nil {
				return
			}
			var g groups.Group
			if err := groups.HandleMyChatMember(ctx, u.MyChatMember, &g); err != nil {
				slog.Warn("groups: ignoring my_chat_member", "tenant_id", tenantID, "error", err)
				return
			}
			if g.TelegramID == 0 {
				return // chat no registrable (private, sin chat, etc.)
			}
			if err := groupsRepo.UpsertByTelegramID(ctx, tenantID, &g); err != nil {
				slog.Error("groups: upsert failed", "tenant_id", tenantID, "telegram_id", g.TelegramID, "error", err)
				return
			}
			slog.Info("groups: group registered", "tenant_id", tenantID, "telegram_id", g.TelegramID, "title", g.Title, "bot_status", g.BotStatus)
		})

		// Solicitudes de ingreso: cada chat_join_request registra el
		// usuario (global, D6) y la solicitud pendiente DEL TENANT
		// (AGENTS.md §10). Idempotente via indice parcial.
		bus.Handle(func(u *telegram.Update) {
			if u.ChatJoinRequest == nil {
				return
			}
			var r joinrequests.Request
			if err := joinrequests.HandleChatJoinRequest(ctx, u.ChatJoinRequest, &r); err != nil {
				slog.Warn("joinrequests: ignoring chat_join_request", "tenant_id", tenantID, "error", err)
				return
			}
			if r.GroupID == 0 || r.UserID == 0 {
				return
			}
			usr := &users.User{
				TelegramID: r.UserID,
				FirstName:  u.ChatJoinRequest.User.FirstName,
			}
			if u.ChatJoinRequest.User.Username != "" {
				usr.Username = &u.ChatJoinRequest.User.Username
			}
			if err := usersRepo.UpsertByTelegramID(ctx, usr); err != nil {
				slog.Error("joinrequests: user upsert failed", "tenant_id", tenantID, "user_id", r.UserID, "error", err)
				return
			}
			if err := joinRequestsRepo.UpsertPending(ctx, tenantID, r.GroupID, r.UserID); err != nil {
				slog.Error("joinrequests: upsert pending failed", "tenant_id", tenantID, "group_id", r.GroupID, "user_id", r.UserID, "error", err)
				return
			}
			slog.Info("joinrequests: request registered", "tenant_id", tenantID, "group_id", r.GroupID, "user_id", r.UserID)
		})
		return bus
	}

	// buildServices construye los servicios del tenant sobre SU adapter
	// (moderacion, publicaciones, automation + scheduler + subscriber).
	// Requiere adapter no-nil (lo crea el registry al boot o en
	// caliente; el legacy viene de env).
	buildServices := func(tenantID int64, slug string, bus *events.Bus, adapter *telegram.Adapter) *tenantStack {
		moderationSvc := moderation.NewService(tenantID, groupsRepo, adapter, joinRequestsRepo, logsRepo)
		pubsSvc := publications.NewService(groupsRepo, adapter, pubsRepo, logsRepo)

		// Moderacion automatica (Fase 3): pipeline por tenant con el
		// adapter de su bot. Gated por cfg.AutomationEnabled.
		//
		// Invariante (bugfix #172): permissionOkAdmin =
		// g.BotStatus == StatusAdministrator; NUNCA claves de permiso
		// individuales.
		var automationSvc *automation.Service
		if cfg.AutomationEnabled {
			autoActionCh := make(chan automation.AutoAction, cfg.AutoActionBufferSize)
			rulesRegistry := automation.NewRegistry()
			// Orden cheap-first (design D4): in-mem → CPU → DB-pre-loaded.
			rulesRegistry.Register(automation.NewFloodRule())
			rulesRegistry.Register(automation.NewAntiSpamRule())
			rulesRegistry.Register(automation.NewAntiLinkRule())
			rulesRegistry.Register(automation.NewBannedWordsRule())
			actioner := automation.NewAutoActioner(adapter, logsRepo, groupsRepo, slog.Default())
			warningSender := automation.NewWarningSender(adapter, logsRepo, automationRepo, groupsRepo, slog.Default(), tenantID)
			automationSvc = automation.NewService(
				tenantID,
				automationRepo, automationRepo, automationRepo, rulesRegistry,
				logsRepo, groupsRepo, autoActionCh, warningSender, slog.Default(),
			)
			automationWorker := automation.NewWorker(autoActionCh, actioner, slog.Default())
			subscriber := automation.NewSubscriber(bus, automationSvc, slog.Default())
			subscriber.Register()
			for i := 0; i < cfg.WorkerConcurrency; i++ {
				go func() {
					automationWorkerErrs <- automationWorker.Run(ctx)
				}()
			}
			slog.Info("automation pipeline started", "tenant_id", tenantID,
				"buffer_size", cfg.AutoActionBufferSize,
				"workers", cfg.WorkerConcurrency,
			)
		} else {
			slog.Info("automation pipeline disabled (AUTOMATION_ENABLED=false)", "tenant_id", tenantID)
		}

		schedulerInterval := time.Duration(cfg.PublicationsSchedulerIntervalSeconds) * time.Second
		scheduler := publications.NewScheduler(
			pubsRepo,
			groupsRepo,
			adapter,
			logsRepo,
			schedulerInterval,
			slog.Default(),
			tenantID,
		)
		go func() {
			schedulerErrCh <- scheduler.Run(ctx)
		}()

		st := &tenantStack{
			tenantID:   tenantID,
			slug:       slug,
			bus:        bus,
			adapter:    adapter,
			moderation: moderationSvc,
			pubs:       pubsSvc,
			automation: automationSvc,
		}
		stacks[tenantID] = st
		return st
	}

	// Signup (slice 0): alta transaccional + poller en caliente. Sin
	// clave de cifrado el servicio rechaza (ErrSignupNotConfigured).
	signupSvc := auth.NewSignupService(db, tenantsRepo, authRepo, crypter, auth.ValidateBotToken)

	var server *api.Server

	switch cfg.TelegramMode {
	case "webhook":
		// Fallo rapido: si Telegram rechaza la URL o el secret, el
		// backend ni arranca. Solo adapter legacy (multiplexado por
		// tenant = futuro, fuera del slice).
		if err := legacyBot.SetWebhook(ctx, cfg.TelegramWebhookURL, cfg.TelegramWebhookSecret, telegram.MVPAllowedUpdates); err != nil {
			return fmt.Errorf("startup: set webhook: %w", err)
		}
		slog.Info("webhook registered", "url", cfg.TelegramWebhookURL)
		defaultStack := buildServices(defaultTenantID, "default", buildBus(defaultTenantID), legacyBot)
		server = api.NewServer(db, legacyBot,
			api.WithWebhook(defaultStack.bus, cfg.TelegramWebhookSecret),
			api.WithAuth(authService, tokenManager, cfg.CookieSecure),
			api.WithSignup(signupSvc, webhookSignupHook),
			api.WithGroups(groupsRepo, legacyBot),
			api.WithUsers(usersRepo),
			api.WithModeration(defaultStack.moderation),
			api.WithJoinRequests(joinRequestsRepo, defaultStack.moderation),
			api.WithLogs(logsRepo),
			api.WithPublications(defaultStack.pubs),
			api.WithAutomation(defaultStack.automation, logsRepo, groupsRepo, automationRepo),
		)

	case "polling":
		// Boot multi-bot: bus por tenant, poller por tenant con token.
		// El tenant `default` sin token propio usa el legacy.
		tenantsWithTokens, err := tenantsRepo.ListWithTokens(ctx)
		if err != nil {
			return fmt.Errorf("startup: list tenants: %w", err)
		}
		withTokens := make(map[int64]bool, len(tenantsWithTokens))
		for _, t := range tenantsWithTokens {
			withTokens[t.ID] = true
			stacks[t.ID] = &tenantStack{tenantID: t.ID, slug: t.Slug, bus: buildBus(t.ID)}
		}
		decrypt := func(blob []byte) (string, error) {
			if crypter == nil {
				return "", fmt.Errorf("startup: cannot decrypt tenant token without TENANT_TOKEN_ENC_KEY")
			}
			plain, err := crypter.Decrypt(blob)
			if err != nil {
				return "", err
			}
			return string(plain), nil
		}
		busFor := func(tenantID int64) telegram.Publisher {
			if st, ok := stacks[tenantID]; ok {
				return st.bus
			}
			return nil
		}
		tgRegistry.BootAll(ctx, tenantsWithTokens, decrypt, busFor)

		// Servicios sobre el adapter que el registry creo (degraded
		// incluido: el runtime existe aunque el token este revocado).
		// Sin adapter (descifrado fallo) → sin stack, solo log.
		for _, t := range tenantsWithTokens {
			adapter, ok := tgRegistry.AdapterFor(t.ID)
			if !ok {
				slog.Error("startup: tenant sin adapter tras boot (descifrado fallo)",
					"tenant_id", t.ID, "slug", t.Slug)
				delete(stacks, t.ID)
				continue
			}
			buildServices(t.ID, t.Slug, stacks[t.ID].bus, adapter)
		}

		// Tenant default: con token propio ya tiene stack; sin token
		// se levanta con el legacy via registry (poller en caliente
		// como cualquier tenant).
		defaultStack := stacks[defaultTenantID]
		if defaultStack == nil {
			bus := buildBus(defaultTenantID)
			tgRegistry.RegisterHot(ctx, defaultTenantID, "default", cfg.TelegramBotToken, bus)
			adapter, ok := tgRegistry.AdapterFor(defaultTenantID)
			if !ok {
				return fmt.Errorf("startup: default tenant sin adapter tras hot register")
			}
			defaultStack = buildServices(defaultTenantID, "default", bus, adapter)
		}

		// Hook de signup: bus + servicios + poller en caliente sin
		// tocar los demas tenants.
		onTenantReady := func(_ context.Context, tenantID int64, tokenPlain string) error {
			t, err := tenantsRepo.GetByID(context.Background(), tenantID)
			if err != nil {
				return err
			}
			bus := buildBus(tenantID)
			tgRegistry.RegisterHot(ctx, tenantID, t.Slug, tokenPlain, bus)
			adapter, ok := tgRegistry.AdapterFor(tenantID)
			if !ok {
				return fmt.Errorf("startup: hot register sin adapter para tenant %d", tenantID)
			}
			buildServices(tenantID, t.Slug, bus, adapter)
			return nil
		}

		server = api.NewServer(db, legacyBot,
			api.WithAuth(authService, tokenManager, cfg.CookieSecure),
			api.WithSignup(signupSvc, onTenantReady),
			api.WithGroups(groupsRepo, legacyBot),
			api.WithUsers(usersRepo),
			api.WithModeration(defaultStack.moderation),
			api.WithJoinRequests(joinRequestsRepo, defaultStack.moderation),
			api.WithLogs(logsRepo),
			api.WithPublications(defaultStack.pubs),
			api.WithAutomation(defaultStack.automation, logsRepo, groupsRepo, automationRepo),
			api.WithTenantResolvers(
				func(tenantID int64) (api.ModerationActions, bool) {
					if st, ok := stacks[tenantID]; ok && st.moderation != nil {
						return st.moderation, true
					}
					return nil, false
				},
				func(tenantID int64) (api.GroupUsersLookup, bool) {
					if adapter, ok := tgRegistry.AdapterFor(tenantID); ok {
						return adapter, true
					}
					return nil, false
				},
				func(tenantID int64) (api.PublicationStore, bool) {
					if st, ok := stacks[tenantID]; ok && st.pubs != nil {
						return st.pubs, true
					}
					return nil, false
				},
				func(tenantID int64) (api.AutomationService, bool) {
					if st, ok := stacks[tenantID]; ok && st.automation != nil {
						return st.automation, true
					}
					return nil, false
				},
			),
		)
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
		tgRegistry.StopAll()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case err := <-schedulerErrCh:
		return fmt.Errorf("publications scheduler: %w", err)
	case err := <-automationWorkerErrs:
		return fmt.Errorf("automation worker: %w", err)
	}
}

// webhookSignupHook es el onTenantReady en modo webhook: el
// multiplexado por tenant queda fuera del slice, asi que el signup
// crea el tenant pero el poller se levanta al pasar a polling (o en
// futuro webhook multiplexado). Retorna error para que el handler lo
// deje en warn log (auditable, sin romper el 201).
func webhookSignupHook(_ context.Context, tenantID int64, _ string) error {
	return fmt.Errorf("modo webhook: tenant %d sin poller (multiplexado fuera del slice)", tenantID)
}
