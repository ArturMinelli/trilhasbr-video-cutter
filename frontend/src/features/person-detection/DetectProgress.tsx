import { plural } from '../../entities/person/model'
import { Button } from '../../shared/ui/Button'
import type { Detection } from './useDetection'

export const DetectProgress = ({ detection }: { detection: Detection }) => {
  const { state } = detection
  if (state.status === 'idle') {
    return null
  }

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
          <div className="max-h-80 space-y-3 overflow-y-auto" aria-label="Detection queue">
            {state.files.map((file) => (
              <div key={file.id} className="rounded-lg bg-[var(--surface-soft)] p-3">
                <div className="flex items-center justify-between gap-3">
                  <span className="min-w-0 truncate" title={`${file.name} (ID ${file.id})`}>{file.name} <span className="text-xs text-[var(--muted)]">#{file.id}</span></span>
                  <span className="shrink-0 text-xs capitalize text-[var(--muted)]">{file.status}{file.status === 'processing' && file.percent !== undefined ? ` · ${Math.round(file.percent)}%` : ''}</span>
                </div>
                <div className="mt-2 h-2 overflow-hidden rounded-full bg-[var(--border)]" role="progressbar" aria-label={`${file.name} ${file.status}`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={file.status === 'completed' || file.status === 'skipped' ? 100 : file.percent}>
                  <div className={`h-full rounded-full bg-[var(--primary)] transition-[width] duration-300 motion-reduce:transition-none ${file.percent === undefined && ['processing', 'finalizing'].includes(file.status) ? 'animate-pulse motion-reduce:animate-none' : ''}`} style={{ width: `${file.status === 'completed' || file.status === 'skipped' ? 100 : file.percent ?? (['processing', 'finalizing'].includes(file.status) ? 100 : 0)}%` }} />
                </div>
              </div>
            ))}
          </div>
        </>
      ) : null}

      {!running && state.files.length > 0 ? (
        <div className="mt-3 max-h-80 space-y-1 overflow-y-auto" aria-label="Detection results">
          {state.files.map((file) => <div key={file.id} className="flex justify-between gap-3"><span className="truncate" title={`${file.name} (ID ${file.id})`}>{file.name} #{file.id}</span><span className="capitalize text-[var(--muted)]">{file.status}</span></div>)}
        </div>
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
