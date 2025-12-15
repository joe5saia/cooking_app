package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/saiaj/cooking_app/backend/internal/config"
	"github.com/saiaj/cooking_app/backend/internal/httpapi"
	"github.com/saiaj/cooking_app/backend/internal/logging"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.FromEnv()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 2
	}

	logger := logging.New(cfg.LogLevel)
	logger.Info("starting", "addr", cfg.HTTPAddr)

	app, err := httpapi.New(ctx, logger, cfg)
	if err != nil {
		logger.Error("failed to init app", "err", err)
		return 1
	}
	defer app.Close()

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down", "reason", "signal")
	case err = <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", "err", err)
			return 1
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "err", err)
		return 1
	}

	return 0
}
