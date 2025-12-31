package app

import (
	"context"
)

func (a *App) runHealth(args []string) int {
	if hasHelpFlag(args) {
		printHealthUsage(a.stdout)
		return exitOK
	}

	flags := newFlagSet("health", a.stderr, printHealthUsage)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	api, err := a.newClient("")
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	resp, err := api.Health(ctx)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runVersion(args []string) int {
	if hasHelpFlag(args) {
		printVersionUsage(a.stdout)
		return exitOK
	}

	flags := newFlagSet("version", a.stderr, printVersionUsage)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		return usageError(a.stderr, "version does not accept arguments")
	}

	info := versionInfo{
		Version: Version,
		Commit:  Commit,
		BuiltAt: BuiltAt,
	}
	return writeOutput(a.stdout, a.cfg.Output, info)
}
