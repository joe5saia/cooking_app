import { useMutation, useQueryClient } from '@tanstack/react-query'
import { type FormEvent, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'

import { login } from '../../api/auth'
import { Button, Card, FormField, Input } from '../components'

import styles from './LoginPage.module.css'

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
  const [usernameError, setUsernameError] = useState<string | null>(null)
  const [passwordError, setPasswordError] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)

  const mutation = useMutation({
    mutationFn: (params: { username: string; password: string }) =>
      login(params),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['me'] })
      navigate(fromPath, { replace: true })
    },
    onError: () => {
      setFormError('Invalid username or password.')
    },
  })

  function onSubmit(e: FormEvent) {
    e.preventDefault()
    setUsernameError(null)
    setPasswordError(null)
    setFormError(null)

    const u = username.trim()
    const p = password.trim()
    if (u === '') {
      setUsernameError('Username is required.')
      return
    }
    if (p === '') {
      setPasswordError('Password is required.')
      return
    }

    mutation.mutate({ username: u, password: p })
  }

  const isPending = mutation.isPending

  return (
    <div className={styles.wrap}>
      <Card className={styles.card} padding="md">
        <h1 className={styles.title}>Login</h1>
        <form
          className={styles.form}
          onSubmit={onSubmit}
          aria-label="Login form"
        >
          {formError ? (
            <div className={styles.formError} role="alert">
              {formError}
            </div>
          ) : null}

          <FormField
            label="Username"
            error={usernameError ?? undefined}
            required
          >
            {({ id, describedBy, invalid }) => (
              <Input
                id={id}
                name="username"
                autoComplete="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                disabled={isPending}
                aria-describedby={describedBy}
                invalid={invalid}
              />
            )}
          </FormField>

          <FormField
            label="Password"
            error={passwordError ?? undefined}
            required
          >
            {({ id, describedBy, invalid }) => (
              <Input
                id={id}
                name="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={isPending}
                aria-describedby={describedBy}
                invalid={invalid}
              />
            )}
          </FormField>

          <div className={styles.actions}>
            <Button type="submit" variant="primary" disabled={isPending}>
              {isPending ? 'Signing in…' : 'Sign in'}
            </Button>
          </div>
        </form>
      </Card>
    </div>
  )
}
