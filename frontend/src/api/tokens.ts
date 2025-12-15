import { apiFetchJSON } from './client'

export type Token = {
  id: string
  name: string
  created_at: string
  last_used_at: string | null
  expires_at: string | null
}

export type CreateTokenResponse = {
  id: string
  name: string
  token: string
  created_at: string
}

export function listTokens(): Promise<Token[]> {
  return apiFetchJSON<Token[]>('/api/v1/tokens')
}

export function createToken(params: {
  name: string
  expires_at?: string
}): Promise<CreateTokenResponse> {
  return apiFetchJSON<CreateTokenResponse>('/api/v1/tokens', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
}

export function revokeToken(id: string): Promise<void> {
  return apiFetchJSON<void>(`/api/v1/tokens/${id}`, { method: 'DELETE' })
}
