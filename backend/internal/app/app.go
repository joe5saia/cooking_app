package app

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// Config captures minimal settings for the CLI entry point.
type Config struct {
	AppName string
	Message string
}

func (cfg *Config) normalize() error {
	if cfg == nil {
		return errors.New("config is required")
	}

	if cfg.AppName == "" {
		return errors.New("app name is required")
	}

	if cfg.Message == "" {
		cfg.Message = "ready"
	}

	return nil
}

// Run prints a startup message and respects cancellation.
func Run(ctx context.Context, out io.Writer, cfg Config) error {
	if ctx == nil {
		return errors.New("context is required")
	}

	if out == nil {
		return errors.New("output writer is required")
	}

	if err := (&cfg).normalize(); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	_, err := fmt.Fprintf(out, "%s: %s\n", cfg.AppName, cfg.Message)
	return err
}
