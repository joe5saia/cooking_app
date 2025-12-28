package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/saiaj/cooking_app/backend/internal/cookctl/client"
)

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
