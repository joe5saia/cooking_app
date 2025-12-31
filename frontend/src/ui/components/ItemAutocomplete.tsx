import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'

import { listItems, type Item } from '../../api/items'
import { Input } from './Input'
import styles from './ItemAutocomplete.module.css'

export type ItemAutocompleteValue = {
  item_id: string
  item_name: string
}

type ItemAutocompleteProps = {
  value: ItemAutocompleteValue
  onChange: (value: ItemAutocompleteValue) => void
  placeholder?: string
  disabled?: boolean
  inputId?: string
  ariaLabel?: string
  ariaDescribedBy?: string
  inputClassName?: string
}

const suggestionLimit = 8

/** Item name picker with lightweight async suggestions. */
export function ItemAutocomplete({
  value,
  onChange,
  placeholder,
  disabled,
  inputId,
  ariaLabel,
  ariaDescribedBy,
  inputClassName,
}: ItemAutocompleteProps) {
  const [isOpen, setIsOpen] = useState(false)
  const query = value.item_name.trim()

  const itemsQuery = useQuery({
    queryKey: ['items', 'autocomplete', query],
    queryFn: () => listItems({ q: query, limit: suggestionLimit }),
    enabled: isOpen && query.length > 0,
  })

  const suggestions = useMemo(() => itemsQuery.data ?? [], [itemsQuery.data])
  const exactMatch = useMemo(
    () =>
      suggestions.find(
        (item) => item.name.toLowerCase() === query.toLowerCase(),
      ),
    [suggestions, query],
  )

  function handleSelect(item: Item) {
    onChange({ item_id: item.id, item_name: item.name })
    setIsOpen(false)
  }

  return (
    <div className={styles.wrapper}>
      <Input
        id={inputId}
        aria-label={ariaLabel}
        aria-describedby={ariaDescribedBy}
        value={value.item_name}
        onChange={(event) =>
          onChange({ item_id: '', item_name: event.target.value })
        }
        onFocus={() => setIsOpen(true)}
        onBlur={() => {
          window.setTimeout(() => setIsOpen(false), 120)
        }}
        placeholder={placeholder}
        disabled={disabled}
        autoComplete="off"
        className={inputClassName}
      />
      {isOpen && query.length > 0 ? (
        <div className={styles.panel} role="listbox">
          {itemsQuery.isPending ? (
            <div className={styles.status}>Searching…</div>
          ) : null}
          {!itemsQuery.isPending && suggestions.length === 0 ? (
            <div className={styles.status}>No matches. Press enter to add.</div>
          ) : null}
          {suggestions.map((item) => (
            <button
              key={item.id}
              type="button"
              className={styles.option}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => handleSelect(item)}
            >
              <span className={styles.optionName}>{item.name}</span>
              {item.aisle ? (
                <span className={styles.optionMeta}>{item.aisle.name}</span>
              ) : null}
            </button>
          ))}
          {exactMatch ? (
            <div className={styles.status}>Using existing item.</div>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
