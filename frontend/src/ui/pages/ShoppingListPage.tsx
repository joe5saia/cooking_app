import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'

import { ApiError } from '../../api/client'
import {
  createShoppingList,
  deleteShoppingList,
  listShoppingLists,
  updateShoppingList,
  type ShoppingList,
} from '../../api/shoppingLists'
import { Button, Card, FormField, Input } from '../components'
import { Page } from './Page'
import styles from './ShoppingListPage.module.css'

type ShoppingListDraft = {
  date: string
  name: string
  notes: string
}

/** Format a date as YYYY-MM-DD for inputs and API calls. */
function formatDate(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

/** Return a new date shifted by a number of days. */
function addDays(date: Date, delta: number): Date {
  const next = new Date(date)
  next.setDate(date.getDate() + delta)
  return next
}

export function ShoppingListPage() {
  const queryClient = useQueryClient()
  const today = useMemo(() => new Date(), [])
  const [range, setRange] = useState({
    start: formatDate(addDays(today, -7)),
    end: formatDate(addDays(today, 21)),
  })
  const [draft, setDraft] = useState<ShoppingListDraft>({
    date: formatDate(today),
    name: '',
    notes: '',
  })
  const [editingID, setEditingID] = useState<string | null>(null)
  const [editing, setEditing] = useState<ShoppingListDraft>({
    date: '',
    name: '',
    notes: '',
  })
  const [error, setError] = useState<string | null>(null)

  const listsQuery = useQuery({
    queryKey: ['shopping-lists', range.start, range.end],
    queryFn: () => listShoppingLists({ start: range.start, end: range.end }),
  })

  const createMutation = useMutation({
    mutationFn: (params: ShoppingListDraft) =>
      createShoppingList({
        list_date: params.date,
        name: params.name.trim(),
        notes: params.notes.trim() === '' ? null : params.notes.trim(),
      }),
    onSuccess: async () => {
      setDraft((prev) => ({ ...prev, name: '', notes: '' }))
      setError(null)
      await queryClient.invalidateQueries({ queryKey: ['shopping-lists'] })
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to create list.')
    },
  })

  const updateMutation = useMutation({
    mutationFn: (params: { id: string; body: ShoppingListDraft }) =>
      updateShoppingList(params.id, {
        list_date: params.body.date,
        name: params.body.name.trim(),
        notes:
          params.body.notes.trim() === '' ? null : params.body.notes.trim(),
      }),
    onSuccess: async () => {
      setEditingID(null)
      setEditing({ date: '', name: '', notes: '' })
      setError(null)
      await queryClient.invalidateQueries({ queryKey: ['shopping-lists'] })
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to update list.')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteShoppingList(id),
    onSuccess: async () => {
      setError(null)
      await queryClient.invalidateQueries({ queryKey: ['shopping-lists'] })
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to delete list.')
    },
  })

  const lists = useMemo(() => listsQuery.data ?? [], [listsQuery.data])

  function handleCreate(event: React.FormEvent) {
    event.preventDefault()
    setError(null)
    const trimmedName = draft.name.trim()
    if (trimmedName === '') {
      setError('Name is required.')
      return
    }
    if (draft.date.trim() === '') {
      setError('Date is required.')
      return
    }
    createMutation.mutate({ ...draft, name: trimmedName })
  }

  function startEdit(list: ShoppingList) {
    setEditingID(list.id)
    setEditing({
      date: list.list_date,
      name: list.name,
      notes: list.notes ?? '',
    })
  }

  function cancelEdit() {
    setEditingID(null)
    setEditing({ date: '', name: '', notes: '' })
  }

  function saveEdit(id: string) {
    setError(null)
    const trimmedName = editing.name.trim()
    if (trimmedName === '') {
      setError('Name is required.')
      return
    }
    if (editing.date.trim() === '') {
      setError('Date is required.')
      return
    }
    updateMutation.mutate({ id, body: { ...editing, name: trimmedName } })
  }

  return (
    <Page title="Shopping Lists">
      <div className={styles.layout}>
        <Card className={styles.panel}>
          <div className={styles.panelHeader}>
            <div>
              <h2 className={styles.panelTitle}>Create list</h2>
              <p className={styles.panelHint}>
                Lists are tied to a date and capture items at the time they are
                added.
              </p>
            </div>
          </div>

          {error ? (
            <div role="alert" className={styles.alert}>
              {error}
            </div>
          ) : null}

          <form className={styles.form} onSubmit={handleCreate}>
            <FormField label="Date">
              {({ id, describedBy }) => (
                <Input
                  id={id}
                  aria-describedby={describedBy}
                  type="date"
                  value={draft.date}
                  onChange={(event) =>
                    setDraft((prev) => ({
                      ...prev,
                      date: event.target.value,
                    }))
                  }
                />
              )}
            </FormField>
            <FormField label="Name">
              {({ id, describedBy }) => (
                <Input
                  id={id}
                  aria-describedby={describedBy}
                  value={draft.name}
                  onChange={(event) =>
                    setDraft((prev) => ({
                      ...prev,
                      name: event.target.value,
                    }))
                  }
                  placeholder="Weekly shop"
                />
              )}
            </FormField>
            <FormField label="Notes">
              {({ id, describedBy }) => (
                <Input
                  id={id}
                  aria-describedby={describedBy}
                  value={draft.notes}
                  onChange={(event) =>
                    setDraft((prev) => ({
                      ...prev,
                      notes: event.target.value,
                    }))
                  }
                  placeholder="Optional notes"
                />
              )}
            </FormField>
            <Button
              type="submit"
              variant="primary"
              disabled={createMutation.isPending}
            >
              {createMutation.isPending ? 'Creating…' : 'Create list'}
            </Button>
          </form>
        </Card>

        <Card className={styles.panel}>
          <div className={styles.panelHeader}>
            <div>
              <h2 className={styles.panelTitle}>Lists</h2>
              <p className={styles.panelHint}>
                Filter by date range to find past and upcoming lists.
              </p>
            </div>
          </div>

          <div className={styles.filters}>
            <FormField label="Start">
              {({ id, describedBy }) => (
                <Input
                  id={id}
                  aria-describedby={describedBy}
                  type="date"
                  value={range.start}
                  onChange={(event) =>
                    setRange((prev) => ({
                      ...prev,
                      start: event.target.value,
                    }))
                  }
                />
              )}
            </FormField>
            <FormField label="End">
              {({ id, describedBy }) => (
                <Input
                  id={id}
                  aria-describedby={describedBy}
                  type="date"
                  value={range.end}
                  onChange={(event) =>
                    setRange((prev) => ({
                      ...prev,
                      end: event.target.value,
                    }))
                  }
                />
              )}
            </FormField>
          </div>

          {listsQuery.isPending ? <div>Loading lists…</div> : null}
          {listsQuery.isError ? (
            <div role="alert" className={styles.alert}>
              Failed to load lists.
            </div>
          ) : null}

          <div className={styles.list}>
            {lists.map((list) => (
              <div key={list.id} className={styles.listRow}>
                <div className={styles.listMain}>
                  {editingID === list.id ? (
                    <>
                      <Input
                        type="date"
                        value={editing.date}
                        onChange={(event) =>
                          setEditing((prev) => ({
                            ...prev,
                            date: event.target.value,
                          }))
                        }
                      />
                      <Input
                        value={editing.name}
                        onChange={(event) =>
                          setEditing((prev) => ({
                            ...prev,
                            name: event.target.value,
                          }))
                        }
                      />
                      <Input
                        value={editing.notes}
                        onChange={(event) =>
                          setEditing((prev) => ({
                            ...prev,
                            notes: event.target.value,
                          }))
                        }
                        placeholder="Notes"
                      />
                    </>
                  ) : (
                    <>
                      <div className={styles.listTitle}>{list.name}</div>
                      <div className={styles.listMeta}>
                        {list.list_date}
                        {list.notes ? ` • ${list.notes}` : ''}
                      </div>
                      <Link
                        className={styles.listLink}
                        to={`/shopping-lists/${list.id}`}
                      >
                        View list
                      </Link>
                    </>
                  )}
                </div>
                <div className={styles.listActions}>
                  {editingID === list.id ? (
                    <>
                      <Button
                        size="sm"
                        type="button"
                        onClick={() => saveEdit(list.id)}
                        disabled={updateMutation.isPending}
                      >
                        Save
                      </Button>
                      <Button
                        size="sm"
                        type="button"
                        onClick={cancelEdit}
                        disabled={updateMutation.isPending}
                      >
                        Cancel
                      </Button>
                    </>
                  ) : (
                    <>
                      <Button
                        size="sm"
                        type="button"
                        onClick={() => startEdit(list)}
                        disabled={deleteMutation.isPending}
                      >
                        Edit
                      </Button>
                      <Button
                        size="sm"
                        variant="danger"
                        type="button"
                        onClick={() => {
                          if (!window.confirm('Delete this list?')) return
                          deleteMutation.mutate(list.id)
                        }}
                        disabled={deleteMutation.isPending}
                      >
                        Delete
                      </Button>
                    </>
                  )}
                </div>
              </div>
            ))}

            {!listsQuery.isPending && lists.length === 0 ? (
              <div>No shopping lists in this range.</div>
            ) : null}
          </div>
        </Card>
      </div>
    </Page>
  )
}
