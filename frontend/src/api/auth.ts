import { apiFetchJSON } from './client'

export type Me = {
  id: string
  username: string
  display_name: string | null
}

export function login(params: {
  username: string
  password: string
}): Promise<void> {
  return apiFetchJSON<void>('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
}

export function getMe(): Promise<Me> {
  return apiFetchJSON<Me>('/api/v1/auth/me')
}
