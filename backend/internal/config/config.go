package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds process configuration for the backend API.
type Config struct {
	HTTPAddr            string
	DatabaseURL         string
	LogLevel            string
	SessionCookieName   string
	SessionTTL          time.Duration
	SessionCookieSecure bool

	// HTTP hardening defaults.
	MaxJSONBodyBytes int64
	StrictJSON       bool
	HTTPReadTimeout  time.Duration
	HTTPWriteTimeout time.Duration
	HTTPIdleTimeout  time.Duration

	// Rate limiting (process-local).
	LoginRateLimitPerMin       int
	LoginRateLimitBurst        int
	TokenCreateRateLimitPerMin int
	TokenCreateRateLimitBurst  int
}

// FromEnv loads the backend configuration from environment variables.
func FromEnv() (Config, error) {
	cfg := Config{
		HTTPAddr: os.Getenv("HTTP_ADDR"),
		LogLevel: os.Getenv("LOG_LEVEL"),
	}

	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	cfg.SessionCookieName = os.Getenv("SESSION_COOKIE_NAME")
	if cfg.SessionCookieName == "" {
		cfg.SessionCookieName = "cooking_app_session"
	}

	cfg.SessionTTL = 7 * 24 * time.Hour
	if raw := os.Getenv("SESSION_TTL_HOURS"); raw != "" {
		hours, err := strconv.Atoi(raw)
		if err != nil || hours <= 0 {
			return Config{}, errors.New("SESSION_TTL_HOURS must be a positive integer")
		}
		cfg.SessionTTL = time.Duration(hours) * time.Hour
	}

	cfg.SessionCookieSecure = false
	if raw := os.Getenv("SESSION_COOKIE_SECURE"); raw != "" {
		secure, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, errors.New("SESSION_COOKIE_SECURE must be a boolean")
		}
		cfg.SessionCookieSecure = secure
	}

	cfg.MaxJSONBodyBytes = 2 << 20 // 2 MiB
	if raw := os.Getenv("MAX_JSON_BODY_BYTES"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			return Config{}, errors.New("MAX_JSON_BODY_BYTES must be a positive integer")
		}
		cfg.MaxJSONBodyBytes = v
	}

	cfg.StrictJSON = true
	if raw := os.Getenv("STRICT_JSON"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, errors.New("STRICT_JSON must be a boolean")
		}
		cfg.StrictJSON = v
	}

	cfg.HTTPReadTimeout = 15 * time.Second
	if raw := os.Getenv("HTTP_READ_TIMEOUT_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			return Config{}, errors.New("HTTP_READ_TIMEOUT_SECONDS must be a positive integer")
		}
		cfg.HTTPReadTimeout = time.Duration(seconds) * time.Second
	}

	cfg.HTTPWriteTimeout = 30 * time.Second
	if raw := os.Getenv("HTTP_WRITE_TIMEOUT_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			return Config{}, errors.New("HTTP_WRITE_TIMEOUT_SECONDS must be a positive integer")
		}
		cfg.HTTPWriteTimeout = time.Duration(seconds) * time.Second
	}

	cfg.HTTPIdleTimeout = 60 * time.Second
	if raw := os.Getenv("HTTP_IDLE_TIMEOUT_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds <= 0 {
			return Config{}, errors.New("HTTP_IDLE_TIMEOUT_SECONDS must be a positive integer")
		}
		cfg.HTTPIdleTimeout = time.Duration(seconds) * time.Second
	}

	cfg.LoginRateLimitPerMin = 20
	if raw := os.Getenv("LOGIN_RATE_LIMIT_PER_MIN"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			return Config{}, errors.New("LOGIN_RATE_LIMIT_PER_MIN must be a non-negative integer")
		}
		cfg.LoginRateLimitPerMin = v
	}
	cfg.LoginRateLimitBurst = 5
	if raw := os.Getenv("LOGIN_RATE_LIMIT_BURST"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			return Config{}, errors.New("LOGIN_RATE_LIMIT_BURST must be a non-negative integer")
		}
		cfg.LoginRateLimitBurst = v
	}

	cfg.TokenCreateRateLimitPerMin = 60
	if raw := os.Getenv("TOKEN_CREATE_RATE_LIMIT_PER_MIN"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			return Config{}, errors.New("TOKEN_CREATE_RATE_LIMIT_PER_MIN must be a non-negative integer")
		}
		cfg.TokenCreateRateLimitPerMin = v
	}
	cfg.TokenCreateRateLimitBurst = 10
	if raw := os.Getenv("TOKEN_CREATE_RATE_LIMIT_BURST"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			return Config{}, errors.New("TOKEN_CREATE_RATE_LIMIT_BURST must be a non-negative integer")
		}
		cfg.TokenCreateRateLimitBurst = v
	}

	return cfg, nil
}

// Redacted returns a log-friendly summary of the config without secrets.
func (c Config) Redacted() string {
	if c.DatabaseURL == "" {
		return fmt.Sprintf("addr=%s log=%s db=<empty>", c.HTTPAddr, c.LogLevel)
	}
	return fmt.Sprintf("addr=%s log=%s db=<set>", c.HTTPAddr, c.LogLevel)
}
