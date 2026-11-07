import { memo, useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'

import { formatDuration } from '../../entities/video/model'
import type { VideoPreviewResponse, VideoExplorer } from '../../infrastructure/backend/types'
import { formatDateTime } from '../../shared/utils/date'

type VideoCardProps = {
  video: VideoExplorer
  loadPreview: (videoId: number) => Promise<VideoPreviewResponse>
  selected: boolean
  selectionDisabled?: boolean
  onSelectionChange: (videoId: number, checked: boolean) => void
}

export const VideoCard = memo(({ video, loadPreview, selected, selectionDisabled = false, onSelectionChange }: VideoCardProps) => {
  const cardRef = useRef<HTMLDivElement | null>(null)
  const [isVisible, setIsVisible] = useState(false)

  useEffect(() => {
    const element = cardRef.current
    if (!element || isVisible) {
      return
    }

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setIsVisible(true)
            observer.disconnect()
            break
          }
        }
      },
      { rootMargin: '0px' },
    )

    observer.observe(element)
    return () => observer.disconnect()
  }, [isVisible])

  const previewQuery = useQuery({
    queryKey: ['video-preview', video.id],
    queryFn: () => loadPreview(video.id),
    enabled: isVisible,
    staleTime: Number.POSITIVE_INFINITY,
    gcTime: Number.POSITIVE_INFINITY,
  })

  const previewData = previewQuery.data
  const previewError = previewData?.error

  return (
    <article
      ref={cardRef}
      style={{ contentVisibility: 'auto', containIntrinsicSize: 'auto 220px' }}
      className={`rounded-xl border p-3 shadow-[0_8px_24px_-18px_rgba(0,0,0,0.9)] transition-colors ${
        selected
          ? 'border-[var(--primary)] bg-[linear-gradient(170deg,rgba(40,160,240,0.16),rgba(17,29,51,0.98))] shadow-[0_14px_34px_-18px_rgba(40,160,240,0.85)]'
          : 'border-[var(--border)] bg-[var(--surface)]'
      }`}
    >
      <div className="relative mb-3 aspect-video overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--surface-soft)]">
        <label
          className={`absolute left-2 top-2 z-10 inline-flex items-center gap-1 rounded-full border px-2 py-1 text-[11px] font-semibold transition-colors ${
            selectionDisabled
              ? 'cursor-not-allowed border-[var(--border)] bg-black/35 text-[var(--muted)]'
              : selected
                ? 'border-[var(--primary)] bg-[rgba(40,160,240,0.24)] text-white'
                : 'cursor-pointer border-[var(--border)] bg-black/35 text-[var(--text)] hover:border-[var(--primary)]'
          }`}
        >
          <input
            type="checkbox"
            checked={selected}
            disabled={selectionDisabled}
            onChange={(event) => onSelectionChange(video.id, event.target.checked)}
            className="h-3.5 w-3.5 accent-[var(--primary)]"
            aria-label={`Select ${video.name || 'video'}`}
          />
          <span>Select</span>
        </label>

        {!isVisible ? (
          <div className="absolute inset-0 flex items-center justify-center text-xs text-[var(--muted)]">Preview on scroll</div>
        ) : previewQuery.isFetching ? (
          <div className="absolute inset-0 animate-pulse bg-[var(--primary-soft)]" />
        ) : previewData?.dataUrl ? (
          <img src={previewData.dataUrl} alt={video.name} className="h-full w-full object-cover" loading="lazy" />
        ) : (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-1 text-xs text-[var(--muted)]">
            <span>No preview</span>
            {previewError ? <span className="max-w-[90%] truncate">{previewError.message}</span> : null}
          </div>
        )}
      </div>

      <p className="truncate text-sm font-semibold text-[var(--text)]">{video.name || 'Untitled video'}</p>
      <p className="mt-1 truncate text-xs text-[var(--muted)]">{video.relativePath}</p>

      <div className="mt-2 grid grid-cols-2 gap-1 text-xs text-[var(--muted)]">
        <span>{formatDuration(video.durationSeconds)}</span>
        <span className="truncate text-right">{formatDateTime(video.startedAt)}</span>
      </div>

      {selectionDisabled ? (
        <p className="mt-2 text-[11px] text-[var(--muted)]">Missing timestamps: cannot shift.</p>
      ) : null}
    </article>
  )
})
