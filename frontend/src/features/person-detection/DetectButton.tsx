import { useState } from 'react'
import { z } from 'zod'

import { Button } from '../../shared/ui/Button'
import { Input } from '../../shared/ui/Input'
import type { Detection } from './useDetection'

const fpsSchema = z.coerce.number().min(1, 'FPS must be 1–15').max(15, 'FPS must be 1–15')

export const DetectButton = ({ detection }: { detection: Detection }) => {
  const [fps, setFps] = useState('5')
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [errorText, setErrorText] = useState<string | null>(null)
  const busy = detection.state.status === 'starting' || detection.state.status === 'running'

  const run = () => {
    const parsed = fpsSchema.safeParse(fps)
    if (!parsed.success) {
      setErrorText(parsed.error.issues[0]?.message ?? 'Invalid FPS')
      setShowAdvanced(true)
      return
    }
    setErrorText(null)
    void detection.start(parsed.data)
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => setShowAdvanced((current) => !current)}
          className="text-xs text-[var(--muted)] hover:text-[var(--text)]"
          aria-expanded={showAdvanced}
        >
          Advanced
        </button>
        <Button variant="secondary" onClick={run} disabled={busy}>
          {busy ? 'Detecting…' : 'Detect people'}
        </Button>
      </div>
      {showAdvanced ? (
        <label className="flex items-center gap-2 text-xs uppercase tracking-[0.08em] text-[var(--muted)]">
          Sampling FPS
          <Input
            value={fps}
            onChange={(event) => setFps(event.target.value)}
            type="number"
            min={1}
            max={15}
            step={1}
            disabled={busy}
            className="w-20"
          />
        </label>
      ) : null}
      {errorText ? <p className="text-xs text-[var(--danger)]">{errorText}</p> : null}
    </div>
  )
}
