package app

import (
	"context"
	"flag"
	"strings"
)

func (a *App) runTag(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printTagUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printTagUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case commandList:
		return a.runTagList(args[1:])
	case commandCreate:
		return a.runTagCreate(args[1:])
	case commandUpdate:
		return a.runTagUpdate(args[1:])
	case commandDelete:
		return a.runTagDelete(args[1:])
	default:
		writef(a.stderr, "unknown tag command: %s\n", args[0])
		printTagUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runTagList(args []string) int {
	if hasHelpFlag(args) {
		printTagListUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("tag list", flag.ContinueOnError)
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

	resp, err := api.Tags(ctx)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runTagCreate(args []string) int {
	if hasHelpFlag(args) {
		printTagCreateUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("tag create", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var name string
	flags.StringVar(&name, "name", "", "Tag name")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	name = strings.TrimSpace(name)
	if name == "" {
		writeLine(a.stderr, "name is required")
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

	resp, err := api.CreateTag(ctx, name)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runTagUpdate(args []string) int {
	if hasHelpFlag(args) {
		printTagUpdateUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("tag update", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var name string
	flags.StringVar(&name, "name", "", "Tag name")

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "tag id is required")
		return exitUsage
	}
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	if name == "" {
		writeLine(a.stderr, "name is required")
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

	resp, err := api.UpdateTag(ctx, id, name)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runTagDelete(args []string) int {
	if hasHelpFlag(args) {
		printTagDeleteUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("tag delete", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var yes bool
	flags.BoolVar(&yes, "yes", false, "Confirm tag deletion")

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "tag id is required")
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

	if err := api.DeleteTag(ctx, id); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, tagDeleteResult{
		ID:      id,
		Deleted: true,
	})
}
