import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'

import { ApiError } from '../../api/client'
import { createToken, listTokens, revokeToken } from '../../api/tokens'

import styles from './CrudList.module.css'
import { Page } from './Page'

export function TokenManagerPage() {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [createdSecret, setCreatedSecret] = useState<string | null>(null)

  const listQuery = useQuery({
    queryKey: ['tokens'],
    queryFn: listTokens,
  })

  const createMutation = useMutation({
    mutationFn: (params: { name: string; expires_at?: string }) =>
      createToken(params),
    onSuccess: async (resp) => {
      setName('')
      setExpiresAt('')
      setCreatedSecret(resp.token)
      setError(null)
      await queryClient.invalidateQueries({ queryKey: ['tokens'] })
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : 'Failed to create token.')
    },
  })

  const revokeMutation = useMutation({
    mutationFn: (id: string) => revokeToken(id),
    onSuccess: async () => {
      setError(null)
      await queryClient.invalidateQueries({ queryKey: ['tokens'] })
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : 'Failed to revoke token.')
    },
  })

  const tokens = useMemo(() => listQuery.data ?? [], [listQuery.data])

  async function onCreate(e: React.FormEvent) {
    e.preventDefault()
    setError(null)

    const trimmedName = name.trim()
    if (trimmedName === '') {
      setError('Name is required.')
      return
    }

    const trimmedExpires = expiresAt.trim()
    if (trimmedExpires !== '' && Number.isNaN(Date.parse(trimmedExpires))) {
      setError('expires_at must be RFC3339 (e.g. 2026-12-31T00:00:00Z).')
      return
    }

    createMutation.mutate({
      name: trimmedName,
      expires_at: trimmedExpires !== '' ? trimmedExpires : undefined,
    })
  }

  async function onCopy() {
    if (!createdSecret) return
    try {
      await navigator.clipboard.writeText(createdSecret)
    } catch {
      setError('Copy failed. Please copy manually.')
    }
  }

  return (
    <Page title="Tokens">
      <div className={styles.section}>
        {error ? (
          <div role="alert" className={styles.alert}>
            {error}
          </div>
        ) : null}

        {createdSecret ? (
          <div className={styles.item} aria-label="New token secret">
            <div>
              <div className={styles.strong}>Token created (copy now)</div>
              <code className={`${styles.codeBlock} ${styles.breakAll}`}>
                {createdSecret}
              </code>
            </div>
            <div className={styles.actions}>
              <button className={styles.button} type="button" onClick={onCopy}>
                Copy
              </button>
              <button
                className={styles.button}
                type="button"
                onClick={() => setCreatedSecret(null)}
              >
                Dismiss
              </button>
            </div>
          </div>
        ) : null}

        <form
          onSubmit={onCreate}
          className={styles.section}
          aria-label="Create token"
        >
          <input
            className={styles.input}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Token name (e.g. laptop-cli)"
            disabled={createMutation.isPending}
          />
          <input
            className={styles.input}
            value={expiresAt}
            onChange={(e) => setExpiresAt(e.target.value)}
            placeholder="Optional expires_at (RFC3339, e.g. 2026-12-31T00:00:00Z)"
            disabled={createMutation.isPending}
          />
          <button
            className={styles.button}
            type="submit"
            disabled={createMutation.isPending}
          >
            {createMutation.isPending ? 'Creating…' : 'Create token'}
          </button>
        </form>

        {listQuery.isPending ? <div>Loading…</div> : null}
        {listQuery.isError ? (
          <div role="alert" className={styles.alert}>
            Failed to load tokens.
          </div>
        ) : null}

        <div className={styles.list} aria-label="Tokens list">
          {tokens.map((t) => (
            <div className={styles.item} key={t.id}>
              <div>
                <div className={styles.strong}>{t.name}</div>
                <div className={styles.meta}>
                  Last used: {t.last_used_at ?? 'never'} · Expires:{' '}
                  {t.expires_at ?? 'never'}
                </div>
              </div>
              <div className={styles.actions}>
                <button
                  className={`${styles.button} ${styles.buttonDanger}`}
                  type="button"
                  onClick={() => revokeMutation.mutate(t.id)}
                  disabled={revokeMutation.isPending}
                >
                  Revoke
                </button>
              </div>
            </div>
          ))}

          {!listQuery.isPending && tokens.length === 0 ? (
            <div>No tokens yet.</div>
          ) : null}
        </div>
      </div>
    </Page>
  )
}
