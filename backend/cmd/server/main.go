// Command server arranca el backend de Telegram Group Manager:
// configuración, base de datos, migraciones, conexión con Telegram
// (validación del token) y el API HTTP.
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

	bot := telegram.NewAdapter(cfg.TelegramBotToken)

	// Validación del token al arrancar: fail fast si Telegram lo rechaza.
	botUser, err := bot.GetMe(ctx)
	if err != nil {
		if errors.Is(err, telegram.ErrInvalidToken) {
			return errors.New("startup: telegram rejected the bot token (invalid TELEGRAM_BOT_TOKEN)")
		}
		return fmt.Errorf("startup: validate bot token: %w", err)
	}
	slog.Info("bot connected", "bot_id", botUser.ID, "bot_username", botUser.Username)

	// El bus centraliza todos los updates de Telegram, tanto de polling
	// como de webhook. El primer consumidor (logging) es el unico del
	// MVP; los modulos de negocio se registraran aca.
	bus := events.NewBus()
	bus.Handle(func(u *telegram.Update) {
		slog.Info("telegram update", "update_id", u.UpdateID, "kind", u.Kind())
	})

	// Deteccion de grupos: cada my_chat_member registra (o actualiza)
	// el grupo en la tabla groups. Fallos de persistencia se loguean y
	// el bus continua; no se rompe la entrega de los demas eventos.
	groupsRepo := groups.NewRepository(db)
	bus.Handle(func(u *telegram.Update) {
		if u.MyChatMember == nil {
			return
		}
		var g groups.Group
		if err := groups.HandleMyChatMember(ctx, u.MyChatMember, &g); err != nil {
			slog.Warn("groups: ignoring my_chat_member", "error", err)
			return
		}
		if g.TelegramID == 0 {
			return // chat no registrable (private, sin chat, etc.)
		}
		if err := groupsRepo.UpsertByTelegramID(ctx, &g); err != nil {
			slog.Error("groups: upsert failed", "telegram_id", g.TelegramID, "error", err)
			return
		}
		slog.Info("groups: group registered", "telegram_id", g.TelegramID, "title", g.Title, "bot_status", g.BotStatus)
	})

	// Autenticacion del panel: seed del primer admin (si tabla vacia),
	// emision/validacion de tokens y servicio de login.
	authRepo := auth.NewRepository(db)
	if err := auth.EnsureInitialAdmin(ctx, authRepo, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		return fmt.Errorf("startup: seed admin: %w", err)
	}
	slog.Info("auth: admin bootstrap ok")
	tokenManager := auth.NewTokenManager(cfg.JWTSecret)
	authService := auth.NewService(authRepo, tokenManager)

	// Moderacion (paso 10): repositorios de solicitudes y logs,
	// servicio que orquesta Grupo→Permiso→Telegram→Log.
	joinRequestsRepo := joinrequests.NewRepository(db)
	logsRepo := logs.NewRepository(db)
	moderationService := moderation.NewService(groupsRepo, bot, joinRequestsRepo, logsRepo)

	// Publicaciones (Fase 2, slice 1 — publish-now text): repositorio y
	// servicio que orquesta Grupo→Permiso→Telegram→Log. Slice 3 agrega
	// el Scheduler (worker in-process) que reclama filas `scheduled`
	// vencidas y las entrega al helper publishOne del Service.
	pubsRepo := publications.NewRepository(db)
	pubsService := publications.NewService(groupsRepo, bot, pubsRepo, logsRepo)
	schedulerInterval := time.Duration(cfg.PublicationsSchedulerIntervalSeconds) * time.Second
	publicationsScheduler := publications.NewScheduler(
		pubsRepo,
		groupsRepo,
		bot,
		logsRepo,
		schedulerInterval,
		slog.Default(),
	)
	slog.Info("publications scheduler started", "interval", schedulerInterval.String())

	// Moderacion automatica (Fase 3, slice 1 — foundation): settings +
	// warning_state + FloodRule + worker de auto-actions. El subscriber
	// se registra en el bus; el worker corre en su propia goroutine.
	// Gated por cfg.AutomationEnabled (kill switch operativo).
	//
	// Invariante (bugfix #172): permissionOkAdmin =
	// g.BotStatus == StatusAdministrator; NUNCA claves can_*.
	var (
		automationSubscriber *automation.Subscriber
		automationWorkerErrs chan error
	)
	if cfg.AutomationEnabled {
		automationRepo := automation.NewRepository(db)
		autoActionCh := make(chan automation.AutoAction, cfg.AutoActionBufferSize)
		registry := automation.NewRegistry()
		registry.Register(automation.NewFloodRule())
		actioner := automation.NewAutoActioner(bot, logsRepo, groupsRepo, slog.Default())
		automationService := automation.NewService(
			automationRepo, automationRepo, registry,
			logsRepo, groupsRepo, autoActionCh, slog.Default(),
		)
		automationWorker := automation.NewWorker(autoActionCh, actioner, slog.Default())
		automationSubscriber = automation.NewSubscriber(bus, automationService, slog.Default())
		automationSubscriber.Register()
		automationWorkerErrs = make(chan error, cfg.WorkerConcurrency)
		for i := 0; i < cfg.WorkerConcurrency; i++ {
			go func() {
				automationWorkerErrs <- automationWorker.Run(ctx)
			}()
		}
		slog.Info("automation pipeline started",
			"buffer_size", cfg.AutoActionBufferSize,
			"workers", cfg.WorkerConcurrency,
		)
	} else {
		slog.Info("automation pipeline disabled (AUTOMATION_ENABLED=false)")
	}

	// Solicitudes de ingreso: cada chat_join_request registra el usuario
	// en users y la solicitud pendiente (AGENTS.md §10). Idempotente:
	// UpsertPending usa el indice parcial (grupo, usuario) pending.
	usersRepo := users.NewRepository(db)
	bus.Handle(func(u *telegram.Update) {
		if u.ChatJoinRequest == nil {
			return
		}
		var r joinrequests.Request
		if err := joinrequests.HandleChatJoinRequest(ctx, u.ChatJoinRequest, &r); err != nil {
			slog.Warn("joinrequests: ignoring chat_join_request", "error", err)
			return
		}
		if r.GroupID == 0 || r.UserID == 0 {
			return
		}
		// Registro del usuario (identidad desde Telegram, D6). El
		// username puede venir vacio; first_name puede ser "".
		usr := &users.User{
			TelegramID: r.UserID,
			FirstName:  u.ChatJoinRequest.User.FirstName,
		}
		if u.ChatJoinRequest.User.Username != "" {
			usr.Username = &u.ChatJoinRequest.User.Username
		}
		if err := usersRepo.UpsertByTelegramID(ctx, usr); err != nil {
			slog.Error("joinrequests: user upsert failed", "user_id", r.UserID, "error", err)
			return
		}
		if err := joinRequestsRepo.UpsertPending(ctx, r.GroupID, r.UserID); err != nil {
			slog.Error("joinrequests: upsert pending failed", "group_id", r.GroupID, "user_id", r.UserID, "error", err)
			return
		}
		slog.Info("joinrequests: request registered", "group_id", r.GroupID, "user_id", r.UserID)
	})

	var pollerErrCh chan error
	var server *api.Server

	switch cfg.TelegramMode {
	case "webhook":
		// Fallo rapido: si Telegram rechaza la URL o el secret, el
		// backend ni arranca.
		if err := bot.SetWebhook(ctx, cfg.TelegramWebhookURL, cfg.TelegramWebhookSecret, telegram.MVPAllowedUpdates); err != nil {
			return fmt.Errorf("startup: set webhook: %w", err)
		}
		slog.Info("webhook registered", "url", cfg.TelegramWebhookURL)
		server = api.NewServer(db, bot,
			api.WithWebhook(bus, cfg.TelegramWebhookSecret),
			api.WithAuth(authService, tokenManager, cfg.CookieSecure),
			api.WithGroups(groupsRepo, bot),
			api.WithModeration(moderationService),
			api.WithJoinRequests(joinRequestsRepo, moderationService),
			api.WithLogs(logsRepo),
			api.WithPublications(pubsService),
		)

	case "polling":
		server = api.NewServer(db, bot,
			api.WithAuth(authService, tokenManager, cfg.CookieSecure),
			api.WithGroups(groupsRepo, bot),
			api.WithModeration(moderationService),
			api.WithJoinRequests(joinRequestsRepo, moderationService),
			api.WithLogs(logsRepo),
			api.WithPublications(pubsService),
		)
		poller := telegram.NewPoller(bot, telegram.WithPollerLogger(slog.Default()))
		pollerErrCh = make(chan error, 1)
		go func() {
			pollerErrCh <- poller.Run(ctx, func(updates []telegram.Update) {
				for i := range updates {
					bus.Publish(&updates[i])
				}
			})
		}()
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Worker in-process de publicaciones programadas (slice 3). Mismo
	// lifecycle que el poller: corre hasta que ctx.Done() (signal).
	schedulerErrCh := make(chan error, 1)
	go func() {
		schedulerErrCh <- publicationsScheduler.Run(ctx)
	}()

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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case err := <-pollerErrCh:
		return fmt.Errorf("poller: %w", err)
	case err := <-schedulerErrCh:
		return fmt.Errorf("publications scheduler: %w", err)
	case err := <-automationWorkerErrs:
		return fmt.Errorf("automation worker: %w", err)
	}
}
