import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ShoppingListPage } from './ShoppingListPage'

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <ShoppingListPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('ShoppingListPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('creates a shopping list', async () => {
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()
        if (url.includes('/api/v1/shopping-lists') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (url.endsWith('/api/v1/shopping-lists') && method === 'POST') {
          const body = JSON.parse(String(init?.body ?? '{}')) as {
            name?: string
          }
          if (body.name !== 'Weekly Shop') {
            return new Response(null, { status: 400 })
          }
          return new Response(
            JSON.stringify({
              id: 'list-1',
              list_date: '2025-02-10',
              name: 'Weekly Shop',
              notes: null,
              created_at: '2025-02-01T00:00:00Z',
              updated_at: '2025-02-01T00:00:00Z',
            }),
            { status: 201, headers: { 'content-type': 'application/json' } },
          )
        }
        return new Response(null, { status: 404 })
      },
    )
    vi.stubGlobal('fetch', fetchMock)

    const user = userEvent.setup()
    renderPage()

    await user.clear(screen.getByLabelText(/date/i))
    await user.type(screen.getByLabelText(/date/i), '2025-02-10')
    await user.type(screen.getByLabelText(/name/i), 'Weekly Shop')
    await user.click(screen.getByRole('button', { name: /create list/i }))

    expect(
      fetchMock.mock.calls.some(([arg, init]) => {
        const url = typeof arg === 'string' ? arg : arg.toString()
        const method = String(
          (init as RequestInit | undefined)?.method ?? 'GET',
        )
        return url.endsWith('/api/v1/shopping-lists') && method === 'POST'
      }),
    ).toBe(true)
  })

  it('updates a shopping list', async () => {
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()
        if (url.includes('/api/v1/shopping-lists') && method === 'GET') {
          return new Response(
            JSON.stringify([
              {
                id: 'list-1',
                list_date: '2025-02-10',
                name: 'Weekly Shop',
                notes: null,
                created_at: '2025-02-01T00:00:00Z',
                updated_at: '2025-02-01T00:00:00Z',
              },
            ]),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        if (url.endsWith('/api/v1/shopping-lists/list-1') && method === 'PUT') {
          return new Response(
            JSON.stringify({
              id: 'list-1',
              list_date: '2025-02-10',
              name: 'Updated Shop',
              notes: null,
              created_at: '2025-02-01T00:00:00Z',
              updated_at: '2025-02-02T00:00:00Z',
            }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        if (url.endsWith('/api/v1/shopping-lists') && method === 'POST') {
          return new Response(null, { status: 400 })
        }
        return new Response(null, { status: 404 })
      },
    )
    vi.stubGlobal('fetch', fetchMock)

    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByRole('button', { name: /edit/i }))
    const nameInput = screen.getByDisplayValue(/weekly shop/i)
    await user.clear(nameInput)
    await user.type(nameInput, 'Updated Shop')
    await user.click(screen.getByRole('button', { name: /save/i }))

    expect(
      fetchMock.mock.calls.some(([arg, init]) => {
        const url = typeof arg === 'string' ? arg : arg.toString()
        const method = String(
          (init as RequestInit | undefined)?.method ?? 'GET',
        )
        return url.endsWith('/api/v1/shopping-lists/list-1') && method === 'PUT'
      }),
    ).toBe(true)
  })
})
