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

	return cfg, nil
}

// Redacted returns a log-friendly summary of the config without secrets.
func (c Config) Redacted() string {
	if c.DatabaseURL == "" {
		return fmt.Sprintf("addr=%s log=%s db=<empty>", c.HTTPAddr, c.LogLevel)
	}
	return fmt.Sprintf("addr=%s log=%s db=<set>", c.HTTPAddr, c.LogLevel)
}
