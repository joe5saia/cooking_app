package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/saiaj/cooking_app/backend/internal/app"
	"github.com/saiaj/cooking_app/backend/internal/bootstrap"
	"github.com/saiaj/cooking_app/backend/internal/config"
	"github.com/saiaj/cooking_app/backend/internal/db/sqlc"
	"github.com/saiaj/cooking_app/backend/internal/logging"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "bootstrap-user" {
		os.Exit(runBootstrapUser())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	cfg := app.Config{
		AppName: "Cooking App",
		Message: "ready to serve",
	}

	err := app.Run(ctx, os.Stdout, cfg)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
}

func runBootstrapUser() int {
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

	if err := flags.Parse(os.Args[2:]); err != nil {
		return 2
	}

	cfg, err := config.FromEnv()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 2
	}

	logger := logging.New(cfg.LogLevel)

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
