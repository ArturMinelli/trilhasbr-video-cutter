import type { ButtonHTMLAttributes } from 'react'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'danger'
}

const variantMap: Record<NonNullable<Props['variant']>, string> = {
  primary: 'bg-[var(--primary)] text-white hover:brightness-110',
  secondary: 'bg-[var(--surface-soft)] text-[var(--text)] hover:bg-[var(--surface)]',
  danger: 'bg-[var(--danger)] text-white hover:brightness-110',
}

export const Button = ({ className = '', variant = 'primary', ...props }: Props) => {
  return (
    <button
      {...props}
      className={`inline-flex items-center justify-center rounded-md px-3 py-2 text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-60 ${variantMap[variant]} ${className}`}
    />
  )
}
