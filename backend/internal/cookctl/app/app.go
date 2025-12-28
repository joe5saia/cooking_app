// Package app implements the cookctl command routing and execution.
package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/config"
	"github.com/saiaj/cooking_app/backend/internal/cookctl/credentials"
	"golang.org/x/term"
)

const (
	exitOK        = 0
	exitError     = 1
	exitUsage     = 2
	exitAuth      = 3
	exitNotFound  = 4
	exitConflict  = 5
	exitRate      = 6
	exitForbidden = 7
	exitTooLarge  = 8
)

const (
	commandList     = "list"
	commandCreate   = "create"
	commandUpdate   = "update"
	commandDelete   = "delete"
	commandExport   = "export"
	commandImport   = "import"
	commandTag      = "tag"
	commandClone    = "clone"
	commandInit     = "init"
	commandEdit     = "edit"
	commandGet      = "get"
	commandRestore  = "restore"
	commandTemplate = "template"
)

const isoDateLayout = "2006-01-02"

var (
	// Version is the cookctl release version.
	Version = "dev"
	// Commit is the git commit SHA for the build.
	Commit = "unknown"
	// BuiltAt is the build timestamp for the binary.
	BuiltAt = "unknown"
)

type globalFlagSpec struct {
	name       string
	takesValue bool
}

var globalFlags = map[string]globalFlagSpec{
	"--api-url":           {name: "--api-url", takesValue: true},
	"--output":            {name: "--output", takesValue: true},
	"--timeout":           {name: "--timeout", takesValue: true},
	"--debug":             {name: "--debug"},
	"--version":           {name: "--version"},
	"--help":              {name: "--help"},
	"--skip-health-check": {name: "--skip-health-check"},
	"-h":                  {name: "-h"},
}

// Run executes the cookctl CLI and returns a process exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return exitUsage
	}

	cfg, err := config.Load("")
	if err != nil {
		writeLine(stderr, err)
		return exitUsage
	}

	globalArgs, commandArgs, err := splitGlobalArgs(args[1:])
	if err != nil {
		writeLine(stderr, err)
		return exitUsage
	}

	flags := flag.NewFlagSet("cookctl", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&cfg.APIURL, "api-url", cfg.APIURL, "API base URL")
	flags.Var(&cfg.Output, "output", "Output format: table|json")
	flags.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "Request timeout (e.g. 30s)")
	flags.BoolVar(&cfg.Debug, "debug", cfg.Debug, "Enable debug logging")
	var showVersion bool
	flags.BoolVar(&showVersion, "version", false, "Show cookctl version and exit")
	var showHelp bool
	flags.BoolVar(&showHelp, "help", false, "Show help and exit")
	flags.BoolVar(&showHelp, "h", false, "Show help and exit")
	var skipHealthCheck bool
	flags.BoolVar(&skipHealthCheck, "skip-health-check", false, "Skip API health preflight")
	var apiURLOverride bool

	if parseErr := flags.Parse(globalArgs); parseErr != nil {
		return exitUsage
	}
	flags.Visit(func(flagItem *flag.Flag) {
		if flagItem.Name == "api-url" {
			apiURLOverride = true
		}
	})

	if showHelp {
		if len(commandArgs) > 1 {
			commandArgs = commandArgs[:1]
		}
		return handleHelp(commandArgs, stdout, stderr)
	}
	if showVersion {
		if len(commandArgs) != 0 {
			writeLine(stderr, "version flag does not accept arguments")
			return exitUsage
		}
		info := versionInfo{
			Version: Version,
			Commit:  Commit,
			BuiltAt: BuiltAt,
		}
		return writeOutput(stdout, cfg.Output, info)
	}
	if len(commandArgs) == 0 {
		printUsage(stderr)
		return exitUsage
	}

	store, err := defaultStore()
	if err != nil {
		writeLine(stderr, err)
		return exitError
	}

	app := &App{
		cfg:            cfg,
		stdin:          stdin,
		stdout:         stdout,
		stderr:         stderr,
		store:          store,
		apiURLOverride: apiURLOverride,
		checkHealth:    !skipHealthCheck,
	}

	switch commandArgs[0] {
	case "health":
		return app.runHealth(commandArgs[1:])
	case "version":
		return app.runVersion(commandArgs[1:])
	case "completion":
		return app.runCompletion(commandArgs[1:])
	case "help":
		return app.runHelp(commandArgs[1:])
	case "auth":
		return app.runAuth(commandArgs[1:])
	case "token":
		return app.runToken(commandArgs[1:])
	case "tag":
		return app.runTag(commandArgs[1:])
	case "book":
		return app.runBook(commandArgs[1:])
	case "user":
		return app.runUser(commandArgs[1:])
	case "recipe":
		return app.runRecipe(commandArgs[1:])
	case "meal-plan":
		return app.runMealPlan(commandArgs[1:])
	case "config":
		return app.runConfig(commandArgs[1:])
	default:
		writef(stderr, "unknown command: %s\n", commandArgs[0])
		printUsage(stderr)
		return exitUsage
	}
}

// splitGlobalArgs extracts global flags from args while preserving command order.
func splitGlobalArgs(args []string) ([]string, []string, error) {
	globalArgs := make([]string, 0, len(args))
	commandArgs := make([]string, 0, len(args))
	seenCommand := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			commandArgs = append(commandArgs, args[i+1:]...)
			break
		}

		if seenCommand && isHelpFlag(arg) {
			commandArgs = append(commandArgs, arg)
			continue
		}

		spec, hasValue, ok := matchGlobalFlag(arg)
		if !ok {
			commandArgs = append(commandArgs, arg)
			seenCommand = true
			continue
		}

		if spec.takesValue && !hasValue {
			if i+1 >= len(args) || isFlagToken(args[i+1]) {
				return nil, nil, fmt.Errorf("flag %s requires a value", spec.name)
			}
			globalArgs = append(globalArgs, arg, args[i+1])
			i++
			continue
		}

		globalArgs = append(globalArgs, arg)
	}

	return globalArgs, commandArgs, nil
}

// matchGlobalFlag returns the global flag spec and whether a value was provided inline.
func matchGlobalFlag(arg string) (globalFlagSpec, bool, bool) {
	if spec, ok := globalFlags[arg]; ok {
		return spec, false, true
	}
	if strings.HasPrefix(arg, "--") {
		name, _, found := strings.Cut(arg, "=")
		if !found {
			return globalFlagSpec{}, false, false
		}
		if spec, ok := globalFlags[name]; ok {
			return spec, true, true
		}
	}
	return globalFlagSpec{}, false, false
}

// isFlagToken reports whether a value looks like a flag token.
func isFlagToken(arg string) bool {
	return strings.HasPrefix(arg, "-") && arg != "-"
}

func isHelpFlag(arg string) bool {
	return arg == "--help" || arg == "-h"
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if isHelpFlag(arg) {
			return true
		}
	}
	return false
}

// App bundles runtime dependencies for CLI commands.
type App struct {
	cfg            config.Config
	stdin          io.Reader
	stdout         io.Writer
	stderr         io.Writer
	store          *credentials.Store
	apiURLOverride bool // True when --api-url is provided on the CLI.
	checkHealth    bool // True when the CLI should preflight API connectivity.
	healthChecked  bool
	healthURL      string
	healthErr      error
}

