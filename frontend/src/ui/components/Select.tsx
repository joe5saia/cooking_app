import { type SelectHTMLAttributes } from 'react'

import styles from './Select.module.css'

export type SelectProps = SelectHTMLAttributes<HTMLSelectElement> & {
  invalid?: boolean
}

export function Select({ invalid, className, ...props }: SelectProps) {
  const base = styles.select
  return (
    <select
      {...props}
      aria-invalid={invalid ? 'true' : undefined}
      data-invalid={invalid ? 'true' : undefined}
      className={className ? `${base} ${className}` : base}
    />
  )
}
