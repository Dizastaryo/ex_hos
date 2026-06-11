package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	OpenAI    OpenAIConfig
	WhatsApp  WhatsAppConfig
	OTP       OTPConfig
	R2        R2Config
	Device    DeviceConfig
}

// DeviceConfig — параметры генерации BLE-устройств.
// SEEU_DEVICE_SECRET — мастер-ключ для HMAC-SHA256 генерации public/private id.
// В dev можно любую строку; в продакшне — минимум 32 случайных байта.
type DeviceConfig struct {
	Secret string `env:"SEEU_DEVICE_SECRET" env-default:"dev-device-secret-change-in-prod"`
}

type AppConfig struct {
	Port string `env:"APP_PORT" env-default:"8001"`
	Env  string `env:"APP_ENV" env-default:"local"`
}

type DatabaseConfig struct {
	URL             string `env:"DATABASE_URL" env-required:"true"`
	MaxOpenConns    int    `env:"DB_MAX_OPEN_CONNS" env-default:"25"`
	MaxIdleConns    int    `env:"DB_MAX_IDLE_CONNS" env-default:"25"`
	ConnMaxLifetime int    `env:"DB_CONN_MAX_LIFETIME" env-default:"300"`
}

type JWTConfig struct {
	AccessSecret   string `env:"JWT_ACCESS_SECRET" env-required:"true"`
	RefreshSecret  string `env:"JWT_REFRESH_SECRET" env-required:"true"`
	AccessExpHours int    `env:"JWT_ACCESS_EXP_HOURS" env-default:"24"`
	RefreshExpDays int    `env:"JWT_REFRESH_EXP_DAYS" env-default:"30"`
}

type OpenAIConfig struct {
	APIKey string `env:"OPENAI_API_KEY" env-default:""`
}

// WhatsAppConfig points at the local `whatps` Node.js bridge that delivers
// OTP codes via the operator's real WhatsApp account. URL empty → OTP is
// dev-mode (logs the code, accepts "0000"); URL set → real send.
type WhatsAppConfig struct {
	ServiceURL string `env:"WHATSAPP_SERVICE_URL" env-default:""`
}

// R2Config holds Cloudflare R2 credentials. All fields empty → fall back to
// local disk storage (dev without internet). All fields set → upload to R2.
type R2Config struct {
	Endpoint  string `env:"R2_ENDPOINT" env-default:""`
	AccessKey string `env:"R2_ACCESS_KEY" env-default:""`
	SecretKey string `env:"R2_SECRET_KEY" env-default:""`
	Bucket    string `env:"R2_BUCKET" env-default:"seeu-uploads"`
	PublicURL string `env:"R2_PUBLIC_URL" env-default:""`
}

func (c R2Config) IsConfigured() bool {
	return c.Endpoint != "" && c.AccessKey != "" && c.SecretKey != "" && c.PublicURL != ""
}

// OTPConfig — knobs for OTP generation/validation. Defaults match the
// industry baseline (5 min TTL, 5 verify attempts, 3 sends/hour per phone).
type OTPConfig struct {
	CodeTTLMinutes int `env:"OTP_CODE_TTL_MINUTES" env-default:"5"`
	MaxAttempts    int `env:"OTP_MAX_ATTEMPTS" env-default:"5"`
	MaxPerHour     int `env:"OTP_MAX_PER_HOUR" env-default:"3"`
}

// Load picks the env file based on APP_ENV (local|staging|production).
// Order of resolution:
//  1. OS env vars (highest priority — set by docker, k8s, CI).
//  2. .env.<APP_ENV> in the current directory.
//  3. defaults declared in struct tags.
func Load() (*Config, error) {
	envName := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if envName == "" {
		envName = "local"
	}
	return LoadFrom(".env." + envName)
}

func LoadFrom(envFile string) (*Config, error) {
	// Parse the dotenv file into the process env without overwriting variables
	// already set by the OS — that way `APP_ENV=staging ./api` keeps working
	// and CI-supplied secrets always win over file values.
	if _, err := os.Stat(envFile); err == nil {
		if err := godotenv.Load(envFile); err != nil {
			return nil, fmt.Errorf("parse %s: %w", envFile, err)
		}
	}

	cfg := &Config{}
	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("read env: %w", err)
	}
	if cfg.App.Env == "" {
		cfg.App.Env = "local"
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}
	return cfg, nil
}

// Validate rejects configurations that would let the service start in a broken
// or insecure state. Anything caught here is a configuration bug, not a runtime one.
func (c *Config) Validate() error {
	var errs []string

	if c.Database.URL == "" {
		errs = append(errs, "DATABASE_URL is required")
	}

	if c.JWT.AccessSecret == "" {
		errs = append(errs, "JWT_ACCESS_SECRET is required")
	}
	if c.JWT.RefreshSecret == "" {
		errs = append(errs, "JWT_REFRESH_SECRET is required")
	}

	// Block placeholder secrets — they ship in .env.example and must never reach a running service.
	if isPlaceholderSecret(c.JWT.AccessSecret) {
		errs = append(errs, "JWT_ACCESS_SECRET is a placeholder; generate a real value")
	}
	if isPlaceholderSecret(c.JWT.RefreshSecret) {
		errs = append(errs, "JWT_REFRESH_SECRET is a placeholder; generate a real value")
	}

	// In non-local environments insist on stronger guarantees.
	if c.App.Env != "local" {
		if len(c.JWT.AccessSecret) < 32 {
			errs = append(errs, "JWT_ACCESS_SECRET must be at least 32 chars outside local env")
		}
		if len(c.JWT.RefreshSecret) < 32 {
			errs = append(errs, "JWT_REFRESH_SECRET must be at least 32 chars outside local env")
		}
		if c.JWT.AccessSecret == c.JWT.RefreshSecret {
			errs = append(errs, "JWT_ACCESS_SECRET and JWT_REFRESH_SECRET must differ outside local env")
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func isPlaceholderSecret(s string) bool {
	low := strings.ToLower(s)
	return low == "" ||
		strings.Contains(low, "change-in-production") ||
		strings.Contains(low, "your-super-secret") ||
		strings.Contains(low, "placeholder") ||
		strings.Contains(low, "example") ||
		strings.Contains(low, "todo")
}
