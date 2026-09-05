package config

import "testing"

// configKeys son las variables de entorno que Load() lee; el test las
// limpia para que el entorno real no filtre estado entre casos.
var configKeys = []string{
	"TELEGRAM_BOT_TOKEN",
	"TELEGRAM_MODE",
	"TELEGRAM_WEBHOOK_URL",
	"TELEGRAM_WEBHOOK_SECRET",
	"DATABASE_URL",
	"JWT_SECRET",
	"PORT",
	"RUN_MIGRATIONS",
}

func TestLoad(t *testing.T) {
	testCases := []struct {
		name    string
		env     map[string]string
		wantErr bool
	}{
		{
			name: "configuration complete",
			env: map[string]string{
				"TELEGRAM_BOT_TOKEN": "123456:test-token",
				"DATABASE_URL":       "postgres://telegram:telegram@localhost:5432/telegram_manager?sslmode=disable",
			},
			wantErr: false,
		},
		{
			name: "missing bot token",
			env: map[string]string{
				"DATABASE_URL": "postgres://telegram:telegram@localhost:5432/telegram_manager?sslmode=disable",
			},
			wantErr: true,
		},
		{
			name: "missing database url",
			env: map[string]string{
				"TELEGRAM_BOT_TOKEN": "123456:test-token",
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
			name: "defaults applied",
			env: map[string]string{
				"TELEGRAM_BOT_TOKEN": "123456:test-token",
				"DATABASE_URL":       "postgres://telegram:telegram@localhost:5432/telegram_manager?sslmode=disable",
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range configKeys {
				t.Setenv(k, "")
			}
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
		})
	}
}
