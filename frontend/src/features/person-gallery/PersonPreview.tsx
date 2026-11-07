import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'

import { personName } from '../../entities/person/model'
import { formatDuration } from '../../entities/video/model'
import { useBackend } from '../../infrastructure/backend/context'
import type { PersonGroupDTO, PersonTrackDTO } from '../../infrastructure/backend/types'
import { Button } from '../../shared/ui/Button'
import { PersonName } from './PersonName'
import { TrackOverlay } from './TrackOverlay'

type Props = {
  group: PersonGroupDTO
  busy: boolean
  onClose: () => void
  onSplit: (trackId: number) => void
  onRemove: (trackId: number) => void
}

export const PersonPreview = ({ group, busy, onClose, onSplit, onRemove }: Props) => {
  const [activeId, setActiveId] = useState<number | undefined>(group.tracks[0]?.id)
  const active = group.tracks.find((track) => track.id === activeId) ?? group.tracks[0]

  const backend = useBackend()
  const [showBoxes, setShowBoxes] = useState(true)
  const [video, setVideo] = useState<HTMLVideoElement | null>(null)
  const videoId = active?.videoId
  const boxesQuery = useQuery({
    queryKey: ['videoTrackBoxes', videoId],
    queryFn: () => backend.getVideoTrackBoxes(videoId as number),
    enabled: videoId !== undefined,
  })
  const boxes = boxesQuery.data
  const boxesError = boxes?.error?.message ?? (boxesQuery.error instanceof Error ? boxesQuery.error.message : undefined)
  const hasBoxes = !boxesError && (boxes?.tracks.length ?? 0) > 0
  const highlightTrackIds = useMemo(() => new Set(group.tracks.map((track) => track.id)), [group.tracks])

  useEffect(() => {
    if (!video || !active) {
      return undefined
    }
    const seekToStart = () => {
      video.currentTime = active.startSec
      void video.play()
    }
    if (video.readyState >= 1) {
      seekToStart()
      return undefined
    }
    video.addEventListener('loadedmetadata', seekToStart, { once: true })
    return () => video.removeEventListener('loadedmetadata', seekToStart)
  }, [video, active?.id, active?.startSec])

  useEffect(() => {
    if (!video || !active) {
      return undefined
    }
    const stopAtEnd = () => {
      if (video.paused) {
        return
      }
      const time = video.currentTime
      if (time < active.endSec || time >= active.endSec + 0.5) {
        return
      }
      video.pause()
    }
    video.addEventListener('timeupdate', stopAtEnd)
    return () => video.removeEventListener('timeupdate', stopAtEnd)
  }, [video, active?.id, active?.endSec])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="min-w-0 flex-1 pr-3 text-lg font-bold">
            <PersonName group={group} className="text-lg" />
          </h2>
          <button type="button" onClick={onClose} className="text-sm text-[var(--muted)] hover:text-[var(--text)]">
            Close
          </button>
        </div>

        {active ? (
          <div className="relative mb-3">
            <video
              key={active.videoId}
              ref={setVideo}
              src={`/media/${active.videoId}`}
              controls
              autoPlay
              muted
              className="max-h-[50vh] w-full rounded-lg bg-black"
            />
            {showBoxes && hasBoxes && boxes ? (
              <TrackOverlay
                video={video}
                tracks={boxes.tracks}
                sampleFps={boxes.sampleFps}
                highlightTrackIds={highlightTrackIds}
                highlightLabel={personName(group.label, group.id)}
              />
            ) : null}
          </div>
        ) : null}

        <div className="mb-3 flex flex-wrap items-center gap-3 text-sm">
          <label className="flex items-center gap-2">
            <input type="checkbox" checked={showBoxes} onChange={(event) => setShowBoxes(event.target.checked)} />
            Show boxes
          </label>
          {boxesError ? <span className="text-xs text-[var(--muted)]">Boxes unavailable: {boxesError}</span> : null}
          {boxes && !boxesError && boxes.tracks.length === 0 ? (
            <span className="text-xs text-[var(--muted)]">
              No box data for this video yet — run Detect people again to enable overlays.
            </span>
          ) : null}
        </div>

        <ul className="flex flex-col gap-2">
          {group.tracks.map((track) => (
            <li
              key={track.id}
              className={`flex flex-wrap items-center justify-between gap-2 rounded-lg border p-2 text-sm ${
                track.id === active?.id ? 'border-[var(--primary)]' : 'border-[var(--border)]'
              }`}
            >
              <button type="button" onClick={() => setActiveId(track.id)} className="min-w-0 flex-1 text-left">
                <span className="block truncate font-semibold">{track.videoName}</span>
                <span className="text-xs text-[var(--muted)]">
                  {formatDuration(track.startSec)} – {formatDuration(track.endSec)}
                </span>
              </button>
              <div className="flex items-center gap-2">
                {group.tracks.length > 1 ? (
                  <Button variant="secondary" disabled={busy} onClick={() => onSplit(track.id)}>
                    Split out
                  </Button>
                ) : null}
                <Button variant="danger" disabled={busy} onClick={() => onRemove(track.id)}>
                  Not a person
                </Button>
              </div>
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}
