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

// recipeIngredientUpsert captures an ingredient payload for recipe upserts.
type recipeIngredientUpsert struct {
	Position     int      `json:"position"`
	Quantity     *float64 `json:"quantity"`
	QuantityText *string  `json:"quantity_text"`
	Unit         *string  `json:"unit"`
	ItemID       *string  `json:"item_id"`
	ItemName     string   `json:"item_name"`
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
	item := strings.TrimSpace(ingredient.Item.Name)
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

// stringPtrIfNotEmpty returns a pointer for non-empty strings.
func stringPtrIfNotEmpty(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
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
				ItemName:     ingredient,
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
			ItemName:     trimmed,
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
		itemID := stringPtrIfNotEmpty(ingredient.Item.ID)
		ingredients = append(ingredients, recipeIngredientUpsert{
			Position:     ingredient.Position,
			Quantity:     ingredient.Quantity,
			QuantityText: ingredient.QuantityText,
			Unit:         ingredient.Unit,
			ItemID:       itemID,
			ItemName:     ingredient.Item.Name,
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
