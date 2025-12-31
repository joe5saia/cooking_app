package app

import (
	"flag"
	"io"
	"strings"
)

// runHelp prints usage for cookctl or a specific command.
func (a *App) runHelp(args []string) int {
	if hasHelpFlag(args) {
		printHelpUsage(a.stdout)
		return exitOK
	}

	flags := newFlagSet("help", a.stderr, printHelpUsage)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	return handleHelp(flags.Args(), a.stdout, a.stderr)
}

// handleHelp prints usage for the CLI or a specific topic.
func handleHelp(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return exitOK
	}
	if len(args) > 1 {
		return usageError(stderr, "help accepts at most one argument")
	}

	cmd := findCommand(commandRegistry(), args[0])
	if cmd == nil || cmd.Usage == nil {
		return usageErrorf(stderr, "unknown help topic: %s", args[0])
	}
	cmd.Usage(stdout)

	return exitOK
}

func printUsage(w io.Writer) {
	writeLine(w, "usage: cookctl [global flags] <command> [args]")
	writeLine(w, "commands:")
	for _, cmd := range commandRegistry() {
		writeLine(w, "  "+cmd.Name)
	}
	printGlobalFlags(w)
}

// printGlobalFlags renders the global flag list from the shared flag spec.
func printGlobalFlags(w io.Writer) {
	writeLine(w, "global flags:")
	for _, spec := range globalFlagSpecs() {
		writeLine(w, formatGlobalFlagLine(spec))
	}
}

// formatGlobalFlagLine formats a single global flag line for help output.
func formatGlobalFlagLine(spec globalFlagSpec) string {
	line := "  " + spec.name
	if spec.takesValue {
		line += " <value>"
	}
	if strings.TrimSpace(spec.description) != "" {
		line += "  " + spec.description
	}
	return line
}

// printCommandUsage prints a usage line and the registered subcommands.
func printCommandUsage(w io.Writer, usageLine, commandName string) {
	printCommandUsageLines(w, []string{usageLine}, commandName)
}

// printCommandUsageLines prints usage lines and the registered subcommands.
func printCommandUsageLines(w io.Writer, usageLines []string, commandName string) {
	for _, line := range usageLines {
		writeLine(w, line)
	}
	printCommandSubcommands(w, commandName)
}

// printCommandSubcommands renders a command's subcommands using the registry.
func printCommandSubcommands(w io.Writer, commandName string) {
	cmd := findCommand(commandRegistry(), commandName)
	if cmd == nil || len(cmd.Subcommands) == 0 {
		return
	}
	writeLine(w, "commands:")
	for _, sub := range cmd.Subcommands {
		writeLine(w, "  "+sub.Name)
	}
}

// printCommandSubcommandsPath renders subcommands for a nested command path.
func printCommandSubcommandsPath(w io.Writer, path ...string) {
	cmd := findCommandPath(commandRegistry(), path...)
	if cmd == nil || len(cmd.Subcommands) == 0 {
		return
	}
	writeLine(w, "commands:")
	for _, sub := range cmd.Subcommands {
		writeLine(w, "  "+sub.Name)
	}
}

// printCompletionUsage renders usage for the completion command.
func printCompletionUsage(w io.Writer) {
	writeLine(w, "usage: cookctl completion <bash|zsh|fish>")
}

func printHelpUsage(w io.Writer) {
	writeLine(w, "usage: cookctl help [topic]")
	writeLine(w, "topics:")
	for _, cmd := range commandRegistry() {
		writeLine(w, "  "+cmd.Name)
	}
}

func printHealthUsage(w io.Writer) {
	writeLine(w, "usage: cookctl health")
}

func printVersionUsage(w io.Writer) {
	writeLine(w, "usage: cookctl version")
}

func printAuthUsage(w io.Writer) {
	printCommandUsage(w, "usage: cookctl auth <command> [flags]", "auth")
	printAuthSetUsage(w)
}

func printAuthLoginUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl auth login --username <user> --password-stdin [--token-name <name>] [--expires-at <rfc3339>]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := authLoginFlagSet(out)
		return flags
	})
}

func printAuthSetUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl auth set --token <pat> [--api-url <url>]",
		"       cookctl auth set --token-stdin [--api-url <url>]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := authSetFlagSet(out)
		return flags
	})
}

func printAuthStatusUsage(w io.Writer) {
	writeLine(w, "usage: cookctl auth status")
}

func printAuthWhoAmIUsage(w io.Writer) {
	writeLine(w, "usage: cookctl auth whoami")
}

func printAuthLogoutUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl auth logout [--revoke]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := authLogoutFlagSet(out)
		return flags
	})
}

func printTokenUsage(w io.Writer) {
	printCommandUsage(w, "usage: cookctl token <command> [flags]", "token")
}

func printTokenListUsage(w io.Writer) {
	writeLine(w, "usage: cookctl token list")
}

func printTokenCreateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl token create --name <name> [--expires-at <rfc3339>]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := tokenCreateFlagSet(out)
		return flags
	})
}

func printTokenRevokeUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl token revoke <id> --yes",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := tokenRevokeFlagSet(out)
		return flags
	})
}

