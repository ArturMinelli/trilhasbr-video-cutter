export const ProgressBar = ({ value }: { value: number }) => {
  const clamped = Math.max(0, Math.min(100, value))
  return (
    <div className="h-2 w-full overflow-hidden rounded-full bg-[var(--surface-soft)]">
      <div
        className="h-full rounded-full bg-[var(--primary)] transition-all duration-300"
        style={{ width: `${clamped}%` }}
      />
    </div>
  )
}
