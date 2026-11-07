import { useEffect, useMemo, useState } from 'react'

import type { VideoExplorer } from '../../infrastructure/backend/types'
import { Button } from '../../shared/ui/Button'
import { Input } from '../../shared/ui/Input'

type ShiftDirection = 'forward' | 'backward'
type ModalStep = 'adjust' | 'confirm'

const quickPresetMilliseconds = [1_000, 5_000, 30_000, 60_000, 5 * 60_000, 60 * 60_000]

const timeFormatter = new Intl.DateTimeFormat(undefined, {
  year: 'numeric',
  month: 'short',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  fractionalSecondDigits: 3,
})

const parsePart = (value: string): number => {
  if (!value.trim()) {
    return 0
  }
  const parsed = Number.parseInt(value, 10)
  if (!Number.isFinite(parsed) || parsed < 0) {
    return 0
  }
  return parsed
}

const toDurationParts = (milliseconds: number): { hours: string; minutes: string; seconds: string; millis: string } => {
  const safe = Math.max(0, Math.floor(milliseconds))
  const hours = Math.floor(safe / 3_600_000)
  const minutes = Math.floor((safe % 3_600_000) / 60_000)
  const seconds = Math.floor((safe % 60_000) / 1_000)
  const millis = safe % 1_000
  return {
    hours: String(hours),
    minutes: String(minutes),
    seconds: String(seconds),
    millis: String(millis),
  }
}

const formatShift = (shiftMilliseconds: number): string => {
  const sign = shiftMilliseconds >= 0 ? '+' : '-'
  const safe = Math.abs(shiftMilliseconds)
  const hours = Math.floor(safe / 3_600_000)
  const minutes = Math.floor((safe % 3_600_000) / 60_000)
  const seconds = Math.floor((safe % 60_000) / 1_000)
  const millis = safe % 1_000
  return `${sign}${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}.${String(
    millis,
  ).padStart(3, '0')}`
}

const formatTimestamp = (value?: string): string => {
  if (!value) {
    return '-'
  }
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }
  return timeFormatter.format(parsed)
}

export type ShiftTimeModalInput = {
  videoIds: number[]
  shiftMilliseconds: number
}

