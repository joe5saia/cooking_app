import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import { AppRoutes } from './routes'

describe('routes', () => {
  afterEach(() => {
    cleanup()
    if (globalThis.localStorage && typeof localStorage.clear === 'function') {
      localStorage.clear()
    }
    vi.unstubAllGlobals()
  })

  it('navigates via TopNav', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = typeof input === 'string' ? input : input.toString()
        if (url.endsWith('/api/v1/auth/me')) {
          return new Response(
            JSON.stringify({ id: 'u', username: 'joe', display_name: null }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        if (url.endsWith('/api/v1/tags')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (url.includes('/api/v1/recipes')) {
          return new Response(
            JSON.stringify({ items: [], next_cursor: null }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        if (url.includes('/api/v1/meal-plans')) {
          return new Response(JSON.stringify({ items: [] }), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (url.includes('/api/v1/shopping-lists')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (url.endsWith('/api/v1/aisles')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        if (url.includes('/api/v1/items')) {
          return new Response(JSON.stringify([]), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }
        return new Response(null, { status: 404 })
      }),
    )

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const user = userEvent.setup()
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/recipes']}>
          <AppRoutes />
        </MemoryRouter>
      </QueryClientProvider>,
    )

    expect(
      await screen.findByRole('heading', { name: /recipes/i }),
    ).toBeVisible()

    await user.click(screen.getByRole('link', { name: /tags/i }))
    expect(await screen.findByRole('heading', { name: /tags/i })).toBeVisible()

    await user.click(screen.getByRole('link', { name: /meal plan/i }))
    expect(
      await screen.findByRole('heading', { name: /meal plan/i }),
    ).toBeVisible()

    await user.click(screen.getByRole('link', { name: /shopping lists/i }))
    expect(
      await screen.findByRole('heading', { name: /shopping lists/i }),
    ).toBeVisible()

    await user.click(screen.getByRole('link', { name: /items/i }))
    expect(
      await screen.findByRole('heading', { name: /items/i, level: 1 }),
    ).toBeVisible()

    await user.click(screen.getByRole('link', { name: /settings/i }))
    expect(
      await screen.findByRole('heading', { name: /settings/i }),
    ).toBeVisible()
  })

  it('keeps Recipes nav active on nested recipe routes', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = typeof input === 'string' ? input : input.toString()
        if (url.endsWith('/api/v1/auth/me')) {
          return new Response(
            JSON.stringify({ id: 'u', username: 'joe', display_name: null }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
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
        if (url.endsWith('/api/v1/recipes/r1')) {
          return new Response(
            JSON.stringify({
              id: 'r1',
              title: 'Pasta',
              servings: 2,
              prep_time_minutes: 5,
              total_time_minutes: 10,
              source_url: null,
              notes: null,
              recipe_book_id: null,
              tags: [],
              ingredients: [],
              steps: [],
              created_at: '2025-01-01T00:00:00Z',
              created_by: 'u',
              updated_at: '2025-01-01T00:00:00Z',
              updated_by: 'u',
              deleted_at: null,
            }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        return new Response(null, { status: 404 })
      }),
    )

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    const { unmount } = render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/recipes/new']}>
          <AppRoutes />
        </MemoryRouter>
      </QueryClientProvider>,
    )

    await screen.findByRole('heading', { name: /new recipe/i })
    expect(
      within(screen.getByRole('navigation', { name: /primary/i })).getByRole(
        'link',
        { name: /recipes/i },
      ),
    ).toHaveAttribute('aria-current', 'page')

    unmount()

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/recipes/r1']}>
          <AppRoutes />
        </MemoryRouter>
      </QueryClientProvider>,
    )

    await screen.findByRole('heading', { name: /pasta/i })
    expect(
      within(screen.getByRole('navigation', { name: /primary/i })).getByRole(
        'link',
        { name: /recipes/i },
      ),
    ).toHaveAttribute('aria-current', 'page')
  })

  it('renders a Skip to content link targeting the main region', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = typeof input === 'string' ? input : input.toString()
        if (url.endsWith('/api/v1/auth/me')) {
          return new Response(
            JSON.stringify({ id: 'u', username: 'joe', display_name: null }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
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
        if (url.includes('/api/v1/recipes')) {
          return new Response(
            JSON.stringify({ items: [], next_cursor: null }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        return new Response(null, { status: 404 })
      }),
    )

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/recipes']}>
          <AppRoutes />
        </MemoryRouter>
      </QueryClientProvider>,
    )

    expect(
      await screen.findByRole('link', { name: /skip to content/i }),
    ).toHaveAttribute('href', '#main-content')
    expect(screen.getByRole('main')).toHaveAttribute('id', 'main-content')
  })

  it('keeps /login outside the authenticated AppShell chrome', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(null, { status: 404 })),
    )

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    const { unmount } = render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/login']}>
          <AppRoutes />
        </MemoryRouter>
      </QueryClientProvider>,
    )

    expect(await screen.findByRole('heading', { name: /login/i })).toBeVisible()
    expect(
      screen.queryByRole('navigation', { name: /primary/i }),
    ).not.toBeInTheDocument()
    expect(screen.queryByText(/cooking app/i)).not.toBeInTheDocument()

    unmount()

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = typeof input === 'string' ? input : input.toString()
        if (url.endsWith('/api/v1/auth/me')) {
          return new Response(
            JSON.stringify({ id: 'u', username: 'joe', display_name: null }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
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
        if (url.includes('/api/v1/recipes')) {
          return new Response(
            JSON.stringify({ items: [], next_cursor: null }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        return new Response(null, { status: 404 })
      }),
    )

    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/recipes']}>
          <AppRoutes />
        </MemoryRouter>
      </QueryClientProvider>,
    )

    expect(
      await screen.findByRole('navigation', { name: /primary/i }),
    ).toBeVisible()
    expect(screen.getByText(/cooking app/i)).toBeVisible()
  })
})
