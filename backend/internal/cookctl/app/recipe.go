package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
)

const (
	recipeTagFlagReplace         = "replace"
	recipeTagFlagCreateMissing   = "create-missing"
	recipeTagFlagNoCreateMissing = "no-create-missing"
)

type recipeListFlags struct {
	query          string
	bookID         string
	bookName       string
	tagID          string
	tagName        string
	includeDeleted bool
	limit          int
	cursor         string
	all            bool
	servings       int
	withCounts     bool
}

type recipeCreateFlags struct {
	filePath       string
	useStdin       bool
	interactive    bool
	allowDuplicate bool
}

type recipeUpdateFlags struct {
	filePath string
	useStdin bool
}

type recipeImportFlags struct {
	filePath       string
	useStdin       bool
	allowDuplicate bool
}

type recipeCloneFlags struct {
	titleOverride  string
	allowDuplicate bool
}

type recipeEditFlags struct {
	editorOverride string
}

type recipeDeleteFlags struct {
	yes bool
}

type recipeRestoreFlags struct {
	yes bool
}

func recipeListFlagSet(out io.Writer) (*flag.FlagSet, *recipeListFlags) {
	opts := &recipeListFlags{}
	flags := newFlagSet("recipe list", out, printRecipeListUsage)
	flags.StringVar(&opts.query, "q", "", "Search query")
	flags.StringVar(&opts.bookID, "book-id", "", "Filter by recipe book id")
	flags.StringVar(&opts.bookName, "book", "", "Filter by recipe book name")
	flags.StringVar(&opts.tagID, "tag-id", "", "Filter by tag id")
	flags.StringVar(&opts.tagName, "tag", "", "Filter by tag name")
	flags.BoolVar(&opts.includeDeleted, "include-deleted", false, "Include deleted recipes")
	flags.IntVar(&opts.limit, "limit", 0, "Max items per page")
	flags.StringVar(&opts.cursor, "cursor", "", "Pagination cursor")
	flags.BoolVar(&opts.all, "all", false, "Fetch all pages")
	flags.IntVar(&opts.servings, "servings", 0, "Filter by servings count")
	flags.BoolVar(&opts.withCounts, "with-counts", false, "Include ingredient and step counts")
	return flags, opts
}

func recipeGetFlagSet(out io.Writer) *flag.FlagSet {
	return newFlagSet("recipe get", out, printRecipeGetUsage)
}

func recipeCreateFlagSet(out io.Writer) (*flag.FlagSet, *recipeCreateFlags) {
	opts := &recipeCreateFlags{}
	flags := newFlagSet("recipe create", out, printRecipeCreateUsage)
	flags.StringVar(&opts.filePath, "file", "", "Path to recipe JSON")
	flags.BoolVar(&opts.useStdin, "stdin", false, "Read recipe JSON from stdin")
	flags.BoolVar(&opts.interactive, "interactive", false, "Create recipe with interactive prompts")
	flags.BoolVar(&opts.allowDuplicate, "allow-duplicate", false, "Allow duplicate recipe titles")
	return flags, opts
}

func recipeUpdateFlagSet(out io.Writer) (*flag.FlagSet, *recipeUpdateFlags) {
	opts := &recipeUpdateFlags{}
	flags := newFlagSet("recipe update", out, printRecipeUpdateUsage)
	flags.StringVar(&opts.filePath, "file", "", "Path to recipe JSON")
	flags.BoolVar(&opts.useStdin, "stdin", false, "Read recipe JSON from stdin")
	return flags, opts
}

func recipeInitFlagSet(out io.Writer) *flag.FlagSet {
	return newFlagSet("recipe init", out, printRecipeInitUsage)
}

func recipeTemplateFlagSet(out io.Writer) *flag.FlagSet {
	return newFlagSet("recipe template", out, printRecipeTemplateUsage)
}

func recipeExportFlagSet(out io.Writer) *flag.FlagSet {
	return newFlagSet("recipe export", out, printRecipeExportUsage)
}

func recipeImportFlagSet(out io.Writer) (*flag.FlagSet, *recipeImportFlags) {
	opts := &recipeImportFlags{}
	flags := newFlagSet("recipe import", out, printRecipeImportUsage)
	flags.StringVar(&opts.filePath, "file", "", "Path to recipe JSON")
	flags.BoolVar(&opts.useStdin, "stdin", false, "Read recipe JSON from stdin")
	flags.BoolVar(&opts.allowDuplicate, "allow-duplicate", false, "Allow duplicate recipe titles")
	return flags, opts
}

func recipeTagFlagSet(out io.Writer) *flag.FlagSet {
	flags := newFlagSet("recipe tag", out, printRecipeTagUsage)
	flags.BoolVar(new(bool), recipeTagFlagReplace, false, "Replace existing tags")
	flags.BoolVar(new(bool), recipeTagFlagCreateMissing, true, "Create missing tags")
	flags.BoolVar(new(bool), recipeTagFlagNoCreateMissing, false, "Do not create missing tags")
	return flags
}

