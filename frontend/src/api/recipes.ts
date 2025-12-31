import { apiFetchJSON } from './client'
import type { Item } from './items'

export type RecipeTag = {
  id: string
  name: string
}

export type RecipeListItem = {
  id: string
  title: string
  servings: number
  prep_time_minutes: number
  total_time_minutes: number
  source_url: string | null
  notes: string | null
  recipe_book_id: string | null
  tags: RecipeTag[]
  deleted_at: string | null
  updated_at: string
}

export type RecipeListResponse = {
  items: RecipeListItem[]
  next_cursor: string | null
}

export type RecipeIngredient = {
  id: string
  position: number
  quantity: number | null
  quantity_text: string | null
  unit: string | null
  item: Item
  prep: string | null
  notes: string | null
  original_text: string | null
}

export type RecipeStep = {
  id: string
  step_number: number
  instruction: string
}

export type RecipeIngredientUpsert = {
  position: number
  quantity: number | null
  quantity_text: string | null
  unit: string | null
  item_id: string | null
  item_name: string | null
  prep: string | null
  notes: string | null
  original_text: string | null
}

export type RecipeStepUpsert = {
  step_number: number
  instruction: string
}

export type RecipeUpsertRequest = {
  title: string
  servings: number
  prep_time_minutes: number
  total_time_minutes: number
  source_url: string | null
  notes: string | null
  recipe_book_id: string | null
  tag_ids: string[]
  ingredients: RecipeIngredientUpsert[]
  steps: RecipeStepUpsert[]
}

export type RecipeDetail = {
  id: string
  title: string
  servings: number
  prep_time_minutes: number
  total_time_minutes: number
  source_url: string | null
  notes: string | null
  recipe_book_id: string | null
  tags: RecipeTag[]
  ingredients: RecipeIngredient[]
  steps: RecipeStep[]
  created_at: string
  created_by: string
  updated_at: string
  updated_by: string
  deleted_at: string | null
}

export type ListRecipesParams = {
  q?: string
  book_id?: string
  tag_id?: string
  include_deleted?: boolean
  limit?: number
  cursor?: string
}

export function listRecipes(
  params: ListRecipesParams = {},
): Promise<RecipeListResponse> {
  const qp = new URLSearchParams()

  if (params.q?.trim()) qp.set('q', params.q.trim())
  if (params.book_id) qp.set('book_id', params.book_id)
  if (params.tag_id) qp.set('tag_id', params.tag_id)
  if (params.include_deleted) qp.set('include_deleted', 'true')
  if (typeof params.limit === 'number') qp.set('limit', String(params.limit))
  if (params.cursor) qp.set('cursor', params.cursor)

  const url = qp.size ? `/api/v1/recipes?${qp.toString()}` : '/api/v1/recipes'
  return apiFetchJSON<RecipeListResponse>(url)
}

export function getRecipe(id: string): Promise<RecipeDetail> {
  return apiFetchJSON<RecipeDetail>(`/api/v1/recipes/${id}`)
}

export function createRecipe(
  params: RecipeUpsertRequest,
): Promise<RecipeDetail> {
  return apiFetchJSON<RecipeDetail>('/api/v1/recipes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
}

export function updateRecipe(
  id: string,
  params: RecipeUpsertRequest,
): Promise<RecipeDetail> {
  return apiFetchJSON<RecipeDetail>(`/api/v1/recipes/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
}

export function deleteRecipe(id: string): Promise<void> {
  return apiFetchJSON<void>(`/api/v1/recipes/${id}`, { method: 'DELETE' })
}

export function restoreRecipe(id: string): Promise<void> {
  return apiFetchJSON<void>(`/api/v1/recipes/${id}/restore`, { method: 'PUT' })
}
