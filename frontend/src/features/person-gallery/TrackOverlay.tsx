import { useEffect, useMemo, useRef } from 'react'

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

type LabelMetric = { text: string; width: number }

// Canvas drawn over the <video>. Boxes are normalized to the frame, so we map them into the
// rect the video actually occupies inside the element (object-fit: contain letterboxes it).
export const TrackOverlay = ({ video, tracks, sampleFps, highlightTrackIds, highlightLabel }: Props) => {
  const ordered = useMemo(
    () => [...tracks].sort((a, b) => Number(highlightTrackIds.has(a.trackId)) - Number(highlightTrackIds.has(b.trackId))),
    [tracks, highlightTrackIds],
  )
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const dataRef = useRef({ ordered, sampleFps, highlightTrackIds, highlightLabel })
  dataRef.current = { ordered, sampleFps, highlightTrackIds, highlightLabel }
  const redrawRef = useRef<() => void>()
  const labelsRef = useRef<Map<number, LabelMetric>>(new Map())
  const labelsKeyRef = useRef('')

  useEffect(() => {
    const key = `${highlightLabel}\0${[...highlightTrackIds].sort((a, b) => a - b).join(',')}\0${ordered
      .map((track) => `${track.trackId}:${track.label ?? ''}:${track.groupId}`)
      .join('\0')}`
    if (key === labelsKeyRef.current) {
      return
    }
    const canvas = canvasRef.current
    const context = canvas?.getContext('2d')
    if (!context) {
      return
    }
    context.font = '600 12px system-ui, sans-serif'
    const next = new Map<number, LabelMetric>()
    for (const track of ordered) {
      const highlighted = highlightTrackIds.has(track.trackId)
      const text = highlighted ? highlightLabel : personName(track.label, track.groupId)
      next.set(track.trackId, { text, width: context.measureText(text).width + 8 })
    }
    labelsRef.current = next
    labelsKeyRef.current = key
  }, [ordered, highlightLabel, highlightTrackIds])

  useEffect(() => {
    const canvas = canvasRef.current
    const ctx = canvas?.getContext('2d')
    if (!video || !canvas || !ctx) {
      return undefined
    }

    const styles = getComputedStyle(canvas)
    const primary = styles.getPropertyValue('--primary').trim() || '#28a0f0'
    const muted = styles.getPropertyValue('--muted').trim() || '#9eb0c8'

    let layoutLeft = -1
    let layoutTop = -1
    let layoutWidth = -1
    let layoutHeight = -1
    let layoutDpr = -1

    const syncCanvasLayout = () => {
      const left = video.offsetLeft
      const top = video.offsetTop
      const width = video.clientWidth
      const height = video.clientHeight
      const dpr = window.devicePixelRatio || 1
      if (
        left === layoutLeft &&
        top === layoutTop &&
        width === layoutWidth &&
        height === layoutHeight &&
        dpr === layoutDpr
      ) {
        return
      }
      layoutLeft = left
      layoutTop = top
      layoutWidth = width
      layoutHeight = height
      layoutDpr = dpr
      canvas.style.left = `${left}px`
      canvas.style.top = `${top}px`
      canvas.style.width = `${width}px`
      canvas.style.height = `${height}px`
      const bitmapWidth = Math.round(width * dpr)
      const bitmapHeight = Math.round(height * dpr)
      if (canvas.width !== bitmapWidth || canvas.height !== bitmapHeight) {
        canvas.width = bitmapWidth
        canvas.height = bitmapHeight
      }
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    }

    const draw = () => {
      syncCanvasLayout()
      const width = layoutWidth
      const height = layoutHeight
      if (width <= 0 || height <= 0) {
        return
      }
      ctx.clearRect(0, 0, width, height)

      const { videoWidth, videoHeight } = video
      if (!videoWidth || !videoHeight) {
        return
      }
      const scale = Math.min(width / videoWidth, height / videoHeight)
      const drawW = videoWidth * scale
      const drawH = videoHeight * scale
      const offX = (width - drawW) / 2
      const offY = (height - drawH) / 2

      const data = dataRef.current
      const maxGapSec = MAX_GAP_INTERVALS / (data.sampleFps > 0 ? data.sampleFps : DEFAULT_FPS)
      const t = video.currentTime
      const labels = labelsRef.current

      ctx.lineWidth = 2
      ctx.font = '600 12px system-ui, sans-serif'
      ctx.textBaseline = 'middle'
      for (const track of data.ordered) {
        if (t < track.startSec - maxGapSec || t > track.endSec + maxGapSec) {
          continue
        }
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

        const metric = labels.get(track.trackId)
        if (!metric) {
          continue
        }
        const labelY = y >= 18 ? y - 18 : y
        ctx.fillStyle = color
        ctx.fillRect(x - 1, labelY, metric.width, 18)
        ctx.fillStyle = '#04111f'
        ctx.fillText(metric.text, x + 3, labelY + 9)
      }
    }
    redrawRef.current = draw

    const frameVideo = video as FrameCallbackVideo
    const hasFrameCallback = typeof frameVideo.requestVideoFrameCallback === 'function'
    let playing = false
    let frameHandle = 0
    let drawSoon = false
    let drawSoonHandle = 0

    const scheduleDrawSoon = () => {
      if (drawSoon) {
        return
      }
      drawSoon = true
      drawSoonHandle = requestAnimationFrame(() => {
        drawSoon = false
        draw()
      })
    }

    const stopFrameLoop = () => {
      if (!frameHandle) {
        playing = false
        return
      }
      if (hasFrameCallback) {
        frameVideo.cancelVideoFrameCallback?.(frameHandle)
      } else {
        cancelAnimationFrame(frameHandle)
      }
      frameHandle = 0
      playing = false
    }

    const tick = () => {
      if (!playing) {
        return
      }
      draw()
      frameHandle = hasFrameCallback
        ? (frameVideo.requestVideoFrameCallback as (callback: () => void) => number)(tick)
        : requestAnimationFrame(tick)
    }

    const onPlay = () => {
      if (playing) {
        return
      }
      playing = true
      tick()
    }

    const onPauseOrEnded = () => {
      stopFrameLoop()
      scheduleDrawSoon()
    }

    const onSeekedOrMetadata = () => scheduleDrawSoon()

    video.addEventListener('play', onPlay)
    video.addEventListener('pause', onPauseOrEnded)
    video.addEventListener('ended', onPauseOrEnded)
    video.addEventListener('seeked', onSeekedOrMetadata)
    video.addEventListener('loadedmetadata', onSeekedOrMetadata)

    const observer = new ResizeObserver(scheduleDrawSoon)
    observer.observe(video)

    if (!video.paused && !video.ended) {
      onPlay()
    } else {
      scheduleDrawSoon()
    }

    return () => {
      stopFrameLoop()
      if (drawSoonHandle) {
        cancelAnimationFrame(drawSoonHandle)
      }
      video.removeEventListener('play', onPlay)
      video.removeEventListener('pause', onPauseOrEnded)
      video.removeEventListener('ended', onPauseOrEnded)
      video.removeEventListener('seeked', onSeekedOrMetadata)
      video.removeEventListener('loadedmetadata', onSeekedOrMetadata)
      observer.disconnect()
      redrawRef.current = undefined
    }
  }, [video])

  useEffect(() => {
    redrawRef.current?.()
  }, [tracks, sampleFps, highlightTrackIds, highlightLabel])

  return <canvas ref={canvasRef} className="pointer-events-none absolute" aria-hidden="true" />
}
