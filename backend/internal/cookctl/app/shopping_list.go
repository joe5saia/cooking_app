package app

import (
	"context"
	"flag"
	"io"
	"strings"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
)

type shoppingListListFlags struct {
	start string
	end   string
}

type shoppingListCreateFlags struct {
	date  string
	name  string
	notes string
}

type shoppingListUpdateFlags struct {
	date  string
	name  string
	notes string
}

type shoppingListDeleteFlags struct {
	yes bool
}

type shoppingListItemsAddFlags struct {
	itemID       string
	quantityRaw  string
	quantityText string
	unit         string
}

type shoppingListItemsFromRecipesFlags struct {
	recipeIDs csvStrings
}

type shoppingListItemsFromMealPlanFlags struct {
	date string
}

type shoppingListItemsPurchaseFlags struct {
	listID    string
	itemID    string
	purchased bool
}

type shoppingListItemsDeleteFlags struct {
	listID string
	itemID string
	yes    bool
}

func shoppingListListFlagSet(out io.Writer) (*flag.FlagSet, *shoppingListListFlags) {
	opts := &shoppingListListFlags{}
	flags := newFlagSet("shopping-list list", out, printShoppingListListUsage)
	flags.StringVar(&opts.start, "start", "", "Start date (YYYY-MM-DD)")
	flags.StringVar(&opts.end, "end", "", "End date (YYYY-MM-DD)")
	return flags, opts
}

func shoppingListCreateFlagSet(out io.Writer) (*flag.FlagSet, *shoppingListCreateFlags) {
	opts := &shoppingListCreateFlags{}
	flags := newFlagSet("shopping-list create", out, printShoppingListCreateUsage)
	flags.StringVar(&opts.date, "date", "", "Shopping list date (YYYY-MM-DD)")
	flags.StringVar(&opts.name, "name", "", "Shopping list name")
	flags.StringVar(&opts.notes, "notes", "", "Shopping list notes")
	return flags, opts
}

func shoppingListUpdateFlagSet(out io.Writer) (*flag.FlagSet, *shoppingListUpdateFlags) {
	opts := &shoppingListUpdateFlags{}
	flags := newFlagSet("shopping-list update", out, printShoppingListUpdateUsage)
	flags.StringVar(&opts.date, "date", "", "Shopping list date (YYYY-MM-DD)")
	flags.StringVar(&opts.name, "name", "", "Shopping list name")
	flags.StringVar(&opts.notes, "notes", "", "Shopping list notes")
	return flags, opts
}

func shoppingListDeleteFlagSet(out io.Writer) (*flag.FlagSet, *shoppingListDeleteFlags) {
	opts := &shoppingListDeleteFlags{}
	flags := newFlagSet("shopping-list delete", out, printShoppingListDeleteUsage)
	flags.BoolVar(&opts.yes, "yes", false, "Confirm shopping list deletion")
	return flags, opts
}

func shoppingListItemsListFlagSet(out io.Writer) *flag.FlagSet {
	return newFlagSet("shopping-list items list", out, printShoppingListItemsListUsage)
}

func shoppingListItemsAddFlagSet(out io.Writer) (*flag.FlagSet, *shoppingListItemsAddFlags) {
	opts := &shoppingListItemsAddFlags{}
	flags := newFlagSet("shopping-list items create", out, printShoppingListItemsCreateUsage)
	flags.StringVar(&opts.itemID, "item-id", "", "Item id")
	flags.StringVar(&opts.quantityRaw, "quantity", "", "Item quantity")
	flags.StringVar(&opts.quantityText, "quantity-text", "", "Item quantity text")
	flags.StringVar(&opts.unit, "unit", "", "Item unit")
	return flags, opts
}

func shoppingListItemsFromRecipesFlagSet(out io.Writer) (*flag.FlagSet, *shoppingListItemsFromRecipesFlags) {
	opts := &shoppingListItemsFromRecipesFlags{}
	flags := newFlagSet("shopping-list items from-recipes", out, printShoppingListItemsFromRecipesUsage)
	flags.Var(&opts.recipeIDs, "recipe-id", "Recipe id (repeatable)")
	return flags, opts
}

