import { afterEach, describe, expect, it, vi } from 'vitest'

import { ApiError, UnauthorizedError, apiFetchJSON } from './client'

describe('apiFetchJSON', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
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
})
