import { useQuery } from '@tanstack/react-query'
import { Navigate, Outlet, useLocation } from 'react-router-dom'

import { getMe } from '../api/auth'
import { UnauthorizedError } from '../api/client'

export function RequireAuth() {
  const location = useLocation()
  const meQuery = useQuery({
    queryKey: ['me'],
    queryFn: getMe,
    retry: false,
  })

  if (meQuery.isPending) {
    return null
  }

  if (meQuery.isError) {
    if (meQuery.error instanceof UnauthorizedError) {
      return <Navigate to="/login" state={{ from: location }} replace />
    }
    return <div>Failed to load session.</div>
  }

  return <Outlet />
}
