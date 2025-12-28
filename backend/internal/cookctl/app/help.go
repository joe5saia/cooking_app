package app

import (
	"flag"
	"io"
)

// runHelp prints usage for cookctl or a specific command.
func (a *App) runHelp(args []string) int {
	if hasHelpFlag(args) {
		printHelpUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("help", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
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
		writeLine(stderr, "help accepts at most one argument")
		return exitUsage
	}

	switch args[0] {
	case "health":
		printHealthUsage(stdout)
	case "version":
		printVersionUsage(stdout)
	case "completion":
		printCompletionUsage(stdout)
	case "help":
		printHelpUsage(stdout)
	case "auth":
		printAuthUsage(stdout)
	case "token":
		printTokenUsage(stdout)
	case "tag":
		printTagUsage(stdout)
	case "book":
		printBookUsage(stdout)
	case "user":
		printUserUsage(stdout)
	case "recipe":
		printRecipeUsage(stdout)
	case "meal-plan":
		printMealPlanUsage(stdout)
	case "config":
		printConfigUsage(stdout)
	default:
		writef(stderr, "unknown help topic: %s\n", args[0])
		return exitUsage
	}

	return exitOK
}

func printUsage(w io.Writer) {
	writeLine(w, "usage: cookctl [global flags] <command> [args]")
	writeLine(w, "commands:")
	writeLine(w, "  health")
	writeLine(w, "  version")
	writeLine(w, "  completion")
	writeLine(w, "  help")
	writeLine(w, "  auth")
	writeLine(w, "  token")
	writeLine(w, "  tag")
	writeLine(w, "  book")
	writeLine(w, "  user")
	writeLine(w, "  recipe")
	writeLine(w, "  meal-plan")
	writeLine(w, "  config")
	writeLine(w, "global flags:")
	writeLine(w, "  --api-url <url>")
	writeLine(w, "  --output <table|json>")
	writeLine(w, "  --timeout <duration>")
	writeLine(w, "  --debug")
	writeLine(w, "  --skip-health-check")
	writeLine(w, "  --version")
	writeLine(w, "  --help")
	writeLine(w, "  -h")
}

// printCompletionUsage renders usage for the completion command.
func printCompletionUsage(w io.Writer) {
	writeLine(w, "usage: cookctl completion <bash|zsh|fish>")
}

func printHelpUsage(w io.Writer) {
	writeLine(w, "usage: cookctl help [topic]")
	writeLine(w, "topics:")
	writeLine(w, "  health")
	writeLine(w, "  version")
	writeLine(w, "  completion")
	writeLine(w, "  help")
	writeLine(w, "  auth")
	writeLine(w, "  token")
	writeLine(w, "  tag")
	writeLine(w, "  book")
	writeLine(w, "  user")
	writeLine(w, "  recipe")
	writeLine(w, "  meal-plan")
	writeLine(w, "  config")
}

func printHealthUsage(w io.Writer) {
	writeLine(w, "usage: cookctl health")
}

func printVersionUsage(w io.Writer) {
	writeLine(w, "usage: cookctl version")
}

func printAuthUsage(w io.Writer) {
	writeLine(w, "usage: cookctl auth <command> [flags]")
	writeLine(w, "commands:")
	writeLine(w, "  login --username <user> --password-stdin [--token-name <name>] [--expires-at <rfc3339>]")
	writeLine(w, "  set --token <pat> [--api-url <url>]")
	writeLine(w, "  set --token-stdin [--api-url <url>]")
	writeLine(w, "  status")
	writeLine(w, "  whoami")
	writeLine(w, "  logout [--revoke]")
}

func printAuthLoginUsage(w io.Writer) {
	writeLine(w, "usage: cookctl auth login --username <user> --password-stdin [--token-name <name>] [--expires-at <rfc3339>]")
	writeLine(w, "flags:")
	writeLine(w, "  --username <user>        Username for login")
	writeLine(w, "  --password-stdin         Read password from stdin")
	writeLine(w, "  --token-name <name>      Name for the new PAT (default: cookctl)")
	writeLine(w, "  --expires-at <rfc3339>   Token expiration")
}

func printAuthSetUsage(w io.Writer) {
	writeLine(w, "usage: cookctl auth set --token <pat> [--api-url <url>]")
	writeLine(w, "       cookctl auth set --token-stdin [--api-url <url>]")
	writeLine(w, "flags:")
	writeLine(w, "  --token <pat>            Personal access token")
	writeLine(w, "  --token-stdin            Read token from stdin")
	writeLine(w, "  --api-url <url>          API base URL override")
}

func printAuthStatusUsage(w io.Writer) {
	writeLine(w, "usage: cookctl auth status")
}

func printAuthWhoAmIUsage(w io.Writer) {
	writeLine(w, "usage: cookctl auth whoami")
}

func printAuthLogoutUsage(w io.Writer) {
	writeLine(w, "usage: cookctl auth logout [--revoke]")
	writeLine(w, "flags:")
	writeLine(w, "  --revoke                 Revoke stored token before clearing credentials")
}

func printTokenUsage(w io.Writer) {
	writeLine(w, "usage: cookctl token <command> [flags]")
	writeLine(w, "commands:")
	writeLine(w, "  list")
	writeLine(w, "  create --name <name> [--expires-at <rfc3339>]")
	writeLine(w, "  revoke <id> --yes")
}

func printTokenListUsage(w io.Writer) {
	writeLine(w, "usage: cookctl token list")
}

func printTokenCreateUsage(w io.Writer) {
	writeLine(w, "usage: cookctl token create --name <name> [--expires-at <rfc3339>]")
	writeLine(w, "flags:")
	writeLine(w, "  --name <name>            Token name")
	writeLine(w, "  --expires-at <rfc3339>   Token expiration")
}

func printTokenRevokeUsage(w io.Writer) {
	writeLine(w, "usage: cookctl token revoke <id> --yes")
	writeLine(w, "flags:")
	writeLine(w, "  --yes                    Confirm token revocation")
}

func printTagUsage(w io.Writer) {
	writeLine(w, "usage: cookctl tag <command> [flags]")
	writeLine(w, "commands:")
	writeLine(w, "  list")
	writeLine(w, "  create --name <name>")
	writeLine(w, "  update <id> --name <name>")
	writeLine(w, "  delete <id> --yes")
}

func printTagListUsage(w io.Writer) {
	writeLine(w, "usage: cookctl tag list")
}

func printTagCreateUsage(w io.Writer) {
	writeLine(w, "usage: cookctl tag create --name <name>")
	writeLine(w, "flags:")
	writeLine(w, "  --name <name>            Tag name")
}

func printTagUpdateUsage(w io.Writer) {
	writeLine(w, "usage: cookctl tag update <id> --name <name>")
	writeLine(w, "flags:")
	writeLine(w, "  --name <name>            Tag name")
}

func printTagDeleteUsage(w io.Writer) {
	writeLine(w, "usage: cookctl tag delete <id> --yes")
	writeLine(w, "flags:")
	writeLine(w, "  --yes                    Confirm tag deletion")
}

func printBookUsage(w io.Writer) {
	writeLine(w, "usage: cookctl book <command> [flags]")
	writeLine(w, "commands:")
	writeLine(w, "  list")
	writeLine(w, "  create --name <name>")
	writeLine(w, "  update <id> --name <name>")
	writeLine(w, "  delete <id> --yes")
}

func printBookListUsage(w io.Writer) {
	writeLine(w, "usage: cookctl book list")
}

func printBookCreateUsage(w io.Writer) {
	writeLine(w, "usage: cookctl book create --name <name>")
	writeLine(w, "flags:")
	writeLine(w, "  --name <name>            Book name")
}

func printBookUpdateUsage(w io.Writer) {
	writeLine(w, "usage: cookctl book update <id> --name <name>")
	writeLine(w, "flags:")
	writeLine(w, "  --name <name>            Book name")
}

func printBookDeleteUsage(w io.Writer) {
	writeLine(w, "usage: cookctl book delete <id> --yes")
	writeLine(w, "flags:")
	writeLine(w, "  --yes                    Confirm book deletion")
}

func printUserUsage(w io.Writer) {
	writeLine(w, "usage: cookctl user <command> [flags]")
	writeLine(w, "commands:")
	writeLine(w, "  list")
	writeLine(w, "  create --username <user> --password-stdin [--display-name <name>]")
	writeLine(w, "  deactivate <id> --yes")
}

func printUserListUsage(w io.Writer) {
	writeLine(w, "usage: cookctl user list")
}

func printUserCreateUsage(w io.Writer) {
	writeLine(w, "usage: cookctl user create --username <user> --password-stdin [--display-name <name>]")
	writeLine(w, "flags:")
	writeLine(w, "  --username <user>        Username for the new user")
	writeLine(w, "  --password-stdin         Read password from stdin")
	writeLine(w, "  --display-name <name>    Display name for the new user")
}

func printUserDeactivateUsage(w io.Writer) {
	writeLine(w, "usage: cookctl user deactivate <id> --yes")
	writeLine(w, "flags:")
	writeLine(w, "  --yes                    Confirm user deactivation")
}

func printRecipeUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe <command> [flags]")
	writeLine(w, "commands:")
	writeLine(w, "  list [--q <text>] [--book <name>|--book-id <id>] [--tag <name>|--tag-id <id>] [--servings <n>] [--include-deleted] [--limit <n>] [--cursor <c>] [--all] [--with-counts]")
	writeLine(w, "  get <id|title>")
	writeLine(w, "  create [--file <path> | --stdin | --interactive] [--allow-duplicate]")
	writeLine(w, "  update <id|title> [--file <path> | --stdin]")
	writeLine(w, "  init [<id|title>]")
	writeLine(w, "  template")
	writeLine(w, "  export <id|title>")
	writeLine(w, "  import [--file <path> | --stdin] [--allow-duplicate]")
	writeLine(w, "  tag <id|title> <tag...> [--replace] [--create-missing|--no-create-missing]")
	writeLine(w, "  clone <id|title> [--title <title>] [--allow-duplicate]")
	writeLine(w, "  edit <id|title> [--editor <cmd>]")
	writeLine(w, "  delete <id|title> --yes")
	writeLine(w, "  restore <id|title> --yes")
}

func printRecipeListUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe list [flags]")
	writeLine(w, "flags:")
	writeLine(w, "  --q <text>               Search query")
	writeLine(w, "  --book <name>            Filter by recipe book name")
	writeLine(w, "  --book-id <id>           Filter by recipe book id")
	writeLine(w, "  --tag <name>             Filter by tag name")
	writeLine(w, "  --tag-id <id>            Filter by tag id")
	writeLine(w, "  --servings <n>           Filter by servings count")
	writeLine(w, "  --include-deleted        Include deleted recipes")
	writeLine(w, "  --limit <n>              Max items per page")
	writeLine(w, "  --cursor <cursor>        Pagination cursor")
	writeLine(w, "  --all                    Fetch all pages")
	writeLine(w, "  --with-counts            Include ingredient and step counts")
}

func printRecipeGetUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe get <id|title>")
}

func printRecipeCreateUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe create [--file <path> | --stdin | --interactive] [--allow-duplicate]")
	writeLine(w, "flags:")
	writeLine(w, "  --file <path>            Read recipe JSON from a file")
	writeLine(w, "  --stdin                  Read recipe JSON from stdin")
	writeLine(w, "  --interactive            Create recipe with interactive prompts")
	writeLine(w, "  --allow-duplicate        Allow duplicate recipe titles")
}

func printRecipeUpdateUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe update <id|title> [--file <path> | --stdin]")
	writeLine(w, "flags:")
	writeLine(w, "  --file <path>            Read recipe JSON from a file")
	writeLine(w, "  --stdin                  Read recipe JSON from stdin")
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
	writeLine(w, "usage: cookctl recipe import [--file <path> | --stdin] [--allow-duplicate]")
	writeLine(w, "flags:")
	writeLine(w, "  --file <path>            Read recipe JSON from a file")
	writeLine(w, "  --stdin                  Read recipe JSON from stdin")
	writeLine(w, "  --allow-duplicate        Allow duplicate recipe titles")
}

func printRecipeTagUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe tag <id|title> <tag...> [--replace] [--create-missing|--no-create-missing]")
	writeLine(w, "flags:")
	writeLine(w, "  --replace                Replace recipe tags instead of merging")
	writeLine(w, "  --create-missing         Create tags that do not exist (default)")
	writeLine(w, "  --no-create-missing      Fail when tags are missing")
}

func printRecipeCloneUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe clone <id|title> [--title <title>] [--allow-duplicate]")
	writeLine(w, "flags:")
	writeLine(w, "  --title <title>          Title for the cloned recipe")
	writeLine(w, "  --allow-duplicate        Allow duplicate recipe titles")
}

func printRecipeEditUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe edit <id|title> [--editor <cmd>]")
	writeLine(w, "flags:")
	writeLine(w, "  --editor <cmd>           Editor command")
}

func printRecipeDeleteUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe delete <id|title> --yes")
	writeLine(w, "flags:")
	writeLine(w, "  --yes                    Confirm recipe deletion")
}

func printRecipeRestoreUsage(w io.Writer) {
	writeLine(w, "usage: cookctl recipe restore <id|title> --yes")
	writeLine(w, "flags:")
	writeLine(w, "  --yes                    Confirm recipe restore")
}

func printMealPlanUsage(w io.Writer) {
	writeLine(w, "usage: cookctl meal-plan <command> [flags]")
	writeLine(w, "commands:")
	writeLine(w, "  list --start <YYYY-MM-DD> --end <YYYY-MM-DD>")
	writeLine(w, "  create --date <YYYY-MM-DD> --recipe-id <id>")
	writeLine(w, "  delete --date <YYYY-MM-DD> --recipe-id <id> --yes")
}

func printMealPlanListUsage(w io.Writer) {
	writeLine(w, "usage: cookctl meal-plan list --start <YYYY-MM-DD> --end <YYYY-MM-DD>")
	writeLine(w, "flags:")
	writeLine(w, "  --start <YYYY-MM-DD>     Start date")
	writeLine(w, "  --end <YYYY-MM-DD>       End date")
}

func printMealPlanCreateUsage(w io.Writer) {
	writeLine(w, "usage: cookctl meal-plan create --date <YYYY-MM-DD> --recipe-id <id>")
	writeLine(w, "flags:")
	writeLine(w, "  --date <YYYY-MM-DD>      Meal plan date")
	writeLine(w, "  --recipe-id <id>         Recipe id")
}

func printMealPlanDeleteUsage(w io.Writer) {
	writeLine(w, "usage: cookctl meal-plan delete --date <YYYY-MM-DD> --recipe-id <id> --yes")
	writeLine(w, "flags:")
	writeLine(w, "  --date <YYYY-MM-DD>      Meal plan date")
	writeLine(w, "  --recipe-id <id>         Recipe id")
	writeLine(w, "  --yes                    Confirm meal plan deletion")
}

func printConfigUsage(w io.Writer) {
	writeLine(w, "usage: cookctl config <command> [flags]")
	writeLine(w, "commands:")
	writeLine(w, "  view [--config <path>]")
	writeLine(w, "  set [--config <path>] [--api-url <url>] [--output <table|json>] [--timeout <duration>] [--debug]")
	writeLine(w, "  unset [--config <path>] [--api-url] [--output] [--timeout] [--debug]")
	writeLine(w, "  path")
}

func printConfigViewUsage(w io.Writer) {
	writeLine(w, "usage: cookctl config view [--config <path>]")
	writeLine(w, "flags:")
	writeLine(w, "  --config <path>          Config file path")
}

func printConfigSetUsage(w io.Writer) {
	writeLine(w, "usage: cookctl config set [--config <path>] [--api-url <url>] [--output <table|json>] [--timeout <duration>] [--debug]")
	writeLine(w, "flags:")
	writeLine(w, "  --config <path>          Config file path")
	writeLine(w, "  --api-url <url>          API base URL")
	writeLine(w, "  --output <table|json>    Output format")
	writeLine(w, "  --timeout <duration>     Request timeout (e.g. 30s)")
	writeLine(w, "  --debug                  Enable debug logging")
}

func printConfigUnsetUsage(w io.Writer) {
	writeLine(w, "usage: cookctl config unset [--config <path>] [--api-url] [--output] [--timeout] [--debug]")
	writeLine(w, "flags:")
	writeLine(w, "  --config <path>          Config file path")
	writeLine(w, "  --api-url                Clear api_url")
	writeLine(w, "  --output                 Clear output")
	writeLine(w, "  --timeout                Clear timeout")
	writeLine(w, "  --debug                  Clear debug")
}

func printConfigPathUsage(w io.Writer) {
	writeLine(w, "usage: cookctl config path")
}
