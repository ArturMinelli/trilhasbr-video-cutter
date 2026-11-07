import type { ReactNode } from 'react'

export const Table = ({ header, children }: { header: ReactNode; children: ReactNode }) => {
  return (
    <div className="overflow-hidden rounded-lg border border-[var(--border)]">
      <table className="min-w-full border-collapse text-left text-sm">
        <thead className="bg-[var(--surface-soft)] text-xs uppercase tracking-[0.08em] text-[var(--muted)]">{header}</thead>
        <tbody className="divide-y divide-[var(--border)]">{children}</tbody>
      </table>
    </div>
  )
}
