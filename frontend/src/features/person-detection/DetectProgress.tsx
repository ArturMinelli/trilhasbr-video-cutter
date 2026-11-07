import { plural } from '../../entities/person/model'
import { Button } from '../../shared/ui/Button'
import { ProgressBar } from '../../shared/ui/ProgressBar'
import type { Detection } from './useDetection'

export const DetectProgress = ({ detection }: { detection: Detection }) => {
  const { state } = detection
  if (state.status === 'idle') {
    return null
  }

  const percent = state.videosTotal > 0 ? Math.round((state.videosDone / state.videosTotal) * 100) : 0
  const running = state.status === 'starting' || state.status === 'running'
  const seconds = state.elapsedMs !== undefined ? ` in ${(state.elapsedMs / 1000).toFixed(1)}s` : ''

  return (
    <div className="mb-4 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 text-sm">
      {running ? (
        <>
          <div className="mb-2 flex items-center justify-between gap-3">
            <span>
              Detecting people… {state.videosDone}/{state.videosTotal} videos · {plural(state.tracksFound, 'track')} found
            </span>
            <Button variant="secondary" onClick={() => void detection.cancel()} disabled={state.status === 'starting'}>
              Cancel
            </Button>
          </div>
          <ProgressBar value={percent} />
          {state.currentVideo ? <p className="mt-2 truncate text-xs text-[var(--muted)]">{state.currentVideo}</p> : null}
        </>
      ) : null}

      {state.status === 'done' && state.tracksFound > 0 ? (
        <p>
          Detection finished{seconds}: {plural(state.videosTotal, 'video')} processed, {plural(state.tracksFound, 'track')} found.
        </p>
      ) : null}
      {state.status === 'done' && state.tracksFound === 0 ? (
        <p className="text-[var(--muted)]">Detection finished{seconds}: no people found in this workspace.</p>
      ) : null}
      {state.status === 'cancelled' ? (
        <p className="text-[var(--muted)]">
          Detection cancelled. Finished videos kept their results ({plural(state.tracksFound, 'track')}).
        </p>
      ) : null}
      {state.status === 'failed' ? <p className="text-[var(--danger)]">Detection failed: {state.errorMessage}</p> : null}
    </div>
  )
}
