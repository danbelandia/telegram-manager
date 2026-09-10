package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

// configKeys son las variables de entorno que Load() lee; el test las
// limpia para que el entorno real no filtre estado entre casos.
var configKeys = []string{
	"TELEGRAM_BOT_TOKEN",
	"TELEGRAM_MODE",
	"TELEGRAM_WEBHOOK_URL",
	"TELEGRAM_WEBHOOK_SECRET",
	"DATABASE_URL",
	"JWT_SECRET",
	"ADMIN_USERNAME",
	"ADMIN_PASSWORD",
	"COOKIE_SECURE",
	"PORT",
	"RUN_MIGRATIONS",
}

// validEnv devuelve un map con las variables minimas que Load() acepta
// (token + DB + auth), para que los casos de config no deban repetirlas.
func validEnv() map[string]string {
	return map[string]string{
		"TELEGRAM_BOT_TOKEN": "123456:test-token",
		"DATABASE_URL":       "postgres://telegram:telegram@localhost:5432/telegram_manager?sslmode=disable",
		"JWT_SECRET":         "clave-suficientemente-larga-para-firmar-jwt-abcdefghijklmnop",
		"ADMIN_USERNAME":     "admin",
		"ADMIN_PASSWORD":     "secret123",
	}
}

func TestLoad(t *testing.T) {
	testCases := []struct {
		name    string
		env     map[string]string
		wantErr bool
	}{
		{
			name: "configuration complete",
			env:  validEnv(),
		},
		{
			// TELEGRAM_BOT_TOKEN es opcional post-rotacion (D10).
			name: "missing bot token (optional after rotation)",
			env: map[string]string{
				"DATABASE_URL": "postgres://telegram:telegram@localhost:5432/telegram_manager?sslmode=disable",
				"JWT_SECRET":   "clave-suficientemente-larga-para-firmar-jwt-abcdefghijklmnop",
				"ADMIN_USERNAME": "admin",
				"ADMIN_PASSWORD": "secret123",
			},
		},
		{
			name: "missing database url",
			env: map[string]string{
				"TELEGRAM_BOT_TOKEN": "123456:test-token",
			},
			wantErr: true,
		},
		{
			name: "missing jwt secret",
			env: map[string]string{
				"TELEGRAM_BOT_TOKEN": "123456:test-token",
				"DATABASE_URL":       "postgres://telegram:telegram@localhost:5432/telegram_manager?sslmode=disable",
			},
			wantErr: true,
		},
		{
			name: "short jwt secret",
			env: map[string]string{
				"TELEGRAM_BOT_TOKEN": "123456:test-token",
				"DATABASE_URL":       "postgres://telegram:telegram@localhost:5432/telegram_manager?sslmode=disable",
				"JWT_SECRET":         "corto",
			},
			wantErr: true,
		},
		{
			name: "missing admin username",
			env: map[string]string{
				"TELEGRAM_BOT_TOKEN": "123456:test-token",
				"DATABASE_URL":       "postgres://telegram:telegram@localhost:5432/telegram_manager?sslmode=disable",
				"JWT_SECRET":         "clave-suficientemente-larga-para-firmar-jwt-abcdefghijklmnop",
			},
			wantErr: true,
		},
		{
			name: "invalid telegram mode",
			env: map[string]string{
				"TELEGRAM_BOT_TOKEN": "123456:test-token",
				"DATABASE_URL":       "postgres://telegram:telegram@localhost:5432/telegram_manager?sslmode=disable",
				"TELEGRAM_MODE":      "bogus",
			},
			wantErr: true,
		},
		{
			name: "webhook without url",
			env: func() map[string]string {
				m := validEnv()
				m["TELEGRAM_MODE"] = "webhook"
				m["TELEGRAM_WEBHOOK_SECRET"] = "safe-secret-123"
				return m
			}(),
			wantErr: true,
		},
		{
			name: "webhook without secret",
			env: func() map[string]string {
				m := validEnv()
				m["TELEGRAM_MODE"] = "webhook"
				m["TELEGRAM_WEBHOOK_URL"] = "https://example.com/webhook"
				return m
			}(),
			wantErr: true,
		},
		{
			name: "webhook invalid secret char",
			env: func() map[string]string {
				m := validEnv()
				m["TELEGRAM_MODE"] = "webhook"
				m["TELEGRAM_WEBHOOK_URL"] = "https://example.com/webhook"
				m["TELEGRAM_WEBHOOK_SECRET"] = "not allowed!"
				return m
			}(),
			wantErr: true,
		},
		{
			name: "webhook complete",
			env: func() map[string]string {
				m := validEnv()
				m["TELEGRAM_MODE"] = "webhook"
				m["TELEGRAM_WEBHOOK_URL"] = "https://example.com/webhook"
				m["TELEGRAM_WEBHOOK_SECRET"] = "safe-secret-123"
				return m
			}(),
		},
		{
			name: "defaults applied",
			env:  validEnv(),
		},
		{
			// D10: ausente → arranque legacy OK.
			name: "tenant enc key absent (legacy boot)",
			env:  validEnv(),
		},
		{
			name: "tenant enc key valid base64",
			env: func() map[string]string {
				m := validEnv()
				m["TENANT_TOKEN_ENC_KEY"] = base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
				return m
			}(),
		},
		{
			name: "tenant enc key valid raw 32",
			env: func() map[string]string {
				m := validEnv()
				m["TENANT_TOKEN_ENC_KEY"] = "0123456789abcdef0123456789abcdef"
				return m
			}(),
		},
		{
			// D10: presente-pero-invalida → fail-fast.
			name: "tenant enc key invalid",
			env: func() map[string]string {
				m := validEnv()
				m["TENANT_TOKEN_ENC_KEY"] = "corta"
				return m
			}(),
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range configKeys {
				t.Setenv(k, "")
			}
			t.Setenv("TENANT_TOKEN_ENC_KEY", "")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			cfg, err := Load()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Load() expected error, got nil (cfg=%+v)", cfg)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if cfg.Port != "8080" {
				t.Errorf("Port: want default 8080, got %q", cfg.Port)
			}
			if !cfg.RunMigrations {
				t.Errorf("RunMigrations: want default true, got false")
			}
			if cfg.CookieSecure {
				t.Errorf("CookieSecure: want default false, got true")
			}
			if strings.HasPrefix(tc.name, "tenant enc key valid") && cfg.TenantTokenEncKey == "" {
				t.Error("TenantTokenEncKey vacia con key valida en env")
			}
		})
	}
}
