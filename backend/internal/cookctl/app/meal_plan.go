package app

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"
)

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
