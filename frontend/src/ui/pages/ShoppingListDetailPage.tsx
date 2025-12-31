import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'

import { ApiError } from '../../api/client'
import { createItem, type Item } from '../../api/items'
import {
  addShoppingListItems,
  deleteShoppingListItem,
  getShoppingList,
  updateShoppingListItemPurchase,
  type ShoppingListItem,
} from '../../api/shoppingLists'
import { Button, Card, FormField, Input, ItemAutocomplete } from '../components'
import { Page } from './Page'
import styles from './ShoppingListDetailPage.module.css'

type ManualItemDraft = {
  item_id: string
  item_name: string
  quantity: string
  quantity_text: string
  unit: string
}

type AisleGroup = {
  key: string
  name: string
  sortGroup: number
  sortOrder: number
  numericValue: number | null
  items: ShoppingListItem[]
}

const emptyDraft: ManualItemDraft = {
  item_id: '',
  item_name: '',
  quantity: '',
  quantity_text: '',
  unit: '',
}

/** Format a number without trailing zeros. */
function formatQuantityNumber(value: number): string {
  if (Number.isInteger(value)) return String(value)
  return value.toFixed(2).replace(/0+$/, '').replace(/\.$/, '')
}

function formatQuantity(item: ShoppingListItem): string {
  if (item.quantity !== null) {
    const qty = formatQuantityNumber(item.quantity)
    return item.unit ? `${qty} ${item.unit}` : qty
  }
  if (item.quantity_text) {
    return item.unit ? `${item.quantity_text} ${item.unit}` : item.quantity_text
  }
  if (item.unit) {
    return item.unit
  }
  return ''
}

function groupItems(items: ShoppingListItem[]): AisleGroup[] {
  const groups = new Map<string, AisleGroup>()

  for (const item of items) {
    const aisle = item.item.aisle
    const key = aisle?.id ?? 'unassigned'
    const existing = groups.get(key)
    const sortGroup = aisle?.sort_group ?? 3
    const sortOrder = aisle?.sort_order ?? 0
    const numericValue = aisle?.numeric_value ?? null
    const name = aisle?.name ?? 'Unassigned'

    if (!existing) {
      groups.set(key, {
        key,
        name,
        sortGroup,
        sortOrder,
        numericValue,
        items: [item],
      })
    } else {
      existing.items.push(item)
    }
  }

  const grouped = Array.from(groups.values())
  grouped.sort((a, b) => {
    if (a.sortGroup !== b.sortGroup) return a.sortGroup - b.sortGroup
    if (a.sortOrder !== b.sortOrder) return a.sortOrder - b.sortOrder
    const aNum = a.numericValue ?? 0
    const bNum = b.numericValue ?? 0
    if (aNum !== bNum) return aNum - bNum
    return a.name.localeCompare(b.name)
  })
  for (const group of grouped) {
    group.items.sort((a, b) => a.item.name.localeCompare(b.item.name))
  }
  return grouped
}

