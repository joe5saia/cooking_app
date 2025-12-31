import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'

import { ApiError } from '../../api/client'
import {
  createAisle,
  createItem,
  deleteAisle,
  deleteItem,
  listAisles,
  listItems,
  updateAisle,
  updateItem,
  type GroceryAisle,
  type Item,
} from '../../api/items'
import { Button, Card, FormField, Input, Select } from '../components'
import { Page } from './Page'
import styles from './ItemListPage.module.css'

type AisleDraft = {
  name: string
  sortGroup: string
  sortOrder: string
  numericValue: string
}

type ItemDraft = {
  name: string
  storeURL: string
  aisleID: string
}

const sortGroupOptions = [
  { value: '0', label: 'Before numbered aisles' },
  { value: '1', label: 'Numbered aisles' },
  { value: '2', label: 'After numbered aisles' },
]

const defaultAisleDraft: AisleDraft = {
  name: '',
  sortGroup: '1',
  sortOrder: '0',
  numericValue: '',
}

const defaultItemDraft: ItemDraft = {
  name: '',
  storeURL: '',
  aisleID: '',
}

/** Parse an optional integer string. */
function parseOptionalInt(raw: string): number | null {
  const trimmed = raw.trim()
  if (trimmed === '') return null
  const parsed = Number(trimmed)
  if (!Number.isInteger(parsed)) {
    throw new Error('Value must be an integer')
  }
  return parsed
}

function sortAisles(a: GroceryAisle, b: GroceryAisle): number {
  if (a.sort_group !== b.sort_group) return a.sort_group - b.sort_group
  if (a.sort_order !== b.sort_order) return a.sort_order - b.sort_order
  const aNumeric = a.numeric_value ?? 0
  const bNumeric = b.numeric_value ?? 0
  if (aNumeric !== bNumeric) return aNumeric - bNumeric
  return a.name.localeCompare(b.name)
}

