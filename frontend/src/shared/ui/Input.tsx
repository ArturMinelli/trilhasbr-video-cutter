import type { InputHTMLAttributes } from 'react'

type Props = InputHTMLAttributes<HTMLInputElement>

export const Input = ({ className = '', ...props }: Props) => {
  return (
    <input
      {...props}
      className={`w-full rounded-md border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm text-[var(--text)] outline-none transition focus:border-[var(--primary)] focus:ring-2 focus:ring-[var(--primary-soft)] ${className}`}
    />
  )
}
