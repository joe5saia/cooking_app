import { apiFetchJSON } from './client'

export type User = {
  id: string
  username: string
  display_name: string | null
  is_active: boolean
  created_at: string
}

export function listUsers(): Promise<User[]> {
  return apiFetchJSON<User[]>('/api/v1/users')
}

export function createUser(params: {
  username: string
  password: string
  display_name?: string | null
}): Promise<User> {
  return apiFetchJSON<User>('/api/v1/users', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
}

export function deactivateUser(id: string): Promise<void> {
  return apiFetchJSON<void>(`/api/v1/users/${id}/deactivate`, { method: 'PUT' })
}
