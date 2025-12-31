package app

import (
	"flag"
	"io"
)

func printItemUsage(w io.Writer) {
	printCommandUsage(w, "usage: cookctl item <command> [flags]", "item")
}

func printItemListUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl item list [flags]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := itemListFlagSet(out)
		return flags
	})
}

func printItemCreateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl item create --name <name> [--store-url <url>] [--aisle-id <id>]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := itemCreateFlagSet(out)
		return flags
	})
}

func printItemUpdateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl item update <id> --name <name> [--store-url <url>] [--aisle-id <id>]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := itemUpdateFlagSet(out)
		return flags
	})
}

func printItemDeleteUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl item delete <id> --yes",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := itemDeleteFlagSet(out)
		return flags
	})
}

func printShoppingListUsage(w io.Writer) {
	printCommandUsage(w, "usage: cookctl shopping-list <command> [flags]", "shopping-list")
}

func printShoppingListListUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl shopping-list list --start <date> --end <date>",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := shoppingListListFlagSet(out)
		return flags
	})
}

func printShoppingListCreateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl shopping-list create --date <date> --name <name> [--notes <text>]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := shoppingListCreateFlagSet(out)
		return flags
	})
}

func printShoppingListGetUsage(w io.Writer) {
	writeLine(w, "usage: cookctl shopping-list get <id>")
}

func printShoppingListUpdateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl shopping-list update <id> --date <date> --name <name> [--notes <text>]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := shoppingListUpdateFlagSet(out)
		return flags
	})
}

func printShoppingListDeleteUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl shopping-list delete <id> --yes",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := shoppingListDeleteFlagSet(out)
		return flags
	})
}

func printShoppingListItemsUsage(w io.Writer) {
	writeLine(w, "usage: cookctl shopping-list items <command> [flags]")
	printCommandSubcommandsPath(w, "shopping-list", "items")
}

func printShoppingListItemsListUsage(w io.Writer) {
	writeLine(w, "usage: cookctl shopping-list items list <list-id>")
}

func printShoppingListItemsCreateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl shopping-list items create <list-id> --item-id <id> [--quantity <n>] [--quantity-text <text>] [--unit <text>]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := shoppingListItemsAddFlagSet(out)
		return flags
	})
}

func printShoppingListItemsFromRecipesUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl shopping-list items from-recipes <list-id> --recipe-id <id> [--recipe-id <id>]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := shoppingListItemsFromRecipesFlagSet(out)
		return flags
	})
}

func printShoppingListItemsFromMealPlanUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl shopping-list items from-meal-plan <list-id> --date <date>",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := shoppingListItemsFromMealPlanFlagSet(out)
		return flags
	})
}

func printShoppingListItemsPurchaseUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl shopping-list items purchase --list-id <id> --item-id <id> [--purchased]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := shoppingListItemsPurchaseFlagSet(out)
		return flags
	})
}

func printShoppingListItemsDeleteUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl shopping-list items delete --list-id <id> --item-id <id> --yes",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := shoppingListItemsDeleteFlagSet(out)
		return flags
	})
}
