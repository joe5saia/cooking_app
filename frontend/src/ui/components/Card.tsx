import { type HTMLAttributes } from 'react'

import styles from './Card.module.css'

export type CardPadding = 'sm' | 'md'

export type CardProps = HTMLAttributes<HTMLDivElement> & {
  padding?: CardPadding
}

export function Card({ padding = 'md', className, ...props }: CardProps) {
  const paddingClass = padding === 'sm' ? styles.paddingSm : styles.paddingMd
  const base = `${styles.card} ${paddingClass}`
  return (
    <div
      {...props}
      data-padding={padding}
      className={className ? `${base} ${className}` : base}
    />
  )
}
