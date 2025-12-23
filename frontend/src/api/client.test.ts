import { afterEach, describe, expect, it, vi } from 'vitest'

import { ApiError, UnauthorizedError, apiFetchJSON } from './client'

describe('apiFetchJSON', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    document.cookie =
      'cooking_app_session_csrf=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/'
  })

  it('includes credentials by default', async () => {
    const fetchMock = vi.fn(async (..._args: unknown[]) => {
      void _args
      return new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    await apiFetchJSON('/api/v1/healthz')

    expect(fetchMock).toHaveBeenCalledTimes(1)
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined
    expect(init?.credentials).toBe('include')
  })

  it('maps 401 to UnauthorizedError with problem body', async () => {
    const fetchMock = vi.fn(async (..._args: unknown[]) => {
      void _args
      return new Response(
        JSON.stringify({ code: 'unauthorized', message: 'unauthorized' }),
        {
          status: 401,
          headers: { 'content-type': 'application/json' },
        },
      )
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(apiFetchJSON('/api/v1/auth/me')).rejects.toBeInstanceOf(
      UnauthorizedError,
    )
  })

  it('maps non-JSON errors to ApiError', async () => {
    const fetchMock = vi.fn(async (..._args: unknown[]) => {
      void _args
      return new Response('nope', {
        status: 500,
        statusText: 'Internal Server Error',
        headers: { 'content-type': 'text/plain' },
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(apiFetchJSON('/api/v1/healthz')).rejects.toEqual(
      expect.objectContaining({
        name: 'ApiError',
        status: 500,
        message: 'nope',
      } satisfies Partial<ApiError>),
    )
  })

  it('adds CSRF token header for unsafe methods when cookie is present', async () => {
    document.cookie = 'cooking_app_session_csrf=token-123; path=/'
    const fetchMock = vi.fn(async (..._args: unknown[]) => {
      void _args
      return new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    await apiFetchJSON('/api/v1/users', {
      method: 'POST',
      body: JSON.stringify({ username: 'test' }),
    })

    const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined
    const headers = new Headers(init?.headers)
    expect(headers.get('X-CSRF-Token')).toBe('token-123')
  })

  it('does not attach CSRF token header for safe methods', async () => {
    document.cookie = 'cooking_app_session_csrf=token-123; path=/'
    const fetchMock = vi.fn(async (..._args: unknown[]) => {
      void _args
      return new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      })
    })
    vi.stubGlobal('fetch', fetchMock)

    await apiFetchJSON('/api/v1/users')

    const init = fetchMock.mock.calls[0]?.[1] as RequestInit | undefined
    const headers = new Headers(init?.headers)
    expect(headers.get('X-CSRF-Token')).toBeNull()
  })
})
