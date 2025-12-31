package app

import (
	"context"
	"flag"
	"io"
	"strings"
	"time"
)

type tokenCreateFlags struct {
	name      string
	expiresAt string
}

type tokenRevokeFlags struct {
	yes bool
}

func tokenListFlagSet(out io.Writer) *flag.FlagSet {
	return newFlagSet("token list", out, printTokenListUsage)
}

func tokenCreateFlagSet(out io.Writer) (*flag.FlagSet, *tokenCreateFlags) {
	opts := &tokenCreateFlags{}
	flags := newFlagSet("token create", out, printTokenCreateUsage)
	flags.StringVar(&opts.name, "name", "", "Token name")
	flags.StringVar(&opts.expiresAt, "expires-at", "", "Token expiration (RFC3339)")
	return flags, opts
}

func tokenRevokeFlagSet(out io.Writer) (*flag.FlagSet, *tokenRevokeFlags) {
	opts := &tokenRevokeFlags{}
	flags := newFlagSet("token revoke", out, printTokenRevokeUsage)
	flags.BoolVar(&opts.yes, "yes", false, "Confirm token revocation")
	return flags, opts
}

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
		usageErrorf(a.stderr, "unknown token command: %s", args[0])
		printTokenUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runTokenList(args []string) int {
	if hasHelpFlag(args) {
		printTokenListUsage(a.stdout)
		return exitOK
	}

	flags := tokenListFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
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

	flags, opts := tokenCreateFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	opts.name = strings.TrimSpace(opts.name)
	if opts.name == "" {
		return usageError(a.stderr, "name is required")
	}

	var expiresAtTime *time.Time
	if opts.expiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, opts.expiresAt)
		if err != nil {
			return usageError(a.stderr, "expires-at must be RFC3339")
		}
		expiresAtTime = &parsed
	} else {
		writeLine(a.stderr, "warning: token will not expire unless revoked")
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resp, err := api.CreateToken(ctx, opts.name, expiresAtTime)
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

	flags, opts := tokenRevokeFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "token id is required")
	}
	if !opts.yes {
		return usageError(a.stderr, "confirmation required; re-run with --yes")
	}
	id = strings.TrimSpace(id)

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	if err := api.RevokeToken(ctx, id); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, tokenRevokeResult{
		ID:      id,
		Revoked: true,
	})
}
