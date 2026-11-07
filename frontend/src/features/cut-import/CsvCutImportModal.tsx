import type { CSVImportCut, CSVImportIssue } from '../../infrastructure/backend/types'
import { Button } from '../../shared/ui/Button'

export const CsvCutImportModal = ({
  open,
  fileName,
  cuts,
  issues,
  submitting,
  onClose,
  onConfirm,
}: {
  open: boolean
  fileName: string
  cuts: CSVImportCut[]
  issues: CSVImportIssue[]
  submitting: boolean
  onClose: () => void
  onConfirm: () => void
}) => {
  if (!open) {
    return null
  }

  const hasIssues = issues.length > 0

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
      <div className="max-h-[90vh] w-full max-w-5xl overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--surface)]">
        <div className="flex items-center justify-between border-b border-[var(--border)] p-4">
          <div>
            <h2 className="text-lg font-bold">CSV Cut Import</h2>
            <p className="text-xs text-[var(--muted)]">{fileName || 'Selected file'}</p>
          </div>
          <button type="button" onClick={onClose} className="text-sm text-[var(--muted)] hover:text-[var(--text)]" disabled={submitting}>
            Close
          </button>
        </div>

        <div className="max-h-[62vh] space-y-4 overflow-auto p-4">
          {hasIssues ? (
            <div className="rounded-lg border border-[var(--danger)] bg-[var(--danger-soft)] p-3">
              <p className="text-sm font-semibold text-[var(--danger)]">Found {issues.length} issue{issues.length === 1 ? '' : 's'} in CSV</p>
              <p className="mt-1 text-xs text-[var(--danger)]">Fix the file and import it again.</p>
            </div>
          ) : null}

          {cuts.length > 0 ? (
            <div className="overflow-hidden rounded-lg border border-[var(--border)]">
              <table className="min-w-full border-collapse text-left text-sm">
                <thead className="bg-[var(--surface-soft)] text-xs uppercase tracking-[0.08em] text-[var(--muted)]">
                  <tr>
                    <th className="px-3 py-2">Row</th>
                    <th className="px-3 py-2">Name</th>
                    <th className="px-3 py-2">Datetime</th>
                    <th className="px-3 py-2">Seconds Before</th>
                    <th className="px-3 py-2">Seconds After</th>
                    <th className="px-3 py-2">Segments</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border)]">
                  {cuts.map((cut) => (
                    <tr key={`${cut.row}-${cut.name}`}>
                      <td className="px-3 py-2">{cut.row}</td>
                      <td className="px-3 py-2 font-semibold">{cut.name}</td>
                      <td className="px-3 py-2">{cut.datetimeLocal}</td>
                      <td className="px-3 py-2">{cut.secondsBefore}</td>
                      <td className="px-3 py-2">{cut.secondsAfter}</td>
                      <td className="px-3 py-2">{cut.totalSegments}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : null}

          {issues.length > 0 ? (
            <div className="overflow-hidden rounded-lg border border-[var(--border)]">
              <table className="min-w-full border-collapse text-left text-sm">
                <thead className="bg-[var(--surface-soft)] text-xs uppercase tracking-[0.08em] text-[var(--muted)]">
                  <tr>
                    <th className="px-3 py-2">Row</th>
                    <th className="px-3 py-2">Column</th>
                    <th className="px-3 py-2">Message</th>
                    <th className="px-3 py-2">Value</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border)]">
                  {issues.map((issue, index) => (
                    <tr key={`${issue.row}-${issue.column}-${index}`}>
                      <td className="px-3 py-2">{issue.row}</td>
                      <td className="px-3 py-2">{issue.column || '-'}</td>
                      <td className="px-3 py-2 text-[var(--danger)]">{issue.message}</td>
                      <td className="px-3 py-2">{issue.value || '-'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : null}
        </div>

        <div className="flex items-center justify-end gap-2 border-t border-[var(--border)] p-4">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="button" onClick={onConfirm} disabled={submitting || hasIssues || cuts.length === 0}>
            {submitting ? 'Starting…' : `Confirm ${cuts.length} Cut${cuts.length === 1 ? '' : 's'}`}
          </Button>
        </div>
      </div>
    </div>
  )
}
