package app

import (
	"context"
	"flag"
	"io"
	"strings"
)

type mealPlanListFlags struct {
	start string
	end   string
}

type mealPlanCreateFlags struct {
	date     string
	recipeID string
}

type mealPlanDeleteFlags struct {
	date     string
	recipeID string
	yes      bool
}

func mealPlanListFlagSet(out io.Writer) (*flag.FlagSet, *mealPlanListFlags) {
	opts := &mealPlanListFlags{}
	flags := newFlagSet("meal-plan list", out, printMealPlanListUsage)
	flags.StringVar(&opts.start, "start", "", "Start date (YYYY-MM-DD)")
	flags.StringVar(&opts.end, "end", "", "End date (YYYY-MM-DD)")
	return flags, opts
}

func mealPlanCreateFlagSet(out io.Writer) (*flag.FlagSet, *mealPlanCreateFlags) {
	opts := &mealPlanCreateFlags{}
	flags := newFlagSet("meal-plan create", out, printMealPlanCreateUsage)
	flags.StringVar(&opts.date, "date", "", "Meal plan date (YYYY-MM-DD)")
	flags.StringVar(&opts.recipeID, "recipe-id", "", "Recipe id")
	return flags, opts
}

func mealPlanDeleteFlagSet(out io.Writer) (*flag.FlagSet, *mealPlanDeleteFlags) {
	opts := &mealPlanDeleteFlags{}
	flags := newFlagSet("meal-plan delete", out, printMealPlanDeleteUsage)
	flags.StringVar(&opts.date, "date", "", "Meal plan date (YYYY-MM-DD)")
	flags.StringVar(&opts.recipeID, "recipe-id", "", "Recipe id")
	flags.BoolVar(&opts.yes, "yes", false, "Confirm meal plan deletion")
	return flags, opts
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
		usageErrorf(a.stderr, "unknown meal-plan command: %s", args[0])
		printMealPlanUsage(a.stderr)
		return exitUsage
	}
}

func (a *App) runMealPlanList(args []string) int {
	if hasHelpFlag(args) {
		printMealPlanListUsage(a.stdout)
		return exitOK
	}

	flags, opts := mealPlanListFlagSet(a.stderr)
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

	flags, opts := mealPlanCreateFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	planDate, err := parseISODate("date", opts.date)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	opts.recipeID = strings.TrimSpace(opts.recipeID)
	if opts.recipeID == "" {
		return usageError(a.stderr, "recipe-id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.Timeout)
	defer cancel()

	api, exitCode := a.authedClient(ctx)
	if exitCode != exitOK {
		return exitCode
	}

	resp, err := api.CreateMealPlan(ctx, planDate.Format(isoDateLayout), opts.recipeID)
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

	flags, opts := mealPlanDeleteFlagSet(a.stderr)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}

	planDate, err := parseISODate("date", opts.date)
	if err != nil {
		return usageError(a.stderr, err.Error())
	}
	opts.recipeID = strings.TrimSpace(opts.recipeID)
	if opts.recipeID == "" {
		return usageError(a.stderr, "recipe-id is required")
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

	if err := api.DeleteMealPlan(ctx, planDate.Format(isoDateLayout), opts.recipeID); err != nil {
		return a.handleAPIError(err)
	}

	return writeOutput(a.stdout, a.cfg.Output, mealPlanDeleteResult{
		Date:     planDate.Format(isoDateLayout),
		RecipeID: opts.recipeID,
		Deleted:  true,
	})
}
