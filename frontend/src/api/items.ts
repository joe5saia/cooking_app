import { apiFetchJSON } from './client'

export type GroceryAisle = {
  id: string
  name: string
  sort_group: number
  sort_order: number
  numeric_value: number | null
}

export type Item = {
  id: string
  name: string
  store_url: string | null
  aisle: GroceryAisle | null
}

export type ItemListParams = {
  q?: string
  limit?: number
}

export type ItemUpsertRequest = {
  name: string
  store_url?: string | null
  aisle_id?: string | null
}

export type GroceryAisleUpsertRequest = {
  name: string
  sort_group: number
  sort_order: number
  numeric_value?: number | null
}

/** Fetch a filtered list of items. */
export function listItems(params: ItemListParams = {}): Promise<Item[]> {
  const qp = new URLSearchParams()
  if (params.q?.trim()) qp.set('q', params.q.trim())
  if (typeof params.limit === 'number') qp.set('limit', String(params.limit))
  const url = qp.size ? `/api/v1/items?${qp.toString()}` : '/api/v1/items'
  return apiFetchJSON<Item[]>(url)
}

/** Create a new grocery item. */
export function createItem(params: ItemUpsertRequest): Promise<Item> {
  return apiFetchJSON<Item>('/api/v1/items', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(params),
  })
}

/** Update an existing grocery item. */
export function updateItem(
  id: string,
  params: ItemUpsertRequest,
): Promise<Item> {
  return apiFetchJSON<Item>(`/api/v1/items/${id}`, {
    method: 'PUT',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(params),
  })
}

/** Delete a grocery item by id. */
export function deleteItem(id: string): Promise<void> {
  return apiFetchJSON<void>(`/api/v1/items/${id}`, { method: 'DELETE' })
}

/** Fetch all grocery aisles in display order. */
export function listAisles(): Promise<GroceryAisle[]> {
  return apiFetchJSON<GroceryAisle[]>('/api/v1/aisles')
}

/** Create a new grocery aisle. */
export function createAisle(
  params: GroceryAisleUpsertRequest,
): Promise<GroceryAisle> {
  return apiFetchJSON<GroceryAisle>('/api/v1/aisles', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(params),
  })
}

/** Update an existing grocery aisle. */
export function updateAisle(
  id: string,
  params: GroceryAisleUpsertRequest,
): Promise<GroceryAisle> {
  return apiFetchJSON<GroceryAisle>(`/api/v1/aisles/${id}`, {
    method: 'PUT',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(params),
  })
}

/** Delete a grocery aisle by id. */
export function deleteAisle(id: string): Promise<void> {
  return apiFetchJSON<void>(`/api/v1/aisles/${id}`, { method: 'DELETE' })
}
