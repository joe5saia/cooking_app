import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'

import { ApiError } from '../../api/client'
import { createTag, deleteTag, listTags, updateTag } from '../../api/tags'

import styles from './CrudList.module.css'
import { Page } from './Page'

export function TagListPage() {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [editingID, setEditingID] = useState<string | null>(null)
  const [editingName, setEditingName] = useState('')

  const listQuery = useQuery({
    queryKey: ['tags'],
    queryFn: listTags,
  })

  const createMutation = useMutation({
    mutationFn: (params: { name: string }) => createTag(params),
    onSuccess: async () => {
      setName('')
      setError(null)
      await queryClient.invalidateQueries({ queryKey: ['tags'] })
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : 'Failed to create tag.')
    },
  })

  const updateMutation = useMutation({
    mutationFn: (params: { id: string; name: string }) =>
      updateTag(params.id, { name: params.name }),
    onSuccess: async () => {
      setEditingID(null)
      setEditingName('')
      setError(null)
      await queryClient.invalidateQueries({ queryKey: ['tags'] })
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : 'Failed to update tag.')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteTag(id),
    onSuccess: async () => {
      setError(null)
      await queryClient.invalidateQueries({ queryKey: ['tags'] })
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : 'Failed to delete tag.')
    },
  })

  const tags = useMemo(() => listQuery.data ?? [], [listQuery.data])

  function startEdit(id: string, currentName: string) {
    setEditingID(id)
    setEditingName(currentName)
  }

  function cancelEdit() {
    setEditingID(null)
    setEditingName('')
  }

  async function onCreate(e: React.FormEvent) {
    e.preventDefault()
    setError(null)

    const trimmed = name.trim()
    if (trimmed === '') {
      setError('Name is required.')
      return
    }
    createMutation.mutate({ name: trimmed })
  }

  async function onSaveEdit(id: string) {
    setError(null)
    const trimmed = editingName.trim()
    if (trimmed === '') {
      setError('Name is required.')
      return
    }
    updateMutation.mutate({ id, name: trimmed })
  }

  return (
    <Page title="Tags">
      <div className={styles.section}>
        {error ? (
          <div role="alert" className={styles.alert}>
            {error}
          </div>
        ) : null}

        <form
          onSubmit={onCreate}
          className={styles.form}
          aria-label="Create tag"
        >
          <input
            className={styles.input}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="New tag name"
            disabled={createMutation.isPending}
          />
          <button
            className={styles.button}
            type="submit"
            disabled={createMutation.isPending}
          >
            Add
          </button>
        </form>

        {listQuery.isPending ? <div>Loading…</div> : null}
        {listQuery.isError ? (
          <div role="alert" className={styles.alert}>
            Failed to load tags.
          </div>
        ) : null}

        <div className={styles.list} aria-label="Tags list">
          {tags.map((t) => (
            <div className={styles.item} key={t.id}>
              {editingID === t.id ? (
                <input
                  className={styles.input}
                  value={editingName}
                  onChange={(e) => setEditingName(e.target.value)}
                  aria-label="Edit name"
                  disabled={updateMutation.isPending}
                />
              ) : (
                <div>{t.name}</div>
              )}

              <div className={styles.actions}>
                {editingID === t.id ? (
                  <>
                    <button
                      className={styles.button}
                      type="button"
                      onClick={() => onSaveEdit(t.id)}
                      disabled={updateMutation.isPending}
                    >
                      Save
                    </button>
                    <button
                      className={styles.button}
                      type="button"
                      onClick={cancelEdit}
                      disabled={updateMutation.isPending}
                    >
                      Cancel
                    </button>
                  </>
                ) : (
                  <>
                    <button
                      className={styles.button}
                      type="button"
                      onClick={() => startEdit(t.id, t.name)}
                      disabled={deleteMutation.isPending}
                    >
                      Rename
                    </button>
                    <button
                      className={`${styles.button} ${styles.buttonDanger}`}
                      type="button"
                      onClick={() => deleteMutation.mutate(t.id)}
                      disabled={deleteMutation.isPending}
                    >
                      Delete
                    </button>
                  </>
                )}
              </div>
            </div>
          ))}

          {!listQuery.isPending && tags.length === 0 ? (
            <div>No tags yet.</div>
          ) : null}
        </div>
      </div>
    </Page>
  )
}
