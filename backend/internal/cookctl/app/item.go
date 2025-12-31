package app

import (
	"context"
	"flag"
	"io"
	"strings"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
)

type itemListFlags struct {
	query string
	limit int
}

type itemCreateFlags struct {
	name     string
	storeURL string
	aisleID  string
}

type itemUpdateFlags struct {
	name     string
	storeURL string
	aisleID  string
}

type itemDeleteFlags struct {
	yes bool
}

func itemListFlagSet(out io.Writer) (*flag.FlagSet, *itemListFlags) {
	opts := &itemListFlags{}
	flags := newFlagSet("item list", out, printItemListUsage)
	flags.StringVar(&opts.query, "q", "", "Search query")
	flags.IntVar(&opts.limit, "limit", 0, "Max items per page")
	return flags, opts
}

func itemCreateFlagSet(out io.Writer) (*flag.FlagSet, *itemCreateFlags) {
	opts := &itemCreateFlags{}
	flags := newFlagSet("item create", out, printItemCreateUsage)
	flags.StringVar(&opts.name, "name", "", "Item name")
	flags.StringVar(&opts.storeURL, "store-url", "", "Store URL")
	flags.StringVar(&opts.aisleID, "aisle-id", "", "Aisle id")
	return flags, opts
}

func itemUpdateFlagSet(out io.Writer) (*flag.FlagSet, *itemUpdateFlags) {
	opts := &itemUpdateFlags{}
	flags := newFlagSet("item update", out, printItemUpdateUsage)
	flags.StringVar(&opts.name, "name", "", "Item name")
	flags.StringVar(&opts.storeURL, "store-url", "", "Store URL")
	flags.StringVar(&opts.aisleID, "aisle-id", "", "Aisle id")
	return flags, opts
}

func itemDeleteFlagSet(out io.Writer) (*flag.FlagSet, *itemDeleteFlags) {
	opts := &itemDeleteFlags{}
	flags := newFlagSet("item delete", out, printItemDeleteUsage)
	flags.BoolVar(&opts.yes, "yes", false, "Confirm item deletion")
	return flags, opts
}

// runItem routes item subcommands.
func (a *App) runItem(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printItemUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printItemUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case commandList:
		return a.runItemList(args[1:])
	case commandCreate:
		return a.runItemCreate(args[1:])
	case commandUpdate:
		return a.runItemUpdate(args[1:])
	case commandDelete:
		return a.runItemDelete(args[1:])
	default:
		usageErrorf(a.stderr, "unknown item command: %s", args[0])
		printItemUsage(a.stderr)
		return exitUsage
	}
}

// runItemList lists items with optional search filters.
func (a *App) runItemList(args []string) int {
	if hasHelpFlag(args) {
		printItemListUsage(a.stdout)
		return exitOK
	}

	flags, opts := itemListFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	opts.query = strings.TrimSpace(opts.query)
	if opts.limit < 0 {
		return usageError(a.stderr, "limit must be positive")
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resp, err := api.Items(ctx, client.ItemListParams{
		Query: opts.query,
		Limit: opts.limit,
	})
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

// runItemCreate creates a new item.
func (a *App) runItemCreate(args []string) int {
	if hasHelpFlag(args) {
		printItemCreateUsage(a.stdout)
		return exitOK
	}

	flags, opts := itemCreateFlagSet(a.stderr)
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

	resp, err := api.CreateItem(ctx, opts.name, stringPtrIfNotEmpty(opts.storeURL), stringPtrIfNotEmpty(opts.aisleID))
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

// runItemUpdate updates an existing item.
func (a *App) runItemUpdate(args []string) int {
	if hasHelpFlag(args) {
		printItemUpdateUsage(a.stdout)
		return exitOK
	}

	flags, opts := itemUpdateFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "item id is required")
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

	resp, err := api.UpdateItem(ctx, id, opts.name, stringPtrIfNotEmpty(opts.storeURL), stringPtrIfNotEmpty(opts.aisleID))
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

// runItemDelete deletes an item.
func (a *App) runItemDelete(args []string) int {
	if hasHelpFlag(args) {
		printItemDeleteUsage(a.stdout)
		return exitOK
	}

	flags, opts := itemDeleteFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "item id is required")
	}
	if !opts.yes {
		return usageError(a.stderr, "confirmation required; re-run with --yes")
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	if err := api.DeleteItem(ctx, id); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, itemDeleteResult{ID: id, Deleted: true})
}
