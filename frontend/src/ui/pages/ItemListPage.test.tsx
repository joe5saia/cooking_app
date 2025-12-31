import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ItemListPage } from './ItemListPage'

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <ItemListPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('ItemListPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('creates a new item', async () => {
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()
        if (url.endsWith('/api/v1/aisles') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (url.includes('/api/v1/items') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (url.endsWith('/api/v1/items') && method === 'POST') {
          const body = JSON.parse(String(init?.body ?? '{}')) as {
            name?: string
          }
          if (body.name !== 'Milk') {
            return new Response(null, { status: 400 })
          }
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
        return new Response(null, { status: 404 })
      },
    )
    vi.stubGlobal('fetch', fetchMock)

    const user = userEvent.setup()
    renderPage()

    await user.type(screen.getByLabelText(/item name/i), 'Milk')
    await user.click(screen.getByRole('button', { name: /add item/i }))

    expect(
      fetchMock.mock.calls.some(([arg, init]) => {
        const url = typeof arg === 'string' ? arg : arg.toString()
        const method = String(
          (init as RequestInit | undefined)?.method ?? 'GET',
        )
        return url.endsWith('/api/v1/items') && method === 'POST'
      }),
    ).toBe(true)
  })

  it('creates a new aisle', async () => {
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()
        if (url.endsWith('/api/v1/aisles') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (url.includes('/api/v1/items') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (url.endsWith('/api/v1/aisles') && method === 'POST') {
          const body = JSON.parse(String(init?.body ?? '{}')) as {
            name?: string
          }
          if (body.name !== 'Bakery') {
            return new Response(null, { status: 400 })
          }
          return new Response(
            JSON.stringify({
              id: 'aisle-1',
              name: 'Bakery',
              sort_group: 0,
              sort_order: 0,
              numeric_value: null,
            }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        return new Response(null, { status: 404 })
      },
    )
    vi.stubGlobal('fetch', fetchMock)

    const user = userEvent.setup()
    renderPage()

    await user.type(screen.getByLabelText(/aisle name/i), 'Bakery')
    await user.click(screen.getByRole('button', { name: /add aisle/i }))

    expect(
      fetchMock.mock.calls.some(([arg, init]) => {
        const url = typeof arg === 'string' ? arg : arg.toString()
        const method = String(
          (init as RequestInit | undefined)?.method ?? 'GET',
        )
        return url.endsWith('/api/v1/aisles') && method === 'POST'
      }),
    ).toBe(true)
  })
})
