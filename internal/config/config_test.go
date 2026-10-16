package config

import "testing"

func setRequiredConfig(t *testing.T) {
	t.Helper()
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TELEGRAM_WEBHOOK_URL", "")
	t.Setenv("DB_PATH", "/tmp/test-bot.db")
	t.Setenv("TEMP_DIR", "/tmp/test-bot")
	t.Setenv("MAX_CONCURRENT_DOWNLOADS", "1")
	t.Setenv("PORT", "3000")
	t.Setenv("NODE_ENV", "development")
}

func TestLoadPprofDefaultsDisabled(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("PPROF_ENABLED", "")
	t.Setenv("PPROF_PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Pprof.Enabled {
		t.Fatal("Pprof.Enabled = true, want false by default")
	}
	if cfg.Pprof.Port != 6060 {
		t.Fatalf("Pprof.Port = %d, want 6060", cfg.Pprof.Port)
	}
}

func TestLoadPprofEnabled(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("PPROF_ENABLED", "true")
	t.Setenv("PPROF_PORT", "6061")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Pprof.Enabled {
		t.Fatal("Pprof.Enabled = false, want true")
	}
	if cfg.Pprof.Port != 6061 {
		t.Fatalf("Pprof.Port = %d, want 6061", cfg.Pprof.Port)
	}
}

func TestLoadRejectsInvalidPprofConfig(t *testing.T) {
	for _, test := range []struct {
		name    string
		enabled string
		port    string
	}{
		{name: "enabled", enabled: "sometimes", port: "6060"},
		{name: "port", enabled: "true", port: "70000"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setRequiredConfig(t)
			t.Setenv("PPROF_ENABLED", test.enabled)
			t.Setenv("PPROF_PORT", test.port)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want validation error")
			}
		})
	}
}
