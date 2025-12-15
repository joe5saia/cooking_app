import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import { AppRoutes } from './routes'

describe('routes', () => {
  afterEach(() => {
    cleanup()
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

    await user.click(screen.getByRole('link', { name: /settings/i }))
    expect(
      await screen.findByRole('heading', { name: /settings/i }),
    ).toBeVisible()
  })
})
