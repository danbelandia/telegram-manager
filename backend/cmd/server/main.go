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

	server := api.NewServer(db, bot)

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
	}
}
