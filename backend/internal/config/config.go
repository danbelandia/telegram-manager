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
	return cfg, nil
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
