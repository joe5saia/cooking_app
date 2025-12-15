import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { BookListPage } from './BookListPage'

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <BookListPage />
    </QueryClientProvider>,
  )
}

describe('BookListPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('creates a recipe book', async () => {
    const books: Array<{ id: string; name: string; created_at: string }> = []

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/recipe-books') && method === 'GET') {
          return new Response(JSON.stringify(books), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        if (url.endsWith('/api/v1/recipe-books') && method === 'POST') {
          const body = JSON.parse(String(init?.body ?? '{}')) as {
            name?: string
          }
          const created = {
            id: 'b1',
            name: body.name ?? '',
            created_at: '2025-01-01T00:00:00Z',
          }
          books.push(created)
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

    expect(await screen.findByText(/no recipe books yet/i)).toBeVisible()

    await user.type(screen.getByPlaceholderText(/new book name/i), 'Main')
    await user.click(screen.getByRole('button', { name: /add/i }))

    await waitFor(() => expect(screen.getByText('Main')).toBeVisible())
  })

  it('shows a conflict message when deleting a book with recipes', async () => {
    const books = [
      { id: 'b1', name: 'WithRecipes', created_at: '2025-01-01T00:00:00Z' },
    ]

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/recipe-books') && method === 'GET') {
          return new Response(JSON.stringify(books), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        if (url.endsWith('/api/v1/recipe-books/b1') && method === 'DELETE') {
          return new Response(
            JSON.stringify({
              code: 'conflict',
              message: 'cannot delete recipe book with recipes',
            }),
            { status: 409, headers: { 'content-type': 'application/json' } },
          )
        }

        return new Response(null, { status: 404 })
      }),
    )

    const user = userEvent.setup()
    renderPage()

    expect(await screen.findByText('WithRecipes')).toBeVisible()

    await user.click(screen.getByRole('button', { name: /delete/i }))
    expect(await screen.findByRole('alert')).toHaveTextContent(
      /cannot delete a recipe book/i,
    )
  })
})
