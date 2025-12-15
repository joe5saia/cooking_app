import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { TokenManagerPage } from './TokenManagerPage'

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <TokenManagerPage />
    </QueryClientProvider>,
  )
}

describe('TokenManagerPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('creates a token and renders secret once', async () => {
    const tokens: Array<{
      id: string
      name: string
      created_at: string
      last_used_at: string | null
      expires_at: string | null
    }> = []

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/tokens') && method === 'GET') {
          return new Response(JSON.stringify(tokens), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        if (url.endsWith('/api/v1/tokens') && method === 'POST') {
          const body = JSON.parse(String(init?.body ?? '{}')) as {
            name?: string
          }
          const created = {
            id: 't1',
            name: body.name ?? '',
            token: 'cooking_app_pat_secret',
            created_at: '2025-01-01T00:00:00Z',
          }
          tokens.push({
            id: created.id,
            name: created.name,
            created_at: created.created_at,
            last_used_at: null,
            expires_at: null,
          })
          return new Response(JSON.stringify(created), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        return new Response(null, { status: 404 })
      }),
    )

    const user = userEvent.setup()
    renderPage()

    expect(await screen.findByText(/no tokens yet/i)).toBeVisible()

    await user.type(screen.getByPlaceholderText(/token name/i), 'laptop-cli')
    await user.click(screen.getByRole('button', { name: /create token/i }))

    expect(await screen.findByText(/cooking_app_pat_secret/i)).toBeVisible()

    await waitFor(() => expect(screen.getByText('laptop-cli')).toBeVisible())
    expect(screen.queryAllByText(/cooking_app_pat_secret/i)).toHaveLength(1)
  })

  it('revokes a token', async () => {
    const tokens = [
      {
        id: 't1',
        name: 'laptop-cli',
        created_at: '2025-01-01T00:00:00Z',
        last_used_at: null,
        expires_at: null,
      },
    ]

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/tokens') && method === 'GET') {
          return new Response(JSON.stringify(tokens), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        if (url.endsWith('/api/v1/tokens/t1') && method === 'DELETE') {
          tokens.splice(0, tokens.length)
          return new Response(null, { status: 204 })
        }

        return new Response(null, { status: 404 })
      }),
    )

    const user = userEvent.setup()
    renderPage()

    expect(await screen.findByText('laptop-cli')).toBeVisible()
    await user.click(screen.getByRole('button', { name: /revoke/i }))
    await waitFor(() =>
      expect(screen.getByText(/no tokens yet/i)).toBeVisible(),
    )
  })
})
