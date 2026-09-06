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
	"github.com/telegram-manager/backend/internal/config"
	"github.com/telegram-manager/backend/internal/database"
	"github.com/telegram-manager/backend/internal/events"
	"github.com/telegram-manager/backend/internal/telegram"
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
		server = api.NewServer(db, bot, api.WithWebhook(bus, cfg.TelegramWebhookSecret))

	case "polling":
		server = api.NewServer(db, bot)
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
	}
}