func shoppingListItemsFromMealPlanFlagSet(out io.Writer) (*flag.FlagSet, *shoppingListItemsFromMealPlanFlags) {
	opts := &shoppingListItemsFromMealPlanFlags{}
	flags := newFlagSet("shopping-list items from-meal-plan", out, printShoppingListItemsFromMealPlanUsage)
	flags.StringVar(&opts.date, "date", "", "Meal plan date (YYYY-MM-DD)")
	return flags, opts
}

func shoppingListItemsPurchaseFlagSet(out io.Writer) (*flag.FlagSet, *shoppingListItemsPurchaseFlags) {
	opts := &shoppingListItemsPurchaseFlags{}
	flags := newFlagSet("shopping-list items purchase", out, printShoppingListItemsPurchaseUsage)
	flags.StringVar(&opts.listID, "list-id", "", "Shopping list id")
	flags.StringVar(&opts.itemID, "item-id", "", "Shopping list item id")
	flags.BoolVar(&opts.purchased, "purchased", false, "Mark item as purchased")
	return flags, opts
}

func shoppingListItemsDeleteFlagSet(out io.Writer) (*flag.FlagSet, *shoppingListItemsDeleteFlags) {
	opts := &shoppingListItemsDeleteFlags{}
	flags := newFlagSet("shopping-list items delete", out, printShoppingListItemsDeleteUsage)
	flags.StringVar(&opts.listID, "list-id", "", "Shopping list id")
	flags.StringVar(&opts.itemID, "item-id", "", "Shopping list item id")
	flags.BoolVar(&opts.yes, "yes", false, "Confirm list item deletion")
	return flags, opts
}

// runShoppingList routes shopping list subcommands.
func (a *App) runShoppingList(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printShoppingListUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printShoppingListUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case commandList:
		return a.runShoppingListList(args[1:])
	case commandCreate:
		return a.runShoppingListCreate(args[1:])
	case commandGet:
		return a.runShoppingListGet(args[1:])
	case commandUpdate:
		return a.runShoppingListUpdate(args[1:])
	case commandDelete:
		return a.runShoppingListDelete(args[1:])
	case "items":
		return a.runShoppingListItems(args[1:])
	default:
		usageErrorf(a.stderr, "unknown shopping-list command: %s", args[0])
		printShoppingListUsage(a.stderr)
		return exitUsage
	}
}