func recipeCloneFlagSet(out io.Writer) (*flag.FlagSet, *recipeCloneFlags) {
	opts := &recipeCloneFlags{}
	flags := newFlagSet("recipe clone", out, printRecipeCloneUsage)
	flags.StringVar(&opts.titleOverride, "title", "", "Title for the cloned recipe")
	flags.BoolVar(&opts.allowDuplicate, "allow-duplicate", false, "Allow duplicate recipe titles")
	return flags, opts
}

func recipeEditFlagSet(out io.Writer) (*flag.FlagSet, *recipeEditFlags) {
	opts := &recipeEditFlags{}
	flags := newFlagSet("recipe edit", out, printRecipeEditUsage)
	flags.StringVar(&opts.editorOverride, "editor", "", "Editor command")
	return flags, opts
}

func recipeDeleteFlagSet(out io.Writer) (*flag.FlagSet, *recipeDeleteFlags) {
	opts := &recipeDeleteFlags{}
	flags := newFlagSet("recipe delete", out, printRecipeDeleteUsage)
	flags.BoolVar(&opts.yes, "yes", false, "Confirm recipe deletion")
	return flags, opts
}

func recipeRestoreFlagSet(out io.Writer) (*flag.FlagSet, *recipeRestoreFlags) {
	opts := &recipeRestoreFlags{}
	flags := newFlagSet("recipe restore", out, printRecipeRestoreUsage)
	flags.BoolVar(&opts.yes, "yes", false, "Confirm recipe restore")
	return flags, opts
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
		usageErrorf(a.stderr, "unknown recipe command: %s", args[0])
		printRecipeUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runRecipeList(args []string) int {
	if hasHelpFlag(args) {
		printRecipeListUsage(a.stdout)
		return exitOK
	}

	flags, opts := recipeListFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if opts.limit < 0 {
		return usageError(a.stderr, "limit must be positive")
	}
	if opts.servings < 0 {
		return usageError(a.stderr, "servings must be positive")
	}
	if strings.TrimSpace(opts.bookID) != "" && strings.TrimSpace(opts.bookName) != "" {
		return usageError(a.stderr, "book and book-id cannot be combined")
	}
	if strings.TrimSpace(opts.tagID) != "" && strings.TrimSpace(opts.tagName) != "" {
		return usageError(a.stderr, "tag and tag-id cannot be combined")
	}

	listParams := client.RecipeListParams{
		Query:          strings.TrimSpace(opts.query),
		BookID:         strings.TrimSpace(opts.bookID),
		TagID:          strings.TrimSpace(opts.tagID),
		IncludeDeleted: opts.includeDeleted,
		Limit:          opts.limit,
		Cursor:         strings.TrimSpace(opts.cursor),
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	if strings.TrimSpace(opts.bookName) != "" {
		resolved, resolveErr := resolveBookIDByName(ctx, api, opts.bookName)
		if resolveErr != nil {
			return usageError(a.stderr, resolveErr.Error())
		}
		listParams.BookID = resolved
	}
	if strings.TrimSpace(opts.tagName) != "" {
		resolved, resolveErr := resolveTagIDByName(ctx, api, opts.tagName)
		if resolveErr != nil {
			return usageError(a.stderr, resolveErr.Error())
		}
		listParams.TagID = resolved
	}
	if opts.servings > 0 {
		opts.all = true
	}

	if !opts.all {
		resp, err := api.Recipes(ctx, listParams)
		if err != nil {
			return a.handleAPIError(err)
		}
		items := filterRecipesByServings(resp.Items, opts.servings)
		if opts.withCounts {
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

	allItems = filterRecipesByServings(allItems, opts.servings)
	if opts.withCounts {
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

	flags := recipeGetFlagSet(a.stderr)

	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "recipe id is required")
	}
	id = strings.TrimSpace(id)

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		return usageError(a.stderr, err.Error())
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

	flags, opts := recipeCreateFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	opts.filePath = strings.TrimSpace(opts.filePath)
	if opts.interactive && (opts.filePath != "" || opts.useStdin) {
		return usageError(a.stderr, "interactive cannot be combined with --file or --stdin")
	}
	if !opts.interactive && (opts.filePath == "" && !opts.useStdin) {
		return usageError(a.stderr, "provide --file, --stdin, or --interactive")
	}
	if !opts.interactive && opts.filePath != "" && opts.useStdin {
		return usageError(a.stderr, "provide --file or --stdin")
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	var payload json.RawMessage
	if opts.interactive {
		interactivePayload, buildErr := buildRecipePayloadInteractive(a.stdin, a.stderr, ctx, api)
		if buildErr != nil {
			return usageError(a.stderr, buildErr.Error())
		}
		if dupErr := ensureUniqueRecipeTitle(ctx, api, interactivePayload.Title, opts.allowDuplicate); dupErr != nil {
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
		if opts.useStdin {
			payload, readErr = readJSONReader(a.stdin)
		} else {
			payload, readErr = readJSONFile(opts.filePath)
		}
		if readErr != nil {
			return usageError(a.stderr, readErr.Error())
		}
		title, titleErr := recipeTitleFromJSON(payload)
		if titleErr != nil {
			return usageError(a.stderr, titleErr.Error())
		}
		if dupErr := ensureUniqueRecipeTitle(ctx, api, title, opts.allowDuplicate); dupErr != nil {
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

	flags, opts := recipeUpdateFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "recipe id is required")
	}
	id = strings.TrimSpace(id)
	opts.filePath = strings.TrimSpace(opts.filePath)
	if (opts.filePath == "" && !opts.useStdin) || (opts.filePath != "" && opts.useStdin) {
		return usageError(a.stderr, "provide --file or --stdin")
	}

	var payload json.RawMessage
	if opts.useStdin {
		payload, err = readJSONReader(a.stdin)
	} else {
		payload, err = readJSONFile(opts.filePath)
	}
	if err != nil {
		return usageError(a.stderr, err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		return usageError(a.stderr, err.Error())
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

	flags := recipeInitFlagSet(a.stderr)

	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}

	if id == "" {
		return writeOutput(a.stdout, a.cfg.Output, recipeTemplatePayload())
	}
	id = strings.TrimSpace(id)

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		return usageError(a.stderr, err.Error())
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

	flags := recipeTemplateFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() > 0 {
		return usageError(a.stderr, "template does not accept arguments")
	}
	return writeOutput(a.stdout, a.cfg.Output, recipeTemplatePayload())
}

func (a *App) runRecipeExport(args []string) int {
	if hasHelpFlag(args) {
		printRecipeExportUsage(a.stdout)
		return exitOK
	}

	flags := recipeExportFlagSet(a.stderr)

	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "recipe id is required")
	}
	id = strings.TrimSpace(id)

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		return usageError(a.stderr, err.Error())
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

	flags, opts := recipeImportFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	opts.filePath = strings.TrimSpace(opts.filePath)
	if opts.filePath == "" && !opts.useStdin {
		if !isTerminal(a.stdin) {
			opts.useStdin = true
		} else {
			return usageError(a.stderr, "provide --file or --stdin")
		}
	}
	if opts.filePath != "" && opts.useStdin {
		return usageError(a.stderr, "provide --file or --stdin")
	}

	var raw []byte
	var err error
	if opts.useStdin {
		raw, err = readRawJSONReader(a.stdin)
	} else {
		raw, err = readRawJSONFile(opts.filePath)
	}
	if err != nil {
		return usageError(a.stderr, err.Error())
	}

	payloads, err := splitJSONPayloads(raw)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	results := make([]recipeImportItemResult, 0, len(payloads))
	for i, payload := range payloads {
		title, titleErr := recipeTitleFromJSON(payload)
		if titleErr != nil {
			return usageErrorf(a.stderr, "payload %d: %v", i+1, titleErr)
		}
		if err := ensureUniqueRecipeTitle(ctx, api, title, opts.allowDuplicate); err != nil {
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
		return usageError(a.stderr, err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}

	tagIDs, err := resolveTagIDsByName(ctx, api, tagNames, createMissing)
	if err != nil {
		return usageError(a.stderr, err.Error())
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

	flags, opts := recipeCloneFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "recipe id is required")
	}
	id = strings.TrimSpace(id)

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}

	recipe, err := api.Recipe(ctx, resolvedID)
	if err != nil {
		return a.handleAPIError(err)
	}

	payload := toUpsertPayload(recipe)
	title := strings.TrimSpace(opts.titleOverride)
	if title == "" {
		title = fmt.Sprintf("%s (copy)", payload.Title)
	}
	payload.Title = title

	if dupErr := ensureUniqueRecipeTitle(ctx, api, payload.Title, opts.allowDuplicate); dupErr != nil {
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

	flags, opts := recipeEditFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "recipe id is required")
	}
	id = strings.TrimSpace(id)

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		return usageError(a.stderr, err.Error())
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

	editor := strings.TrimSpace(opts.editorOverride)
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
		return usageError(a.stderr, err.Error())
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

	flags, opts := recipeDeleteFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "recipe id is required")
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

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		return usageError(a.stderr, err.Error())
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

	flags, opts := recipeRestoreFlagSet(a.stderr)
	id, err := parseIDArgs(flags, args)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	if id == "" {
		return usageError(a.stderr, "recipe id is required")
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

	resolvedID, err := resolveRecipeID(ctx, api, id)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}

	if err := api.RestoreRecipe(ctx, resolvedID); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, recipeRestoreResult{
		ID:       resolvedID,
		Restored: true,
	})
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
		case "--" + recipeTagFlagReplace:
			replace = true
		case "--" + recipeTagFlagCreateMissing:
			createMissing = true
		case "--" + recipeTagFlagNoCreateMissing:
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
