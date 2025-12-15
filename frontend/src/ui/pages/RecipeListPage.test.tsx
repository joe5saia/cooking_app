import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'

import { RecipeListPage } from './RecipeListPage'

function renderPage(initialEntry = '/recipes') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/recipes" element={<RecipeListPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('RecipeListPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('renders list items and supports search', async () => {
    const recipes = [
      {
        id: 'r1',
        title: 'Chicken Soup',
        servings: 4,
        prep_time_minutes: 10,
        total_time_minutes: 30,
        source_url: null,
        notes: null,
        recipe_book_id: 'b1',
        tags: [{ id: 't1', name: 'Soup' }],
        deleted_at: null,
        updated_at: '2025-01-01T00:00:00Z',
      },
      {
        id: 'r2',
        title: 'Beef Stew',
        servings: 6,
        prep_time_minutes: 20,
        total_time_minutes: 90,
        source_url: null,
        notes: null,
        recipe_book_id: 'b2',
        tags: [{ id: 't2', name: 'Beef' }],
        deleted_at: null,
        updated_at: '2025-01-02T00:00:00Z',
      },
    ]

    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString()
      const parsed = new URL(url, 'http://example.test')

      if (parsed.pathname === '/api/v1/recipe-books') {
        return new Response(
          JSON.stringify([
            { id: 'b1', name: 'Dinner', created_at: '2025-01-01T00:00:00Z' },
            { id: 'b2', name: 'Lunch', created_at: '2025-01-01T00:00:00Z' },
          ]),
          { status: 200, headers: { 'content-type': 'application/json' } },
        )
      }

      if (parsed.pathname === '/api/v1/tags') {
        return new Response(
          JSON.stringify([
            { id: 't1', name: 'Soup', created_at: '2025-01-01T00:00:00Z' },
            { id: 't2', name: 'Beef', created_at: '2025-01-01T00:00:00Z' },
          ]),
          { status: 200, headers: { 'content-type': 'application/json' } },
        )
      }

      if (parsed.pathname === '/api/v1/recipes') {
        const q = (parsed.searchParams.get('q') ?? '').toLowerCase()
        const items = q
          ? recipes.filter((r) => r.title.toLowerCase().includes(q))
          : recipes
        return new Response(JSON.stringify({ items, next_cursor: null }), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        })
      }

      return new Response(null, { status: 404 })
    })
    vi.stubGlobal('fetch', fetchMock)

    const user = userEvent.setup()
    renderPage()

    expect(
      await screen.findByRole('link', { name: /create recipe/i }),
    ).toBeVisible()
    expect(await screen.findByText('Beef Stew')).toBeVisible()
    expect(await screen.findByText('Chicken Soup')).toBeVisible()
    expect(
      await screen.findByText(
        /serves 4.*prep 10 min.*total 30 min.*updated 2025-01-01t00:00:00z/i,
      ),
    ).toBeVisible()
    expect(await screen.findByRole('link', { name: /^dinner$/i })).toBeVisible()
    expect(await screen.findByRole('link', { name: /^soup$/i })).toBeVisible()

    await user.type(screen.getByPlaceholderText(/search recipes/i), 'chicken')

    await waitFor(() => {
      expect(screen.getByText('Chicken Soup')).toBeVisible()
      expect(screen.queryByText('Beef Stew')).toBeNull()
    })

    await waitFor(() => {
      expect(
        fetchMock.mock.calls.some(([arg]) => {
          const u = typeof arg === 'string' ? arg : arg.toString()
          return u.includes('/api/v1/recipes') && u.includes('q=chicken')
        }),
      ).toBe(true)
    })
  })

  it('shows empty state', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = typeof input === 'string' ? input : input.toString()
        const parsed = new URL(url, 'http://example.test')

        if (parsed.pathname === '/api/v1/recipe-books') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (parsed.pathname === '/api/v1/tags') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (parsed.pathname === '/api/v1/recipes') {
          return new Response(
            JSON.stringify({ items: [], next_cursor: null }),
            {
              status: 200,
              headers: { 'content-type': 'application/json' },
            },
          )
        }
        return new Response(null, { status: 404 })
      }),
    )

    renderPage()

    expect(await screen.findByText(/no recipes yet/i)).toBeVisible()
  })

  it('shows API error message on list failure', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = typeof input === 'string' ? input : input.toString()
        const parsed = new URL(url, 'http://example.test')

        if (parsed.pathname === '/api/v1/recipe-books') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (parsed.pathname === '/api/v1/tags') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (parsed.pathname === '/api/v1/recipes') {
          return new Response(
            JSON.stringify({ code: 'internal_error', message: 'boom' }),
            { status: 500, headers: { 'content-type': 'application/json' } },
          )
        }
        return new Response(null, { status: 404 })
      }),
    )

    renderPage()

    expect(await screen.findByRole('alert')).toHaveTextContent('boom')
  })

  it('can include deleted recipes and marks them', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString()
      const parsed = new URL(url, 'http://example.test')

      if (parsed.pathname === '/api/v1/recipe-books') {
        return new Response(JSON.stringify([]), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        })
      }
      if (parsed.pathname === '/api/v1/tags') {
        return new Response(JSON.stringify([]), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        })
      }
      if (parsed.pathname === '/api/v1/recipes') {
        const includeDeleted =
          parsed.searchParams.get('include_deleted') === 'true'
        const items = includeDeleted
          ? [
              {
                id: 'r1',
                title: 'Deleted Recipe',
                servings: 1,
                prep_time_minutes: 0,
                total_time_minutes: 0,
                source_url: null,
                notes: null,
                recipe_book_id: null,
                tags: [],
                deleted_at: '2025-01-03T00:00:00Z',
                updated_at: '2025-01-03T00:00:00Z',
              },
            ]
          : []
        return new Response(JSON.stringify({ items, next_cursor: null }), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        })
      }

      return new Response(null, { status: 404 })
    })
    vi.stubGlobal('fetch', fetchMock)

    const user = userEvent.setup()
    renderPage()

    expect(
      await screen.findByRole('checkbox', { name: /include deleted/i }),
    ).toBeVisible()
    expect(await screen.findByText(/no recipes yet/i)).toBeVisible()

    await user.click(screen.getByRole('checkbox', { name: /include deleted/i }))

    expect(await screen.findByText('Deleted Recipe')).toBeVisible()
    expect(await screen.findByText('Deleted')).toBeVisible()
    expect(screen.queryByRole('link', { name: /^edit$/i })).toBeNull()
    expect(screen.getByRole('button', { name: /^restore$/i })).toBeVisible()

    await waitFor(() => {
      expect(
        fetchMock.mock.calls.some(([arg]) => {
          const u = typeof arg === 'string' ? arg : arg.toString()
          return (
            u.includes('/api/v1/recipes') && u.includes('include_deleted=true')
          )
        }),
      ).toBe(true)
    })
  })

  it('restores deleted recipe from list', async () => {
    let restored = false

    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const parsed = new URL(url, 'http://example.test')
        const method = String(init?.method ?? 'GET').toUpperCase()

        if (parsed.pathname === '/api/v1/recipe-books') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (parsed.pathname === '/api/v1/tags') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        if (
          parsed.pathname === '/api/v1/recipes/r1/restore' &&
          method === 'PUT'
        ) {
          await new Promise((r) => setTimeout(r, 250))
          restored = true
          return new Response(null, { status: 204 })
        }

        if (parsed.pathname === '/api/v1/recipes' && method === 'GET') {
          const includeDeleted =
            parsed.searchParams.get('include_deleted') === 'true'
          if (!includeDeleted) {
            return new Response(
              JSON.stringify({ items: [], next_cursor: null }),
              {
                status: 200,
                headers: { 'content-type': 'application/json' },
              },
            )
          }

          const items = restored
            ? [
                {
                  id: 'r1',
                  title: 'Restored Recipe',
                  servings: 1,
                  prep_time_minutes: 0,
                  total_time_minutes: 0,
                  source_url: null,
                  notes: null,
                  recipe_book_id: null,
                  tags: [],
                  deleted_at: null,
                  updated_at: '2025-01-03T00:00:00Z',
                },
              ]
            : [
                {
                  id: 'r1',
                  title: 'Deleted Recipe',
                  servings: 1,
                  prep_time_minutes: 0,
                  total_time_minutes: 0,
                  source_url: null,
                  notes: null,
                  recipe_book_id: null,
                  tags: [],
                  deleted_at: '2025-01-03T00:00:00Z',
                  updated_at: '2025-01-03T00:00:00Z',
                },
              ]

          return new Response(JSON.stringify({ items, next_cursor: null }), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        return new Response(null, { status: 404 })
      },
    )
    vi.stubGlobal('fetch', fetchMock)

    const user = userEvent.setup()
    renderPage('/recipes?include_deleted=true')

    expect(await screen.findByText('Deleted Recipe')).toBeVisible()
    await user.click(screen.getByRole('button', { name: /^restore$/i }))
    expect(
      await screen.findByRole('button', { name: /restoring/i }),
    ).toBeDisabled()

    await waitFor(() => expect(restored).toBe(true))
    expect(await screen.findByText('Restored Recipe')).toBeVisible()
    expect(screen.queryByText('Deleted')).toBeNull()
    expect(screen.getByRole('link', { name: /^edit$/i })).toBeVisible()
  })

  it('shows error when restore fails', async () => {
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const parsed = new URL(url, 'http://example.test')
        const method = String(init?.method ?? 'GET').toUpperCase()

        if (parsed.pathname === '/api/v1/recipe-books') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (parsed.pathname === '/api/v1/tags') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        if (
          parsed.pathname === '/api/v1/recipes/r1/restore' &&
          method === 'PUT'
        ) {
          return new Response(
            JSON.stringify({ code: 'internal_error', message: 'boom' }),
            { status: 500, headers: { 'content-type': 'application/json' } },
          )
        }

        if (parsed.pathname === '/api/v1/recipes' && method === 'GET') {
          const includeDeleted =
            parsed.searchParams.get('include_deleted') === 'true'
          const items = includeDeleted
            ? [
                {
                  id: 'r1',
                  title: 'Deleted Recipe',
                  servings: 1,
                  prep_time_minutes: 0,
                  total_time_minutes: 0,
                  source_url: null,
                  notes: null,
                  recipe_book_id: null,
                  tags: [],
                  deleted_at: '2025-01-03T00:00:00Z',
                  updated_at: '2025-01-03T00:00:00Z',
                },
              ]
            : []

          return new Response(JSON.stringify({ items, next_cursor: null }), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        return new Response(null, { status: 404 })
      },
    )
    vi.stubGlobal('fetch', fetchMock)

    const user = userEvent.setup()
    renderPage('/recipes?include_deleted=true')

    expect(await screen.findByText('Deleted Recipe')).toBeVisible()
    await user.click(screen.getByRole('button', { name: /^restore$/i }))

    expect(await screen.findByRole('alert')).toHaveTextContent('boom')

    await user.click(screen.getByRole('button', { name: /dismiss/i }))
    expect(screen.queryByText('boom')).toBeNull()
  })

  it('initializes filters from URL querystring', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString()
      const parsed = new URL(url, 'http://example.test')

      if (parsed.pathname === '/api/v1/recipe-books') {
        return new Response(
          JSON.stringify([
            { id: 'b1', name: 'Dinner', created_at: '2025-01-01T00:00:00Z' },
          ]),
          { status: 200, headers: { 'content-type': 'application/json' } },
        )
      }

      if (parsed.pathname === '/api/v1/tags') {
        return new Response(
          JSON.stringify([
            { id: 't1', name: 'Soup', created_at: '2025-01-01T00:00:00Z' },
          ]),
          { status: 200, headers: { 'content-type': 'application/json' } },
        )
      }

      if (parsed.pathname === '/api/v1/recipes') {
        return new Response(
          JSON.stringify({
            items: [
              {
                id: 'r1',
                title: 'Chicken Soup',
                servings: 4,
                prep_time_minutes: 10,
                total_time_minutes: 30,
                source_url: null,
                notes: null,
                recipe_book_id: 'b1',
                tags: [{ id: 't1', name: 'Soup' }],
                deleted_at: null,
                updated_at: '2025-01-01T00:00:00Z',
              },
            ],
            next_cursor: null,
          }),
          { status: 200, headers: { 'content-type': 'application/json' } },
        )
      }

      return new Response(null, { status: 404 })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage('/recipes?q=chicken&book_id=b1&tag_id=t1&include_deleted=true')

    expect(await screen.findByText('Chicken Soup')).toBeVisible()
    expect(
      await screen.findByText(
        /serves 4.*prep 10 min.*total 30 min.*updated 2025-01-01t00:00:00z/i,
      ),
    ).toBeVisible()
    expect(await screen.findByRole('link', { name: /^dinner$/i })).toBeVisible()
    expect(await screen.findByRole('link', { name: /^soup$/i })).toBeVisible()
    expect(screen.getByPlaceholderText(/search recipes/i)).toHaveValue(
      'chicken',
    )
    expect(screen.getByLabelText(/filter by book/i)).toHaveValue('b1')
    expect(screen.getByLabelText(/filter by tag/i)).toHaveValue('t1')
    expect(
      screen.getByRole('checkbox', { name: /include deleted/i }),
    ).toBeChecked()

    await waitFor(() => {
      expect(
        fetchMock.mock.calls.some(([arg]) => {
          const u = typeof arg === 'string' ? arg : arg.toString()
          return (
            u.includes('/api/v1/recipes') &&
            u.includes('q=chicken') &&
            u.includes('book_id=b1') &&
            u.includes('tag_id=t1') &&
            u.includes('include_deleted=true')
          )
        }),
      ).toBe(true)
    })
  })

  it('clears filters', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString()
      const parsed = new URL(url, 'http://example.test')

      if (parsed.pathname === '/api/v1/recipe-books') {
        return new Response(JSON.stringify([]), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        })
      }

      if (parsed.pathname === '/api/v1/tags') {
        return new Response(JSON.stringify([]), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        })
      }

      if (parsed.pathname === '/api/v1/recipes') {
        return new Response(JSON.stringify({ items: [], next_cursor: null }), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        })
      }

      return new Response(null, { status: 404 })
    })
    vi.stubGlobal('fetch', fetchMock)

    const user = userEvent.setup()
    renderPage('/recipes?q=chicken&include_deleted=true')

    expect(
      await screen.findByRole('button', { name: /clear filters/i }),
    ).toBeVisible()
    expect(screen.getByPlaceholderText(/search recipes/i)).toHaveValue(
      'chicken',
    )
    expect(
      screen.getByRole('checkbox', { name: /include deleted/i }),
    ).toBeChecked()

    await user.click(screen.getByRole('button', { name: /clear filters/i }))

    expect(screen.getByPlaceholderText(/search recipes/i)).toHaveValue('')
    expect(
      screen.getByRole('checkbox', { name: /include deleted/i }),
    ).not.toBeChecked()

    await waitFor(() => {
      expect(
        fetchMock.mock.calls.some(([arg]) => {
          const u = typeof arg === 'string' ? arg : arg.toString()
          const parsed = new URL(u, 'http://example.test')
          return (
            parsed.pathname === '/api/v1/recipes' &&
            !parsed.searchParams.has('q') &&
            !parsed.searchParams.has('include_deleted') &&
            parsed.searchParams.get('limit') === '25'
          )
        }),
      ).toBe(true)
    })

    await waitFor(() => {
      expect(
        fetchMock.mock.calls.some(([arg]) => {
          const u = typeof arg === 'string' ? arg : arg.toString()
          return u.includes('/api/v1/recipes?') && u.includes('q=chicken')
        }),
      ).toBe(true)
    })
  })

  it('preserves include_deleted on book/tag links when enabled', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input.toString()
      const parsed = new URL(url, 'http://example.test')

      if (parsed.pathname === '/api/v1/recipe-books') {
        return new Response(
          JSON.stringify([
            { id: 'b1', name: 'Dinner', created_at: '2025-01-01T00:00:00Z' },
          ]),
          { status: 200, headers: { 'content-type': 'application/json' } },
        )
      }

      if (parsed.pathname === '/api/v1/tags') {
        return new Response(
          JSON.stringify([
            { id: 't1', name: 'Soup', created_at: '2025-01-01T00:00:00Z' },
          ]),
          { status: 200, headers: { 'content-type': 'application/json' } },
        )
      }

      if (parsed.pathname === '/api/v1/recipes') {
        return new Response(
          JSON.stringify({
            items: [
              {
                id: 'r1',
                title: 'Deleted Recipe',
                servings: 1,
                prep_time_minutes: 0,
                total_time_minutes: 0,
                source_url: null,
                notes: null,
                recipe_book_id: 'b1',
                tags: [{ id: 't1', name: 'Soup' }],
                deleted_at: '2025-01-03T00:00:00Z',
                updated_at: '2025-01-03T00:00:00Z',
              },
            ],
            next_cursor: null,
          }),
          { status: 200, headers: { 'content-type': 'application/json' } },
        )
      }

      return new Response(null, { status: 404 })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderPage('/recipes?include_deleted=true')

    const bookLink = await screen.findByRole('link', { name: /^dinner$/i })
    expect(bookLink).toHaveAttribute(
      'href',
      '/recipes?book_id=b1&include_deleted=true',
    )

    const tagLink = await screen.findByRole('link', { name: /^soup$/i })
    expect(tagLink).toHaveAttribute(
      'href',
      '/recipes?tag_id=t1&include_deleted=true',
    )
  })
})