export const ShiftTimeModal = ({
  open,
  videos,
  submitting,
  onClose,
  onSubmit,
}: {
  open: boolean
  videos: VideoExplorer[]
  submitting: boolean
  onClose: () => void
  onSubmit: (input: ShiftTimeModalInput) => void
}) => {
  const [direction, setDirection] = useState<ShiftDirection>('forward')
  const [hours, setHours] = useState('0')
  const [minutes, setMinutes] = useState('0')
  const [seconds, setSeconds] = useState('30')
  const [millis, setMillis] = useState('0')
  const [step, setStep] = useState<ModalStep>('adjust')
  const [confirmCountdown, setConfirmCountdown] = useState(0)
  const [errorText, setErrorText] = useState<string | null>(null)

  useEffect(() => {
    if (!open) {
      return
    }
    setDirection('forward')
    setHours('0')
    setMinutes('0')
    setSeconds('30')
    setMillis('0')
    setStep('adjust')
    setConfirmCountdown(0)
    setErrorText(null)
  }, [open])

  useEffect(() => {
    if (!open || step !== 'confirm') {
      return
    }

    setConfirmCountdown(5)
    const handle = window.setInterval(() => {
      setConfirmCountdown((current) => {
        if (current <= 1) {
          window.clearInterval(handle)
          return 0
        }
        return current - 1
      })
    }, 1_000)

    return () => window.clearInterval(handle)
  }, [open, step])

  const shiftMagnitudeMilliseconds = useMemo(() => {
    const parsedHours = parsePart(hours)
    const parsedMinutes = parsePart(minutes)
    const parsedSeconds = parsePart(seconds)
    const parsedMillis = parsePart(millis)

    return parsedHours * 3_600_000 + parsedMinutes * 60_000 + parsedSeconds * 1_000 + parsedMillis
  }, [hours, minutes, seconds, millis])

  const signedShiftMilliseconds = direction === 'forward' ? shiftMagnitudeMilliseconds : -shiftMagnitudeMilliseconds

  const previewRows = useMemo(() => {
    return videos.map((video) => {
      const startedAt = video.startedAt ? new Date(video.startedAt) : null
      const finishedAt = video.finishedAt ? new Date(video.finishedAt) : null

      const startedAtAfter = startedAt ? new Date(startedAt.getTime() + signedShiftMilliseconds) : null
      const finishedAtAfter = finishedAt ? new Date(finishedAt.getTime() + signedShiftMilliseconds) : null

      return {
        id: video.id,
        name: video.name || 'Untitled video',
        relativePath: video.relativePath,
        beforeStart: startedAt,
        beforeFinish: finishedAt,
        afterStart: startedAtAfter,
        afterFinish: finishedAtAfter,
      }
    })
  }, [videos, signedShiftMilliseconds])

  if (!open) {
    return null
  }

  const applyPreset = (milliseconds: number) => {
    const parts = toDurationParts(milliseconds)
    setHours(parts.hours)
    setMinutes(parts.minutes)
    setSeconds(parts.seconds)
    setMillis(parts.millis)
  }

  const openConfirmStep = () => {
    if (shiftMagnitudeMilliseconds <= 0) {
      setErrorText('Choose a shift greater than 0 milliseconds.')
      return
    }
    if (videos.length === 0) {
      setErrorText('Select at least one video to continue.')
      return
    }

    setErrorText(null)
    setStep('confirm')
  }

  const submit = () => {
    if (shiftMagnitudeMilliseconds <= 0) {
      setErrorText('Choose a shift greater than 0 milliseconds.')
      return
    }

    setErrorText(null)
    onSubmit({
      videoIds: videos.map((video) => video.id),
      shiftMilliseconds: signedShiftMilliseconds,
    })
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-[radial-gradient(circle_at_top,rgba(40,160,240,0.36),rgba(4,10,22,0.94))] p-4 backdrop-blur-sm">
      <div className="max-h-[92vh] w-full max-w-6xl overflow-hidden rounded-2xl border border-[rgba(158,176,200,0.32)] bg-[linear-gradient(155deg,rgba(17,29,51,0.98),rgba(10,18,34,0.98))] shadow-[0_28px_60px_-28px_rgba(0,0,0,0.95)]">
        <div className="flex items-center justify-between border-b border-[var(--border)] px-5 py-4">
          <div>
            <p className="text-xs font-bold uppercase tracking-[0.15em] text-[var(--muted)]">Batch Timestamp Shift</p>
            <h2 className="mt-1 text-xl font-black tracking-[0.04em]">
              {step === 'adjust' ? 'Adjust Time Shift' : 'Confirm Timestamp Rewrite'}
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md border border-[var(--border)] px-3 py-1.5 text-sm text-[var(--muted)] transition hover:border-[var(--primary)] hover:text-[var(--text)]"
          >
            Close
          </button>
        </div>

        {step === 'adjust' ? (
          <div className="grid max-h-[calc(92vh-88px)] grid-cols-1 gap-5 overflow-y-auto p-5 xl:grid-cols-[340px_minmax(0,1fr)]">
            <aside className="rounded-xl border border-[var(--border)] bg-[rgba(13,24,44,0.75)] p-4">
              <p className="text-xs font-bold uppercase tracking-[0.12em] text-[var(--muted)]">1. Direction</p>
              <div className="mt-2 grid grid-cols-2 gap-2">
                <button
                  type="button"
                  onClick={() => setDirection('backward')}
                  className={`rounded-lg border px-3 py-2 text-sm font-semibold transition ${
                    direction === 'backward'
                      ? 'border-[var(--primary)] bg-[var(--primary-soft)] text-[var(--text)]'
                      : 'border-[var(--border)] text-[var(--muted)] hover:border-[var(--primary)] hover:text-[var(--text)]'
                  }`}
                >
                  Shift Backward
                </button>
                <button
                  type="button"
                  onClick={() => setDirection('forward')}
                  className={`rounded-lg border px-3 py-2 text-sm font-semibold transition ${
                    direction === 'forward'
                      ? 'border-[var(--primary)] bg-[var(--primary-soft)] text-[var(--text)]'
                      : 'border-[var(--border)] text-[var(--muted)] hover:border-[var(--primary)] hover:text-[var(--text)]'
                  }`}
                >
                  Shift Forward
                </button>
              </div>

              <p className="mt-5 text-xs font-bold uppercase tracking-[0.12em] text-[var(--muted)]">2. Exact Amount</p>
              <div className="mt-2 grid grid-cols-2 gap-2">
                <label className="text-xs text-[var(--muted)]">
                  Hours
                  <Input type="number" min={0} value={hours} onChange={(event) => setHours(event.target.value)} />
                </label>
                <label className="text-xs text-[var(--muted)]">
                  Minutes
                  <Input type="number" min={0} value={minutes} onChange={(event) => setMinutes(event.target.value)} />
                </label>
                <label className="text-xs text-[var(--muted)]">
                  Seconds
                  <Input type="number" min={0} value={seconds} onChange={(event) => setSeconds(event.target.value)} />
                </label>
                <label className="text-xs text-[var(--muted)]">
                  Milliseconds
                  <Input type="number" min={0} value={millis} onChange={(event) => setMillis(event.target.value)} />
                </label>
              </div>

              <p className="mt-5 text-xs font-bold uppercase tracking-[0.12em] text-[var(--muted)]">Quick Presets</p>
              <div className="mt-2 flex flex-wrap gap-2">
                {quickPresetMilliseconds.map((preset) => (
                  <button
                    key={preset}
                    type="button"
                    onClick={() => applyPreset(preset)}
                    className="rounded-full border border-[var(--border)] bg-[rgba(40,160,240,0.08)] px-3 py-1 text-xs font-semibold text-[var(--text)] transition hover:border-[var(--primary)]"
                  >
                    {formatShift(preset).slice(1)}
                  </button>
                ))}
              </div>

              <div className="mt-5 rounded-xl border border-[var(--primary)] bg-[rgba(40,160,240,0.15)] p-3">
                <p className="text-xs uppercase tracking-[0.1em] text-[var(--muted)]">Resulting Shift</p>
                <p className="mt-1 text-2xl font-black tracking-[0.05em] text-[var(--text)]">{formatShift(signedShiftMilliseconds)}</p>
                <p className="mt-1 text-xs text-[var(--muted)]">Applies to {videos.length} selected video{videos.length === 1 ? '' : 's'}.</p>
              </div>
            </aside>

            <section className="rounded-xl border border-[var(--border)] bg-[rgba(13,24,44,0.58)] p-4">
              <p className="text-xs font-bold uppercase tracking-[0.12em] text-[var(--muted)]">3. Before and After Preview</p>
              <div className="mt-3 max-h-[54vh] overflow-auto rounded-lg border border-[var(--border)]">
                <table className="w-full border-collapse text-left text-xs">
                  <thead className="sticky top-0 bg-[rgba(9,18,36,0.95)]">
                    <tr className="text-[var(--muted)]">
                      <th className="border-b border-[var(--border)] px-3 py-2 font-semibold">Video</th>
                      <th className="border-b border-[var(--border)] px-3 py-2 font-semibold">Start Before</th>
                      <th className="border-b border-[var(--border)] px-3 py-2 font-semibold">Start After</th>
                      <th className="border-b border-[var(--border)] px-3 py-2 font-semibold">Finish Before</th>
                      <th className="border-b border-[var(--border)] px-3 py-2 font-semibold">Finish After</th>
                    </tr>
                  </thead>
                  <tbody>
                    {previewRows.map((row) => (
                      <tr key={row.id} className="odd:bg-[rgba(255,255,255,0.01)]">
                        <td className="border-b border-[var(--border)] px-3 py-2 align-top">
                          <p className="max-w-[260px] truncate font-semibold text-[var(--text)]">{row.name}</p>
                          <p className="max-w-[260px] truncate text-[var(--muted)]">{row.relativePath}</p>
                        </td>
                        <td className="border-b border-[var(--border)] px-3 py-2 text-[var(--muted)]">
                          {formatTimestamp(row.beforeStart?.toISOString())}
                        </td>
                        <td className="border-b border-[var(--border)] px-3 py-2 font-semibold text-[#9de7c8]">
                          {formatTimestamp(row.afterStart?.toISOString())}
                        </td>
                        <td className="border-b border-[var(--border)] px-3 py-2 text-[var(--muted)]">
                          {formatTimestamp(row.beforeFinish?.toISOString())}
                        </td>
                        <td className="border-b border-[var(--border)] px-3 py-2 font-semibold text-[#9de7c8]">
                          {formatTimestamp(row.afterFinish?.toISOString())}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {errorText ? <p className="mt-3 text-sm text-[var(--danger)]">{errorText}</p> : null}

              <div className="mt-4 flex justify-end gap-2">
                <Button type="button" variant="secondary" onClick={onClose} disabled={submitting}>
                  Cancel
                </Button>
                <Button type="button" onClick={openConfirmStep} disabled={submitting || shiftMagnitudeMilliseconds <= 0 || videos.length === 0}>
                  Continue
                </Button>
              </div>
            </section>
          </div>
        ) : (
          <div className="space-y-5 p-5">
            <div className="rounded-xl border border-[rgba(242,87,87,0.45)] bg-[rgba(242,87,87,0.1)] p-4">
              <h3 className="text-base font-bold">Please confirm this irreversible timestamp rewrite.</h3>
              <p className="mt-2 text-sm text-[var(--muted)]">
                This action updates `started_at` and `finished_at` for {videos.length} selected video{videos.length === 1 ? '' : 's'} in the
                database.
              </p>
              <p className="mt-1 text-sm text-[var(--muted)]">Original media files are not modified.</p>
            </div>

            <div className="rounded-xl border border-[var(--border)] bg-[rgba(13,24,44,0.7)] p-4">
              <p className="text-xs uppercase tracking-[0.1em] text-[var(--muted)]">Summary</p>
              <p className="mt-1 text-lg font-bold">Shift: {formatShift(signedShiftMilliseconds)}</p>
              <p className="text-sm text-[var(--muted)]">Target videos: {videos.length}</p>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3">
              <p className="text-sm text-[var(--muted)]">
                {confirmCountdown > 0
                  ? `Confirm button unlocks in ${confirmCountdown}s. Review the message before applying.`
                  : 'You can now confirm and apply the update.'}
              </p>

              <div className="flex gap-2">
                <Button type="button" variant="secondary" onClick={() => setStep('adjust')} disabled={submitting}>
                  Back
                </Button>
                <Button type="button" variant="danger" onClick={submit} disabled={submitting || confirmCountdown > 0}>
                  {submitting ? 'Applying…' : 'Confirm Shift'}
                </Button>
              </div>
            </div>

            {errorText ? <p className="text-sm text-[var(--danger)]">{errorText}</p> : null}
          </div>
        )}
      </div>
    </div>
  )
}
