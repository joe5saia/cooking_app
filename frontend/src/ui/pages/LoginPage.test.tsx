import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { AppRoutes } from '../../routes'

function renderWithClient(initialEntries: string[]) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={initialEntries}>
        <AppRoutes />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('LoginPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('validates required fields', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        return new Response(
          JSON.stringify({ code: 'unauthorized', message: 'unauthorized' }),
          { status: 401, headers: { 'content-type': 'application/json' } },
        )
      }),
    )

    const user = userEvent.setup()
    renderWithClient(['/login'])

    await user.click(screen.getByRole('button', { name: /sign in/i }))
    expect(screen.getByRole('alert')).toHaveTextContent(/username is required/i)
  })

  it('logs in and redirects to recipes', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = typeof input === 'string' ? input : input.toString()
        if (url.endsWith('/api/v1/auth/login')) {
          return new Response(null, { status: 204 })
        }
        if (url.endsWith('/api/v1/auth/me')) {
          return new Response(
            JSON.stringify({ id: 'u', username: 'joe', display_name: null }),
            { status: 200, headers: { 'content-type': 'application/json' } },
          )
        }
        return new Response(null, { status: 404 })
      }),
    )

    const user = userEvent.setup()
    renderWithClient(['/login'])

    await user.type(screen.getByLabelText(/username/i), 'joe')
    await user.type(screen.getByLabelText(/password/i), 'pw')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() =>
      expect(screen.getByRole('heading', { name: /recipes/i })).toBeVisible(),
    )
  })

  it('redirects unauthenticated access to /login', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = typeof input === 'string' ? input : input.toString()
        if (url.endsWith('/api/v1/auth/me')) {
          return new Response(
            JSON.stringify({ code: 'unauthorized', message: 'unauthorized' }),
            { status: 401, headers: { 'content-type': 'application/json' } },
          )
        }
        return new Response(null, { status: 404 })
      }),
    )

    renderWithClient(['/recipes'])

    await waitFor(() =>
      expect(screen.getByRole('heading', { name: /login/i })).toBeVisible(),
    )
  })
})
