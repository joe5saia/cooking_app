import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { TagListPage } from './TagListPage'

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <TagListPage />
    </QueryClientProvider>,
  )
}

describe('TagListPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('creates and deletes a tag', async () => {
    const tags: Array<{ id: string; name: string; created_at: string }> = []

    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = typeof input === 'string' ? input : input.toString()
        const method = (init?.method ?? 'GET').toUpperCase()

        if (url.endsWith('/api/v1/tags') && method === 'GET') {
          return new Response(JSON.stringify(tags), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        if (url.endsWith('/api/v1/tags') && method === 'POST') {
          const body = JSON.parse(String(init?.body ?? '{}')) as {
            name?: string
          }
          const created = {
            id: 't1',
            name: body.name ?? '',
            created_at: '2025-01-01T00:00:00Z',
          }
          tags.push(created)
          return new Response(JSON.stringify(created), {
            status: 200,
            headers: { 'content-type': 'application/json' },
          })
        }

        if (url.endsWith('/api/v1/tags/t1') && method === 'DELETE') {
          tags.splice(
            tags.findIndex((t) => t.id === 't1'),
            1,
          )
          return new Response(null, { status: 204 })
        }

        return new Response(null, { status: 404 })
      }),
    )

    const user = userEvent.setup()
    renderPage()

    expect(await screen.findByText(/no tags yet/i)).toBeVisible()

    await user.type(screen.getByPlaceholderText(/new tag name/i), 'Soup')
    await user.click(screen.getByRole('button', { name: /add/i }))

    await waitFor(() => expect(screen.getByText('Soup')).toBeVisible())

    await user.click(screen.getByRole('button', { name: /delete/i }))

    await waitFor(() => expect(screen.getByText(/no tags yet/i)).toBeVisible())
  })
})
