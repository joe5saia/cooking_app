package app

import (
	"context"
	"flag"
	"strings"
)

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
		writef(a.stderr, "unknown book command: %s\n", args[0])
		printBookUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runBookList(args []string) int {
	if hasHelpFlag(args) {
		printBookListUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("book list", flag.ContinueOnError)
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

	flags := flag.NewFlagSet("book create", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var name string
	flags.StringVar(&name, "name", "", "Recipe book name")

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

	resp, err := api.CreateRecipeBook(ctx, name)
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

	flags := flag.NewFlagSet("book update", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var name string
	flags.StringVar(&name, "name", "", "Recipe book name")

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "recipe book id is required")
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

	resp, err := api.UpdateRecipeBook(ctx, id, name)
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

	flags := flag.NewFlagSet("book delete", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var yes bool
	flags.BoolVar(&yes, "yes", false, "Confirm recipe book deletion")

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "recipe book id is required")
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

	if err := api.DeleteRecipeBook(ctx, id); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, bookDeleteResult{
		ID:      id,
		Deleted: true,
	})
}
