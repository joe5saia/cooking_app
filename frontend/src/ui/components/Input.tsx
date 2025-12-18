import { type InputHTMLAttributes } from 'react'

import styles from './Input.module.css'

export type InputProps = InputHTMLAttributes<HTMLInputElement> & {
  invalid?: boolean
}

export function Input({ invalid, className, ...props }: InputProps) {
  const base = styles.input
  return (
    <input
      {...props}
      aria-invalid={invalid ? 'true' : undefined}
      data-invalid={invalid ? 'true' : undefined}
      className={className ? `${base} ${className}` : base}
    />
  )
}
