export const StatusBadge = ({ status }: { status: 'PENDING' | 'RUNNING' | 'SUCCEEDED' | 'FAILED' }) => {
  const palette =
    status === 'SUCCEEDED'
      ? 'bg-[var(--success-soft)] text-[var(--success)]'
      : status === 'FAILED'
      ? 'bg-[var(--danger-soft)] text-[var(--danger)]'
      : status === 'RUNNING'
      ? 'bg-[var(--primary-soft)] text-[var(--primary)]'
      : 'bg-[var(--surface-soft)] text-[var(--muted)]'

  return <span className={`rounded-full px-3 py-1 text-xs font-semibold ${palette}`}>{status}</span>
}
