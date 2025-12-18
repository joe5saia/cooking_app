import { type ReactNode, useId } from 'react'

import styles from './FormField.module.css'

export type FormFieldControlProps = {
  id: string
  describedBy?: string
  invalid: boolean
}

export type FormFieldProps = {
  label: string
  description?: string
  error?: string
  required?: boolean
  id?: string
  children: (control: FormFieldControlProps) => ReactNode
}

function joinIDs(ids: Array<string | undefined>) {
  const value = ids.filter(Boolean).join(' ')
  return value ? value : undefined
}

export function FormField({
  label,
  description,
  error,
  required,
  id,
  children,
}: FormFieldProps) {
  const reactID = useId()
  const controlID = id ?? `field-${reactID}`
  const descriptionID = description ? `${controlID}-description` : undefined
  const errorID = error ? `${controlID}-error` : undefined

  const describedBy = joinIDs([descriptionID, errorID])

  return (
    <div className={styles.field}>
      <div className={styles.labelRow}>
        <label className={styles.label} htmlFor={controlID}>
          {label}
        </label>
        {required ? <div className={styles.required}>Required</div> : null}
      </div>

      {children({ id: controlID, describedBy, invalid: Boolean(error) })}

      {description ? (
        <div id={descriptionID} className={styles.help}>
          {description}
        </div>
      ) : null}

      {error ? (
        <div id={errorID} className={styles.error} role="alert">
          {error}
        </div>
      ) : null}
    </div>
  )
}
