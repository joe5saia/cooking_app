import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'

import { ApiError } from '../../api/client'
import { createUser, deactivateUser, listUsers } from '../../api/users'

import styles from './CrudList.module.css'
import { Page } from './Page'

export function UserManagerPage() {
  const queryClient = useQueryClient()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [error, setError] = useState<string | null>(null)

  const listQuery = useQuery({
    queryKey: ['users'],
    queryFn: listUsers,
  })

  const createMutation = useMutation({
    mutationFn: (params: {
      username: string
      password: string
      display_name?: string | null
    }) => createUser(params),
    onSuccess: async () => {
      setUsername('')
      setPassword('')
      setDisplayName('')
      setError(null)
      await queryClient.invalidateQueries({ queryKey: ['users'] })
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : 'Failed to create user.')
    },
  })

  const deactivateMutation = useMutation({
    mutationFn: (id: string) => deactivateUser(id),
    onSuccess: async () => {
      setError(null)
      await queryClient.invalidateQueries({ queryKey: ['users'] })
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : 'Failed to deactivate user.')
    },
  })

  const users = useMemo(() => listQuery.data ?? [], [listQuery.data])

  async function onCreate(e: React.FormEvent) {
    e.preventDefault()
    setError(null)

    const u = username.trim()
    const p = password.trim()
    const dn = displayName.trim()

    if (u === '') {
      setError('Username is required.')
      return
    }
    if (p === '') {
      setError('Password is required.')
      return
    }

    createMutation.mutate({
      username: u,
      password: p,
      display_name: dn === '' ? null : dn,
    })
  }

  return (
    <Page title="Users">
      <div className={styles.section}>
        {error ? (
          <div role="alert" className={styles.alert}>
            {error}
          </div>
        ) : null}

        <form
          onSubmit={onCreate}
          className={styles.section}
          aria-label="Create user"
        >
          <input
            className={styles.input}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="Username"
            disabled={createMutation.isPending}
          />
          <input
            className={styles.input}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Password"
            type="password"
            disabled={createMutation.isPending}
          />
          <input
            className={styles.input}
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="Display name (optional)"
            disabled={createMutation.isPending}
          />
          <button
            className={styles.button}
            type="submit"
            disabled={createMutation.isPending}
          >
            {createMutation.isPending ? 'Creating…' : 'Create user'}
          </button>
        </form>

        {listQuery.isPending ? <div>Loading…</div> : null}
        {listQuery.isError ? (
          <div role="alert" className={styles.alert}>
            Failed to load users.
          </div>
        ) : null}

        <div className={styles.list} aria-label="Users list">
          {users.map((u) => (
            <div className={styles.item} key={u.id}>
              <div>
                <div style={{ fontWeight: 600 }}>{u.username}</div>
                <div style={{ opacity: 0.8, fontSize: 12, marginTop: 4 }}>
                  {u.display_name ?? 'No display name'} ·{' '}
                  {u.is_active ? 'Active' : 'Deactivated'}
                </div>
              </div>
              <div className={styles.actions}>
                <button
                  className={`${styles.button} ${styles.buttonDanger}`}
                  type="button"
                  onClick={() => deactivateMutation.mutate(u.id)}
                  disabled={!u.is_active || deactivateMutation.isPending}
                >
                  Deactivate
                </button>
              </div>
            </div>
          ))}

          {!listQuery.isPending && users.length === 0 ? (
            <div>No users found.</div>
          ) : null}
        </div>
      </div>
    </Page>
  )
}
