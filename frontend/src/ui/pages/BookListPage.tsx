import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'

import {
  createRecipeBook,
  deleteRecipeBook,
  listRecipeBooks,
  updateRecipeBook,
} from '../../api/recipeBooks'
import { ApiError } from '../../api/client'

import styles from './CrudList.module.css'
import { Page } from './Page'

export function BookListPage() {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [editingID, setEditingID] = useState<string | null>(null)
  const [editingName, setEditingName] = useState('')

  const listQuery = useQuery({
    queryKey: ['recipe-books'],
    queryFn: listRecipeBooks,
  })

  const createMutation = useMutation({
    mutationFn: (params: { name: string }) => createRecipeBook(params),
    onSuccess: async () => {
      setName('')
      setError(null)
      await queryClient.invalidateQueries({ queryKey: ['recipe-books'] })
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : 'Failed to create book.')
    },
  })

  const updateMutation = useMutation({
    mutationFn: (params: { id: string; name: string }) =>
      updateRecipeBook(params.id, { name: params.name }),
    onSuccess: async () => {
      setEditingID(null)
      setEditingName('')
      setError(null)
      await queryClient.invalidateQueries({ queryKey: ['recipe-books'] })
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : 'Failed to update book.')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteRecipeBook(id),
    onSuccess: async () => {
      setError(null)
      await queryClient.invalidateQueries({ queryKey: ['recipe-books'] })
    },
    onError: (e) => {
      if (e instanceof ApiError && e.status === 409) {
        setError('Cannot delete a recipe book that still has recipes.')
        return
      }
      setError(e instanceof ApiError ? e.message : 'Failed to delete book.')
    },
  })

  const books = useMemo(() => listQuery.data ?? [], [listQuery.data])

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
    <Page title="Recipe Books">
      <div className={styles.section}>
        {error ? (
          <div role="alert" className={styles.alert}>
            {error}
          </div>
        ) : null}

        <form
          onSubmit={onCreate}
          className={styles.form}
          aria-label="Create book"
        >
          <input
            className={styles.input}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="New book name"
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
            Failed to load recipe books.
          </div>
        ) : null}

        <div className={styles.list} aria-label="Recipe books list">
          {books.map((b) => (
            <div className={styles.item} key={b.id}>
              {editingID === b.id ? (
                <input
                  className={styles.input}
                  value={editingName}
                  onChange={(e) => setEditingName(e.target.value)}
                  aria-label="Edit name"
                  disabled={updateMutation.isPending}
                />
              ) : (
                <div>{b.name}</div>
              )}

              <div className={styles.actions}>
                {editingID === b.id ? (
                  <>
                    <button
                      className={styles.button}
                      type="button"
                      onClick={() => onSaveEdit(b.id)}
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
                      onClick={() => startEdit(b.id, b.name)}
                      disabled={deleteMutation.isPending}
                    >
                      Rename
                    </button>
                    <button
                      className={`${styles.button} ${styles.buttonDanger}`}
                      type="button"
                      onClick={() => deleteMutation.mutate(b.id)}
                      disabled={deleteMutation.isPending}
                    >
                      Delete
                    </button>
                  </>
                )}
              </div>
            </div>
          ))}

          {!listQuery.isPending && books.length === 0 ? (
            <div>No recipe books yet.</div>
          ) : null}
        </div>
      </div>
    </Page>
  )
}
