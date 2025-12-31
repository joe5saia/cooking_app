package app

import (
	"flag"
	"io"
	"sort"
)

// command describes a cookctl command or subcommand.
type command struct {
	Name        string
	Synopsis    string
	Usage       func(io.Writer)
	Run         func(*App, []string) int
	FlagSet     flagSetBuilder
	Subcommands []*command
}

func commandRegistry() []*command {
	return []*command{
		{
			Name:     "health",
			Synopsis: "Check API health",
			Usage:    printHealthUsage,
			Run:      (*App).runHealth,
		},
		{
			Name:     "version",
			Synopsis: "Show version info",
			Usage:    printVersionUsage,
			Run:      (*App).runVersion,
		},
		{
			Name:     "completion",
			Synopsis: "Generate shell completions",
			Usage:    printCompletionUsage,
			Run:      (*App).runCompletion,
			Subcommands: []*command{
				{Name: "bash"},
				{Name: "zsh"},
				{Name: "fish"},
			},
		},
		{
			Name:     "help",
			Synopsis: "Show help",
			Usage:    printHelpUsage,
			Run:      (*App).runHelp,
		},
		{
			Name:     "auth",
			Synopsis: "Manage credentials",
			Usage:    printAuthUsage,
			Run:      (*App).runAuth,
			Subcommands: []*command{
				{Name: "login", Usage: printAuthLoginUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := authLoginFlagSet(out); return fs }},
				{Name: "set", Usage: printAuthSetUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := authSetFlagSet(out); return fs }},
				{Name: "status", Usage: printAuthStatusUsage, FlagSet: authStatusFlagSet},
				{Name: "whoami", Usage: printAuthWhoAmIUsage, FlagSet: authWhoAmIFlagSet},
				{Name: "logout", Usage: printAuthLogoutUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := authLogoutFlagSet(out); return fs }},
			},
		},
		{
			Name:     "token",
			Synopsis: "Manage personal access tokens",
			Usage:    printTokenUsage,
			Run:      (*App).runToken,
			Subcommands: []*command{
				{Name: commandList, Usage: printTokenListUsage, FlagSet: tokenListFlagSet},
				{Name: commandCreate, Usage: printTokenCreateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := tokenCreateFlagSet(out); return fs }},
				{Name: "revoke", Usage: printTokenRevokeUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := tokenRevokeFlagSet(out); return fs }},
			},
		},
		{
			Name:     "tag",
			Synopsis: "Manage tags",
			Usage:    printTagUsage,
			Run:      (*App).runTag,
			Subcommands: []*command{
				{Name: commandList, Usage: printTagListUsage},
				{Name: commandCreate, Usage: printTagCreateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := tagCreateFlagSet(out); return fs }},
				{Name: commandUpdate, Usage: printTagUpdateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := tagUpdateFlagSet(out); return fs }},
				{Name: commandDelete, Usage: printTagDeleteUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := tagDeleteFlagSet(out); return fs }},
			},
		},
		{
			Name:     "item",
			Synopsis: "Manage items",
			Usage:    printItemUsage,
			Run:      (*App).runItem,
			Subcommands: []*command{
				{Name: commandList, Usage: printItemListUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := itemListFlagSet(out); return fs }},
				{Name: commandCreate, Usage: printItemCreateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := itemCreateFlagSet(out); return fs }},
				{Name: commandUpdate, Usage: printItemUpdateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := itemUpdateFlagSet(out); return fs }},
				{Name: commandDelete, Usage: printItemDeleteUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := itemDeleteFlagSet(out); return fs }},
			},
		},
		{
			Name:     "book",
			Synopsis: "Manage recipe books",
			Usage:    printBookUsage,
			Run:      (*App).runBook,
			Subcommands: []*command{
				{Name: commandList, Usage: printBookListUsage},
				{Name: commandCreate, Usage: printBookCreateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := bookCreateFlagSet(out); return fs }},
				{Name: commandUpdate, Usage: printBookUpdateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := bookUpdateFlagSet(out); return fs }},
				{Name: commandDelete, Usage: printBookDeleteUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := bookDeleteFlagSet(out); return fs }},
			},
		},
		{
			Name:     "user",
			Synopsis: "Manage users",
			Usage:    printUserUsage,
			Run:      (*App).runUser,
			Subcommands: []*command{
				{Name: commandList, Usage: printUserListUsage, FlagSet: userListFlagSet},
				{Name: commandCreate, Usage: printUserCreateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := userCreateFlagSet(out); return fs }},
				{Name: "deactivate", Usage: printUserDeactivateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := userDeactivateFlagSet(out); return fs }},
			},
		},
		{
			Name:     "recipe",
			Synopsis: "Manage recipes",
			Usage:    printRecipeUsage,
			Run:      (*App).runRecipe,
			Subcommands: []*command{
				{Name: commandList, Usage: printRecipeListUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := recipeListFlagSet(out); return fs }},
				{Name: commandGet, Usage: printRecipeGetUsage, FlagSet: recipeGetFlagSet},
				{Name: commandCreate, Usage: printRecipeCreateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := recipeCreateFlagSet(out); return fs }},
				{Name: commandUpdate, Usage: printRecipeUpdateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := recipeUpdateFlagSet(out); return fs }},
				{Name: commandInit, Usage: printRecipeInitUsage, FlagSet: recipeInitFlagSet},
				{Name: commandTemplate, Usage: printRecipeTemplateUsage, FlagSet: recipeTemplateFlagSet},
				{Name: commandExport, Usage: printRecipeExportUsage, FlagSet: recipeExportFlagSet},
				{Name: commandImport, Usage: printRecipeImportUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := recipeImportFlagSet(out); return fs }},
				{Name: commandTag, Usage: printRecipeTagUsage, FlagSet: recipeTagFlagSet},
				{Name: commandClone, Usage: printRecipeCloneUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := recipeCloneFlagSet(out); return fs }},
				{Name: commandEdit, Usage: printRecipeEditUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := recipeEditFlagSet(out); return fs }},
				{Name: commandDelete, Usage: printRecipeDeleteUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := recipeDeleteFlagSet(out); return fs }},
				{Name: commandRestore, Usage: printRecipeRestoreUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := recipeRestoreFlagSet(out); return fs }},
			},
		},
		{
			Name:     "meal-plan",
			Synopsis: "Manage meal plans",
			Usage:    printMealPlanUsage,
			Run:      (*App).runMealPlan,
			Subcommands: []*command{
				{Name: commandList, Usage: printMealPlanListUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := mealPlanListFlagSet(out); return fs }},
				{Name: commandCreate, Usage: printMealPlanCreateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := mealPlanCreateFlagSet(out); return fs }},
				{Name: commandDelete, Usage: printMealPlanDeleteUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := mealPlanDeleteFlagSet(out); return fs }},
			},
		},
		{
			Name:     "shopping-list",
			Synopsis: "Manage shopping lists",
			Usage:    printShoppingListUsage,
			Run:      (*App).runShoppingList,
			Subcommands: []*command{
				{Name: commandList, Usage: printShoppingListListUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := shoppingListListFlagSet(out); return fs }},
				{Name: commandCreate, Usage: printShoppingListCreateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := shoppingListCreateFlagSet(out); return fs }},
				{Name: commandGet, Usage: printShoppingListGetUsage},
				{Name: commandUpdate, Usage: printShoppingListUpdateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := shoppingListUpdateFlagSet(out); return fs }},
				{Name: commandDelete, Usage: printShoppingListDeleteUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := shoppingListDeleteFlagSet(out); return fs }},
				{
					Name:  "items",
					Usage: printShoppingListItemsUsage,
					Subcommands: []*command{
						{Name: commandList, Usage: printShoppingListItemsListUsage, FlagSet: shoppingListItemsListFlagSet},
						{Name: commandCreate, Usage: printShoppingListItemsCreateUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := shoppingListItemsAddFlagSet(out); return fs }},
						{Name: "from-recipes", Usage: printShoppingListItemsFromRecipesUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := shoppingListItemsFromRecipesFlagSet(out); return fs }},
						{Name: "from-meal-plan", Usage: printShoppingListItemsFromMealPlanUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := shoppingListItemsFromMealPlanFlagSet(out); return fs }},
						{Name: "purchase", Usage: printShoppingListItemsPurchaseUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := shoppingListItemsPurchaseFlagSet(out); return fs }},
						{Name: commandDelete, Usage: printShoppingListItemsDeleteUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := shoppingListItemsDeleteFlagSet(out); return fs }},
					},
				},
			},
		},
		{
			Name:     "config",
			Synopsis: "Manage config values",
			Usage:    printConfigUsage,
			Run:      (*App).runConfig,
			Subcommands: []*command{
				{Name: "view", Usage: printConfigViewUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := configViewFlagSet(out); return fs }},
				{Name: "set", Usage: printConfigSetUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := configSetFlagSet(out); return fs }},
				{Name: "unset", Usage: printConfigUnsetUsage, FlagSet: func(out io.Writer) *flag.FlagSet { fs, _ := configUnsetFlagSet(out); return fs }},
				{Name: "path", Usage: printConfigPathUsage},
			},
		},
	}
}

func findCommand(commands []*command, name string) *command {
	for _, cmd := range commands {
		if cmd.Name == name {
			return cmd
		}
	}
	return nil
}

// findCommandPath locates a command by walking the registry path.
func findCommandPath(commands []*command, names ...string) *command {
	if len(names) == 0 {
		return nil
	}
	current := findCommand(commands, names[0])
	for _, name := range names[1:] {
		if current == nil {
			return nil
		}
		current = findCommand(current.Subcommands, name)
	}
	return current
}

func commandNames(commands []*command) []string {
	names := make([]string, 0, len(commands))
	for _, cmd := range commands {
		names = append(names, cmd.Name)
	}
	sort.Strings(names)
	return names
}