export function ShoppingListDetailPage() {
  const { id } = useParams()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<ManualItemDraft>(emptyDraft)
  const [error, setError] = useState<string | null>(null)

  const listQuery = useQuery({
    queryKey: ['shopping-lists', 'detail', id],
    queryFn: () => getShoppingList(id ?? ''),
    enabled: Boolean(id),
  })

  const addItemMutation = useMutation({
    mutationFn: async (params: {
      listID: string
      itemID: string
      quantity: number | null
      quantityText: string | null
      unit: string | null
    }) =>
      addShoppingListItems(params.listID, [
        {
          item_id: params.itemID,
          quantity: params.quantity,
          quantity_text: params.quantityText,
          unit: params.unit,
        },
      ]),
    onSuccess: async () => {
      setDraft(emptyDraft)
      setError(null)
      await queryClient.invalidateQueries({
        queryKey: ['shopping-lists', 'detail', id],
      })
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to add item.')
    },
  })

  const purchaseMutation = useMutation({
    mutationFn: (params: {
      listID: string
      itemID: string
      isPurchased: boolean
    }) =>
      updateShoppingListItemPurchase(
        params.listID,
        params.itemID,
        params.isPurchased,
      ),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['shopping-lists', 'detail', id],
      })
    },
  })

  const deleteItemMutation = useMutation({
    mutationFn: (params: { listID: string; itemID: string }) =>
      deleteShoppingListItem(params.listID, params.itemID),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['shopping-lists', 'detail', id],
      })
    },
  })

  async function handleAddManualItem(event: React.FormEvent) {
    event.preventDefault()
    setError(null)
    if (!id) return

    const trimmedName = draft.item_name.trim()
    if (trimmedName === '') {
      setError('Item name is required.')
      return
    }
    let quantity: number | null = null
    const trimmedQuantity = draft.quantity.trim()
    if (trimmedQuantity !== '') {
      const parsed = Number(trimmedQuantity)
      if (!Number.isFinite(parsed)) {
        setError('Quantity must be a number.')
        return
      }
      quantity = parsed
    }
    let itemID = draft.item_id.trim()
    if (itemID === '') {
      try {
        const created: Item = await createItem({ name: trimmedName })
        itemID = created.id
        await queryClient.invalidateQueries({ queryKey: ['items'] })
      } catch (err) {
        setError(
          err instanceof ApiError ? err.message : 'Failed to create item.',
        )
        return
      }
    }
    addItemMutation.mutate({
      listID: id,
      itemID,
      quantity,
      quantityText:
        draft.quantity_text.trim() === '' ? null : draft.quantity_text.trim(),
      unit: draft.unit.trim() === '' ? null : draft.unit.trim(),
    })
  }

  if (!id) {
    return (
      <Page title="Shopping List">
        <div role="alert" className={styles.alert}>
          Missing shopping list id.
        </div>
      </Page>
    )
  }

  const list = listQuery.data ?? null
  const groupedItems = list ? groupItems(list.items) : []

  return (
    <Page title={list?.name ?? 'Shopping List'}>
      <div className={styles.layout}>
        {error ? (
          <div role="alert" className={styles.alert}>
            {error}
          </div>
        ) : null}

        {listQuery.isPending ? <div>Loading list…</div> : null}
        {listQuery.isError ? (
          <div role="alert" className={styles.alert}>
            {listQuery.error instanceof ApiError &&
            listQuery.error.status === 404
              ? 'Shopping list not found.'
              : 'Failed to load shopping list.'}
          </div>
        ) : null}

        {list ? (
          <>
            <Card className={styles.headerCard}>
              <div className={styles.headerTitle}>{list.name}</div>
              <div className={styles.headerMeta}>
                <span>Date: {list.list_date}</span>
                <span>Updated: {list.updated_at}</span>
              </div>
              {list.notes ? <div>{list.notes}</div> : null}
              <Link to="/shopping-lists">Back to lists</Link>
            </Card>

            <Card className={styles.panel}>
              <div>
                <h2 className={styles.panelTitle}>Add item</h2>
                <div className={styles.panelHint}>
                  Add a custom line or choose an existing item.
                </div>
              </div>
              <form onSubmit={handleAddManualItem} className={styles.formGrid}>
                <FormField label="Item">
                  {({ id: fieldID, describedBy }) => (
                    <ItemAutocomplete
                      inputId={fieldID}
                      ariaLabel="Item"
                      ariaDescribedBy={describedBy}
                      value={{
                        item_id: draft.item_id,
                        item_name: draft.item_name,
                      }}
                      onChange={(next) =>
                        setDraft((prev) => ({
                          ...prev,
                          item_id: next.item_id,
                          item_name: next.item_name,
                        }))
                      }
                      disabled={addItemMutation.isPending}
                    />
                  )}
                </FormField>
                <div className={styles.formRow}>
                  <FormField label="Quantity">
                    {({ id: fieldID, describedBy }) => (
                      <Input
                        id={fieldID}
                        aria-describedby={describedBy}
                        value={draft.quantity}
                        onChange={(event) =>
                          setDraft((prev) => ({
                            ...prev,
                            quantity: event.target.value,
                          }))
                        }
                        placeholder="e.g., 2"
                      />
                    )}
                  </FormField>
                  <FormField label="Quantity text">
                    {({ id: fieldID, describedBy }) => (
                      <Input
                        id={fieldID}
                        aria-describedby={describedBy}
                        value={draft.quantity_text}
                        onChange={(event) =>
                          setDraft((prev) => ({
                            ...prev,
                            quantity_text: event.target.value,
                          }))
                        }
                        placeholder="e.g., to taste"
                      />
                    )}
                  </FormField>
                  <FormField label="Unit">
                    {({ id: fieldID, describedBy }) => (
                      <Input
                        id={fieldID}
                        aria-describedby={describedBy}
                        value={draft.unit}
                        onChange={(event) =>
                          setDraft((prev) => ({
                            ...prev,
                            unit: event.target.value,
                          }))
                        }
                        placeholder="e.g., lb"
                      />
                    )}
                  </FormField>
                </div>
                <Button
                  type="submit"
                  variant="primary"
                  disabled={addItemMutation.isPending}
                >
                  {addItemMutation.isPending ? 'Adding…' : 'Add to list'}
                </Button>
              </form>
            </Card>

            <Card className={styles.panel}>
              <div>
                <h2 className={styles.panelTitle}>Items</h2>
                <div className={styles.panelHint}>
                  Quantities are aggregated across the recipes you added.
                </div>
              </div>
              {groupedItems.length === 0 ? (
                <div className={styles.muted}>No items yet.</div>
              ) : (
                <div className={styles.listGroup}>
                  {groupedItems.map((group) => (
                    <div key={group.key} className={styles.aisleBlock}>
                      <div className={styles.aisleTitle}>{group.name}</div>
                      <div className={styles.items}>
                        {group.items.map((item) => {
                          const quantityLabel = formatQuantity(item)
                          return (
                            <div key={item.id} className={styles.itemRow}>
                              <div className={styles.itemInfo}>
                                <div className={styles.itemTitle}>
                                  <span
                                    className={
                                      item.is_purchased ? styles.purchased : ''
                                    }
                                  >
                                    <span className={styles.itemName}>
                                      {item.item.name}
                                    </span>
                                  </span>
                                  {quantityLabel ? (
                                    <span className={styles.itemQuantity}>
                                      {quantityLabel}
                                    </span>
                                  ) : null}
                                </div>
                                <div className={styles.itemMeta}>
                                  {item.item.store_url ? (
                                    <a
                                      href={item.item.store_url}
                                      target="_blank"
                                      rel="noreferrer"
                                    >
                                      Store link
                                    </a>
                                  ) : (
                                    'No store link'
                                  )}
                                </div>
                              </div>
                              <div className={styles.itemActions}>
                                <label className={styles.purchaseToggle}>
                                  <input
                                    type="checkbox"
                                    checked={item.is_purchased}
                                    onChange={() =>
                                      purchaseMutation.mutate({
                                        listID: list.id,
                                        itemID: item.id,
                                        isPurchased: !item.is_purchased,
                                      })
                                    }
                                  />
                                  <span className={styles.purchaseLabel}>
                                    Purchased
                                  </span>
                                </label>
                                <Button
                                  size="sm"
                                  variant="danger"
                                  type="button"
                                  onClick={() => {
                                    if (
                                      !window.confirm(
                                        'Remove this item from the list?',
                                      )
                                    ) {
                                      return
                                    }
                                    deleteItemMutation.mutate({
                                      listID: list.id,
                                      itemID: item.id,
                                    })
                                  }}
                                  disabled={deleteItemMutation.isPending}
                                >
                                  Remove
                                </Button>
                              </div>
                            </div>
                          )
                        })}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </Card>
          </>
        ) : null}
      </div>
    </Page>
  )
}
