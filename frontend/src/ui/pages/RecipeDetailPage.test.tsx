import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'

import { RecipeDetailPage } from './RecipeDetailPage'

function renderPage(initialPath: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/recipes" element={<div>Recipes list</div>} />
          <Route path="/recipes/:id" element={<RecipeDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('RecipeDetailPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('shows loading then renders recipe', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/recipe-books') && method === 'GET') {
          return new Response(
            JSON.stringify([
              { id: 'b1', name: 'Dinner', created_at: '2025-01-01T00:00:00Z' },
            ]),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }

        if (url.includes('/api/v1/shopping-lists') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        if (url.endsWith('/api/v1/recipes/r1') && method === 'GET') {
          await Promise.resolve()
          return new Response(
            JSON.stringify({
              id: 'r1',
              title: 'Chicken Soup',
              servings: 4,
              prep_time_minutes: 10,
              total_time_minutes: 30,
              source_url: null,
              notes: 'Line1\nLine2',
              recipe_book_id: 'b1',
              tags: [{ id: 't1', name: 'Soup' }],
              ingredients: [
                {
                  id: 'i1',
                  position: 1,
                  quantity: 1,
                  quantity_text: '1',
                  unit: 'lb',
                  item: {
                    id: 'item-1',
                    name: 'chicken',
                    store_url: null,
                    aisle: null,
                  },
                  prep: null,
                  notes: 'Organic',
                  original_text: '1 lb chicken',
                },
              ],
              steps: [{ id: 's1', step_number: 1, instruction: 'Boil.' }],
              created_at: '2025-01-01T00:00:00Z',
              created_by: 'u1',
              updated_at: '2025-01-01T00:00:00Z',
              updated_by: 'u1',
              deleted_at: null,
            }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }

        return new Response(null, { status: 404 })
      }),
    )

    renderPage('/recipes/r1')
    expect(screen.getByText(/loading/i)).toBeVisible()

    expect(await screen.findByText('Chicken Soup')).toBeVisible()
    expect(
      await screen.findByRole('heading', { name: /ingredients/i, level: 3 }),
    ).toBeVisible()
    expect(
      await screen.findByText(/1 lb chicken — organic \[1 lb chicken\]/i),
    ).toBeVisible()
    expect(await screen.findByText(/boil\./i)).toBeVisible()
    expect(await screen.findByRole('link', { name: /dinner/i })).toBeVisible()
    expect(
      await screen.findByRole('link', { name: /back to recipes/i }),
    ).toBeVisible()
    expect(await screen.findByRole('link', { name: /soup/i })).toBeVisible()
    expect(await screen.findByText(/created:/i)).toBeVisible()
    expect(await screen.findByText(/updated:/i)).toBeVisible()
    const notes = await screen.findByText(/line1/i)
    expect(notes.textContent).toContain('Line1\nLine2')
  })

  it('preserves include_deleted on book/tag links for deleted recipe', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/recipe-books') && method === 'GET') {
          return new Response(
            JSON.stringify([
              { id: 'b1', name: 'Dinner', created_at: '2025-01-01T00:00:00Z' },
            ]),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }

        if (url.includes('/api/v1/shopping-lists') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        if (url.endsWith('/api/v1/recipes/r1') && method === 'GET') {
          return new Response(
            JSON.stringify({
              id: 'r1',
              title: 'Deleted Recipe',
              servings: 1,
              prep_time_minutes: 0,
              total_time_minutes: 0,
              source_url: null,
              notes: null,
              recipe_book_id: 'b1',
              tags: [{ id: 't1', name: 'Soup' }],
              ingredients: [],
              steps: [{ id: 's1', step_number: 1, instruction: 'Boil.' }],
              created_at: '2025-01-01T00:00:00Z',
              created_by: 'u1',
              updated_at: '2025-01-01T00:00:00Z',
              updated_by: 'u1',
              deleted_at: '2025-01-02T00:00:00Z',
            }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }

        return new Response(null, { status: 404 })
      }),
    )

    renderPage('/recipes/r1')

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

  it('handles 404', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = typeof input === 'string' ? input : input.toString()
        if (url.endsWith('/api/v1/recipes/missing')) {
          return new Response(
            JSON.stringify({ code: 'not_found', message: 'not found' }),
            { status: 404, headers: { 'content-type': 'application/json' } },
          )
        }
        if (url.includes('/api/v1/shopping-lists')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        return new Response(null, { status: 404 })
      }),
    )

    renderPage('/recipes/missing')
    expect(await screen.findByText(/recipe not found/i)).toBeVisible()
  })

  it('shows API error message on non-404 load failure', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = typeof input === 'string' ? input : input.toString()
        if (url.endsWith('/api/v1/recipes/r1')) {
          return new Response(
            JSON.stringify({ code: 'internal_error', message: 'boom' }),
            { status: 500, headers: { 'content-type': 'application/json' } },
          )
        }
        if (url.includes('/api/v1/shopping-lists')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        return new Response(null, { status: 404 })
      }),
    )

    renderPage('/recipes/r1')
    expect(await screen.findByRole('alert')).toHaveTextContent('boom')
  })

  it('deletes and navigates to list', async () => {
    let deleted = false

    vi.spyOn(window, 'confirm').mockReturnValue(true)

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/recipes/r1') && method === 'GET') {
          return new Response(
            JSON.stringify({
              id: 'r1',
              title: 'Chicken Soup',
              servings: 4,
              prep_time_minutes: 10,
              total_time_minutes: 30,
              source_url: null,
              notes: null,
              recipe_book_id: null,
              tags: [],
              ingredients: [],
              steps: [],
              created_at: '2025-01-01T00:00:00Z',
              created_by: 'u1',
              updated_at: '2025-01-01T00:00:00Z',
              updated_by: 'u1',
              deleted_at: null,
            }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }

        if (url.endsWith('/api/v1/recipes/r1') && method === 'DELETE') {
          await new Promise((r) => setTimeout(r, 25))
          deleted = true
          return new Response(null, { status: 204 })
        }

        if (url.includes('/api/v1/recipes') && method === 'GET') {
          return new Response(
            JSON.stringify({ items: [], next_cursor: null }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        if (url.includes('/api/v1/shopping-lists') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        return new Response(null, { status: 404 })
      }),
    )

    const user = userEvent.setup()
    renderPage('/recipes/r1')

    expect(await screen.findByText('Chicken Soup')).toBeVisible()
    await user.click(screen.getByRole('button', { name: /delete/i }))
    expect(
      await screen.findByRole('button', { name: /deleting/i }),
    ).toBeDisabled()

    await waitFor(() => expect(deleted).toBe(true))
    expect(await screen.findByText('Recipes list')).toBeVisible()
  })

  it('does not delete when confirmation is canceled', async () => {
    let deleted = false

    vi.spyOn(window, 'confirm').mockReturnValue(false)

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/recipes/r1') && method === 'GET') {
          return new Response(
            JSON.stringify({
              id: 'r1',
              title: 'Chicken Soup',
              servings: 4,
              prep_time_minutes: 10,
              total_time_minutes: 30,
              source_url: null,
              notes: null,
              recipe_book_id: null,
              tags: [],
              ingredients: [],
              steps: [],
              created_at: '2025-01-01T00:00:00Z',
              created_by: 'u1',
              updated_at: '2025-01-01T00:00:00Z',
              updated_by: 'u1',
              deleted_at: null,
            }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }

        if (url.endsWith('/api/v1/recipes/r1') && method === 'DELETE') {
          deleted = true
          return new Response(null, { status: 204 })
        }

        if (url.includes('/api/v1/recipes') && method === 'GET') {
          return new Response(
            JSON.stringify({ items: [], next_cursor: null }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }

        if (url.includes('/api/v1/shopping-lists') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        return new Response(null, { status: 404 })
      }),
    )

    const user = userEvent.setup()
    renderPage('/recipes/r1')

    expect(await screen.findByText('Chicken Soup')).toBeVisible()
    await user.click(screen.getByRole('button', { name: /delete/i }))

    await waitFor(() => {
      expect(window.confirm).toHaveBeenCalled()
    })
    expect(deleted).toBe(false)
  })

  it('shows error when delete fails', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/recipes/r1') && method === 'GET') {
          return new Response(
            JSON.stringify({
              id: 'r1',
              title: 'Chicken Soup',
              servings: 4,
              prep_time_minutes: 10,
              total_time_minutes: 30,
              source_url: null,
              notes: null,
              recipe_book_id: null,
              tags: [],
              ingredients: [],
              steps: [],
              created_at: '2025-01-01T00:00:00Z',
              created_by: 'u1',
              updated_at: '2025-01-01T00:00:00Z',
              updated_by: 'u1',
              deleted_at: null,
            }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }

        if (url.endsWith('/api/v1/recipes/r1') && method === 'DELETE') {
          return new Response(
            JSON.stringify({ code: 'internal_error', message: 'boom' }),
            { status: 500, headers: { 'content-type': 'application/json' } },
          )
        }

        if (url.includes('/api/v1/recipes') && method === 'GET') {
          return new Response(
            JSON.stringify({ items: [], next_cursor: null }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }

        if (url.includes('/api/v1/shopping-lists') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        return new Response(null, { status: 404 })
      }),
    )

    const user = userEvent.setup()
    renderPage('/recipes/r1')

    expect(await screen.findByText('Chicken Soup')).toBeVisible()
    await user.click(screen.getByRole('button', { name: /delete/i }))

    expect(await screen.findByRole('alert')).toHaveTextContent('boom')
    expect(screen.queryByText('Recipes list')).toBeNull()
  })

  it('shows error when restore fails', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/recipes/r1') && method === 'GET') {
          return new Response(
            JSON.stringify({
              id: 'r1',
              title: 'Deleted Recipe',
              servings: 1,
              prep_time_minutes: 0,
              total_time_minutes: 0,
              source_url: null,
              notes: null,
              recipe_book_id: null,
              tags: [],
              ingredients: [],
              steps: [],
              created_at: '2025-01-01T00:00:00Z',
              created_by: 'u1',
              updated_at: '2025-01-01T00:00:00Z',
              updated_by: 'u1',
              deleted_at: '2025-01-02T00:00:00Z',
            }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }

        if (url.endsWith('/api/v1/recipes/r1/restore') && method === 'PUT') {
          await new Promise((r) => setTimeout(r, 25))
          return new Response(
            JSON.stringify({ code: 'internal_error', message: 'boom' }),
            { status: 500, headers: { 'content-type': 'application/json' } },
          )
        }

        if (url.includes('/api/v1/shopping-lists') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        return new Response(null, { status: 404 })
      }),
    )

    const user = userEvent.setup()
    renderPage('/recipes/r1')

    expect(await screen.findByText('Deleted Recipe')).toBeVisible()
    await user.click(screen.getByRole('button', { name: /restore/i }))

    expect(await screen.findByText('boom')).toBeVisible()
    await user.click(screen.getByRole('button', { name: /dismiss/i }))
    expect(screen.queryByText('boom')).toBeNull()
  })

  it('does not offer edit for deleted recipe', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/recipes/r1') && method === 'GET') {
          return new Response(
            JSON.stringify({
              id: 'r1',
              title: 'Deleted Recipe',
              servings: 1,
              prep_time_minutes: 0,
              total_time_minutes: 0,
              source_url: null,
              notes: null,
              recipe_book_id: null,
              tags: [],
              ingredients: [],
              steps: [],
              created_at: '2025-01-01T00:00:00Z',
              created_by: 'u1',
              updated_at: '2025-01-01T00:00:00Z',
              updated_by: 'u1',
              deleted_at: '2025-01-02T00:00:00Z',
            }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }

        return new Response(null, { status: 404 })
      }),
    )

    renderPage('/recipes/r1')

    expect(await screen.findByText(/this recipe is deleted/i)).toBeVisible()
    expect(await screen.findByText(/deleted:/i)).toBeVisible()
    expect(screen.queryByRole('link', { name: /edit/i })).toBeNull()
    expect(screen.getByRole('button', { name: /restore/i })).toBeVisible()
  })
})
