import type { ReactNode } from 'react'

export const Card = ({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) => {
  return (
    <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 shadow-[0_8px_28px_-18px_rgba(0,0,0,0.8)]">
      <header className="mb-4 flex items-center justify-between gap-3">
        <h2 className="text-sm font-bold uppercase tracking-[0.14em] text-[var(--muted)]">{title}</h2>
        {action}
      </header>
      {children}
    </section>
  )
}