export function ItemListPage() {
  const queryClient = useQueryClient()
  const [itemQuery, setItemQuery] = useState('')
  const [itemDraft, setItemDraft] = useState<ItemDraft>(defaultItemDraft)
  const [aisleDraft, setAisleDraft] = useState<AisleDraft>(defaultAisleDraft)
  const [itemError, setItemError] = useState<string | null>(null)
  const [aisleError, setAisleError] = useState<string | null>(null)
  const [editingItemID, setEditingItemID] = useState<string | null>(null)
  const [editingItem, setEditingItem] = useState<ItemDraft>(defaultItemDraft)
  const [editingAisleID, setEditingAisleID] = useState<string | null>(null)
  const [editingAisle, setEditingAisle] =
    useState<AisleDraft>(defaultAisleDraft)

  const aislesQuery = useQuery({
    queryKey: ['aisles'],
    queryFn: listAisles,
  })

  const itemsQuery = useQuery({
    queryKey: ['items', itemQuery],
    queryFn: () => listItems({ q: itemQuery, limit: 200 }),
  })

  const aisles = useMemo(
    () => (aislesQuery.data ?? []).slice().sort(sortAisles),
    [aislesQuery.data],
  )
  const items = useMemo(() => itemsQuery.data ?? [], [itemsQuery.data])

  const createItemMutation = useMutation({
    mutationFn: (params: ItemDraft) =>
      createItem({
        name: params.name,
        store_url:
          params.storeURL.trim() === '' ? null : params.storeURL.trim(),
        aisle_id: params.aisleID === '' ? null : params.aisleID,
      }),
    onSuccess: async () => {
      setItemDraft(defaultItemDraft)
      setItemError(null)
      await queryClient.invalidateQueries({ queryKey: ['items'] })
    },
    onError: (err) => {
      setItemError(
        err instanceof ApiError ? err.message : 'Failed to add item.',
      )
    },
  })

  const updateItemMutation = useMutation({
    mutationFn: (params: { id: string; body: ItemDraft }) =>
      updateItem(params.id, {
        name: params.body.name,
        store_url:
          params.body.storeURL.trim() === ''
            ? null
            : params.body.storeURL.trim(),
        aisle_id: params.body.aisleID === '' ? null : params.body.aisleID,
      }),
    onSuccess: async () => {
      setEditingItemID(null)
      setEditingItem(defaultItemDraft)
      setItemError(null)
      await queryClient.invalidateQueries({ queryKey: ['items'] })
    },
    onError: (err) => {
      setItemError(
        err instanceof ApiError ? err.message : 'Failed to update item.',
      )
    },
  })

  const deleteItemMutation = useMutation({
    mutationFn: (id: string) => deleteItem(id),
    onSuccess: async () => {
      setItemError(null)
      await queryClient.invalidateQueries({ queryKey: ['items'] })
    },
    onError: (err) => {
      setItemError(
        err instanceof ApiError ? err.message : 'Failed to delete item.',
      )
    },
  })

  const createAisleMutation = useMutation({
    mutationFn: (params: AisleDraft) =>
      createAisle({
        name: params.name.trim(),
        sort_group: Number(params.sortGroup),
        sort_order: Number(params.sortOrder),
        numeric_value: parseOptionalInt(params.numericValue),
      }),
    onSuccess: async () => {
      setAisleDraft(defaultAisleDraft)
      setAisleError(null)
      await queryClient.invalidateQueries({ queryKey: ['aisles'] })
    },
    onError: (err) => {
      setAisleError(
        err instanceof ApiError ? err.message : 'Failed to add aisle.',
      )
    },
  })

  const updateAisleMutation = useMutation({
    mutationFn: (params: { id: string; body: AisleDraft }) =>
      updateAisle(params.id, {
        name: params.body.name.trim(),
        sort_group: Number(params.body.sortGroup),
        sort_order: Number(params.body.sortOrder),
        numeric_value: parseOptionalInt(params.body.numericValue),
      }),
    onSuccess: async () => {
      setEditingAisleID(null)
      setEditingAisle(defaultAisleDraft)
      setAisleError(null)
      await queryClient.invalidateQueries({ queryKey: ['aisles'] })
    },
    onError: (err) => {
      setAisleError(
        err instanceof ApiError ? err.message : 'Failed to update aisle.',
      )
    },
  })

  const deleteAisleMutation = useMutation({
    mutationFn: (id: string) => deleteAisle(id),
    onSuccess: async () => {
      setAisleError(null)
      await queryClient.invalidateQueries({ queryKey: ['aisles'] })
      await queryClient.invalidateQueries({ queryKey: ['items'] })
    },
    onError: (err) => {
      setAisleError(
        err instanceof ApiError ? err.message : 'Failed to delete aisle.',
      )
    },
  })

  function handleCreateItem(event: React.FormEvent) {
    event.preventDefault()
    setItemError(null)
    const trimmedName = itemDraft.name.trim()
    if (trimmedName === '') {
      setItemError('Name is required.')
      return
    }
    createItemMutation.mutate({ ...itemDraft, name: trimmedName })
  }

  function handleCreateAisle(event: React.FormEvent) {
    event.preventDefault()
    setAisleError(null)
    const trimmedName = aisleDraft.name.trim()
    if (trimmedName === '') {
      setAisleError('Name is required.')
      return
    }
    const sortGroup = Number(aisleDraft.sortGroup)
    const sortOrder = Number(aisleDraft.sortOrder)
    if (!Number.isInteger(sortGroup) || sortGroup < 0 || sortGroup > 2) {
      setAisleError('Sort group must be 0, 1, or 2.')
      return
    }
    if (!Number.isInteger(sortOrder) || sortOrder < 0) {
      setAisleError('Sort order must be 0 or higher.')
      return
    }
    try {
      parseOptionalInt(aisleDraft.numericValue)
    } catch {
      setAisleError('Numeric value must be an integer.')
      return
    }
    createAisleMutation.mutate({ ...aisleDraft, name: trimmedName })
  }

  function startItemEdit(item: Item) {
    setEditingItemID(item.id)
    setEditingItem({
      name: item.name,
      storeURL: item.store_url ?? '',
      aisleID: item.aisle?.id ?? '',
    })
  }

  function cancelItemEdit() {
    setEditingItemID(null)
    setEditingItem(defaultItemDraft)
  }

  function saveItemEdit(id: string) {
    setItemError(null)
    const trimmedName = editingItem.name.trim()
    if (trimmedName === '') {
      setItemError('Name is required.')
      return
    }
    updateItemMutation.mutate({
      id,
      body: { ...editingItem, name: trimmedName },
    })
  }

  function startAisleEdit(aisle: GroceryAisle) {
    setEditingAisleID(aisle.id)
    setEditingAisle({
      name: aisle.name,
      sortGroup: String(aisle.sort_group),
      sortOrder: String(aisle.sort_order),
      numericValue:
        aisle.numeric_value === null ? '' : String(aisle.numeric_value),
    })
  }

  function cancelAisleEdit() {
    setEditingAisleID(null)
    setEditingAisle(defaultAisleDraft)
  }

  function saveAisleEdit(id: string) {
    setAisleError(null)
    const trimmedName = editingAisle.name.trim()
    if (trimmedName === '') {
      setAisleError('Name is required.')
      return
    }
    const sortGroup = Number(editingAisle.sortGroup)
    const sortOrder = Number(editingAisle.sortOrder)
    if (!Number.isInteger(sortGroup) || sortGroup < 0 || sortGroup > 2) {
      setAisleError('Sort group must be 0, 1, or 2.')
      return
    }
    if (!Number.isInteger(sortOrder) || sortOrder < 0) {
      setAisleError('Sort order must be 0 or higher.')
      return
    }
    try {
      parseOptionalInt(editingAisle.numericValue)
    } catch {
      setAisleError('Numeric value must be an integer.')
      return
    }
    updateAisleMutation.mutate({
      id,
      body: { ...editingAisle, name: trimmedName },
    })
  }

  return (
    <Page title="Items">
      <div className={styles.layout}>
        <div className={styles.column}>
          <Card className={styles.panel}>
            <div className={styles.panelHeader}>
              <div>
                <h2 className={styles.panelTitle}>Items</h2>
                <p className={styles.panelHint}>
                  Search shared items and link them to aisles for shopping list
                  grouping.
                </p>
              </div>
            </div>

            {itemError ? (
              <div role="alert" className={styles.alert}>
                {itemError}
              </div>
            ) : null}

            <form onSubmit={handleCreateItem} className={styles.form}>
              <FormField label="Item name">
                {({ id, describedBy }) => (
                  <Input
                    id={id}
                    aria-describedby={describedBy}
                    value={itemDraft.name}
                    onChange={(event) =>
                      setItemDraft((prev) => ({
                        ...prev,
                        name: event.target.value,
                      }))
                    }
                    placeholder="e.g., whole milk"
                  />
                )}
              </FormField>
              <FormField label="Store URL">
                {({ id, describedBy }) => (
                  <Input
                    id={id}
                    aria-describedby={describedBy}
                    value={itemDraft.storeURL}
                    onChange={(event) =>
                      setItemDraft((prev) => ({
                        ...prev,
                        storeURL: event.target.value,
                      }))
                    }
                    placeholder="https://shop.example/item"
                  />
                )}
              </FormField>
              <FormField label="Aisle">
                {({ id, describedBy }) => (
                  <Select
                    id={id}
                    aria-describedby={describedBy}
                    value={itemDraft.aisleID}
                    onChange={(event) =>
                      setItemDraft((prev) => ({
                        ...prev,
                        aisleID: event.target.value,
                      }))
                    }
                  >
                    <option value="">Unassigned</option>
                    {aisles.map((aisle) => (
                      <option key={aisle.id} value={aisle.id}>
                        {aisle.name}
                      </option>
                    ))}
                  </Select>
                )}
              </FormField>
              <Button
                type="submit"
                variant="primary"
                disabled={createItemMutation.isPending}
              >
                {createItemMutation.isPending ? 'Adding…' : 'Add item'}
              </Button>
            </form>

            <div className={styles.searchRow}>
              <FormField label="Search items">
                {({ id, describedBy }) => (
                  <Input
                    id={id}
                    aria-describedby={describedBy}
                    value={itemQuery}
                    onChange={(event) => setItemQuery(event.target.value)}
                    placeholder="Type to filter"
                  />
                )}
              </FormField>
            </div>

            {itemsQuery.isPending ? <div>Loading items…</div> : null}
            {itemsQuery.isError ? (
              <div role="alert" className={styles.alert}>
                Failed to load items.
              </div>
            ) : null}

            <div className={styles.list}>
              {items.map((item) => (
                <div key={item.id} className={styles.listRow}>
                  <div className={styles.listMain}>
                    {editingItemID === item.id ? (
                      <>
                        <Input
                          value={editingItem.name}
                          onChange={(event) =>
                            setEditingItem((prev) => ({
                              ...prev,
                              name: event.target.value,
                            }))
                          }
                        />
                        <Input
                          value={editingItem.storeURL}
                          onChange={(event) =>
                            setEditingItem((prev) => ({
                              ...prev,
                              storeURL: event.target.value,
                            }))
                          }
                          placeholder="Store URL"
                        />
                        <Select
                          value={editingItem.aisleID}
                          onChange={(event) =>
                            setEditingItem((prev) => ({
                              ...prev,
                              aisleID: event.target.value,
                            }))
                          }
                        >
                          <option value="">Unassigned</option>
                          {aisles.map((aisle) => (
                            <option key={aisle.id} value={aisle.id}>
                              {aisle.name}
                            </option>
                          ))}
                        </Select>
                      </>
                    ) : (
                      <>
                        <div className={styles.listTitle}>{item.name}</div>
                        <div className={styles.listMeta}>
                          {item.aisle
                            ? `Aisle: ${item.aisle.name}`
                            : 'No aisle'}
                        </div>
                        {item.store_url ? (
                          <a
                            className={styles.listLink}
                            href={item.store_url}
                            target="_blank"
                            rel="noreferrer"
                          >
                            {item.store_url}
                          </a>
                        ) : null}
                      </>
                    )}
                  </div>
                  <div className={styles.listActions}>
                    {editingItemID === item.id ? (
                      <>
                        <Button
                          size="sm"
                          type="button"
                          onClick={() => saveItemEdit(item.id)}
                          disabled={updateItemMutation.isPending}
                        >
                          Save
                        </Button>
                        <Button
                          size="sm"
                          type="button"
                          onClick={cancelItemEdit}
                          disabled={updateItemMutation.isPending}
                        >
                          Cancel
                        </Button>
                      </>
                    ) : (
                      <>
                        <Button
                          size="sm"
                          type="button"
                          onClick={() => startItemEdit(item)}
                          disabled={deleteItemMutation.isPending}
                        >
                          Edit
                        </Button>
                        <Button
                          size="sm"
                          variant="danger"
                          type="button"
                          onClick={() => {
                            if (!window.confirm('Delete this item?')) return
                            deleteItemMutation.mutate(item.id)
                          }}
                          disabled={deleteItemMutation.isPending}
                        >
                          Delete
                        </Button>
                      </>
                    )}
                  </div>
                </div>
              ))}

              {!itemsQuery.isPending && items.length === 0 ? (
                <div>No items yet.</div>
              ) : null}
            </div>
          </Card>
        </div>

        <div className={styles.column}>
          <Card className={styles.panel}>
            <div className={styles.panelHeader}>
              <div>
                <h2 className={styles.panelTitle}>Aisles</h2>
                <p className={styles.panelHint}>
                  Sort groups control where aisles appear relative to numbered
                  aisles. Use numeric value for aisle numbers.
                </p>
              </div>
            </div>

            {aisleError ? (
              <div role="alert" className={styles.alert}>
                {aisleError}
              </div>
            ) : null}

            <form onSubmit={handleCreateAisle} className={styles.form}>
              <FormField label="Aisle name">
                {({ id, describedBy }) => (
                  <Input
                    id={id}
                    aria-describedby={describedBy}
                    value={aisleDraft.name}
                    onChange={(event) =>
                      setAisleDraft((prev) => ({
                        ...prev,
                        name: event.target.value,
                      }))
                    }
                    placeholder="e.g., Bakery"
                  />
                )}
              </FormField>
              <FormField label="Sort group">
                {({ id, describedBy }) => (
                  <Select
                    id={id}
                    aria-describedby={describedBy}
                    value={aisleDraft.sortGroup}
                    onChange={(event) =>
                      setAisleDraft((prev) => ({
                        ...prev,
                        sortGroup: event.target.value,
                      }))
                    }
                  >
                    {sortGroupOptions.map((opt) => (
                      <option key={opt.value} value={opt.value}>
                        {opt.label}
                      </option>
                    ))}
                  </Select>
                )}
              </FormField>
              <FormField label="Sort order">
                {({ id, describedBy }) => (
                  <Input
                    id={id}
                    aria-describedby={describedBy}
                    type="number"
                    inputMode="numeric"
                    min={0}
                    value={aisleDraft.sortOrder}
                    onChange={(event) =>
                      setAisleDraft((prev) => ({
                        ...prev,
                        sortOrder: event.target.value,
                      }))
                    }
                  />
                )}
              </FormField>
              <FormField label="Numeric value">
                {({ id, describedBy }) => (
                  <Input
                    id={id}
                    aria-describedby={describedBy}
                    type="number"
                    inputMode="numeric"
                    min={0}
                    value={aisleDraft.numericValue}
                    onChange={(event) =>
                      setAisleDraft((prev) => ({
                        ...prev,
                        numericValue: event.target.value,
                      }))
                    }
                    placeholder="Optional"
                  />
                )}
              </FormField>
              <Button
                type="submit"
                variant="primary"
                disabled={createAisleMutation.isPending}
              >
                {createAisleMutation.isPending ? 'Adding…' : 'Add aisle'}
              </Button>
            </form>

            {aislesQuery.isPending ? <div>Loading aisles…</div> : null}
            {aislesQuery.isError ? (
              <div role="alert" className={styles.alert}>
                Failed to load aisles.
              </div>
            ) : null}

            <div className={styles.list}>
              {aisles.map((aisle) => (
                <div key={aisle.id} className={styles.listRow}>
                  <div className={styles.listMain}>
                    {editingAisleID === aisle.id ? (
                      <>
                        <Input
                          value={editingAisle.name}
                          onChange={(event) =>
                            setEditingAisle((prev) => ({
                              ...prev,
                              name: event.target.value,
                            }))
                          }
                        />
                        <Select
                          value={editingAisle.sortGroup}
                          onChange={(event) =>
                            setEditingAisle((prev) => ({
                              ...prev,
                              sortGroup: event.target.value,
                            }))
                          }
                        >
                          {sortGroupOptions.map((opt) => (
                            <option key={opt.value} value={opt.value}>
                              {opt.label}
                            </option>
                          ))}
                        </Select>
                        <Input
                          type="number"
                          inputMode="numeric"
                          min={0}
                          value={editingAisle.sortOrder}
                          onChange={(event) =>
                            setEditingAisle((prev) => ({
                              ...prev,
                              sortOrder: event.target.value,
                            }))
                          }
                        />
                        <Input
                          type="number"
                          inputMode="numeric"
                          min={0}
                          placeholder="Numeric value"
                          value={editingAisle.numericValue}
                          onChange={(event) =>
                            setEditingAisle((prev) => ({
                              ...prev,
                              numericValue: event.target.value,
                            }))
                          }
                        />
                      </>
                    ) : (
                      <>
                        <div className={styles.listTitle}>{aisle.name}</div>
                        <div className={styles.listMeta}>
                          Group {aisle.sort_group} • Order {aisle.sort_order}
                          {aisle.numeric_value !== null
                            ? ` • #${aisle.numeric_value}`
                            : ''}
                        </div>
                      </>
                    )}
                  </div>
                  <div className={styles.listActions}>
                    {editingAisleID === aisle.id ? (
                      <>
                        <Button
                          size="sm"
                          type="button"
                          onClick={() => saveAisleEdit(aisle.id)}
                          disabled={updateAisleMutation.isPending}
                        >
                          Save
                        </Button>
                        <Button
                          size="sm"
                          type="button"
                          onClick={cancelAisleEdit}
                          disabled={updateAisleMutation.isPending}
                        >
                          Cancel
                        </Button>
                      </>
                    ) : (
                      <>
                        <Button
                          size="sm"
                          type="button"
                          onClick={() => startAisleEdit(aisle)}
                          disabled={deleteAisleMutation.isPending}
                        >
                          Edit
                        </Button>
                        <Button
                          size="sm"
                          variant="danger"
                          type="button"
                          onClick={() => {
                            if (!window.confirm('Delete this aisle?')) return
                            deleteAisleMutation.mutate(aisle.id)
                          }}
                          disabled={deleteAisleMutation.isPending}
                        >
                          Delete
                        </Button>
                      </>
                    )}
                  </div>
                </div>
              ))}

              {!aislesQuery.isPending && aisles.length === 0 ? (
                <div>No aisles yet.</div>
              ) : null}
            </div>
          </Card>
        </div>
      </div>
    </Page>
  )
}
