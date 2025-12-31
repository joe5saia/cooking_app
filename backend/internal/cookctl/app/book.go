package app

import (
	"context"
	"flag"
	"io"
	"strings"
)

type bookCreateFlags struct {
	name string
}

type bookUpdateFlags struct {
	name string
}

type bookDeleteFlags struct {
	yes bool
}

func bookCreateFlagSet(out io.Writer) (*flag.FlagSet, *bookCreateFlags) {
	opts := &bookCreateFlags{}
	flags := newFlagSet("book create", out, printBookCreateUsage)
	flags.StringVar(&opts.name, "name", "", "Recipe book name")
	return flags, opts
}

func bookUpdateFlagSet(out io.Writer) (*flag.FlagSet, *bookUpdateFlags) {
	opts := &bookUpdateFlags{}
	flags := newFlagSet("book update", out, printBookUpdateUsage)
	flags.StringVar(&opts.name, "name", "", "Recipe book name")
	return flags, opts
}

func bookDeleteFlagSet(out io.Writer) (*flag.FlagSet, *bookDeleteFlags) {
	opts := &bookDeleteFlags{}
	flags := newFlagSet("book delete", out, printBookDeleteUsage)
	flags.BoolVar(&opts.yes, "yes", false, "Confirm recipe book deletion")
	return flags, opts
}

func (a *App) runBook(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printBookUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printBookUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case commandList:
		return a.runBookList(args[1:])
	case commandCreate:
		return a.runBookCreate(args[1:])
	case commandUpdate:
		return a.runBookUpdate(args[1:])
	case commandDelete:
		return a.runBookDelete(args[1:])
	default:
		usageErrorf(a.stderr, "unknown book command: %s", args[0])
		printBookUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runBookList(args []string) int {
	if hasHelpFlag(args) {
		printBookListUsage(a.stdout)
		return exitOK
	}

	flags := newFlagSet("book list", a.stderr, printBookListUsage)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resp, err := api.RecipeBooks(ctx)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runBookCreate(args []string) int {
	if hasHelpFlag(args) {
		printBookCreateUsage(a.stdout)
		return exitOK
	}

	flags, opts := bookCreateFlagSet(a.stderr)
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

	resp, err := api.CreateRecipeBook(ctx, opts.name)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runBookUpdate(args []string) int {
	if hasHelpFlag(args) {
		printBookUpdateUsage(a.stdout)
		return exitOK
	}

	flags, opts := bookUpdateFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "recipe book id is required")
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

	resp, err := api.UpdateRecipeBook(ctx, id, opts.name)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runBookDelete(args []string) int {
	if hasHelpFlag(args) {
		printBookDeleteUsage(a.stdout)
		return exitOK
	}

	flags, opts := bookDeleteFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "recipe book id is required")
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

	if err := api.DeleteRecipeBook(ctx, id); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, bookDeleteResult{
		ID:      id,
		Deleted: true,
	})
}