func (a *App) runHealth(args []string) int {
	if hasHelpFlag(args) {
		printHealthUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("health", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	api, err := a.newClient("")
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	resp, err := api.Health(ctx)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runVersion(args []string) int {
	if hasHelpFlag(args) {
		printVersionUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		writeLine(a.stderr, "version does not accept arguments")
		return exitUsage
	}

	info := versionInfo{
		Version: Version,
		Commit:  Commit,
		BuiltAt: BuiltAt,
	}
	return writeOutput(a.stdout, a.cfg.Output, info)
}

// runCompletion prints shell completion scripts.
func (a *App) runCompletion(args []string) int {
	if hasHelpFlag(args) {
		printCompletionUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("completion", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 1 {
		printCompletionUsage(a.stderr)
		return exitUsage
	}

	shell := strings.ToLower(strings.TrimSpace(flags.Arg(0)))
	script, ok := completionScript(shell)
	if !ok {
		writef(a.stderr, "unsupported shell: %s\n", shell)
		printCompletionUsage(a.stderr)
		return exitUsage
	}

	if _, err := io.WriteString(a.stdout, script); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	return exitOK
}

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

func (a *App) runAuth(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printAuthUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printAuthUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case "login":
		return a.runAuthLogin(args[1:])
	case "set":
		return a.runAuthSet(args[1:])
	case "status":
		return a.runAuthStatus(args[1:])
	case "whoami":
		return a.runAuthWhoAmI(args[1:])
	case "logout":
		return a.runAuthLogout(args[1:])
	default:
		writef(a.stderr, "unknown auth command: %s\n", args[0])
		printAuthUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runToken(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printTokenUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printTokenUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case commandList:
		return a.runTokenList(args[1:])
	case commandCreate:
		return a.runTokenCreate(args[1:])
	case "revoke":
		return a.runTokenRevoke(args[1:])
	default:
		writef(a.stderr, "unknown token command: %s\n", args[0])
		printTokenUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runTokenList(args []string) int {
	if hasHelpFlag(args) {
		printTokenListUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("token list", flag.ContinueOnError)
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

	resp, err := api.Tokens(ctx)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runTokenCreate(args []string) int {
	if hasHelpFlag(args) {
		printTokenCreateUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("token create", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var name string
	var expiresAt string

	flags.StringVar(&name, "name", "", "Token name")
	flags.StringVar(&expiresAt, "expires-at", "", "Token expiration (RFC3339)")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	name = strings.TrimSpace(name)
	if name == "" {
		writeLine(a.stderr, "name is required")
		return exitUsage
	}

	var expiresAtTime *time.Time
	if expiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			writeLine(a.stderr, "expires-at must be RFC3339")
			return exitUsage
		}
		expiresAtTime = &parsed
	} else {
		writeLine(a.stderr, "warning: token will not expire unless revoked")
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

	resp, err := api.CreateToken(ctx, name, expiresAtTime)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runTokenRevoke(args []string) int {
	if hasHelpFlag(args) {
		printTokenRevokeUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("token revoke", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var yes bool
	flags.BoolVar(&yes, "yes", false, "Confirm token revocation")

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "token id is required")
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

	if err := api.RevokeToken(ctx, id); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, tokenRevokeResult{
		ID:      id,
		Revoked: true,
	})
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

func (a *App) runUser(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printUserUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printUserUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case commandList:
		return a.runUserList(args[1:])
	case commandCreate:
		return a.runUserCreate(args[1:])
	case "deactivate":
		return a.runUserDeactivate(args[1:])
	default:
		writef(a.stderr, "unknown user command: %s\n", args[0])
		printUserUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runUserList(args []string) int {
	if hasHelpFlag(args) {
		printUserListUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("user list", flag.ContinueOnError)
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

	resp, err := api.Users(ctx)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runUserCreate(args []string) int {
	if hasHelpFlag(args) {
		printUserCreateUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("user create", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var username string
	var passwordStdin bool
	var displayName string

	flags.StringVar(&username, "username", "", "Username")
	flags.BoolVar(&passwordStdin, "password-stdin", false, "Read password from stdin")
	flags.StringVar(&displayName, "display-name", "", "Display name")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	username = strings.TrimSpace(username)
	if username == "" {
		writeLine(a.stderr, "username is required")
		return exitUsage
	}
	if !passwordStdin {
		writeLine(a.stderr, "password-stdin is required for user create")
		return exitUsage
	}

	password, err := readPassword(a.stdin)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	if password == "" {
		writeLine(a.stderr, "password is required")
		return exitUsage
	}

	var displayNamePtr *string
	displayName = strings.TrimSpace(displayName)
	if displayName != "" {
		displayNamePtr = &displayName
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

	resp, err := api.CreateUser(ctx, username, password, displayNamePtr)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runUserDeactivate(args []string) int {
	if hasHelpFlag(args) {
		printUserDeactivateUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("user deactivate", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var yes bool
	flags.BoolVar(&yes, "yes", false, "Confirm user deactivation")

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "user id is required")
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

	if err := api.DeactivateUser(ctx, id); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, userDeactivateResult{
		ID:          id,
		Deactivated: true,
	})
}

func (a *App) runRecipe(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printRecipeUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printRecipeUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case commandList:
		return a.runRecipeList(args[1:])
	case commandGet:
		return a.runRecipeGet(args[1:])
	case commandCreate:
		return a.runRecipeCreate(args[1:])
	case commandUpdate:
		return a.runRecipeUpdate(args[1:])
	case commandInit:
		return a.runRecipeInit(args[1:])
	case commandTemplate:
		return a.runRecipeTemplate(args[1:])
	case commandExport:
		return a.runRecipeExport(args[1:])
	case commandImport:
		return a.runRecipeImport(args[1:])
	case commandTag:
		return a.runRecipeTag(args[1:])
	case commandClone:
		return a.runRecipeClone(args[1:])
	case commandEdit:
		return a.runRecipeEdit(args[1:])
	case commandDelete:
		return a.runRecipeDelete(args[1:])
	case commandRestore:
		return a.runRecipeRestore(args[1:])
	default:
		writef(a.stderr, "unknown recipe command: %s\n", args[0])
		printRecipeUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runRecipeList(args []string) int {
	if hasHelpFlag(args) {
		printRecipeListUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("recipe list", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var query string
	var bookID string
	var bookName string
	var tagID string
	var tagName string
	var includeDeleted bool
	var limit int
	var cursor string
	var all bool
	var servings int
	var withCounts bool

	flags.StringVar(&query, "q", "", "Search query")
	flags.StringVar(&bookID, "book-id", "", "Filter by recipe book id")
	flags.StringVar(&bookName, "book", "", "Filter by recipe book name")
	flags.StringVar(&tagID, "tag-id", "", "Filter by tag id")
	flags.StringVar(&tagName, "tag", "", "Filter by tag name")
	flags.BoolVar(&includeDeleted, "include-deleted", false, "Include deleted recipes")
	flags.IntVar(&limit, "limit", 0, "Max items per page")
	flags.StringVar(&cursor, "cursor", "", "Pagination cursor")
	flags.BoolVar(&all, "all", false, "Fetch all pages")
	flags.IntVar(&servings, "servings", 0, "Filter by servings count")
	flags.BoolVar(&withCounts, "with-counts", false, "Include ingredient and step counts")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if limit < 0 {
		writeLine(a.stderr, "limit must be positive")
		return exitUsage
	}
	if servings < 0 {
		writeLine(a.stderr, "servings must be positive")
		return exitUsage
	}
	if strings.TrimSpace(bookID) != "" && strings.TrimSpace(bookName) != "" {
		writeLine(a.stderr, "book and book-id cannot be combined")
		return exitUsage
	}
	if strings.TrimSpace(tagID) != "" && strings.TrimSpace(tagName) != "" {
		writeLine(a.stderr, "tag and tag-id cannot be combined")
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

	listParams := client.RecipeListParams{
		Query:          strings.TrimSpace(query),
		BookID:         strings.TrimSpace(bookID),
		TagID:          strings.TrimSpace(tagID),
		IncludeDeleted: includeDeleted,
		Limit:          limit,
		Cursor:         strings.TrimSpace(cursor),
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, err := a.apiClient(ctx, token)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	if strings.TrimSpace(bookName) != "" {
		resolved, resolveErr := resolveBookIDByName(ctx, api, bookName)
		if resolveErr != nil {
			writeLine(a.stderr, resolveErr)
			return exitUsage
		}
		listParams.BookID = resolved
	}
	if strings.TrimSpace(tagName) != "" {
		resolved, resolveErr := resolveTagIDByName(ctx, api, tagName)
		if resolveErr != nil {
			writeLine(a.stderr, resolveErr)
			return exitUsage
		}
		listParams.TagID = resolved
	}
	if servings > 0 {
		all = true
	}

	if !all {
		resp, err := api.Recipes(ctx, listParams)
		if err != nil {
			return a.handleAPIError(err)
		}
		items := filterRecipesByServings(resp.Items, servings)
		if withCounts {
			counted, countErr := addRecipeCounts(ctx, api, items)
			if countErr != nil {
				return a.handleAPIError(countErr)
			}
			return writeOutput(a.stdout, a.cfg.Output, recipeListWithCountsResponse{
				Items:      counted,
				NextCursor: resp.NextCursor,
			})
		}
		return writeOutput(a.stdout, a.cfg.Output, client.RecipeListResponse{
			Items:      items,
			NextCursor: resp.NextCursor,
		})
	}

	var allItems []client.RecipeListItem
	nextCursor := listParams.Cursor

	for {
		listParams.Cursor = nextCursor
		resp, err := api.Recipes(ctx, listParams)
		if err != nil {
			return a.handleAPIError(err)
		}
		allItems = append(allItems, resp.Items...)
		if resp.NextCursor == nil || strings.TrimSpace(*resp.NextCursor) == "" {
			break
		}
		nextCursor = *resp.NextCursor
	}

	allItems = filterRecipesByServings(allItems, servings)
	if withCounts {
		counted, err := addRecipeCounts(ctx, api, allItems)
		if err != nil {
			return a.handleAPIError(err)
		}
		return writeOutput(a.stdout, a.cfg.Output, recipeListWithCountsResponse{
			Items:      counted,
			NextCursor: nil,
		})
	}
	return writeOutput(a.stdout, a.cfg.Output, client.RecipeListResponse{
		Items:      allItems,
		NextCursor: nil,
	})
}

func (a *App) runRecipeGet(args []string) int {
	if hasHelpFlag(args) {
		printRecipeGetUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("recipe get", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "recipe id is required")
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

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	resp, err := api.Recipe(ctx, resolvedID)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runRecipeCreate(args []string) int {
	if hasHelpFlag(args) {
		printRecipeCreateUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("recipe create", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var filePath string
	var useStdin bool
	var interactive bool
	var allowDuplicate bool
	flags.StringVar(&filePath, "file", "", "Path to recipe JSON")
	flags.BoolVar(&useStdin, "stdin", false, "Read recipe JSON from stdin")
	flags.BoolVar(&interactive, "interactive", false, "Create recipe with interactive prompts")
	flags.BoolVar(&allowDuplicate, "allow-duplicate", false, "Allow duplicate recipe titles")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	filePath = strings.TrimSpace(filePath)
	if interactive && (filePath != "" || useStdin) {
		writeLine(a.stderr, "interactive cannot be combined with --file or --stdin")
		return exitUsage
	}
	if !interactive && (filePath == "" && !useStdin) {
		writeLine(a.stderr, "provide --file, --stdin, or --interactive")
		return exitUsage
	}
	if !interactive && filePath != "" && useStdin {
		writeLine(a.stderr, "provide --file or --stdin")
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

	var payload json.RawMessage
	if interactive {
		interactivePayload, buildErr := buildRecipePayloadInteractive(a.stdin, a.stderr, ctx, api)
		if buildErr != nil {
			writeLine(a.stderr, buildErr)
			return exitUsage
		}
		if dupErr := ensureUniqueRecipeTitle(ctx, api, interactivePayload.Title, allowDuplicate); dupErr != nil {
			writeLine(a.stderr, dupErr)
			return exitConflict
		}
		raw, marshalErr := json.Marshal(interactivePayload)
		if marshalErr != nil {
			writeLine(a.stderr, marshalErr)
			return exitError
		}
		payload = raw
	} else {
		var readErr error
		if useStdin {
			payload, readErr = readJSONReader(a.stdin)
		} else {
			payload, readErr = readJSONFile(filePath)
		}
		if readErr != nil {
			writeLine(a.stderr, readErr)
			return exitUsage
		}
		title, titleErr := recipeTitleFromJSON(payload)
		if titleErr != nil {
			writeLine(a.stderr, titleErr)
			return exitUsage
		}
		if dupErr := ensureUniqueRecipeTitle(ctx, api, title, allowDuplicate); dupErr != nil {
			writeLine(a.stderr, dupErr)
			return exitConflict
		}
	}

	resp, err := api.CreateRecipe(ctx, payload)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runRecipeUpdate(args []string) int {
	if hasHelpFlag(args) {
		printRecipeUpdateUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("recipe update", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var filePath string
	var useStdin bool
	flags.StringVar(&filePath, "file", "", "Path to recipe JSON")
	flags.BoolVar(&useStdin, "stdin", false, "Read recipe JSON from stdin")

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "recipe id is required")
		return exitUsage
	}
	id = strings.TrimSpace(id)
	filePath = strings.TrimSpace(filePath)
	if (filePath == "" && !useStdin) || (filePath != "" && useStdin) {
		writeLine(a.stderr, "provide --file or --stdin")
		return exitUsage
	}

	var payload json.RawMessage
	if useStdin {
		payload, err = readJSONReader(a.stdin)
	} else {
		payload, err = readJSONFile(filePath)
	}
	if err != nil {
		writeLine(a.stderr, err)
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

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	resp, err := api.UpdateRecipe(ctx, resolvedID, payload)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runRecipeInit(args []string) int {
	if hasHelpFlag(args) {
		printRecipeInitUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("recipe init", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	token, _, err := a.resolveToken()
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	if id == "" {
		return writeOutput(a.stdout, a.cfg.Output, recipeTemplatePayload())
	}

	if token == "" {
		writeLine(a.stderr, "no token found; run `cookctl auth set --token <pat>`")
		return exitAuth
	}
	id = strings.TrimSpace(id)

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, err := a.apiClient(ctx, token)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	recipe, err := api.Recipe(ctx, resolvedID)
	if err != nil {
		return a.handleAPIError(err)
	}

	payload := toUpsertPayload(recipe)
	return writeOutput(a.stdout, a.cfg.Output, payload)
}

func (a *App) runRecipeTemplate(args []string) int {
	if hasHelpFlag(args) {
		printRecipeTemplateUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("recipe template", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() > 0 {
		writeLine(a.stderr, "template does not accept arguments")
		return exitUsage
	}
	return writeOutput(a.stdout, a.cfg.Output, recipeTemplatePayload())
}

func (a *App) runRecipeExport(args []string) int {
	if hasHelpFlag(args) {
		printRecipeExportUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("recipe export", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "recipe id is required")
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

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	recipe, err := api.Recipe(ctx, resolvedID)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, toUpsertPayload(recipe))
}

func (a *App) runRecipeImport(args []string) int {
	if hasHelpFlag(args) {
		printRecipeImportUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("recipe import", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var filePath string
	var useStdin bool
	var allowDuplicate bool
	flags.StringVar(&filePath, "file", "", "Path to recipe JSON")
	flags.BoolVar(&useStdin, "stdin", false, "Read recipe JSON from stdin")
	flags.BoolVar(&allowDuplicate, "allow-duplicate", false, "Allow duplicate recipe titles")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" && !useStdin {
		if !isTerminal(a.stdin) {
			useStdin = true
		} else {
			writeLine(a.stderr, "provide --file or --stdin")
			return exitUsage
		}
	}
	if filePath != "" && useStdin {
		writeLine(a.stderr, "provide --file or --stdin")
		return exitUsage
	}

	var raw []byte
	var err error
	if useStdin {
		raw, err = readRawJSONReader(a.stdin)
	} else {
		raw, err = readRawJSONFile(filePath)
	}
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	payloads, err := splitJSONPayloads(raw)
	if err != nil {
		writeLine(a.stderr, err)
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

	results := make([]recipeImportItemResult, 0, len(payloads))
	for i, payload := range payloads {
		title, titleErr := recipeTitleFromJSON(payload)
		if titleErr != nil {
			writeLine(a.stderr, fmt.Sprintf("payload %d: %v", i+1, titleErr))
			return exitUsage
		}
		if err := ensureUniqueRecipeTitle(ctx, api, title, allowDuplicate); err != nil {
			writeLine(a.stderr, fmt.Sprintf("payload %d: %v", i+1, err))
			return exitConflict
		}
		created, err := api.CreateRecipe(ctx, payload)
		if err != nil {
			return a.handleAPIError(err)
		}
		results = append(results, recipeImportItemResult{
			ID:    created.ID,
			Title: created.Title,
		})
	}

	return writeOutput(a.stdout, a.cfg.Output, recipeImportResult{Items: results})
}

func (a *App) runRecipeTag(args []string) int {
	if hasHelpFlag(args) {
		printRecipeTagUsage(a.stdout)
		return exitOK
	}

	id, tagNames, replace, createMissing, err := parseRecipeTagArgs(args)
	if err != nil {
		writeLine(a.stderr, err)
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

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	tagIDs, err := resolveTagIDsByName(ctx, api, tagNames, createMissing)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	recipe, err := api.Recipe(ctx, resolvedID)
	if err != nil {
		return a.handleAPIError(err)
	}

	payload := toUpsertPayload(recipe)
	if replace {
		payload.TagIDs = tagIDs
	} else {
		payload.TagIDs = mergeIDs(payload.TagIDs, tagIDs)
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	resp, err := api.UpdateRecipe(ctx, resolvedID, raw)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runRecipeClone(args []string) int {
	if hasHelpFlag(args) {
		printRecipeCloneUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("recipe clone", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var titleOverride string
	var allowDuplicate bool
	flags.StringVar(&titleOverride, "title", "", "Title for the cloned recipe")
	flags.BoolVar(&allowDuplicate, "allow-duplicate", false, "Allow duplicate recipe titles")

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "recipe id is required")
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

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	recipe, err := api.Recipe(ctx, resolvedID)
	if err != nil {
		return a.handleAPIError(err)
	}

	payload := toUpsertPayload(recipe)
	title := strings.TrimSpace(titleOverride)
	if title == "" {
		title = fmt.Sprintf("%s (copy)", payload.Title)
	}
	payload.Title = title

	if dupErr := ensureUniqueRecipeTitle(ctx, api, payload.Title, allowDuplicate); dupErr != nil {
		writeLine(a.stderr, dupErr)
		return exitConflict
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	resp, err := api.CreateRecipe(ctx, raw)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runRecipeEdit(args []string) int {
	if hasHelpFlag(args) {
		printRecipeEditUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("recipe edit", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var editorOverride string
	flags.StringVar(&editorOverride, "editor", "", "Editor command")

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "recipe id is required")
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

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	recipe, err := api.Recipe(ctx, resolvedID)
	if err != nil {
		return a.handleAPIError(err)
	}

	payload := toUpsertPayload(recipe)
	jsonBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	tempDir, err := os.MkdirTemp("", "cookctl-recipe-")
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			writeLine(a.stderr, removeErr)
		}
	}()

	tempFile := filepath.Join(tempDir, "recipe.json")
	if writeErr := os.WriteFile(tempFile, jsonBytes, 0o600); writeErr != nil {
		writeLine(a.stderr, writeErr)
		return exitError
	}

	editor := strings.TrimSpace(editorOverride)
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("VISUAL"))
	}
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		editor = "vi"
	}

	editorArgs, err := splitEditorArgs(editor)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	if _, lookupErr := exec.LookPath(editorArgs[0]); lookupErr != nil {
		writeLine(a.stderr, fmt.Errorf("editor not found: %s", editorArgs[0]))
		return exitError
	}
	//nolint:gosec // Editor command is user-configured and expected to run locally.
	cmd := exec.CommandContext(ctx, editorArgs[0], append(editorArgs[1:], tempFile)...)
	cmd.Stdin = a.stdin
	cmd.Stdout = a.stdout
	cmd.Stderr = a.stderr
	if runErr := cmd.Run(); runErr != nil {
		writeLine(a.stderr, runErr)
		return exitError
	}

	updated, err := readJSONFile(tempFile)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	resp, err := api.UpdateRecipe(ctx, resolvedID, updated)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runRecipeDelete(args []string) int {
	if hasHelpFlag(args) {
		printRecipeDeleteUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("recipe delete", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var yes bool
	flags.BoolVar(&yes, "yes", false, "Confirm recipe deletion")

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "recipe id is required")
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

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	if err := api.DeleteRecipe(ctx, resolvedID); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, recipeDeleteResult{
		ID:      resolvedID,
		Deleted: true,
	})
}

func (a *App) runRecipeRestore(args []string) int {
	if hasHelpFlag(args) {
		printRecipeRestoreUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("recipe restore", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var yes bool
	flags.BoolVar(&yes, "yes", false, "Confirm recipe restore")

	id, err := parseIDArgs(flags, args)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if id == "" {
		writeLine(a.stderr, "recipe id is required")
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

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	if err := api.RestoreRecipe(ctx, resolvedID); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, recipeRestoreResult{
		ID:       resolvedID,
		Restored: true,
	})
}

func (a *App) runMealPlan(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printMealPlanUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printMealPlanUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case commandList:
		return a.runMealPlanList(args[1:])
	case commandCreate:
		return a.runMealPlanCreate(args[1:])
	case commandDelete:
		return a.runMealPlanDelete(args[1:])
	default:
		writef(a.stderr, "unknown meal-plan command: %s\n", args[0])
		printMealPlanUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runMealPlanList(args []string) int {
	if hasHelpFlag(args) {
		printMealPlanListUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("meal-plan list", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var start string
	var end string
	flags.StringVar(&start, "start", "", "Start date (YYYY-MM-DD)")
	flags.StringVar(&end, "end", "", "End date (YYYY-MM-DD)")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	startDate, err := parseISODate("start", start)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	endDate, err := parseISODate("end", end)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	if startDate.After(endDate) {
		writeLine(a.stderr, "end must be on or after start")
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

	resp, err := api.MealPlans(ctx, startDate.Format(isoDateLayout), endDate.Format(isoDateLayout))
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runMealPlanCreate(args []string) int {
	if hasHelpFlag(args) {
		printMealPlanCreateUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("meal-plan create", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var date string
	var recipeID string
	flags.StringVar(&date, "date", "", "Meal plan date (YYYY-MM-DD)")
	flags.StringVar(&recipeID, "recipe-id", "", "Recipe id")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	planDate, err := parseISODate("date", date)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	recipeID = strings.TrimSpace(recipeID)
	if recipeID == "" {
		writeLine(a.stderr, "recipe-id is required")
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

	resp, err := api.CreateMealPlan(ctx, planDate.Format(isoDateLayout), recipeID)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runMealPlanDelete(args []string) int {
	if hasHelpFlag(args) {
		printMealPlanDeleteUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("meal-plan delete", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var date string
	var recipeID string
	var yes bool
	flags.StringVar(&date, "date", "", "Meal plan date (YYYY-MM-DD)")
	flags.StringVar(&recipeID, "recipe-id", "", "Recipe id")
	flags.BoolVar(&yes, "yes", false, "Confirm meal plan deletion")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	planDate, err := parseISODate("date", date)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	recipeID = strings.TrimSpace(recipeID)
	if recipeID == "" {
		writeLine(a.stderr, "recipe-id is required")
		return exitUsage
	}
	if !yes {
		writeLine(a.stderr, "confirmation required; re-run with --yes")
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

	if err := api.DeleteMealPlan(ctx, planDate.Format(isoDateLayout), recipeID); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, mealPlanDeleteResult{
		Date:     planDate.Format(isoDateLayout),
		RecipeID: recipeID,
		Deleted:  true,
	})
}

func (a *App) runConfig(args []string) int {
	if len(args) > 0 && isHelpFlag(args[0]) {
		printConfigUsage(a.stdout)
		return exitOK
	}
	if len(args) == 0 {
		printConfigUsage(a.stderr)
		return exitUsage
	}

	switch args[0] {
	case "view":
		return a.runConfigView(args[1:])
	case "set":
		return a.runConfigSet(args[1:])
	case "unset":
		return a.runConfigUnset(args[1:])
	case "path":
		return a.runConfigPath(args[1:])
	default:
		writef(a.stderr, "unknown config command: %s\n", args[0])
		printConfigUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runConfigView(args []string) int {
	if hasHelpFlag(args) {
		printConfigViewUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("config view", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var configPath string
	flags.StringVar(&configPath, "config", "", "Config file path")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		path, err := config.DefaultConfigPath()
		if err != nil {
			writeLine(a.stderr, err)
			return exitError
		}
		configPath = path
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	view := configView{
		ConfigPath: configPath,
		APIURL:     cfg.APIURL,
		Output:     string(cfg.Output),
		Timeout:    cfg.Timeout.String(),
		Debug:      cfg.Debug,
	}
	return writeOutput(a.stdout, a.cfg.Output, view)
}

func (a *App) runConfigSet(args []string) int {
	if hasHelpFlag(args) {
		printConfigSetUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("config set", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var configPath string
	var apiURL optionalString
	var output optionalString
	var timeoutStr optionalString
	var debug optionalBool

	flags.StringVar(&configPath, "config", "", "Config file path")
	flags.Var(&apiURL, "api-url", "API base URL")
	flags.Var(&output, "output", "Output format: table|json")
	flags.Var(&timeoutStr, "timeout", "Request timeout (e.g. 30s)")
	flags.Var(&debug, "debug", "Enable debug logging")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		path, err := config.DefaultConfigPath()
		if err != nil {
			writeLine(a.stderr, err)
			return exitError
		}
		configPath = path
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}

	if apiURL.set {
		cfg.APIURL = apiURL.value
	}
	if output.set {
		parsed, err := config.ParseOutput(output.value)
		if err != nil {
			writeLine(a.stderr, err)
			return exitUsage
		}
		cfg.Output = parsed
	}
	if timeoutStr.set {
		timeout, err := time.ParseDuration(timeoutStr.value)
		if err != nil {
			writeLine(a.stderr, "timeout must be a duration (e.g. 30s)")
			return exitUsage
		}
		cfg.Timeout = timeout
	}
	if debug.set {
		cfg.Debug = debug.value
	}

	if err := config.Save(configPath, cfg); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	view := configView{
		ConfigPath: configPath,
		APIURL:     cfg.APIURL,
		Output:     string(cfg.Output),
		Timeout:    cfg.Timeout.String(),
		Debug:      cfg.Debug,
	}
	return writeOutput(a.stdout, a.cfg.Output, view)
}

func (a *App) runConfigUnset(args []string) int {
	if hasHelpFlag(args) {
		printConfigUnsetUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("config unset", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var configPath string
	var apiURL bool
	var output bool
	var timeout bool
	var debug bool

	flags.StringVar(&configPath, "config", "", "Config file path")
	flags.BoolVar(&apiURL, "api-url", false, "Clear api_url")
	flags.BoolVar(&output, "output", false, "Clear output")
	flags.BoolVar(&timeout, "timeout", false, "Clear timeout")
	flags.BoolVar(&debug, "debug", false, "Clear debug")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if !apiURL && !output && !timeout && !debug {
		writeLine(a.stderr, "at least one flag is required")
		return exitUsage
	}

	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		path, err := config.DefaultConfigPath()
		if err != nil {
			writeLine(a.stderr, err)
			return exitError
		}
		configPath = path
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		writeLine(a.stderr, err)
		return exitUsage
	}
	defaults := config.Default()

	if apiURL {
		cfg.APIURL = defaults.APIURL
	}
	if output {
		cfg.Output = defaults.Output
	}
	if timeout {
		cfg.Timeout = defaults.Timeout
	}
	if debug {
		cfg.Debug = defaults.Debug
	}

	if err := config.Save(configPath, cfg); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	view := configView{
		ConfigPath: configPath,
		APIURL:     cfg.APIURL,
		Output:     string(cfg.Output),
		Timeout:    cfg.Timeout.String(),
		Debug:      cfg.Debug,
	}
	return writeOutput(a.stdout, a.cfg.Output, view)
}

func (a *App) runConfigPath(args []string) int {
	if hasHelpFlag(args) {
		printConfigPathUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("config path", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	path, err := config.DefaultConfigPath()
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	view := configView{
		ConfigPath: path,
		APIURL:     a.cfg.APIURL,
		Output:     string(a.cfg.Output),
		Timeout:    a.cfg.Timeout.String(),
		Debug:      a.cfg.Debug,
	}
	return writeOutput(a.stdout, a.cfg.Output, view)
}

func (a *App) runAuthSet(args []string) int {
	if hasHelpFlag(args) {
		printAuthSetUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("auth set", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var token string
	var tokenStdin bool
	var apiURL string
	flags.StringVar(&token, "token", "", "Personal access token")
	flags.BoolVar(&tokenStdin, "token-stdin", false, "Read token from stdin")
	flags.StringVar(&apiURL, "api-url", "", "API base URL override")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if tokenStdin && strings.TrimSpace(token) != "" {
		writeLine(a.stderr, "token and token-stdin cannot be combined")
		return exitUsage
	}
	if tokenStdin {
		var err error
		token, err = readToken(a.stdin)
		if err != nil {
			writeLine(a.stderr, err)
			return exitUsage
		}
	}
	token = strings.TrimSpace(token)
	if token == "" {
		writeLine(a.stderr, "token is required (use --token or --token-stdin)")
		return exitUsage
	}
	if strings.TrimSpace(apiURL) == "" {
		apiURL = a.cfg.APIURL
	}

	if err := a.store.Save(credentials.Credentials{
		Token:  strings.TrimSpace(token),
		APIURL: strings.TrimSpace(apiURL),
	}); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	return writeOutput(a.stdout, a.cfg.Output, actionResult{Message: "token saved"})
}

func (a *App) runAuthLogin(args []string) int {
	if hasHelpFlag(args) {
		printAuthLoginUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("auth login", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var username string
	var passwordStdin bool
	var tokenName string
	var expiresAt string

	flags.StringVar(&username, "username", "", "Username for login")
	flags.BoolVar(&passwordStdin, "password-stdin", false, "Read password from stdin")
	flags.StringVar(&tokenName, "token-name", "cookctl", "Name for the new PAT")
	flags.StringVar(&expiresAt, "expires-at", "", "Token expiration (RFC3339)")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if username == "" {
		writeLine(a.stderr, "username is required")
		return exitUsage
	}
	if !passwordStdin {
		writeLine(a.stderr, "password-stdin is required for auth login")
		return exitUsage
	}
	tokenName = strings.TrimSpace(tokenName)
	if tokenName == "" {
		writeLine(a.stderr, "token-name is required")
		return exitUsage
	}

	password, err := readPassword(a.stdin)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	if password == "" {
		writeLine(a.stderr, "password is required")
		return exitUsage
	}

	var expiresAtTime *time.Time
	if expiresAt != "" {
		parsed, parseErr := time.Parse(time.RFC3339, expiresAt)
		if parseErr != nil {
			writeLine(a.stderr, "expires-at must be RFC3339")
			return exitUsage
		}
		expiresAtTime = &parsed
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	if healthErr := a.ensureHealthyURL(ctx, a.cfg.APIURL); healthErr != nil {
		writeLine(a.stderr, healthErr)
		return exitError
	}

	sessionClient, err := client.NewSessionClient(a.cfg.APIURL, a.cfg.Timeout, a.cfg.Debug, a.stderr)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	resp, err := sessionClient.BootstrapToken(ctx, username, password, tokenName, expiresAtTime)
	if err != nil {
		return a.handleAPIError(err)
	}

	creds := credentials.Credentials{
		Token:     resp.Token,
		TokenID:   resp.ID,
		TokenName: resp.Name,
		CreatedAt: &resp.CreatedAt,
		ExpiresAt: expiresAtTime,
		APIURL:    a.cfg.APIURL,
	}
	if err := a.store.Save(creds); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	return writeOutput(a.stdout, a.cfg.Output, authLoginResult{
		ID:        resp.ID,
		Name:      resp.Name,
		Token:     resp.Token,
		CreatedAt: resp.CreatedAt,
		ExpiresAt: expiresAtTime,
	})
}

func (a *App) runAuthStatus(args []string) int {
	if hasHelpFlag(args) {
		printAuthStatusUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("auth status", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	token, source, err := a.resolveToken()
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	apiURL := a.cfg.APIURL
	var creds credentials.Credentials
	if source == tokenSourceCredentials {
		loadedCreds, ok, err := a.store.Load()
		if err != nil {
			writeLine(a.stderr, err)
			return exitError
		}
		if ok {
			creds = loadedCreds
			if !a.apiURLOverride {
				if storedURL := strings.TrimSpace(creds.APIURL); storedURL != "" {
					apiURL = storedURL
				}
			}
		}
	}

	status := authStatus{
		Source:       string(source),
		TokenPresent: token != "",
		MaskedToken:  maskToken(token),
		APIURL:       apiURL,
	}
	if source == tokenSourceCredentials {
		status.TokenID = creds.TokenID
		status.TokenName = creds.TokenName
		status.CreatedAt = creds.CreatedAt
		status.ExpiresAt = creds.ExpiresAt
	}
	return writeOutput(a.stdout, a.cfg.Output, status)
}

func (a *App) runAuthWhoAmI(args []string) int {
	if hasHelpFlag(args) {
		printAuthWhoAmIUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("auth whoami", flag.ContinueOnError)
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

	resp, err := api.Me(ctx)
	if err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, resp)
}

func (a *App) runAuthLogout(args []string) int {
	if hasHelpFlag(args) {
		printAuthLogoutUsage(a.stdout)
		return exitOK
	}

	flags := flag.NewFlagSet("auth logout", flag.ContinueOnError)
	flags.SetOutput(a.stderr)

	var revoke bool
	flags.BoolVar(&revoke, "revoke", false, "Revoke stored token before clearing credentials")

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	if revoke {
		return a.runAuthLogoutRevoke()
	}

	if err := a.store.Clear(); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	if env := os.Getenv("COOKING_PAT"); env != "" {
		return writeOutput(a.stdout, a.cfg.Output, actionResult{Message: "credentials cleared; COOKING_PAT is still set"})
	}

	return writeOutput(a.stdout, a.cfg.Output, actionResult{Message: "credentials cleared"})
}

func (a *App) runAuthLogoutRevoke() int {
	creds, ok, err := a.store.Load()
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}
	if !ok || creds.Token == "" {
		writeLine(a.stderr, "no stored token found to revoke")
		return exitAuth
	}
	if creds.TokenID == "" {
		writeLine(a.stderr, "stored token id is missing; cannot revoke")
		return exitError
	}

	apiURL := a.cfg.APIURL
	if !a.apiURLOverride {
		if storedURL := strings.TrimSpace(creds.APIURL); storedURL != "" {
			apiURL = storedURL
		}
	}

	api, err := client.New(apiURL, creds.Token, a.cfg.Timeout, a.cfg.Debug, a.stderr)
	if err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	if err := a.ensureHealthy(ctx, api, apiURL); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	if err := api.RevokeToken(ctx, creds.TokenID); err != nil {
		return a.handleAPIError(err)
	}

	if err := a.store.Clear(); err != nil {
		writeLine(a.stderr, err)
		return exitError
	}

	if env := os.Getenv("COOKING_PAT"); env != "" {
		return writeOutput(a.stdout, a.cfg.Output, actionResult{Message: "token revoked and credentials cleared; COOKING_PAT is still set"})
	}

	return writeOutput(a.stdout, a.cfg.Output, actionResult{Message: "token revoked and credentials cleared"})
}

// apiURLForToken resolves the API URL for a token and config context.
func (a *App) apiURLForToken(token string) (string, error) {
	apiURL := a.cfg.APIURL
	if token != "" && !a.apiURLOverride {
		if envToken := strings.TrimSpace(os.Getenv("COOKING_PAT")); envToken == "" {
			creds, ok, err := a.store.Load()
			if err != nil {
				return "", err
			}
			if ok && creds.Token == token {
				if storedURL := strings.TrimSpace(creds.APIURL); storedURL != "" {
					apiURL = storedURL
				}
			}
		}
	}
	return apiURL, nil
}

// newClientWithURL builds a cookctl API client and returns the resolved API URL.
func (a *App) newClientWithURL(token string) (*client.Client, string, error) {
	writer := io.Discard
	if a.cfg.Debug {
		writer = a.stderr
	}
	apiURL, err := a.apiURLForToken(token)
	if err != nil {
		return nil, "", err
	}
	api, err := client.New(apiURL, token, a.cfg.Timeout, a.cfg.Debug, writer)
	if err != nil {
		return nil, "", err
	}
	return api, apiURL, nil
}

// newClient builds a cookctl API client with the resolved API URL.
func (a *App) newClient(token string) (*client.Client, error) {
	api, _, err := a.newClientWithURL(token)
	return api, err
}

// ensureHealthy checks API connectivity once per URL when enabled.
func (a *App) ensureHealthy(ctx context.Context, api *client.Client, apiURL string) error {
	if !a.checkHealth {
		return nil
	}
	if a.healthChecked && a.healthURL == apiURL {
		return a.healthErr
	}
	a.healthChecked = true
	a.healthURL = apiURL
	_, err := api.Health(ctx)
	if err != nil {
		a.healthErr = fmt.Errorf("unable to reach API at %s: %w", apiURL, err)
		return a.healthErr
	}
	a.healthErr = nil
	return nil
}

// ensureHealthyURL checks API connectivity for commands that do not have a client yet.
func (a *App) ensureHealthyURL(ctx context.Context, apiURL string) error {
	if !a.checkHealth {
		return nil
	}
	if a.healthChecked && a.healthURL == apiURL {
		return a.healthErr
	}
	api, err := client.New(apiURL, "", a.cfg.Timeout, a.cfg.Debug, io.Discard)
	if err != nil {
		return err
	}
	return a.ensureHealthy(ctx, api, apiURL)
}

// apiClient returns a client after running the optional preflight check.
func (a *App) apiClient(ctx context.Context, token string) (*client.Client, error) {
	api, apiURL, err := a.newClientWithURL(token)
	if err != nil {
		return nil, err
	}
	if err := a.ensureHealthy(ctx, api, apiURL); err != nil {
		return nil, err
	}
	return api, nil
}

func (a *App) resolveToken() (string, tokenSource, error) {
	if token := strings.TrimSpace(os.Getenv("COOKING_PAT")); token != "" {
		return token, tokenSourceEnv, nil
	}

	creds, ok, err := a.store.Load()
	if err != nil {
		return "", tokenSourceNone, err
	}
	if !ok {
		return "", tokenSourceNone, nil
	}
	return creds.Token, tokenSourceCredentials, nil
}

// apiErrorOutput is the JSON envelope for API errors.
type apiErrorOutput struct {
	Error apiErrorDetail `json:"error"`
}

// apiErrorDetail captures API error metadata for JSON output.
type apiErrorDetail struct {
	Status  int                 `json:"status"`
	Code    string              `json:"code,omitempty"`
	Message string              `json:"message,omitempty"`
	Details []client.FieldError `json:"details,omitempty"`
}

func (a *App) handleAPIError(err error) int {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		if a.cfg.Output == config.OutputJSON {
			if writeErr := writeAPIErrorJSON(a.stdout, apiErr); writeErr != nil {
				writeLine(a.stderr, writeErr)
			}
		} else {
			writeLine(a.stderr, apiErr.UserMessage())
		}
		switch apiErr.StatusCode {
		case 401:
			return exitAuth
		case 403:
			return exitForbidden
		case 404:
			return exitNotFound
		case 409:
			return exitConflict
		case 413:
			return exitTooLarge
		case 429:
			return exitRate
		default:
			return exitError
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		writeLine(a.stderr, "request timed out")
		return exitError
	}
	writeLine(a.stderr, err)
	return exitError
}

// writeAPIErrorJSON writes an API error in JSON for script-friendly output.
func writeAPIErrorJSON(w io.Writer, apiErr *client.APIError) error {
	if apiErr == nil {
		return errors.New("api error is nil")
	}
	message := strings.TrimSpace(apiErr.Problem.Message)
	if message == "" {
		message = fmt.Sprintf("request failed with status %d", apiErr.StatusCode)
	}
	payload := apiErrorOutput{
		Error: apiErrorDetail{
			Status:  apiErr.StatusCode,
			Code:    apiErr.Problem.Code,
			Message: message,
			Details: apiErr.Problem.Details,
		},
	}
	return writeJSON(w, payload)
}

type tokenSource string

const (
	tokenSourceNone        tokenSource = "none"
	tokenSourceEnv         tokenSource = "env"
	tokenSourceCredentials tokenSource = "credentials"
)

type authStatus struct {
	Source       string     `json:"source"`
	TokenPresent bool       `json:"token_present"`
	MaskedToken  string     `json:"masked_token"`
	APIURL       string     `json:"api_url"`
	TokenID      string     `json:"token_id,omitempty"`
	TokenName    string     `json:"token_name,omitempty"`
	CreatedAt    *time.Time `json:"created_at,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

type versionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"built_at"`
}

type authLoginResult struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Token     string     `json:"token"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// actionResult captures a simple CLI success message.
type actionResult struct {
	Message string `json:"message"`
}

type tokenRevokeResult struct {
	ID      string `json:"id"`
	Revoked bool   `json:"revoked"`
}

type tagDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

type bookDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

type userDeactivateResult struct {
	ID          string `json:"id"`
	Deactivated bool   `json:"deactivated"`
}

type recipeDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

type recipeRestoreResult struct {
	ID       string `json:"id"`
	Restored bool   `json:"restored"`
}

// mealPlanDeleteResult captures delete responses for meal plan entries.
type mealPlanDeleteResult struct {
	Date     string `json:"date"`
	RecipeID string `json:"recipe_id"`
	Deleted  bool   `json:"deleted"`
}

type recipeUpsertPayload struct {
	Title            string                   `json:"title"`
	Servings         int                      `json:"servings"`
	PrepTimeMinutes  int                      `json:"prep_time_minutes"`
	TotalTimeMinutes int                      `json:"total_time_minutes"`
	SourceURL        *string                  `json:"source_url"`
	Notes            *string                  `json:"notes"`
	RecipeBookID     *string                  `json:"recipe_book_id"`
	TagIDs           []string                 `json:"tag_ids"`
	Ingredients      []recipeIngredientUpsert `json:"ingredients"`
	Steps            []recipeStepUpsert       `json:"steps"`
}

// recipeListItemWithCounts adds counts to recipe list results.
type recipeListItemWithCounts struct {
	client.RecipeListItem
	IngredientCount int `json:"ingredient_count"`
	StepCount       int `json:"step_count"`
}

// recipeListWithCountsResponse wraps list results with ingredient/step counts.
type recipeListWithCountsResponse struct {
	Items      []recipeListItemWithCounts `json:"items"`
	NextCursor *string                    `json:"next_cursor,omitempty"`
}

// recipeImportItemResult captures a single imported recipe summary.
type recipeImportItemResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// recipeImportResult captures imported recipes in a batch.
type recipeImportResult struct {
	Items []recipeImportItemResult `json:"items"`
}

type configView struct {
	ConfigPath string `json:"config_path"`
	APIURL     string `json:"api_url"`
	Output     string `json:"output"`
	Timeout    string `json:"timeout"`
	Debug      bool   `json:"debug"`
}

// optionalString tracks whether a flag was explicitly set.
type optionalString struct {
	value string
	set   bool
}

func (o *optionalString) Set(value string) error {
	o.value = strings.TrimSpace(value)
	o.set = true
	return nil
}

func (o *optionalString) String() string {
	return o.value
}

// optionalBool tracks whether a boolean flag was explicitly set.
type optionalBool struct {
	value bool
	set   bool
}

func (o *optionalBool) Set(value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	o.value = parsed
	o.set = true
	return nil
}

func (o *optionalBool) String() string {
	if !o.set {
		return ""
	}
	return strconv.FormatBool(o.value)
}

func (o *optionalBool) IsBoolFlag() bool {
	return true
}

type recipeIngredientUpsert struct {
	Position     int      `json:"position"`
	Quantity     *float64 `json:"quantity"`
	QuantityText *string  `json:"quantity_text"`
	Unit         *string  `json:"unit"`
	Item         string   `json:"item"`
	Prep         *string  `json:"prep"`
	Notes        *string  `json:"notes"`
	OriginalText *string  `json:"original_text"`
}

type recipeStepUpsert struct {
	StepNumber  int    `json:"step_number"`
	Instruction string `json:"instruction"`
}

func defaultStore() (*credentials.Store, error) {
	path, err := credentials.DefaultPath()
	if err != nil {
		return nil, err
	}
	return credentials.NewStore(path), nil
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}

// writeLine writes output and ignores failures because there's no recovery path.
func writeLine(w io.Writer, args ...any) {
	if _, err := fmt.Fprintln(w, args...); err != nil {
		// Best-effort output only.
		_ = err
	}
}

// writef writes formatted output and ignores failures because there's no recovery path.
func writef(w io.Writer, format string, args ...any) {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		// Best-effort output only.
		_ = err
	}
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

// completionScript returns the completion script for a supported shell.
func completionScript(shell string) (string, bool) {
	switch shell {
	case "bash":
		return bashCompletion, true
	case "zsh":
		return zshCompletion, true
	case "fish":
		return fishCompletion, true
	default:
		return "", false
	}
}

func writeOutput(w io.Writer, format config.OutputFormat, data interface{}) int {
	switch format {
	case config.OutputJSON:
		if err := writeJSON(w, data); err != nil {
			return exitError
		}
		return exitOK
	case config.OutputTable:
		return writeTable(w, data)
	default:
		return exitError
	}
}

func writeJSON(w io.Writer, data interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func writeTable(w io.Writer, data interface{}) int {
	switch value := data.(type) {
	case client.HealthResponse:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "OK")
		writef(writer, "%t\n", value.OK)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case versionInfo:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "FIELD\tVALUE")
		writef(writer, "version\t%s\n", value.Version)
		writef(writer, "commit\t%s\n", value.Commit)
		writef(writer, "built_at\t%s\n", value.BuiltAt)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case client.MeResponse:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tUSERNAME\tDISPLAY_NAME")
		displayName := ""
		if value.DisplayName != nil {
			displayName = *value.DisplayName
		}
		writef(writer, "%s\t%s\t%s\n", value.ID, value.Username, displayName)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case authStatus:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "SOURCE\tAPI_URL\tTOKEN_PRESENT\tMASKED_TOKEN\tTOKEN_NAME\tTOKEN_ID\tCREATED_AT\tEXPIRES_AT")
		createdAt := ""
		if value.CreatedAt != nil {
			createdAt = value.CreatedAt.Format(time.RFC3339)
		}
		expiresAt := ""
		if value.ExpiresAt != nil {
			expiresAt = value.ExpiresAt.Format(time.RFC3339)
		}
		writef(
			writer,
			"%s\t%s\t%t\t%s\t%s\t%s\t%s\t%s\n",
			value.Source,
			value.APIURL,
			value.TokenPresent,
			value.MaskedToken,
			value.TokenName,
			value.TokenID,
			createdAt,
			expiresAt,
		)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case authLoginResult:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tNAME\tCREATED_AT\tEXPIRES_AT\tTOKEN")
		expiresAt := ""
		if value.ExpiresAt != nil {
			expiresAt = value.ExpiresAt.Format(time.RFC3339)
		}
		writef(writer, "%s\t%s\t%s\t%s\t%s\n",
			value.ID,
			value.Name,
			value.CreatedAt.Format(time.RFC3339),
			expiresAt,
			value.Token,
		)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case actionResult:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "MESSAGE")
		writeLine(writer, value.Message)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case []client.Token:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tNAME\tCREATED_AT\tLAST_USED_AT\tEXPIRES_AT")
		for _, token := range value {
			lastUsed := ""
			if token.LastUsedAt != nil {
				lastUsed = token.LastUsedAt.Format(time.RFC3339)
			}
			expiresAt := ""
			if token.ExpiresAt != nil {
				expiresAt = token.ExpiresAt.Format(time.RFC3339)
			}
			writef(writer, "%s\t%s\t%s\t%s\t%s\n",
				token.ID,
				token.Name,
				token.CreatedAt.Format(time.RFC3339),
				lastUsed,
				expiresAt,
			)
		}
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case client.CreateTokenResponse:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tNAME\tCREATED_AT\tTOKEN")
		writef(writer, "%s\t%s\t%s\t%s\n",
			value.ID,
			value.Name,
			value.CreatedAt.Format(time.RFC3339),
			value.Token,
		)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case tokenRevokeResult:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tREVOKED")
		writef(writer, "%s\t%t\n", value.ID, value.Revoked)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case []client.Tag:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tNAME\tCREATED_AT")
		for _, tag := range value {
			writef(writer, "%s\t%s\t%s\n",
				tag.ID,
				tag.Name,
				tag.CreatedAt.Format(time.RFC3339),
			)
		}
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case client.Tag:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tNAME\tCREATED_AT")
		writef(writer, "%s\t%s\t%s\n",
			value.ID,
			value.Name,
			value.CreatedAt.Format(time.RFC3339),
		)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case tagDeleteResult:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tDELETED")
		writef(writer, "%s\t%t\n", value.ID, value.Deleted)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case []client.RecipeBook:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tNAME\tCREATED_AT")
		for _, book := range value {
			writef(writer, "%s\t%s\t%s\n",
				book.ID,
				book.Name,
				book.CreatedAt.Format(time.RFC3339),
			)
		}
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case client.RecipeBook:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tNAME\tCREATED_AT")
		writef(writer, "%s\t%s\t%s\n",
			value.ID,
			value.Name,
			value.CreatedAt.Format(time.RFC3339),
		)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case bookDeleteResult:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tDELETED")
		writef(writer, "%s\t%t\n", value.ID, value.Deleted)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case []client.User:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tUSERNAME\tDISPLAY_NAME\tACTIVE\tCREATED_AT")
		for _, user := range value {
			displayName := ""
			if user.DisplayName != nil {
				displayName = *user.DisplayName
			}
			writef(writer, "%s\t%s\t%s\t%t\t%s\n",
				user.ID,
				user.Username,
				displayName,
				user.IsActive,
				user.CreatedAt.Format(time.RFC3339),
			)
		}
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case client.User:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tUSERNAME\tDISPLAY_NAME\tACTIVE\tCREATED_AT")
		displayName := ""
		if value.DisplayName != nil {
			displayName = *value.DisplayName
		}
		writef(writer, "%s\t%s\t%s\t%t\t%s\n",
			value.ID,
			value.Username,
			displayName,
			value.IsActive,
			value.CreatedAt.Format(time.RFC3339),
		)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case userDeactivateResult:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tDEACTIVATED")
		writef(writer, "%s\t%t\n", value.ID, value.Deactivated)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case client.MealPlanListResponse:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "DATE\tRECIPE_ID\tRECIPE_TITLE")
		for _, item := range value.Items {
			writef(writer, "%s\t%s\t%s\n", item.Date, item.Recipe.ID, item.Recipe.Title)
		}
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case client.MealPlanEntry:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "DATE\tRECIPE_ID\tRECIPE_TITLE")
		writef(writer, "%s\t%s\t%s\n", value.Date, value.Recipe.ID, value.Recipe.Title)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case mealPlanDeleteResult:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "DATE\tRECIPE_ID\tDELETED")
		writef(writer, "%s\t%s\t%t\n", value.Date, value.RecipeID, value.Deleted)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case recipeListWithCountsResponse:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tTITLE\tSERVINGS\tTOTAL_MIN\tBOOK_ID\tTAGS\tINGREDIENTS\tSTEPS\tUPDATED_AT")
		for _, item := range value.Items {
			bookID := ""
			if item.RecipeBookID != nil {
				bookID = *item.RecipeBookID
			}
			writef(writer, "%s\t%s\t%d\t%d\t%s\t%s\t%d\t%d\t%s\n",
				item.ID,
				item.Title,
				item.Servings,
				item.TotalTimeMinutes,
				bookID,
				formatRecipeTags(item.Tags),
				item.IngredientCount,
				item.StepCount,
				item.UpdatedAt.Format(time.RFC3339),
			)
		}
		if err := writer.Flush(); err != nil {
			return exitError
		}
		if value.NextCursor != nil {
			nextCursor := strings.TrimSpace(*value.NextCursor)
			if nextCursor != "" {
				writef(w, "next_cursor=%s\n", nextCursor)
			}
		}
		return exitOK
	case recipeImportResult:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tTITLE")
		for _, item := range value.Items {
			writef(writer, "%s\t%s\n", item.ID, item.Title)
		}
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case client.RecipeListResponse:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tTITLE\tSERVINGS\tTOTAL_MIN\tBOOK_ID\tTAGS\tUPDATED_AT")
		for _, item := range value.Items {
			bookID := ""
			if item.RecipeBookID != nil {
				bookID = *item.RecipeBookID
			}
			writef(writer, "%s\t%s\t%d\t%d\t%s\t%s\t%s\n",
				item.ID,
				item.Title,
				item.Servings,
				item.TotalTimeMinutes,
				bookID,
				formatRecipeTags(item.Tags),
				item.UpdatedAt.Format(time.RFC3339),
			)
		}
		if err := writer.Flush(); err != nil {
			return exitError
		}
		if value.NextCursor != nil {
			nextCursor := strings.TrimSpace(*value.NextCursor)
			if nextCursor != "" {
				writef(w, "next_cursor=%s\n", nextCursor)
			}
		}
		return exitOK
	case client.RecipeDetail:
		if err := writeRecipeDetailTable(w, value); err != nil {
			return exitError
		}
		return exitOK
	case recipeDeleteResult:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tDELETED")
		writef(writer, "%s\t%t\n", value.ID, value.Deleted)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case recipeRestoreResult:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "ID\tRESTORED")
		writef(writer, "%s\t%t\n", value.ID, value.Restored)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	case recipeUpsertPayload:
		if err := writeJSON(w, value); err != nil {
			return exitError
		}
		return exitOK
	case configView:
		writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		writeLine(writer, "KEY\tVALUE")
		writef(writer, "config_path\t%s\n", value.ConfigPath)
		writef(writer, "api_url\t%s\n", value.APIURL)
		writef(writer, "output\t%s\n", value.Output)
		writef(writer, "timeout\t%s\n", value.Timeout)
		writef(writer, "debug\t%t\n", value.Debug)
		if err := writer.Flush(); err != nil {
			return exitError
		}
		return exitOK
	default:
		writeLine(w, "unsupported output format for this command")
		return exitError
	}
}

// writeRecipeDetailTable renders a human-readable recipe detail view.
func writeRecipeDetailTable(w io.Writer, recipe client.RecipeDetail) error {
	writer := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	writeLine(writer, "FIELD\tVALUE")
	writef(writer, "id\t%s\n", recipe.ID)
	writef(writer, "title\t%s\n", recipe.Title)
	writef(writer, "servings\t%d\n", recipe.Servings)
	writef(writer, "prep_time_minutes\t%d\n", recipe.PrepTimeMinutes)
	writef(writer, "total_time_minutes\t%d\n", recipe.TotalTimeMinutes)
	bookID := ""
	if recipe.RecipeBookID != nil {
		bookID = *recipe.RecipeBookID
	}
	writef(writer, "recipe_book_id\t%s\n", bookID)
	writef(writer, "tags\t%s\n", formatRecipeTags(recipe.Tags))
	if recipe.SourceURL != nil {
		writef(writer, "source_url\t%s\n", strings.TrimSpace(*recipe.SourceURL))
	} else {
		writeLine(writer, "source_url\t")
	}
	if recipe.Notes != nil {
		writef(writer, "notes\t%s\n", strings.TrimSpace(*recipe.Notes))
	} else {
		writeLine(writer, "notes\t")
	}
	writef(writer, "updated_at\t%s\n", recipe.UpdatedAt.Format(time.RFC3339))
	if err := writer.Flush(); err != nil {
		return err
	}

	writeLine(w, "ingredients:")
	if len(recipe.Ingredients) == 0 {
		writeLine(w, "  (none)")
	} else {
		for _, ingredient := range recipe.Ingredients {
			line := formatIngredientLine(ingredient)
			writef(w, "  %d. %s\n", ingredient.Position, line)
		}
	}

	writeLine(w, "steps:")
	if len(recipe.Steps) == 0 {
		writeLine(w, "  (none)")
	} else {
		for _, step := range recipe.Steps {
			writef(w, "  %d. %s\n", step.StepNumber, strings.TrimSpace(step.Instruction))
		}
	}
	return nil
}

// formatIngredientLine renders a single ingredient line.
func formatIngredientLine(ingredient client.RecipeIngredient) string {
	if ingredient.OriginalText != nil {
		trimmed := strings.TrimSpace(*ingredient.OriginalText)
		if trimmed != "" {
			return trimmed
		}
	}
	parts := make([]string, 0, 4)
	if ingredient.QuantityText != nil {
		text := strings.TrimSpace(*ingredient.QuantityText)
		if text != "" {
			parts = append(parts, text)
		}
	} else if ingredient.Quantity != nil {
		parts = append(parts, formatQuantity(*ingredient.Quantity))
	}
	if ingredient.Unit != nil {
		unit := strings.TrimSpace(*ingredient.Unit)
		if unit != "" {
			parts = append(parts, unit)
		}
	}
	item := strings.TrimSpace(ingredient.Item)
	if item != "" {
		parts = append(parts, item)
	}
	line := strings.TrimSpace(strings.Join(parts, " "))
	if ingredient.Prep != nil {
		prep := strings.TrimSpace(*ingredient.Prep)
		if prep != "" {
			line = strings.TrimSpace(fmt.Sprintf("%s, %s", line, prep))
		}
	}
	if ingredient.Notes != nil {
		notes := strings.TrimSpace(*ingredient.Notes)
		if notes != "" {
			line = strings.TrimSpace(fmt.Sprintf("%s (%s)", line, notes))
		}
	}
	if line == "" {
		return "(unnamed ingredient)"
	}
	return line
}

// formatQuantity formats a numeric quantity without trailing zeros.
func formatQuantity(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
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

// readSecret reads a sensitive value from stdin without logging it.
func readSecret(r io.Reader, label string) (string, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", label, err)
	}
	return strings.TrimSpace(string(raw)), nil
}

// readPassword reads a password from stdin.
func readPassword(r io.Reader) (string, error) {
	return readSecret(r, "password")
}

// readToken reads a token from stdin.
func readToken(r io.Reader) (string, error) {
	return readSecret(r, "token")
}

func readJSONFile(path string) (json.RawMessage, error) {
	//nolint:gosec // Path is user-supplied by design for reading recipe payloads.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("file is empty")
	}
	return readJSONBytes(raw)
}

func readJSONReader(r io.Reader) (json.RawMessage, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("input is empty")
	}
	return readJSONBytes(raw)
}

func readJSONBytes(raw []byte) (json.RawMessage, error) {
	var payload map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	if len(payload) == 0 {
		return nil, errors.New("json object is empty")
	}
	return json.RawMessage(raw), nil
}

// readRawJSONFile reads JSON bytes without enforcing object-only payloads.
func readRawJSONFile(path string) ([]byte, error) {
	//nolint:gosec // Path is user-supplied by design for reading recipe payloads.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return readRawJSONBytes(raw)
}

// readRawJSONReader reads JSON bytes from a reader without enforcing object-only payloads.
func readRawJSONReader(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	return readRawJSONBytes(raw)
}

// readRawJSONBytes validates that JSON is non-empty.
func readRawJSONBytes(raw []byte) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("input is empty")
	}
	return raw, nil
}

// splitJSONPayloads splits JSON into one or more object payloads.
func splitJSONPayloads(raw []byte) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("input is empty")
	}
	if trimmed[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, fmt.Errorf("invalid json array: %w", err)
		}
		if len(items) == 0 {
			return nil, errors.New("json array is empty")
		}
		return items, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		return nil, fmt.Errorf("invalid json object: %w", err)
	}
	if len(payload) == 0 {
		return nil, errors.New("json object is empty")
	}
	return []json.RawMessage{json.RawMessage(trimmed)}, nil
}

// recipeTitleFromJSON extracts the title field for duplicate checks.
func recipeTitleFromJSON(raw json.RawMessage) (string, error) {
	var payload struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("invalid recipe payload: %w", err)
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		return "", errors.New("title is required")
	}
	return title, nil
}

// isTerminal reports whether the reader is a terminal.
func isTerminal(r io.Reader) bool {
	file, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

func formatRecipeTags(tags []client.RecipeTag) string {
	if len(tags) == 0 {
		return ""
	}
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	return strings.Join(names, ",")
}

// recipeTemplatePayload returns a starter payload with placeholders.
func recipeTemplatePayload() recipeUpsertPayload {
	ingredient := "Ingredient"
	step := "Add steps here"
	return recipeUpsertPayload{
		Title:            "New Recipe",
		Servings:         1,
		PrepTimeMinutes:  0,
		TotalTimeMinutes: 0,
		RecipeBookID:     nil,
		TagIDs:           []string{},
		Ingredients: []recipeIngredientUpsert{
			{
				Position:     1,
				Item:         ingredient,
				OriginalText: &ingredient,
			},
		},
		Steps: []recipeStepUpsert{
			{
				StepNumber:  1,
				Instruction: step,
			},
		},
	}
}

// buildRecipePayloadInteractive prompts for recipe fields and returns a payload.
func buildRecipePayloadInteractive(stdin io.Reader, stderr io.Writer, ctx context.Context, api *client.Client) (recipeUpsertPayload, error) {
	prompter := newPromptInput(stdin, stderr)
	title, err := prompter.askRequired("Title")
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	servings, err := prompter.askInt("Servings", 1, 1)
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	prepMinutes, err := prompter.askInt("Prep time minutes", 0, 0)
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	totalMinutes, err := prompter.askInt("Total time minutes", 0, 0)
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	sourceURL, err := prompter.askOptional("Source URL (optional)")
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	notes, err := prompter.askOptional("Notes (optional)")
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	bookRaw, err := prompter.askOptional("Recipe book (name or id, optional)")
	if err != nil {
		return recipeUpsertPayload{}, err
	}

	var bookID *string
	if bookRaw != nil && strings.TrimSpace(*bookRaw) != "" {
		resolvedID, resolveErr := resolveBookID(ctx, api, *bookRaw)
		if resolveErr != nil {
			return recipeUpsertPayload{}, resolveErr
		}
		bookID = &resolvedID
	}

	tagRaw, err := prompter.askOptional("Tags (comma-separated, optional)")
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	tagIDs := []string{}
	if tagRaw != nil && strings.TrimSpace(*tagRaw) != "" {
		tagNames := splitCommaSeparated(*tagRaw)
		resolvedIDs, resolveErr := resolveTagIDsByName(ctx, api, tagNames, true)
		if resolveErr != nil {
			return recipeUpsertPayload{}, resolveErr
		}
		tagIDs = resolvedIDs
	}

	writeLine(stderr, "Enter ingredients (blank to finish):")
	ingredients, err := prompter.askIngredients()
	if err != nil {
		return recipeUpsertPayload{}, err
	}

	writeLine(stderr, "Enter steps (blank to finish):")
	steps, err := prompter.askSteps()
	if err != nil {
		return recipeUpsertPayload{}, err
	}
	if len(steps) == 0 {
		return recipeUpsertPayload{}, errors.New("at least one step is required")
	}

	return recipeUpsertPayload{
		Title:            title,
		Servings:         servings,
		PrepTimeMinutes:  prepMinutes,
		TotalTimeMinutes: totalMinutes,
		SourceURL:        sourceURL,
		Notes:            notes,
		RecipeBookID:     bookID,
		TagIDs:           tagIDs,
		Ingredients:      ingredients,
		Steps:            steps,
	}, nil
}

// resolveBookID resolves a book identifier (name or id).
func resolveBookID(ctx context.Context, api *client.Client, input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", errors.New("book identifier is required")
	}
	if _, err := uuid.Parse(trimmed); err == nil {
		return trimmed, nil
	}
	return resolveBookIDByName(ctx, api, trimmed)
}

// resolveBookIDByName resolves a recipe book id from a name.
func resolveBookIDByName(ctx context.Context, api *client.Client, name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("book name is required")
	}
	books, err := api.RecipeBooks(ctx)
	if err != nil {
		return "", err
	}
	matches := make([]client.RecipeBook, 0, len(books))
	for _, book := range books {
		if strings.EqualFold(book.Name, trimmed) {
			matches = append(matches, book)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no recipe book found matching %q", trimmed)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple recipe books match %q", trimmed)
	}
	return matches[0].ID, nil
}

// resolveTagIDByName resolves a tag id from a name.
func resolveTagIDByName(ctx context.Context, api *client.Client, name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("tag name is required")
	}
	tags, err := api.Tags(ctx)
	if err != nil {
		return "", err
	}
	matches := make([]client.Tag, 0, len(tags))
	for _, tag := range tags {
		if strings.EqualFold(tag.Name, trimmed) {
			matches = append(matches, tag)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no tag found matching %q", trimmed)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple tags match %q", trimmed)
	}
	return matches[0].ID, nil
}

// resolveTagIDsByName resolves tag IDs, optionally creating missing tags.
func resolveTagIDsByName(ctx context.Context, api *client.Client, names []string, createMissing bool) ([]string, error) {
	tags, err := api.Tags(ctx)
	if err != nil {
		return nil, err
	}
	tagByName := make(map[string]client.Tag, len(tags))
	for _, tag := range tags {
		tagByName[strings.ToLower(tag.Name)] = tag
	}
	ids := make([]string, 0, len(names))
	seen := map[string]struct{}{}
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, err := uuid.Parse(trimmed); err == nil {
			if _, ok := seen[trimmed]; !ok {
				ids = append(ids, trimmed)
				seen[trimmed] = struct{}{}
			}
			continue
		}
		lookup := strings.ToLower(trimmed)
		if tag, ok := tagByName[lookup]; ok {
			if _, ok := seen[tag.ID]; !ok {
				ids = append(ids, tag.ID)
				seen[tag.ID] = struct{}{}
			}
			continue
		}
		if !createMissing {
			return nil, fmt.Errorf("tag not found: %s", trimmed)
		}
		created, err := api.CreateTag(ctx, trimmed)
		if err != nil {
			return nil, err
		}
		tagByName[strings.ToLower(created.Name)] = created
		if _, ok := seen[created.ID]; !ok {
			ids = append(ids, created.ID)
			seen[created.ID] = struct{}{}
		}
	}
	return ids, nil
}

// resolveRecipeID resolves a recipe identifier, using fuzzy title lookup if needed.
func resolveRecipeID(ctx context.Context, api *client.Client, input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", errors.New("recipe id is required")
	}
	if _, err := uuid.Parse(trimmed); err == nil {
		return trimmed, nil
	}
	resp, err := api.Recipes(ctx, client.RecipeListParams{
		Query: strings.TrimSpace(trimmed),
		Limit: 10,
	})
	if err != nil {
		return "", err
	}
	if len(resp.Items) == 0 {
		return "", fmt.Errorf("no recipe found matching %q", trimmed)
	}
	exactMatches := make([]client.RecipeListItem, 0, len(resp.Items))
	for _, item := range resp.Items {
		if strings.EqualFold(item.Title, trimmed) {
			exactMatches = append(exactMatches, item)
		}
	}
	if len(exactMatches) == 1 {
		return exactMatches[0].ID, nil
	}
	if len(exactMatches) > 1 {
		return "", fmt.Errorf("multiple recipes match %q: %s", trimmed, formatRecipeCandidates(exactMatches))
	}
	if len(resp.Items) == 1 {
		return resp.Items[0].ID, nil
	}
	return "", fmt.Errorf("multiple recipes match %q: %s", trimmed, formatRecipeCandidates(resp.Items))
}

// ensureUniqueRecipeTitle returns an error when a title already exists.
func ensureUniqueRecipeTitle(ctx context.Context, api *client.Client, title string, allowDuplicate bool) error {
	trimmed := strings.TrimSpace(title)
	if allowDuplicate {
		return nil
	}
	if trimmed == "" {
		return errors.New("title is required")
	}
	resp, err := api.Recipes(ctx, client.RecipeListParams{
		Query: trimmed,
		Limit: 25,
	})
	if err != nil {
		return err
	}
	matches := make([]client.RecipeListItem, 0, len(resp.Items))
	for _, item := range resp.Items {
		if strings.EqualFold(item.Title, trimmed) {
			matches = append(matches, item)
		}
	}
	if len(matches) == 0 {
		return nil
	}
	return fmt.Errorf("recipe title already exists: %s (use --allow-duplicate to override). matches: %s", trimmed, formatRecipeCandidates(matches))
}

// filterRecipesByServings filters list items by servings when provided.
func filterRecipesByServings(items []client.RecipeListItem, servings int) []client.RecipeListItem {
	if servings <= 0 {
		return items
	}
	filtered := make([]client.RecipeListItem, 0, len(items))
	for _, item := range items {
		if item.Servings == servings {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// addRecipeCounts loads recipe detail counts for list results.
func addRecipeCounts(ctx context.Context, api *client.Client, items []client.RecipeListItem) ([]recipeListItemWithCounts, error) {
	out := make([]recipeListItemWithCounts, 0, len(items))
	for _, item := range items {
		detail, err := api.Recipe(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, recipeListItemWithCounts{
			RecipeListItem:  item,
			IngredientCount: len(detail.Ingredients),
			StepCount:       len(detail.Steps),
		})
	}
	return out, nil
}

// mergeIDs merges two ID lists, preserving order and uniqueness.
func mergeIDs(existing, additions []string) []string {
	out := make([]string, 0, len(existing)+len(additions))
	seen := map[string]struct{}{}
	add := func(id string) {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	for _, id := range existing {
		add(id)
	}
	for _, id := range additions {
		add(id)
	}
	return out
}

// parseRecipeTagArgs parses recipe tag arguments and flags.
func parseRecipeTagArgs(args []string) (string, []string, bool, bool, error) {
	var id string
	var tags []string
	replace := false
	createMissing := true

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--replace":
			replace = true
		case "--create-missing":
			createMissing = true
		case "--no-create-missing":
			createMissing = false
		case "--":
			tags = append(tags, args[i+1:]...)
			i = len(args)
		default:
			if strings.HasPrefix(arg, "-") {
				return "", nil, false, createMissing, fmt.Errorf("unknown flag: %s", arg)
			}
			if id == "" {
				id = arg
			} else {
				tags = append(tags, arg)
			}
		}
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", nil, false, createMissing, errors.New("recipe id is required")
	}
	trimmedTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			trimmedTags = append(trimmedTags, tag)
		}
	}
	if len(trimmedTags) == 0 {
		return "", nil, false, createMissing, errors.New("at least one tag is required")
	}
	return id, trimmedTags, replace, createMissing, nil
}

// splitCommaSeparated splits comma-separated values into trimmed fields.
func splitCommaSeparated(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// formatRecipeCandidates renders candidate titles for disambiguation.
func formatRecipeCandidates(items []client.RecipeListItem) string {
	if len(items) == 0 {
		return ""
	}
	candidates := make([]string, 0, len(items))
	for _, item := range items {
		candidates = append(candidates, fmt.Sprintf("%s (%s)", item.Title, item.ID))
	}
	sort.Strings(candidates)
	return strings.Join(candidates, "; ")
}

// promptInput wraps interactive prompting helpers.
type promptInput struct {
	reader *bufio.Reader
	writer io.Writer
}

// newPromptInput creates a prompt helper for interactive input.
func newPromptInput(r io.Reader, w io.Writer) *promptInput {
	return &promptInput{
		reader: bufio.NewReader(r),
		writer: w,
	}
}

// ask prompts for a single line of input.
func (p *promptInput) ask(label string) (string, error) {
	if _, err := fmt.Fprintf(p.writer, "%s: ", label); err != nil {
		return "", err
	}
	line, err := p.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if errors.Is(err, io.EOF) && line == "" {
		return "", io.EOF
	}
	return line, nil
}

// askRequired prompts until a non-empty value is provided.
func (p *promptInput) askRequired(label string) (string, error) {
	for {
		value, err := p.ask(label)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(value) != "" {
			return value, nil
		}
	}
}

// askOptional prompts for an optional value.
func (p *promptInput) askOptional(label string) (*string, error) {
	value, err := p.ask(label)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	return &trimmed, nil
}

// askInt prompts for an integer with defaults and a minimum.
func (p *promptInput) askInt(label string, defaultValue, minValue int) (int, error) {
	for {
		value, err := p.ask(fmt.Sprintf("%s [%d]", label, defaultValue))
		if err != nil {
			return 0, err
		}
		if strings.TrimSpace(value) == "" {
			return defaultValue, nil
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			writeLine(p.writer, "Enter a valid number.")
			continue
		}
		if parsed < minValue {
			writeLine(p.writer, fmt.Sprintf("Value must be >= %d.", minValue))
			continue
		}
		return parsed, nil
	}
}

// askIngredients collects ingredient lines.
func (p *promptInput) askIngredients() ([]recipeIngredientUpsert, error) {
	ingredients := []recipeIngredientUpsert{}
	for {
		line, err := p.ask("Ingredient")
		if err != nil {
			if errors.Is(err, io.EOF) {
				return ingredients, nil
			}
			return nil, err
		}
		if strings.TrimSpace(line) == "" {
			return ingredients, nil
		}
		trimmed := strings.TrimSpace(line)
		ingredients = append(ingredients, recipeIngredientUpsert{
			Position:     len(ingredients) + 1,
			Item:         trimmed,
			OriginalText: &trimmed,
		})
	}
}

// askSteps collects recipe steps.
func (p *promptInput) askSteps() ([]recipeStepUpsert, error) {
	steps := []recipeStepUpsert{}
	for {
		line, err := p.ask("Step")
		if err != nil {
			if errors.Is(err, io.EOF) {
				return steps, nil
			}
			return nil, err
		}
		if strings.TrimSpace(line) == "" {
			return steps, nil
		}
		trimmed := strings.TrimSpace(line)
		steps = append(steps, recipeStepUpsert{
			StepNumber:  len(steps) + 1,
			Instruction: trimmed,
		})
	}
}

func splitEditorArgs(command string) ([]string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, errors.New("editor is empty")
	}
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, errors.New("editor is empty")
	}
	return parts, nil
}

func toUpsertPayload(recipe client.RecipeDetail) recipeUpsertPayload {
	tagIDs := make([]string, 0, len(recipe.Tags))
	for _, tag := range recipe.Tags {
		tagIDs = append(tagIDs, tag.ID)
	}

	ingredients := make([]recipeIngredientUpsert, 0, len(recipe.Ingredients))
	for _, ingredient := range recipe.Ingredients {
		ingredients = append(ingredients, recipeIngredientUpsert{
			Position:     ingredient.Position,
			Quantity:     ingredient.Quantity,
			QuantityText: ingredient.QuantityText,
			Unit:         ingredient.Unit,
			Item:         ingredient.Item,
			Prep:         ingredient.Prep,
			Notes:        ingredient.Notes,
			OriginalText: ingredient.OriginalText,
		})
	}

	steps := make([]recipeStepUpsert, 0, len(recipe.Steps))
	for _, step := range recipe.Steps {
		steps = append(steps, recipeStepUpsert{
			StepNumber:  step.StepNumber,
			Instruction: step.Instruction,
		})
	}

	return recipeUpsertPayload{
		Title:            recipe.Title,
		Servings:         recipe.Servings,
		PrepTimeMinutes:  recipe.PrepTimeMinutes,
		TotalTimeMinutes: recipe.TotalTimeMinutes,
		SourceURL:        recipe.SourceURL,
		Notes:            recipe.Notes,
		RecipeBookID:     recipe.RecipeBookID,
		TagIDs:           tagIDs,
		Ingredients:      ingredients,
		Steps:            steps,
	}
}

func parseIDArgs(flags *flag.FlagSet, args []string) (string, error) {
	var id string
	rest := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		id = args[0]
		rest = args[1:]
	}
	if err := flags.Parse(rest); err != nil {
		return "", err
	}
	if id == "" && flags.NArg() > 0 {
		id = flags.Arg(0)
	}
	if flags.NArg() > 1 {
		return "", errors.New("too many arguments")
	}
	return id, nil
}

// parseISODate validates a YYYY-MM-DD date string and returns its time value.
func parseISODate(field, raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("%s is required", field)
	}
	parsed, err := time.Parse(isoDateLayout, trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be YYYY-MM-DD", field)
	}
	return parsed, nil
}

// bashCompletion provides bash completion for cookctl.
const bashCompletion = `# bash completion for cookctl
_cookctl() {
  local cur first second flags
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  first="${COMP_WORDS[1]}"
  second="${COMP_WORDS[2]}"

  if [[ $COMP_CWORD -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "health version completion help auth token tag book user recipe meal-plan config" -- "$cur") )
    return 0
  fi

  if [[ "$cur" == -* ]]; then
    flags="--api-url --output --timeout --debug --skip-health-check --version --help -h"
    case "$first" in
      completion)
        flags=""
        ;;
      auth)
        case "$second" in
          login)
            flags="$flags --username --password-stdin --token-name --expires-at"
            ;;
          set)
            flags="$flags --token --token-stdin --api-url"
            ;;
          logout)
            flags="$flags --revoke"
            ;;
        esac
        ;;
      token)
        case "$second" in
          create)
            flags="$flags --name --expires-at"
            ;;
          revoke)
            flags="$flags --yes"
            ;;
        esac
        ;;
      tag)
        case "$second" in
          create|update)
            flags="$flags --name"
            ;;
          delete)
            flags="$flags --yes"
            ;;
        esac
        ;;
      book)
        case "$second" in
          create|update)
            flags="$flags --name"
            ;;
          delete)
            flags="$flags --yes"
            ;;
        esac
        ;;
      user)
        case "$second" in
          create)
            flags="$flags --username --password-stdin --display-name"
            ;;
          deactivate)
            flags="$flags --yes"
            ;;
        esac
        ;;
      recipe)
        case "$second" in
          list)
            flags="$flags --q --book --book-id --tag --tag-id --servings --include-deleted --limit --cursor --all --with-counts"
            ;;
          create)
            flags="$flags --file --stdin --interactive --allow-duplicate"
            ;;
          update)
            flags="$flags --file --stdin"
            ;;
          import)
            flags="$flags --file --stdin --allow-duplicate"
            ;;
          edit)
            flags="$flags --editor"
            ;;
          clone)
            flags="$flags --title --allow-duplicate"
            ;;
          tag)
            flags="$flags --replace --create-missing --no-create-missing"
            ;;
          delete|restore)
            flags="$flags --yes"
            ;;
        esac
        ;;
      meal-plan)
        case "$second" in
          list)
            flags="$flags --start --end"
            ;;
          create)
            flags="$flags --date --recipe-id"
            ;;
          delete)
            flags="$flags --date --recipe-id --yes"
            ;;
        esac
        ;;
      config)
        case "$second" in
          view)
            flags="$flags --config"
            ;;
          set)
            flags="$flags --config --api-url --output --timeout --debug"
            ;;
          unset)
            flags="$flags --config --api-url --output --timeout --debug"
            ;;
        esac
        ;;
    esac
    if [[ -n "$flags" ]]; then
      COMPREPLY=( $(compgen -W "$flags" -- "$cur") )
    fi
    return 0
  fi

  case "$first" in
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
      ;;
    help)
      COMPREPLY=( $(compgen -W "health version completion help auth token tag book user recipe meal-plan config" -- "$cur") )
      ;;
    auth)
      COMPREPLY=( $(compgen -W "login set status whoami logout" -- "$cur") )
      ;;
    token)
      COMPREPLY=( $(compgen -W "list create revoke" -- "$cur") )
      ;;
    tag)
      COMPREPLY=( $(compgen -W "list create update delete" -- "$cur") )
      ;;
    book)
      COMPREPLY=( $(compgen -W "list create update delete" -- "$cur") )
      ;;
    user)
      COMPREPLY=( $(compgen -W "list create deactivate" -- "$cur") )
      ;;
    recipe)
      COMPREPLY=( $(compgen -W "list get create update init template export import tag clone edit delete restore" -- "$cur") )
      ;;
    meal-plan)
      COMPREPLY=( $(compgen -W "list create delete" -- "$cur") )
      ;;
    config)
      COMPREPLY=( $(compgen -W "view set unset path" -- "$cur") )
      ;;
  esac
}

complete -F _cookctl cookctl
`

// zshCompletion provides zsh completion for cookctl.
const zshCompletion = `#compdef cookctl

_cookctl() {
  local -a commands auth_cmds token_cmds tag_cmds book_cmds user_cmds recipe_cmds meal_plan_cmds config_cmds
  commands=(
    'health:Check API health'
    'version:Show version'
    'completion:Generate shell completions'
    'help:Show command help'
    'auth:Authentication commands'
    'token:Token management'
    'tag:Tag management'
    'book:Recipe book management'
    'user:User management'
    'recipe:Recipe management'
    'meal-plan:Meal plan commands'
    'config:Config commands'
  )
  auth_cmds=(login set status whoami logout)
  token_cmds=(list create revoke)
  tag_cmds=(list create update delete)
  book_cmds=(list create update delete)
  user_cmds=(list create deactivate)
  recipe_cmds=(list get create update init template export import tag clone edit delete restore)
  meal_plan_cmds=(list create delete)
  config_cmds=(view set unset path)
  help_topics=(health version completion help auth token tag book user recipe meal-plan config)

  _arguments -C \
    '1:command:->command' \
    '*::arg:->args'

  case $state in
    command)
      _describe 'command' commands
      ;;
    args)
      case $words[2] in
        completion)
          _values 'shell' bash zsh fish
          ;;
        help)
          _values 'help topic' $help_topics
          ;;
        auth)
          _values 'auth command' $auth_cmds
          ;;
        token)
          _values 'token command' $token_cmds
          ;;
        tag)
          _values 'tag command' $tag_cmds
          ;;
        book)
          _values 'book command' $book_cmds
          ;;
        user)
          _values 'user command' $user_cmds
          ;;
        recipe)
          _values 'recipe command' $recipe_cmds
          ;;
        meal-plan)
          _values 'meal plan command' $meal_plan_cmds
          ;;
        config)
          _values 'config command' $config_cmds
          ;;
      esac
      ;;
  esac
}

compdef _cookctl cookctl
`

// fishCompletion provides fish completion for cookctl.
const fishCompletion = `# fish completion for cookctl
complete -c cookctl -f -l api-url -d 'API base URL'
complete -c cookctl -f -l output -d 'Output format (table|json)'
complete -c cookctl -f -l timeout -d 'Request timeout'
complete -c cookctl -f -l debug -d 'Enable debug logging'
complete -c cookctl -f -l skip-health-check -d 'Skip API health preflight'
complete -c cookctl -f -l version -d 'Show version and exit'
complete -c cookctl -f -s h -l help -d 'Show help and exit'

complete -c cookctl -f -n '__fish_use_subcommand' -a 'health version completion help auth token tag book user recipe meal-plan config'
complete -c cookctl -f -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
complete -c cookctl -f -n '__fish_seen_subcommand_from help' -a 'health version completion help auth token tag book user recipe meal-plan config'
complete -c cookctl -f -n '__fish_seen_subcommand_from auth' -a 'login set status whoami logout'
complete -c cookctl -f -n '__fish_seen_subcommand_from token' -a 'list create revoke'
complete -c cookctl -f -n '__fish_seen_subcommand_from tag' -a 'list create update delete'
complete -c cookctl -f -n '__fish_seen_subcommand_from book' -a 'list create update delete'
complete -c cookctl -f -n '__fish_seen_subcommand_from user' -a 'list create deactivate'
complete -c cookctl -f -n '__fish_seen_subcommand_from recipe' -a 'list get create update init template export import tag clone edit delete restore'
complete -c cookctl -f -n '__fish_seen_subcommand_from meal-plan' -a 'list create delete'
complete -c cookctl -f -n '__fish_seen_subcommand_from config' -a 'view set unset path'
`
