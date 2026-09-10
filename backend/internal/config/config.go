// Package config carga toda la configuracion desde variables de entorno
// en un unico lugar. El resto de la app recibe la struct tipada Config.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
)

// Config reune la configuracion del servidor.
type Config struct {
	TelegramBotToken      string
	TelegramMode          string
	TelegramWebhookURL    string
	TelegramWebhookSecret string
	DatabaseURL           string
	JWTSecret             string
	AdminUsername         string
	AdminPassword         string
	CookieSecure          bool
	Port                  string
	RunMigrations         bool
	// TenantTokenEncKey es la clave de cifrado AES-GCM de los tokens de
	// bot por tenant (slice 0). OPCIONAL en Load (D10, backward-compat:
	// deploys legacy sin la var deben arrancar): solo fail-fast si esta
	// presente pero es invalida. Requerida solo para signup de nuevos
	// tenants (sin clave valida no se puede cifrar el token).
	TenantTokenEncKey string
	// PublicationsSchedulerIntervalSeconds es el intervalo entre ticks
	// del worker de publicaciones programadas (slice 3). Default 30s.
	// Tests/operacion: bajar para mayor reactividad a costa de carga DB.
	PublicationsSchedulerIntervalSeconds int
	// AutomationEnabled (Fase 3, slice 1) corta el pipeline completo:
	// si false, el subscriber NO se registra y el worker NO arranca.
	// Default true. Operacion: poner en false para detener el modulo
	// sin re-deploy (ej. investigar falsos positivos).
	AutomationEnabled bool
	// AutoActionBufferSize es el tamano del canal `autoActionCh` que
	// el Service usa para encolar AutoActions. Default 100. Buffer
	// lleno → non-blocking send + log warn + drop.
	AutoActionBufferSize int
	// WorkerConcurrency es la cantidad de workers que drenan el
	// canal. Slice 1: 1 (secuencial). Subir en slice futuro si la
	// carga lo justifica — respetando rate-limit del adapter.
	WorkerConcurrency int
}

// Load lee las variables de entorno y valida que la configuracion
// critica exista. Falla rapido (devuelve error) si falta algo.
func Load() (Config, error) {
	cfg := Config{
		TelegramBotToken:                     os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramMode:                         envOr("TELEGRAM_MODE", "polling"),
		TelegramWebhookURL:                   os.Getenv("TELEGRAM_WEBHOOK_URL"),
		TelegramWebhookSecret:                os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		DatabaseURL:                          os.Getenv("DATABASE_URL"),
		JWTSecret:                            os.Getenv("JWT_SECRET"),
		AdminUsername:                        os.Getenv("ADMIN_USERNAME"),
		AdminPassword:                        os.Getenv("ADMIN_PASSWORD"),
		CookieSecure:                         envBoolOr("COOKIE_SECURE", false),
		Port:                                 envOr("PORT", "8080"),
		RunMigrations:                        envBoolOr("RUN_MIGRATIONS", true),
		TenantTokenEncKey:                    os.Getenv("TENANT_TOKEN_ENC_KEY"),
		PublicationsSchedulerIntervalSeconds: envIntOr("PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS", 30),
		AutomationEnabled:                    envBoolOr("AUTOMATION_ENABLED", true),
		AutoActionBufferSize:                 envIntOr("AUTOMATION_AUTOACTION_BUFFER_SIZE", 100),
		WorkerConcurrency:                    envIntOr("AUTOMATION_WORKER_CONCURRENCY", 1),
	}

	// TELEGRAM_BOT_TOKEN es opcional: si el tenant default ya tiene su
	// token propio cifrado en DB (post-rotacion), el backend lo usa via
	// registry sin necesidad del env var. Requerido solo para el
	// arranque inicial (seeder del tenant default).
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return cfg, fmt.Errorf("config: JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		// El secret firma tokens; uno corto es trivialmente forzable.
		return cfg, fmt.Errorf("config: JWT_SECRET too short (min 32 chars)")
	}
	if cfg.AdminUsername == "" {
		return cfg, fmt.Errorf("config: ADMIN_USERNAME is required")
	}
	if cfg.AdminPassword == "" {
		return cfg, fmt.Errorf("config: ADMIN_PASSWORD is required")
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
	if cfg.PublicationsSchedulerIntervalSeconds <= 0 {
		return cfg, fmt.Errorf("config: PUBLICATIONS_SCHEDULER_INTERVAL_SECONDS must be > 0, got %d",
			cfg.PublicationsSchedulerIntervalSeconds)
	}
	if cfg.AutoActionBufferSize <= 0 {
		return cfg, fmt.Errorf("config: AUTOMATION_AUTOACTION_BUFFER_SIZE must be > 0, got %d",
			cfg.AutoActionBufferSize)
	}
	if cfg.WorkerConcurrency < 1 {
		return cfg, fmt.Errorf("config: AUTOMATION_WORKER_CONCURRENCY must be >= 1, got %d",
			cfg.WorkerConcurrency)
	}
	if cfg.TenantTokenEncKey != "" {
		// D10: solo fail-fast si presente-pero-invalida. La clave debe
		// resultar en 32 B exactos (base64 de 32 B o string crudo de
		// 32 chars); la validacion canonica vive en internal/tenants.
		if !validTenantEncKey(cfg.TenantTokenEncKey) {
			return cfg, fmt.Errorf("config: TENANT_TOKEN_ENC_KEY must decode to exactly 32 bytes (base64 of 32 B or raw 32-char string)")
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

// validTenantEncKey espeja tenants.ParseKey sin importar el paquete
// (config es hoja de dependencias): base64 de 32 B o crudo de 32.
func validTenantEncKey(raw string) bool {
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) == 32 {
		return true
	}
	return len(raw) == 32
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

func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
