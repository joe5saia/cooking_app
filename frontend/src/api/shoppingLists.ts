import { apiFetchJSON } from './client'
import type { Item } from './items'

export type ShoppingList = {
  id: string
  list_date: string
  name: string
  notes: string | null
  created_at: string
  updated_at: string
}

export type ShoppingListItem = {
  id: string
  item: Item
  quantity: number | null
  quantity_text: string | null
  unit: string | null
  is_purchased: boolean
  purchased_at: string | null
}

export type ShoppingListDetail = ShoppingList & {
  items: ShoppingListItem[]
}

export type ShoppingListItemInput = {
  item_id: string
  quantity?: number | null
  quantity_text?: string | null
  unit?: string | null
}

export type ShoppingListUpsertRequest = {
  list_date: string
  name: string
  notes?: string | null
}

/** Fetch shopping lists within a date range. */
export function listShoppingLists(params: {
  start: string
  end: string
}): Promise<ShoppingList[]> {
  const qp = new URLSearchParams()
  qp.set('start', params.start)
  qp.set('end', params.end)
  return apiFetchJSON<ShoppingList[]>(`/api/v1/shopping-lists?${qp}`)
}

/** Create a new shopping list. */
export function createShoppingList(
  params: ShoppingListUpsertRequest,
): Promise<ShoppingList> {
  return apiFetchJSON<ShoppingList>('/api/v1/shopping-lists', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(params),
  })
}

/** Fetch a shopping list and its items. */
export function getShoppingList(id: string): Promise<ShoppingListDetail> {
  return apiFetchJSON<ShoppingListDetail>(`/api/v1/shopping-lists/${id}`)
}

/** Update a shopping list. */
export function updateShoppingList(
  id: string,
  params: ShoppingListUpsertRequest,
): Promise<ShoppingList> {
  return apiFetchJSON<ShoppingList>(`/api/v1/shopping-lists/${id}`, {
    method: 'PUT',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(params),
  })
}

/** Delete a shopping list. */
export function deleteShoppingList(id: string): Promise<void> {
  return apiFetchJSON<void>(`/api/v1/shopping-lists/${id}`, {
    method: 'DELETE',
  })
}

/** Add explicit items to a shopping list. */
export function addShoppingListItems(
  listID: string,
  items: ShoppingListItemInput[],
): Promise<ShoppingListItem[]> {
  return apiFetchJSON<ShoppingListItem[]>(
    `/api/v1/shopping-lists/${listID}/items`,
    {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ items }),
    },
  )
}

/** Add items from recipe IDs to a shopping list. */
export function addShoppingListItemsFromRecipes(
  listID: string,
  recipeIDs: string[],
): Promise<ShoppingListItem[]> {
  return apiFetchJSON<ShoppingListItem[]>(
    `/api/v1/shopping-lists/${listID}/items/from-recipes`,
    {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ recipe_ids: recipeIDs }),
    },
  )
}

/** Add items from a meal plan date to a shopping list. */
export function addShoppingListItemsFromMealPlan(
  listID: string,
  date: string,
): Promise<ShoppingListItem[]> {
  return apiFetchJSON<ShoppingListItem[]>(
    `/api/v1/shopping-lists/${listID}/items/from-meal-plan`,
    {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ date }),
    },
  )
}

/** Toggle purchase state for a shopping list item. */
export function updateShoppingListItemPurchase(
  listID: string,
  itemID: string,
  isPurchased: boolean,
): Promise<ShoppingListItem> {
  return apiFetchJSON<ShoppingListItem>(
    `/api/v1/shopping-lists/${listID}/items/${itemID}`,
    {
      method: 'PATCH',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ is_purchased: isPurchased }),
    },
  )
}

/** Remove an item from a shopping list. */
export function deleteShoppingListItem(
  listID: string,
  itemID: string,
): Promise<void> {
  return apiFetchJSON<void>(
    `/api/v1/shopping-lists/${listID}/items/${itemID}`,
    { method: 'DELETE' },
  )
}
