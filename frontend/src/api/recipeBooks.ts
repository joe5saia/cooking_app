import { apiFetchJSON } from './client'

export type RecipeBook = {
  id: string
  name: string
  created_at: string
}

export function listRecipeBooks(): Promise<RecipeBook[]> {
  return apiFetchJSON<RecipeBook[]>('/api/v1/recipe-books')
}

export function createRecipeBook(params: {
  name: string
}): Promise<RecipeBook> {
  return apiFetchJSON<RecipeBook>('/api/v1/recipe-books', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
}

export function updateRecipeBook(
  id: string,
  params: { name: string },
): Promise<RecipeBook> {
  return apiFetchJSON<RecipeBook>(`/api/v1/recipe-books/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
}

export function deleteRecipeBook(id: string): Promise<void> {
  return apiFetchJSON<void>(`/api/v1/recipe-books/${id}`, { method: 'DELETE' })
}