func printTagUsage(w io.Writer) {
	printCommandUsage(w, "usage: cookctl tag <command> [flags]", "tag")
}

func printTagListUsage(w io.Writer) {
	writeLine(w, "usage: cookctl tag list")
}

func printTagCreateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl tag create --name <name>",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := tagCreateFlagSet(out)
		return flags
	})
}

func printTagUpdateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl tag update <id> --name <name>",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := tagUpdateFlagSet(out)
		return flags
	})
}

func printTagDeleteUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl tag delete <id> --yes",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := tagDeleteFlagSet(out)
		return flags
	})
}

// Item and shopping list usage helpers live in shopping_list_usage.go

func printBookUsage(w io.Writer) {
	printCommandUsage(w, "usage: cookctl book <command> [flags]", "book")
}

func printBookListUsage(w io.Writer) {
	writeLine(w, "usage: cookctl book list")
}

func printBookCreateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl book create --name <name>",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := bookCreateFlagSet(out)
		return flags
	})
}

func printBookUpdateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl book update <id> --name <name>",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := bookUpdateFlagSet(out)
		return flags
	})
}

func printBookDeleteUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl book delete <id> --yes",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := bookDeleteFlagSet(out)
		return flags
	})
}

func printUserUsage(w io.Writer) {
	printCommandUsage(w, "usage: cookctl user <command> [flags]", "user")
}

func printUserListUsage(w io.Writer) {
	writeLine(w, "usage: cookctl user list")
}

func printUserCreateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl user create --username <user> --password-stdin [--display-name <name>]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := userCreateFlagSet(out)
		return flags
	})
}

func printUserDeactivateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl user deactivate <id> --yes",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := userDeactivateFlagSet(out)
		return flags
	})
}

func printRecipeUsage(w io.Writer) {
	printCommandUsage(w, "usage: cookctl recipe <command> [flags]", "recipe")
}

func printRecipeListUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl recipe list [flags]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := recipeListFlagSet(out)
		return flags
	})
}

func printRecipeGetUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe get <id|title>")
}

func printRecipeCreateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl recipe create [--file <path> | --stdin | --interactive] [--allow-duplicate]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := recipeCreateFlagSet(out)
		return flags
	})
}

func printRecipeUpdateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl recipe update <id|title> [--file <path> | --stdin]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := recipeUpdateFlagSet(out)
		return flags
	})
}

func printRecipeInitUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe init [<id|title>]")
}

func printRecipeTemplateUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe template")
}

func printRecipeExportUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe export <id|title>")
}

func printRecipeImportUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl recipe import [--file <path> | --stdin] [--allow-duplicate]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := recipeImportFlagSet(out)
		return flags
	})
}

func printRecipeTagUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl recipe tag <id|title> <tag...> [--replace] [--create-missing|--no-create-missing]",
	}, recipeTagFlagSet)
}

func printRecipeCloneUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl recipe clone <id|title> [--title <title>] [--allow-duplicate]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := recipeCloneFlagSet(out)
		return flags
	})
}

func printRecipeEditUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl recipe edit <id|title> [--editor <cmd>]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := recipeEditFlagSet(out)
		return flags
	})
}

func printRecipeDeleteUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl recipe delete <id|title> --yes",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := recipeDeleteFlagSet(out)
		return flags
	})
}

func printRecipeRestoreUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl recipe restore <id|title> --yes",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := recipeRestoreFlagSet(out)
		return flags
	})
}

func printMealPlanUsage(w io.Writer) {
	printCommandUsage(w, "usage: cookctl meal-plan <command> [flags]", "meal-plan")
}

func printMealPlanListUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl meal-plan list --start <YYYY-MM-DD> --end <YYYY-MM-DD>",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := mealPlanListFlagSet(out)
		return flags
	})
}

func printMealPlanCreateUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl meal-plan create --date <YYYY-MM-DD> --recipe-id <id>",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := mealPlanCreateFlagSet(out)
		return flags
	})
}

func printMealPlanDeleteUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl meal-plan delete --date <YYYY-MM-DD> --recipe-id <id> --yes",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := mealPlanDeleteFlagSet(out)
		return flags
	})
}

func printConfigUsage(w io.Writer) {
	printCommandUsage(w, "usage: cookctl config <command> [flags]", "config")
}

func printConfigViewUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl config view [--config <path>]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := configViewFlagSet(out)
		return flags
	})
}

func printConfigSetUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl config set [--config <path>] [--api-url <url>] [--output <table|json>] [--timeout <duration>] [--debug]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := configSetFlagSet(out)
		return flags
	})
}

func printConfigUnsetUsage(w io.Writer) {
	printUsageWithFlags(w, []string{
		"usage: cookctl config unset [--config <path>] [--api-url] [--output] [--timeout] [--debug]",
	}, func(out io.Writer) *flag.FlagSet {
		flags, _ := configUnsetFlagSet(out)
		return flags
	})
}

func printConfigPathUsage(w io.Writer) {
	writeLine(w, "usage: cookctl config path")
}
