package app

import (
	"context"
	"flag"
	"strings"
)

func (a *App) runUser(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printUserUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printUserUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case commandList:
		return a.runUserList(args[1:])
	case commandCreate:
		return a.runUserCreate(args[1:])
	case "deactivate":
		return a.runUserDeactivate(args[1:])
	default:
		writef(a.stderr, "unknown user command: %s\n", args[0])
		printUserUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runUserList(args []string) int {
	if hasHelpFlag(args) {
		printUserListUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("user list", flag.ContinueOnError)
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

	resp, err := api.Users(ctx)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runUserCreate(args []string) int {
	if hasHelpFlag(args) {
		printUserCreateUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("user create", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var username string
	var passwordStdin bool
	var displayName string

	flags.StringVar(&username, "username", "", "Username")
	flags.BoolVar(&passwordStdin, "password-stdin", false, "Read password from stdin")
	flags.StringVar(&displayName, "display-name", "", "Display name")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	username = strings.TrimSpace(username)
	if username == "" {
		writeLine(a.stderr, "username is required")
		return exitUsage
	}
	if !passwordStdin {
		writeLine(a.stderr, "password-stdin is required for user create")
		return exitUsage
	}

	password, err := readPassword(a.stdin)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	if password == "" {
		writeLine(a.stderr, "password is required")
		return exitUsage
	}

	var displayNamePtr *string
	displayName = strings.TrimSpace(displayName)
	if displayName != "" {
		displayNamePtr = &displayName
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

	resp, err := api.CreateUser(ctx, username, password, displayNamePtr)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runUserDeactivate(args []string) int {
	if hasHelpFlag(args) {
		printUserDeactivateUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("user deactivate", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var yes bool
	flags.BoolVar(&yes, "yes", false, "Confirm user deactivation")

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "user id is required")
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

	if err := api.DeactivateUser(ctx, id); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, userDeactivateResult{
		ID:          id,
		Deactivated: true,
	})
}
