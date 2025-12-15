export type Problem = {
  code: string
  message: string
  details?: unknown
}

export class ApiError extends Error {
  readonly status: number
  readonly problem?: Problem

  constructor(status: number, message: string, problem?: Problem) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.problem = problem
  }
}

export class UnauthorizedError extends ApiError {
  constructor(problem?: Problem) {
    super(401, problem?.message ?? 'unauthorized', problem)
    this.name = 'UnauthorizedError'
  }
}

function isProblem(value: unknown): value is Problem {
  if (!value || typeof value !== 'object') return false
  const v = value as Record<string, unknown>
  return typeof v.code === 'string' && typeof v.message === 'string'
}

export async function apiFetchJSON<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')

  const res = await fetch(path, {
    ...init,
    headers,
    credentials: init.credentials ?? 'include',
  })

  if (res.status === 204) {
    return undefined as T
  }

  let body: unknown = undefined
  const contentType = res.headers.get('content-type') ?? ''
  if (contentType.includes('application/json')) {
    try {
      body = await res.json()
    } catch {
      body = undefined
    }
  } else {
    try {
      body = await res.text()
    } catch {
      body = undefined
    }
  }

  if (!res.ok) {
    const problem = isProblem(body) ? body : undefined
    if (res.status === 401) {
      throw new UnauthorizedError(problem)
    }

    const message =
      problem?.message ??
      (typeof body === 'string' && body.trim() !== '' ? body : res.statusText)
    throw new ApiError(res.status, message, problem)
  }

  return body as T
}
