import { type ButtonHTMLAttributes } from 'react'
import { Link, type LinkProps } from 'react-router-dom'

import styles from './Button.module.css'

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'
export type ButtonSize = 'sm' | 'md'

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant
  size?: ButtonSize
}

function buttonClassName(params: { variant: ButtonVariant; size: ButtonSize }) {
  const variantClassByVariant: Record<ButtonVariant, string> = {
    primary: styles.variantPrimary,
    secondary: styles.variantSecondary,
    ghost: styles.variantGhost,
    danger: styles.variantDanger,
  }

  const sizeClassBySize: Record<ButtonSize, string> = {
    sm: styles.sizeSm,
    md: styles.sizeMd,
  }

  const variantClass = variantClassByVariant[params.variant]
  const sizeClass = sizeClassBySize[params.size]

  return `${styles.button} ${variantClass} ${sizeClass}`
}

export function Button({
  variant = 'secondary',
  size = 'md',
  type = 'button',
  className,
  ...props
}: ButtonProps) {
  const base = buttonClassName({ variant, size })
  return (
    <button
      {...props}
      type={type}
      data-variant={variant}
      data-size={size}
      className={className ? `${base} ${className}` : base}
    />
  )
}

export type ButtonLinkProps = LinkProps & {
  variant?: ButtonVariant
  size?: ButtonSize
}

export function ButtonLink({
  variant = 'secondary',
  size = 'md',
  className,
  ...props
}: ButtonLinkProps) {
  const base = buttonClassName({ variant, size })
  return (
    <Link
      {...props}
      data-variant={variant}
      data-size={size}
      className={className ? `${base} ${className}` : base}
    />
  )
}
