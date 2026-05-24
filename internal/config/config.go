package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	Server Server  `koanf:"server"`
	JN66   JN66   `koanf:"jn66"`
	Ollama Ollama  `koanf:"ollama"`
	Gmail  Gmail   `koanf:"gmail"`
	Log    Logging `koanf:"log"`
}

type Server struct {
	Port    int    `koanf:"port"    validate:"required"`
	BaseURL string `koanf:"base_url" validate:"required,url"`
}

type JN66 struct {
	BaseURL string `koanf:"base_url" validate:"required,url"`
}

type Ollama struct {
	BaseURL        string `koanf:"base_url"        validate:"required,url"`
	RouterModel    string `koanf:"router_model"    validate:"required"`
	ExtractorModel string `koanf:"extractor_model" validate:"required"`
}

type Gmail struct {
	PollInterval time.Duration `koanf:"poll_interval" validate:"required"`
	Accounts     []Account     `koanf:"accounts"      validate:"required,min=1,dive"`
}

type Account struct {
	Email string `koanf:"email" validate:"required,email"`
}

type Logging struct {
	Level  string `koanf:"level"  validate:"oneof=debug info warn error"`
	Format string `koanf:"format" validate:"oneof=text json"`
}

func (l Logging) SlogLevel() slog.Level {
	switch strings.ToLower(l.Level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Load(path string) (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("config: load file %s: %w", path, err)
	}

	// AP5_SERVER__BASE_URL -> server.base_url
	err := k.Load(env.Provider("AP5_", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(s, "AP5_")), "__", ".")
	}), nil)
	if err != nil {
		return nil, fmt.Errorf("config: load env: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}

	return &cfg, nil
}
