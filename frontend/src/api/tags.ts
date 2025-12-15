import { apiFetchJSON } from './client'

export type Tag = {
  id: string
  name: string
  created_at: string
}

export function listTags(): Promise<Tag[]> {
  return apiFetchJSON<Tag[]>('/api/v1/tags')
}

export function createTag(params: { name: string }): Promise<Tag> {
  return apiFetchJSON<Tag>('/api/v1/tags', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
}

export function updateTag(id: string, params: { name: string }): Promise<Tag> {
  return apiFetchJSON<Tag>(`/api/v1/tags/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
}

export function deleteTag(id: string): Promise<void> {
  return apiFetchJSON<void>(`/api/v1/tags/${id}`, { method: 'DELETE' })
}
