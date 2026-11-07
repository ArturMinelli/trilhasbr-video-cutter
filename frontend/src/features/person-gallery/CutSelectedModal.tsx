import { useState } from 'react'
import { z } from 'zod'

import { Button } from '../../shared/ui/Button'
import { Input } from '../../shared/ui/Input'

const schema = z.object({
  name: z.string().trim().min(1, 'Name is required').max(80, 'Name too long'),
  secondsBefore: z.coerce.number().int().min(0).max(86_400),
  secondsAfter: z.coerce.number().int().min(0).max(86_400),
})

export type CutSelectedInput = z.infer<typeof schema>

const defaultName = (): string => {
  const now = new Date()
  const date = now.toISOString().slice(0, 10).replaceAll('-', '')
  const time = `${String(now.getHours()).padStart(2, '0')}${String(now.getMinutes()).padStart(2, '0')}`
  return `people-${date}-${time}`
}

export const CutSelectedModal = ({
  open,
  count,
  submitting,
  onClose,
  onSubmit,
}: {
  open: boolean
  count: number
  submitting: boolean
  onClose: () => void
  onSubmit: (input: CutSelectedInput) => void
}) => {
  const [name, setName] = useState(defaultName)
  const [secondsBefore, setSecondsBefore] = useState('20')
  const [secondsAfter, setSecondsAfter] = useState('20')
  const [errorText, setErrorText] = useState<string | null>(null)

  if (!open) {
    return null
  }

  const submit = () => {
    const parsed = schema.safeParse({ name, secondsBefore, secondsAfter })
    if (!parsed.success) {
      setErrorText(parsed.error.issues.map((issue) => issue.message).join('; '))
      return
    }
    setErrorText(null)
    onSubmit(parsed.data)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="w-full max-w-xl rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-bold">
            Cut {count} {count === 1 ? 'person' : 'people'}
          </h2>
          <button type="button" onClick={onClose} className="text-sm text-[var(--muted)] hover:text-[var(--text)]">
            Close
          </button>
        </div>

        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <label className="flex flex-col gap-1 text-xs uppercase tracking-[0.08em] text-[var(--muted)] md:col-span-2">
            Name (output folder)
            <Input value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <label className="flex flex-col gap-1 text-xs uppercase tracking-[0.08em] text-[var(--muted)]">
            Seconds Before
            <Input value={secondsBefore} onChange={(event) => setSecondsBefore(event.target.value)} type="number" min={0} />
          </label>
          <label className="flex flex-col gap-1 text-xs uppercase tracking-[0.08em] text-[var(--muted)]">
            Seconds After
            <Input value={secondsAfter} onChange={(event) => setSecondsAfter(event.target.value)} type="number" min={0} />
          </label>
        </div>

        {errorText ? <p className="mt-3 text-sm text-[var(--danger)]">{errorText}</p> : null}

        <div className="mt-4 flex items-center justify-end gap-2">
          <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button type="button" onClick={submit} disabled={submitting}>
            {submitting ? 'Creating…' : 'Create Cuts'}
          </Button>
        </div>
      </div>
    </div>
  )
}
