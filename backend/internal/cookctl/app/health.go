package app

import (
	"context"
	"flag"
)

func (a *App) runHealth(args []string) int {
	if hasHelpFlag(args) {
		printHealthUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("health", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
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

	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		writeLine(a.stderr, "version does not accept arguments")
		return exitUsage
	}

	info := versionInfo{
		Version: Version,
		Commit:  Commit,
		BuiltAt: BuiltAt,
	}
	return writeOutput(a.stdout, a.cfg.Output, info)
}
