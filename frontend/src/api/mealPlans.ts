import { apiFetchJSON } from './client'

export type MealPlanRecipe = {
  id: string
  title: string
}

export type MealPlanEntry = {
  date: string
  recipe: MealPlanRecipe
}

export type MealPlanListResponse = {
  items: MealPlanEntry[]
}

export type CreateMealPlanRequest = {
  date: string
  recipe_id: string
}

/**
 * Fetches meal plan entries for a date range (inclusive).
 */
export function listMealPlans(params: {
  start: string
  end: string
}): Promise<MealPlanListResponse> {
  const qp = new URLSearchParams({
    start: params.start,
    end: params.end,
  })
  return apiFetchJSON<MealPlanListResponse>(
    `/api/v1/meal-plans?${qp.toString()}`,
  )
}

/**
 * Creates a new meal plan entry for a given date and recipe.
 */
export function createMealPlan(
  params: CreateMealPlanRequest,
): Promise<MealPlanEntry> {
  return apiFetchJSON<MealPlanEntry>('/api/v1/meal-plans', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
}

/**
 * Removes a recipe from a specific date on the meal plan.
 */
export function deleteMealPlan(params: {
  date: string
  recipe_id: string
}): Promise<void> {
  const date = encodeURIComponent(params.date)
  const recipeID = encodeURIComponent(params.recipe_id)
  return apiFetchJSON<void>(`/api/v1/meal-plans/${date}/${recipeID}`, {
    method: 'DELETE',
  })
}