// runShoppingListList lists shopping lists within a date range.
func (a *App) runShoppingListList(args []string) int {
	if hasHelpFlag(args) {
		printShoppingListListUsage(a.stdout)
		return exitOK
	}

	flags, opts := shoppingListListFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	startDate, err := parseISODate("start", opts.start)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	endDate, err := parseISODate("end", opts.end)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if startDate.After(endDate) {
		return usageError(a.stderr, "end must be on or after start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resp, err := api.ShoppingLists(ctx, startDate.Format(isoDateLayout), endDate.Format(isoDateLayout))
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

// runShoppingListCreate creates a new shopping list.
func (a *App) runShoppingListCreate(args []string) int {
	if hasHelpFlag(args) {
		printShoppingListCreateUsage(a.stdout)
		return exitOK
	}

	flags, opts := shoppingListCreateFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	listDate, err := parseISODate("date", opts.date)
	if err != nil {
		return usageError(a.stderr, err.Error())
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

	resp, err := api.CreateShoppingList(ctx, listDate.Format(isoDateLayout), opts.name, stringPtrIfNotEmpty(opts.notes))
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

// runShoppingListGet returns shopping list details.
func (a *App) runShoppingListGet(args []string) int {
	if hasHelpFlag(args) {
		printShoppingListGetUsage(a.stdout)
		return exitOK
	}

	flags := newFlagSet("shopping-list get", a.stderr, printShoppingListGetUsage)

	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "shopping list id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resp, err := api.ShoppingList(ctx, id)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

// runShoppingListUpdate updates a shopping list.
func (a *App) runShoppingListUpdate(args []string) int {
	if hasHelpFlag(args) {
		printShoppingListUpdateUsage(a.stdout)
		return exitOK
	}

	flags, opts := shoppingListUpdateFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "shopping list id is required")
	}

	listDate, err := parseISODate("date", opts.date)
	if err != nil {
		return usageError(a.stderr, err.Error())
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

	resp, err := api.UpdateShoppingList(ctx, id, listDate.Format(isoDateLayout), opts.name, stringPtrIfNotEmpty(opts.notes))
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

// runShoppingListDelete deletes a shopping list.
func (a *App) runShoppingListDelete(args []string) int {
	if hasHelpFlag(args) {
		printShoppingListDeleteUsage(a.stdout)
		return exitOK
	}

	flags, opts := shoppingListDeleteFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "shopping list id is required")
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

	if err := api.DeleteShoppingList(ctx, id); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, shoppingListDeleteResult{ID: id, Deleted: true})
}

// runShoppingListItems routes item subcommands.
func (a *App) runShoppingListItems(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printShoppingListItemsUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printShoppingListItemsUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case commandList:
		return a.runShoppingListItemsList(args[1:])
	case commandCreate:
		return a.runShoppingListItemsAdd(args[1:])
	case "from-recipes":
		return a.runShoppingListItemsFromRecipes(args[1:])
	case "from-meal-plan":
		return a.runShoppingListItemsFromMealPlan(args[1:])
	case "purchase":
		return a.runShoppingListItemsPurchase(args[1:])
	case commandDelete:
		return a.runShoppingListItemsDelete(args[1:])
	default:
		usageErrorf(a.stderr, "unknown shopping-list items command: %s", args[0])
		printShoppingListItemsUsage(a.stderr)
		return exitUsage
	}
}

// runShoppingListItemsList lists shopping list items.
func (a *App) runShoppingListItemsList(args []string) int {
	if hasHelpFlag(args) {
		printShoppingListItemsListUsage(a.stdout)
		return exitOK
	}

	flags := shoppingListItemsListFlagSet(a.stderr)

	listID, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if listID == "" {
		return usageError(a.stderr, "shopping list id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resp, err := api.ShoppingListItems(ctx, listID)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

// runShoppingListItemsAdd adds items to a shopping list.
func (a *App) runShoppingListItemsAdd(args []string) int {
	if hasHelpFlag(args) {
		printShoppingListItemsCreateUsage(a.stdout)
		return exitOK
	}

	flags, opts := shoppingListItemsAddFlagSet(a.stderr)
	listID, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if listID == "" {
		return usageError(a.stderr, "shopping list id is required")
	}
	opts.itemID = strings.TrimSpace(opts.itemID)
	if opts.itemID == "" {
		return usageError(a.stderr, "item-id is required")
	}

	input := client.ShoppingListItemInput{ItemID: opts.itemID}
	quantity, err := parseOptionalFloat(opts.quantityRaw)
	if err != nil {
		return usageError(a.stderr, "quantity must be a number")
	}
	if quantity != nil {
		input.Quantity = quantity
	}
	if trimmed := strings.TrimSpace(opts.quantityText); trimmed != "" {
		input.QuantityText = &trimmed
	}
	if trimmed := strings.TrimSpace(opts.unit); trimmed != "" {
		input.Unit = &trimmed
	}

	resp, exitCode := a.withShoppingListClient(func(ctx context.Context, api *client.Client) (interface{}, error) {
		return api.AddShoppingListItems(ctx, listID, []client.ShoppingListItemInput{input})
	})
	if exitCode != exitOK {
		return exitCode
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

// runShoppingListItemsFromRecipes adds items from recipes to a shopping list.
func (a *App) runShoppingListItemsFromRecipes(args []string) int {
	if hasHelpFlag(args) {
		printShoppingListItemsFromRecipesUsage(a.stdout)
		return exitOK
	}

	flags, opts := shoppingListItemsFromRecipesFlagSet(a.stderr)
	listID, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if listID == "" {
		return usageError(a.stderr, "shopping list id is required")
	}
	ids := opts.recipeIDs.Values()
	if len(ids) == 0 {
		return usageError(a.stderr, "recipe-id is required")
	}

	resp, exitCode := a.withShoppingListClient(func(ctx context.Context, api *client.Client) (interface{}, error) {
		return api.AddShoppingListItemsFromRecipes(ctx, listID, ids)
	})
	if exitCode != exitOK {
		return exitCode
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

// runShoppingListItemsFromMealPlan adds items from a meal plan date.
func (a *App) runShoppingListItemsFromMealPlan(args []string) int {
	if hasHelpFlag(args) {
		printShoppingListItemsFromMealPlanUsage(a.stdout)
		return exitOK
	}

	flags, opts := shoppingListItemsFromMealPlanFlagSet(a.stderr)
	listID, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if listID == "" {
		return usageError(a.stderr, "shopping list id is required")
	}
	planDate, err := parseISODate("date", opts.date)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}

	resp, exitCode := a.withShoppingListClient(func(ctx context.Context, api *client.Client) (interface{}, error) {
		return api.AddShoppingListItemsFromMealPlan(ctx, listID, planDate.Format(isoDateLayout))
	})
	if exitCode != exitOK {
		return exitCode
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

// runShoppingListItemsPurchase toggles purchase state.
func (a *App) runShoppingListItemsPurchase(args []string) int {
	if hasHelpFlag(args) {
		printShoppingListItemsPurchaseUsage(a.stdout)
		return exitOK
	}

	flags, opts := shoppingListItemsPurchaseFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	opts.listID = strings.TrimSpace(opts.listID)
	opts.itemID = strings.TrimSpace(opts.itemID)
	if opts.listID == "" {
		return usageError(a.stderr, "list-id is required")
	}
	if opts.itemID == "" {
		return usageError(a.stderr, "item-id is required")
	}

	resp, exitCode := a.withShoppingListClient(func(ctx context.Context, api *client.Client) (interface{}, error) {
		return api.UpdateShoppingListItemPurchase(ctx, opts.listID, opts.itemID, opts.purchased)
	})
	if exitCode != exitOK {
		return exitCode
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

// runShoppingListItemsDelete deletes a shopping list item.
func (a *App) runShoppingListItemsDelete(args []string) int {
	if hasHelpFlag(args) {
		printShoppingListItemsDeleteUsage(a.stdout)
		return exitOK
	}

	flags, opts := shoppingListItemsDeleteFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	opts.listID = strings.TrimSpace(opts.listID)
	opts.itemID = strings.TrimSpace(opts.itemID)
	if opts.listID == "" {
		return usageError(a.stderr, "list-id is required")
	}
	if opts.itemID == "" {
		return usageError(a.stderr, "item-id is required")
	}
	if !opts.yes {
		return usageError(a.stderr, "confirmation required; re-run with --yes")
	}

	exitCode := a.withShoppingListClientVoid(func(ctx context.Context, api *client.Client) error {
		return api.DeleteShoppingListItem(ctx, opts.listID, opts.itemID)
	})
	if exitCode != exitOK {
		return exitCode
	}

	return writeOutput(a.stdout, a.cfg.Output, shoppingListItemDeleteResult{
		ShoppingListID: opts.listID,
		ItemID:         opts.itemID,
		Deleted:        true,
	})
}

func (a *App) withShoppingListClient(fn func(context.Context, *client.Client) (interface{}, error)) (interface{}, int) {
	var zero interface{}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return zero, exitCode
	}

	resp, err := fn(ctx, api)
	if err != nil {
		return zero, a.handleAPIError(err)
	}
	return resp, exitOK
}

func (a *App) withShoppingListClientVoid(fn func(context.Context, *client.Client) error) int {
	_, exitCode := a.withShoppingListClient(func(ctx context.Context, api *client.Client) (interface{}, error) {
		return nil, fn(ctx, api)
	})
	return exitCode
}
