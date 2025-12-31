package app

import (
	"context"
	"flag"
	"io"
	"strings"
)

type tagCreateFlags struct {
	name string
}

type tagUpdateFlags struct {
	name string
}

type tagDeleteFlags struct {
	yes bool
}

func tagCreateFlagSet(out io.Writer) (*flag.FlagSet, *tagCreateFlags) {
	opts := &tagCreateFlags{}
	flags := newFlagSet("tag create", out, printTagCreateUsage)
	flags.StringVar(&opts.name, "name", "", "Tag name")
	return flags, opts
}

func tagUpdateFlagSet(out io.Writer) (*flag.FlagSet, *tagUpdateFlags) {
	opts := &tagUpdateFlags{}
	flags := newFlagSet("tag update", out, printTagUpdateUsage)
	flags.StringVar(&opts.name, "name", "", "Tag name")
	return flags, opts
}

func tagDeleteFlagSet(out io.Writer) (*flag.FlagSet, *tagDeleteFlags) {
	opts := &tagDeleteFlags{}
	flags := newFlagSet("tag delete", out, printTagDeleteUsage)
	flags.BoolVar(&opts.yes, "yes", false, "Confirm tag deletion")
	return flags, opts
}

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
		usageErrorf(a.stderr, "unknown tag command: %s", args[0])
		printTagUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runTagList(args []string) int {
	if hasHelpFlag(args) {
		printTagListUsage(a.stdout)
		return exitOK
	}

	flags := newFlagSet("tag list", a.stderr, printTagListUsage)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
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

	flags, opts := tagCreateFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	opts.name = strings.TrimSpace(opts.name)
	if opts.name == "" {
		return usageError(a.stderr, "name is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resp, err := api.CreateTag(ctx, opts.name)
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

	flags, opts := tagUpdateFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "tag id is required")
	}
	id = strings.TrimSpace(id)
	opts.name = strings.TrimSpace(opts.name)
	if opts.name == "" {
		return usageError(a.stderr, "name is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resp, err := api.UpdateTag(ctx, id, opts.name)
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

	flags, opts := tagDeleteFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "tag id is required")
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

	if err := api.DeleteTag(ctx, id); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, tagDeleteResult{
		ID:      id,
		Deleted: true,
	})
}
