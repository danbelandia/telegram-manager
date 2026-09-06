// Package config carga toda la configuracion desde variables de entorno
// en un unico lugar. El resto de la app recibe la struct tipada Config.
package config

import (
	"fmt"
	"os"
)

// Config reune la configuracion del servidor.
type Config struct {
	TelegramBotToken      string
	TelegramMode          string
	TelegramWebhookURL    string
	TelegramWebhookSecret string
	DatabaseURL           string
	JWTSecret             string
	Port                  string
	RunMigrations         bool
}

// Load lee las variables de entorno y valida que la configuracion
// critica exista. Falla rapido (devuelve error) si falta algo.
func Load() (Config, error) {
	cfg := Config{
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramMode:          envOr("TELEGRAM_MODE", "polling"),
		TelegramWebhookURL:    os.Getenv("TELEGRAM_WEBHOOK_URL"),
		TelegramWebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		Port:                  envOr("PORT", "8080"),
		RunMigrations:         envBoolOr("RUN_MIGRATIONS", true),
	}

	if cfg.TelegramBotToken == "" {
		return cfg, fmt.Errorf("config: TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.TelegramMode != "polling" && cfg.TelegramMode != "webhook" {
		return cfg, fmt.Errorf("config: TELEGRAM_MODE must be %q or %q, got %q",
			"polling", "webhook", cfg.TelegramMode)
	}
	if cfg.TelegramMode == "webhook" {
		// El webhook sin URL publica no tiene sentido; y sin secret, el
		// endpoint quedaria abierto a eventos falsos (AGENTS.md 19.1).
		if cfg.TelegramWebhookURL == "" {
			return cfg, fmt.Errorf("config: TELEGRAM_WEBHOOK_URL is required when TELEGRAM_MODE=webhook")
		}
		if err := validateWebhookSecret(cfg.TelegramWebhookSecret); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

// validateWebhookSecret chequea el formato que pide la Bot API para
// secret_token: 1-256 caracteres de [A-Za-z0-9_-].
func validateWebhookSecret(secret string) error {
	if secret == "" {
		return fmt.Errorf("config: TELEGRAM_WEBHOOK_SECRET is required when TELEGRAM_MODE=webhook")
	}
	if len(secret) > 256 {
		return fmt.Errorf("config: TELEGRAM_WEBHOOK_SECRET too long (max 256 chars)")
	}
	for _, r := range secret {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("config: TELEGRAM_WEBHOOK_SECRET contains invalid char %q (allowed: A-Za-z0-9_-)", r)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBoolOr(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v == "true" || v == "1"
}
