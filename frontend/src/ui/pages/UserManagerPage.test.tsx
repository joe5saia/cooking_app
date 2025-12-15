import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { UserManagerPage } from './UserManagerPage'

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <UserManagerPage />
    </QueryClientProvider>,
  )
}

describe('UserManagerPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('validates required fields', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = typeof input === 'string' ? input : input.toString()
        if (url.endsWith('/api/v1/users')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        return new Response(null, { status: 404 })
      }),
    )

    const user = userEvent.setup()
    renderPage()

    await user.click(screen.getByRole('button', { name: /create user/i }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      /username is required/i,
    )
  })

  it('creates a user', async () => {
    const users: Array<{
      id: string
      username: string
      display_name: string | null
      is_active: boolean
      created_at: string
    }> = []

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/users') && method === 'GET') {
          return new Response(JSON.stringify(users), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        if (url.endsWith('/api/v1/users') && method === 'POST') {
          const body = JSON.parse(String(init?.body ?? '{}')) as {
            username?: string
            display_name?: string | null
          }
          const created = {
            id: 'u1',
            username: body.username ?? '',
            display_name: body.display_name ?? null,
            is_active: true,
            created_at: '2025-01-01T00:00:00Z',
          }
          users.push(created)
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

    await user.type(screen.getByPlaceholderText(/^username$/i), 'shannon')
    await user.type(screen.getByPlaceholderText(/^password$/i), 'pw')
    await user.type(screen.getByPlaceholderText(/display name/i), 'Shannon')
    await user.click(screen.getByRole('button', { name: /create user/i }))

    await waitFor(() => expect(screen.getByText('shannon')).toBeVisible())
  })
})
