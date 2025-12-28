package app

import (
	"context"
	"flag"
	"strings"
	"time"
)

func (a *App) runToken(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printTokenUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printTokenUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case commandList:
		return a.runTokenList(args[1:])
	case commandCreate:
		return a.runTokenCreate(args[1:])
	case "revoke":
		return a.runTokenRevoke(args[1:])
	default:
		writef(a.stderr, "unknown token command: %s\n", args[0])
		printTokenUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runTokenList(args []string) int {
	if hasHelpFlag(args) {
		printTokenListUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("token list", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	token, _, err := a.resolveToken()
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	if token == "" {
		writeLine(a.stderr, "no token found; run `cookctl auth set --token <pat>`")
		return exitAuth
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, err := a.apiClient(ctx, token)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	resp, err := api.Tokens(ctx)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runTokenCreate(args []string) int {
	if hasHelpFlag(args) {
		printTokenCreateUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("token create", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var name string
	var expiresAt string

	flags.StringVar(&name, "name", "", "Token name")
	flags.StringVar(&expiresAt, "expires-at", "", "Token expiration (RFC3339)")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	name = strings.TrimSpace(name)
	if name == "" {
		writeLine(a.stderr, "name is required")
		return exitUsage
	}

	var expiresAtTime *time.Time
	if expiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			writeLine(a.stderr, "expires-at must be RFC3339")
			return exitUsage
		}
		expiresAtTime = &parsed
	} else {
		writeLine(a.stderr, "warning: token will not expire unless revoked")
	}

	token, _, err := a.resolveToken()
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	if token == "" {
		writeLine(a.stderr, "no token found; run `cookctl auth set --token <pat>`")
		return exitAuth
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, err := a.apiClient(ctx, token)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	resp, err := api.CreateToken(ctx, name, expiresAtTime)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runTokenRevoke(args []string) int {
	if hasHelpFlag(args) {
		printTokenRevokeUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("token revoke", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var yes bool
	flags.BoolVar(&yes, "yes", false, "Confirm token revocation")

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "token id is required")
		return exitUsage
	}
	if !yes {
		writeLine(a.stderr, "confirmation required; re-run with --yes")
		return exitUsage
	}
	id = strings.TrimSpace(id)

	token, _, err := a.resolveToken()
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	if token == "" {
		writeLine(a.stderr, "no token found; run `cookctl auth set --token <pat>`")
		return exitAuth
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, err := a.apiClient(ctx, token)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	if err := api.RevokeToken(ctx, id); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, tokenRevokeResult{
		ID:      id,
		Revoked: true,
	})
}
