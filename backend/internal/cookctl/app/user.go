package app

import (
	"context"
	"flag"
	"io"
	"strings"
)

type userCreateFlags struct {
	username      string
	passwordStdin bool
	displayName   string
}

type userDeactivateFlags struct {
	yes bool
}

func userListFlagSet(out io.Writer) *flag.FlagSet {
	return newFlagSet("user list", out, printUserListUsage)
}

func userCreateFlagSet(out io.Writer) (*flag.FlagSet, *userCreateFlags) {
	opts := &userCreateFlags{}
	flags := newFlagSet("user create", out, printUserCreateUsage)
	flags.StringVar(&opts.username, "username", "", "Username")
	flags.BoolVar(&opts.passwordStdin, "password-stdin", false, "Read password from stdin")
	flags.StringVar(&opts.displayName, "display-name", "", "Display name")
	return flags, opts
}

func userDeactivateFlagSet(out io.Writer) (*flag.FlagSet, *userDeactivateFlags) {
	opts := &userDeactivateFlags{}
	flags := newFlagSet("user deactivate", out, printUserDeactivateUsage)
	flags.BoolVar(&opts.yes, "yes", false, "Confirm user deactivation")
	return flags, opts
}

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
		usageErrorf(a.stderr, "unknown user command: %s", args[0])
		printUserUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runUserList(args []string) int {
	if hasHelpFlag(args) {
		printUserListUsage(a.stdout)
		return exitOK
	}

	flags := userListFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
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

	flags, opts := userCreateFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	opts.username = strings.TrimSpace(opts.username)
	if opts.username == "" {
		return usageError(a.stderr, "username is required")
	}
	if !opts.passwordStdin {
		return usageError(a.stderr, "password-stdin is required for user create")
	}

	password, err := readPassword(a.stdin)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	if password == "" {
		return usageError(a.stderr, "password is required")
	}

	var displayNamePtr *string
	opts.displayName = strings.TrimSpace(opts.displayName)
	if opts.displayName != "" {
		displayNamePtr = &opts.displayName
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resp, err := api.CreateUser(ctx, opts.username, password, displayNamePtr)
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

	flags, opts := userDeactivateFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "user id is required")
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

	if err := api.DeactivateUser(ctx, id); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, userDeactivateResult{
		ID:          id,
		Deactivated: true,
	})
}
