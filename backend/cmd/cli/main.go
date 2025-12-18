package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/saiaj/cooking_app/backend/internal/bootstrap"
	"github.com/saiaj/cooking_app/backend/internal/config"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/logging"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		_, _ = fmt.Fprintln(os.Stderr, "missing command")
		_, _ = fmt.Fprintln(os.Stderr, "usage: cli <command> [flags]")
		_, _ = fmt.Fprintln(os.Stderr, "commands:")
		_, _ = fmt.Fprintln(os.Stderr, "  bootstrap-user")
		return 2
	}

	cfg, err := config.FromEnv()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 2
	}

	logger := logging.New(cfg.LogLevel)

	switch os.Args[1] {
	case "bootstrap-user":
		return runBootstrapUser(cfg, logger, os.Args[2:])
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		return 2
	}
}

func runBootstrapUser(cfg config.Config, logger *slog.Logger, args []string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	flags := flag.NewFlagSet("bootstrap-user", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	var username string
	var password string
	var displayName string

	flags.StringVar(&username, "username", "", "username (required)")
	flags.StringVar(&password, "password", "", "password (required; never printed)")
	flags.StringVar(&displayName, "display-name", "", "display name (optional)")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect db", "err", err)
		return 1
	}
	defer pool.Close()

	queries := sqlc.New(pool)

	var displayNamePtr *string
	if displayName != "" {
		displayNamePtr = &displayName
	}

	user, err := bootstrap.CreateFirstUser(ctx, queries, bootstrap.FirstUserParams{
		Username:    username,
		Password:    password,
		DisplayName: displayNamePtr,
	})
	if err != nil {
		if errors.Is(err, bootstrap.ErrAlreadyBootstrapped) {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		logger.Error("bootstrap failed", "err", err)
		return 1
	}

	if _, err := fmt.Fprintf(os.Stdout, "created first user %s (%s)\n", user.Username, uuid.UUID(user.ID.Bytes).String()); err != nil {
		logger.Warn("write failed", "err", err)
		return 1
	}
	return 0
}
