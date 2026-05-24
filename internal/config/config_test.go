package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pushkar-anand/ap-5/internal/config"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoad_ValidConfig(t *testing.T) {
	path := writeConfig(t, `
server:
  port: 9090
  base_url: "http://localhost:9090"
jn66:
  base_url: "http://localhost:8081"
ollama:
  base_url: "http://localhost:11434/v1"
  router_model: "qwen3:4b"
  extractor_model: "qwen3:14b"
gmail:
  poll_interval: 30s
  accounts:
    - email: "test@gmail.com"
log:
  level: "debug"
  format: "json"
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("server.port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Server.BaseURL != "http://localhost:9090" {
		t.Errorf("server.base_url = %q", cfg.Server.BaseURL)
	}
	if cfg.Ollama.RouterModel != "qwen3:4b" {
		t.Errorf("ollama.router_model = %q", cfg.Ollama.RouterModel)
	}
	if cfg.Ollama.ExtractorModel != "qwen3:14b" {
		t.Errorf("ollama.extractor_model = %q", cfg.Ollama.ExtractorModel)
	}
	if cfg.Gmail.PollInterval != 30*time.Second {
		t.Errorf("gmail.poll_interval = %v, want 30s", cfg.Gmail.PollInterval)
	}
	if len(cfg.Gmail.Accounts) != 1 || cfg.Gmail.Accounts[0].Email != "test@gmail.com" {
		t.Errorf("gmail.accounts = %+v", cfg.Gmail.Accounts)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("log.level = %q", cfg.Log.Level)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	path := writeConfig(t, `
server:
  port: 8080
  base_url: "http://localhost:8080"
jn66:
  base_url: "http://localhost:8081"
ollama:
  base_url: "http://localhost:11434/v1"
  router_model: "qwen3:14b"
  extractor_model: "qwen3:14b"
gmail:
  poll_interval: 60s
  accounts:
    - email: "test@gmail.com"
log:
  level: "info"
  format: "text"
`)

	t.Setenv("AP5_SERVER__PORT", "7070")
	t.Setenv("AP5_OLLAMA__ROUTER_MODEL", "qwen3:4b")

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 7070 {
		t.Errorf("server.port after env override = %d, want 7070", cfg.Server.Port)
	}
	if cfg.Ollama.RouterModel != "qwen3:4b" {
		t.Errorf("ollama.router_model after env override = %q", cfg.Ollama.RouterModel)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestLogging_SlogLevel(t *testing.T) {
	cases := []struct {
		level string
		want  string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"error", "ERROR"},
		{"", "INFO"},
		{"unknown", "INFO"},
	}

	for _, tc := range cases {
		l := config.Logging{Level: tc.level}
		got := l.SlogLevel().String()
		if got != tc.want {
			t.Errorf("SlogLevel(%q) = %q, want %q", tc.level, got, tc.want)
		}
	}
}
