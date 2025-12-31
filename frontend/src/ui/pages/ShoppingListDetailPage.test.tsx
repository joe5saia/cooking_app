import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ShoppingListDetailPage } from './ShoppingListDetailPage'

function renderPage(initialPath: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route
            path="/shopping-lists/:id"
            element={<ShoppingListDetailPage />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('ShoppingListDetailPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('adds a manual item to the list', async () => {
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()
        if (url.includes('/api/v1/items') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (url.endsWith('/api/v1/items') && method === 'POST') {
          return new Response(
            JSON.stringify({
              id: 'item-1',
              name: 'Milk',
              store_url: null,
              aisle: null,
            }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        if (url.endsWith('/api/v1/shopping-lists/list-1') && method === 'GET') {
          return new Response(
            JSON.stringify({
              id: 'list-1',
              list_date: '2025-02-10',
              name: 'Weekly Shop',
              notes: null,
              items: [],
              created_at: '2025-02-01T00:00:00Z',
              updated_at: '2025-02-01T00:00:00Z',
            }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        if (
          url.endsWith('/api/v1/shopping-lists/list-1/items') &&
          method === 'POST'
        ) {
          return new Response(
            JSON.stringify([
              {
                id: 'list-item-1',
                item: {
                  id: 'item-1',
                  name: 'Milk',
                  store_url: null,
                  aisle: null,
                },
                quantity: 2,
                quantity_text: null,
                unit: 'lb',
                is_purchased: false,
                purchased_at: null,
              },
            ]),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        return new Response(null, { status: 404 })
      },
    )
    vi.stubGlobal('fetch', fetchMock)

    const user = userEvent.setup()
    renderPage('/shopping-lists/list-1')

    expect(
      await screen.findByRole('heading', { name: /weekly shop/i, level: 1 }),
    ).toBeVisible()

    await user.type(screen.getByLabelText(/^item$/i), 'Milk')
    await user.type(screen.getByLabelText(/quantity$/i), '2')
    await user.type(screen.getByLabelText(/unit/i), 'lb')
    await user.click(screen.getByRole('button', { name: /add to list/i }))

    await waitFor(() =>
      expect(
        fetchMock.mock.calls.some(([arg, init]) => {
          const url = typeof arg === 'string' ? arg : arg.toString()
          const method = String(
            (init as RequestInit | undefined)?.method ?? 'GET',
          )
          return (
            url.endsWith('/api/v1/shopping-lists/list-1/items') &&
            method === 'POST'
          )
        }),
      ).toBe(true),
    )
  })
})
