import { useEffect, useRef } from 'react'

import { personName } from '../../entities/person/model'
import type { TrackBoxRow, VideoTrackBoxesDTO } from '../../infrastructure/backend/types'

const DEFAULT_FPS = 5
const MAX_GAP_INTERVALS = 3 // hide a box instead of interpolating across a gap longer than this many samples
const EDGE_SEC = 0.05 // tolerance so a box shows at the exact first/last sample time

type Box = [number, number, number, number]

const rowBox = (row: TrackBoxRow): Box => [row[1], row[2], row[3], row[4]]

// Box at time t, linearly interpolated between the two surrounding samples.
// null before the first / after the last sample, or across a gap longer than maxGapSec.
export const boxAt = (rows: TrackBoxRow[], t: number, maxGapSec: number): Box | null => {
  const n = rows.length
  if (n === 0 || t < rows[0][0] - EDGE_SEC || t > rows[n - 1][0] + EDGE_SEC) {
    return null
  }
  let lo = 0
  let hi = n
  while (lo < hi) {
    const mid = (lo + hi) >> 1
    if (rows[mid][0] > t) {
      hi = mid
    } else {
      lo = mid + 1
    }
  }
  if (lo === 0) {
    return rowBox(rows[0])
  }
  if (lo === n) {
    return rowBox(rows[n - 1])
  }
  const a = rows[lo - 1]
  const b = rows[lo]
  if (b[0] - a[0] > maxGapSec) {
    if (t - a[0] <= EDGE_SEC) {
      return rowBox(a)
    }
    return b[0] - t <= EDGE_SEC ? rowBox(b) : null
  }
  const k = (t - a[0]) / (b[0] - a[0])
  return [a[1] + (b[1] - a[1]) * k, a[2] + (b[2] - a[2]) * k, a[3] + (b[3] - a[3]) * k, a[4] + (b[4] - a[4]) * k]
}

type Props = {
  video: HTMLVideoElement | null
  tracks: VideoTrackBoxesDTO[]
  sampleFps: number
  highlightTrackIds: ReadonlySet<number>
  highlightLabel: string
}

type FrameCallbackVideo = HTMLVideoElement & {
  requestVideoFrameCallback?: (callback: () => void) => number
  cancelVideoFrameCallback?: (handle: number) => void
}

// Canvas drawn over the <video>. Boxes are normalized to the frame, so we map them into the
// rect the video actually occupies inside the element (object-fit: contain letterboxes it).
export const TrackOverlay = ({ video, tracks, sampleFps, highlightTrackIds, highlightLabel }: Props) => {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const dataRef = useRef({ tracks, sampleFps, highlightTrackIds, highlightLabel })
  dataRef.current = { tracks, sampleFps, highlightTrackIds, highlightLabel }
  const redrawRef = useRef<() => void>()

  useEffect(() => {
    const canvas = canvasRef.current
    const ctx = canvas?.getContext('2d')
    if (!video || !canvas || !ctx) {
      return undefined
    }

    const draw = () => {
      const width = video.clientWidth
      const height = video.clientHeight
      const dpr = window.devicePixelRatio || 1
      // Track the <video> element's box exactly (wrapper margins/padding can't skew the mapping).
      canvas.style.left = `${video.offsetLeft}px`
      canvas.style.top = `${video.offsetTop}px`
      canvas.style.width = `${width}px`
      canvas.style.height = `${height}px`
      if (canvas.width !== Math.round(width * dpr) || canvas.height !== Math.round(height * dpr)) {
        canvas.width = Math.round(width * dpr)
        canvas.height = Math.round(height * dpr)
      }
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
      ctx.clearRect(0, 0, width, height)

      const { videoWidth, videoHeight } = video
      if (!videoWidth || !videoHeight || !width || !height) {
        return
      }
      const scale = Math.min(width / videoWidth, height / videoHeight)
      const drawW = videoWidth * scale
      const drawH = videoHeight * scale
      const offX = (width - drawW) / 2
      const offY = (height - drawH) / 2

      const data = dataRef.current
      const maxGapSec = MAX_GAP_INTERVALS / (data.sampleFps > 0 ? data.sampleFps : DEFAULT_FPS)
      const styles = getComputedStyle(canvas)
      const primary = styles.getPropertyValue('--primary').trim() || '#28a0f0'
      const muted = styles.getPropertyValue('--muted').trim() || '#9eb0c8'
      const t = video.currentTime

      ctx.lineWidth = 2
      ctx.font = '600 12px system-ui, sans-serif'
      ctx.textBaseline = 'middle'
      // Other people first so the previewed person's boxes end up on top.
      const ordered = [...data.tracks].sort(
        (a, b) => Number(data.highlightTrackIds.has(a.trackId)) - Number(data.highlightTrackIds.has(b.trackId)),
      )
      for (const track of ordered) {
        const box = boxAt(track.boxes, t, maxGapSec)
        if (!box) {
          continue
        }
        const highlighted = data.highlightTrackIds.has(track.trackId)
        const color = highlighted ? primary : muted
        const x = offX + box[0] * drawW
        const y = offY + box[1] * drawH
        const w = (box[2] - box[0]) * drawW
        const h = (box[3] - box[1]) * drawH
        ctx.strokeStyle = color
        ctx.strokeRect(x, y, w, h)

        const label = highlighted ? data.highlightLabel : personName(track.label, track.groupId)
        const labelW = ctx.measureText(label).width + 8
        const labelY = y >= 18 ? y - 18 : y // inside the box when there's no room above
        ctx.fillStyle = color
        ctx.fillRect(x - 1, labelY, labelW, 18)
        ctx.fillStyle = '#04111f'
        ctx.fillText(label, x + 3, labelY + 9)
      }
    }
    redrawRef.current = draw

    // requestVideoFrameCallback fires per presented frame; fall back to rAF where unsupported.
    const frameVideo = video as FrameCallbackVideo
    const hasFrameCallback = typeof frameVideo.requestVideoFrameCallback === 'function'
    let handle = 0
    const tick = () => {
      draw()
      handle = hasFrameCallback
        ? (frameVideo.requestVideoFrameCallback as (callback: () => void) => number)(tick)
        : requestAnimationFrame(tick)
    }
    tick()

    // Paused/seeking/metadata don't produce frame callbacks reliably, so redraw on those too.
    const events = ['seeked', 'seeking', 'pause', 'loadedmetadata', 'timeupdate'] as const
    events.forEach((name) => video.addEventListener(name, draw))
    const observer = new ResizeObserver(draw)
    observer.observe(video)

    return () => {
      if (hasFrameCallback) {
        frameVideo.cancelVideoFrameCallback?.(handle)
      } else {
        cancelAnimationFrame(handle)
      }
      events.forEach((name) => video.removeEventListener(name, draw))
      observer.disconnect()
      redrawRef.current = undefined
    }
  }, [video])

  // Repaint immediately when the data changes while paused.
  useEffect(() => {
    redrawRef.current?.()
  }, [tracks, sampleFps, highlightTrackIds, highlightLabel])

  return <canvas ref={canvasRef} className="pointer-events-none absolute" aria-hidden="true" />
}
