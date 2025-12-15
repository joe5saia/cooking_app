import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter, Route, Routes } from 'react-router-dom'

import { RecipeEditorPage } from './RecipeEditorPage'

function renderPage(initialPath: string, mode: 'create' | 'edit') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>
          <Route path="/recipes" element={<div>Recipes list</div>} />
          <Route
            path="/recipes/new"
            element={<RecipeEditorPage mode={mode} />}
          />
          <Route
            path="/recipes/:id/edit"
            element={<RecipeEditorPage mode={mode} />}
          />
          <Route path="/recipes/:id" element={<div>Recipe detail</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('RecipeEditorPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('validates required fields', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = typeof input === 'string' ? input : input.toString()
        if (url.endsWith('/api/v1/recipe-books')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (url.endsWith('/api/v1/tags')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        return new Response(null, { status: 404 })
      }),
    )

    const user = userEvent.setup()
    renderPage('/recipes/new', 'create')

    await user.click(screen.getByRole('button', { name: /create/i }))

    expect(await screen.findByText(/title is required/i)).toBeVisible()
    expect(await screen.findByText(/instruction is required/i)).toBeVisible()
  })

  it('submits create and navigates', async () => {
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
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

        if (url.endsWith('/api/v1/tags') && method === 'GET') {
          return new Response(
            JSON.stringify([
              { id: 't1', name: 'Soup', created_at: '2025-01-01T00:00:00Z' },
            ]),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }

        if (url.endsWith('/api/v1/recipes') && method === 'POST') {
          const body = JSON.parse(String(init?.body ?? '{}')) as {
            title?: string
            steps?: Array<{ step_number: number; instruction: string }>
            ingredients?: Array<{
              notes: string | null
              original_text: string | null
            }>
          }
          if (body.title !== 'Chicken Soup') {
            return new Response(null, { status: 400 })
          }
          if (!body.steps?.length || body.steps[0]?.step_number !== 1) {
            return new Response(null, { status: 400 })
          }
          if (body.ingredients?.[0]?.notes !== 'Organic') {
            return new Response(null, { status: 400 })
          }
          if (body.ingredients?.[0]?.original_text !== '1 lb chicken') {
            return new Response(null, { status: 400 })
          }
          return new Response(
            JSON.stringify({
              id: 'r1',
              title: 'Chicken Soup',
              servings: 4,
              prep_time_minutes: 0,
              total_time_minutes: 0,
              source_url: null,
              notes: null,
              recipe_book_id: null,
              tags: [],
              ingredients: [],
              steps: [{ id: 's1', step_number: 1, instruction: 'Boil.' }],
              created_at: '2025-01-01T00:00:00Z',
              created_by: 'u1',
              updated_at: '2025-01-01T00:00:00Z',
              updated_by: 'u1',
              deleted_at: null,
            }),
            { status: 201, headers: { 'content-type': 'application/json' } },
          )
        }

        return new Response(null, { status: 404 })
      },
    )
    vi.stubGlobal('fetch', fetchMock)

    const user = userEvent.setup()
    renderPage('/recipes/new', 'create')

    await user.type(screen.getByLabelText(/title/i), 'Chicken Soup')
    await user.clear(screen.getByLabelText(/servings/i))
    await user.type(screen.getByLabelText(/servings/i), '4')

    await user.click(screen.getByRole('button', { name: /add ingredient/i }))
    await user.type(screen.getByPlaceholderText(/item/i), 'chicken')
    await user.type(screen.getByPlaceholderText(/ingredient notes/i), 'Organic')
    await user.type(
      screen.getByPlaceholderText(/original text/i),
      '1 lb chicken',
    )

    await user.type(screen.getByPlaceholderText(/instruction/i), 'Boil.')

    await user.click(screen.getByRole('button', { name: /^create$/i }))

    expect(await screen.findByText(/recipe detail/i)).toBeVisible()
    await waitFor(() =>
      expect(
        fetchMock.mock.calls.some(([arg, init]) => {
          const u = typeof arg === 'string' ? arg : arg.toString()
          const m = String(
            (init as RequestInit | undefined)?.method ?? 'GET',
          ).toUpperCase()
          return u.endsWith('/api/v1/recipes') && m === 'POST'
        }),
      ).toBe(true),
    )
  })

  it('loads recipe in edit mode and submits update', async () => {
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
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

        if (url.endsWith('/api/v1/tags') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        if (url.endsWith('/api/v1/recipes/r1') && method === 'GET') {
          return new Response(
            JSON.stringify({
              id: 'r1',
              title: 'Old Title',
              servings: 4,
              prep_time_minutes: 0,
              total_time_minutes: 0,
              source_url: null,
              notes: null,
              recipe_book_id: null,
              tags: [],
              ingredients: [],
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

        if (url.endsWith('/api/v1/recipes/r1') && method === 'PUT') {
          const body = JSON.parse(String(init?.body ?? '{}')) as {
            title?: string
          }
          if (body.title !== 'New Title') {
            return new Response(null, { status: 400 })
          }
          return new Response(
            JSON.stringify({
              id: 'r1',
              title: 'New Title',
              servings: 4,
              prep_time_minutes: 0,
              total_time_minutes: 0,
              source_url: null,
              notes: null,
              recipe_book_id: null,
              tags: [],
              ingredients: [],
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
      },
    )
    vi.stubGlobal('fetch', fetchMock)

    const user = userEvent.setup()
    renderPage('/recipes/r1/edit', 'edit')

    const title = await screen.findByLabelText(/title/i)
    await waitFor(() => expect(title).toHaveValue('Old Title'))

    await user.clear(title)
    await user.type(title, 'New Title')
    await user.click(screen.getByRole('button', { name: /save/i }))

    expect(await screen.findByText(/recipe detail/i)).toBeVisible()
  })

  it('prevents editing a deleted recipe', async () => {
    const fetchMock = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/recipe-books') && method === 'GET') {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        if (url.endsWith('/api/v1/tags') && method === 'GET') {
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
              recipe_book_id: null,
              tags: [],
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

        if (url.endsWith('/api/v1/recipes/r1') && method === 'PUT') {
          return new Response(null, { status: 500 })
        }

        return new Response(null, { status: 404 })
      },
    )
    vi.stubGlobal('fetch', fetchMock)

    renderPage('/recipes/r1/edit', 'edit')

    expect(await screen.findByText(/restore it before editing/i)).toBeVisible()

    const save = screen.getByRole('button', { name: /save/i })
    expect(save).toBeDisabled()

    await waitFor(() => {
      expect(
        fetchMock.mock.calls.some(([arg, init]) => {
          const u = typeof arg === 'string' ? arg : arg.toString()
          const m = String(
            (init as RequestInit | undefined)?.method ?? 'GET',
          ).toUpperCase()
          return u.endsWith('/api/v1/recipes/r1') && m === 'PUT'
        }),
      ).toBe(false)
    })
  })
})
