import { useMutation, useQueryClient } from '@tanstack/react-query'
import { type FormEvent, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'

import { login } from '../../api/auth'

export function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const queryClient = useQueryClient()

  const fromPath = useMemo(() => {
    const s = location.state as { from?: { pathname?: string } } | null
    return s?.from?.pathname ?? '/recipes'
  }, [location.state])

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: () => login({ username, password }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['me'] })
      navigate(fromPath, { replace: true })
    },
    onError: () => {
      setError('Invalid username or password.')
    },
  })

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)

    const u = username.trim()
    const p = password.trim()
    if (u === '') {
      setError('Username is required.')
      return
    }
    if (p === '') {
      setError('Password is required.')
      return
    }

    mutation.mutate()
  }

  return (
    <div style={{ maxWidth: 420, margin: '0 auto', padding: 16 }}>
      <h1>Login</h1>
      <form onSubmit={onSubmit} aria-label="Login form">
        <div style={{ display: 'grid', gap: 12 }}>
          <label style={{ display: 'grid', gap: 6 }}>
            <span>Username</span>
            <input
              name="username"
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              disabled={mutation.isPending}
            />
          </label>

          <label style={{ display: 'grid', gap: 6 }}>
            <span>Password</span>
            <input
              name="password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={mutation.isPending}
            />
          </label>

          {error ? (
            <div role="alert" style={{ color: '#ffb4b4' }}>
              {error}
            </div>
          ) : null}

          <button type="submit" disabled={mutation.isPending}>
            {mutation.isPending ? 'Signing in…' : 'Sign in'}
          </button>
        </div>
      </form>
    </div>
  )
}
