import { useCallback, useEffect, useRef, useState } from 'react'

import { useBackend } from '../../infrastructure/backend/context'
import { useToast } from '../../shared/toast/context'

export type DetectionState = {
  status: 'idle' | 'starting' | 'running' | 'done' | 'cancelled' | 'failed'
  videosTotal: number
  videosDone: number
  currentVideo: string
  tracksFound: number
  elapsedMs?: number
  files: { id: number; name: string; status: string; percent?: number }[]
  errorMessage?: string
}

const idle: DetectionState = { status: 'idle', videosTotal: 0, videosDone: 0, currentVideo: '', tracksFound: 0, files: [] }

export type Detection = ReturnType<typeof useDetection>

export const useDetection = (workspaceId: number, onSettled: () => void) => {
  const backend = useBackend()
  const { showToast } = useToast()
  const [state, setState] = useState<DetectionState>(idle)
  const jobRef = useRef<string | null>(null)
  const startingRef = useRef(false)
  const settledRef = useRef(onSettled)
  settledRef.current = onSettled

  // Events can arrive before startDetection resolves, so adopt the first job id seen while starting.
  const matches = useCallback((jobId: string): boolean => {
    if (jobRef.current === null) {
      if (!startingRef.current) {
        return false
      }
      jobRef.current = jobId
      return true
    }
    return jobRef.current === jobId
  }, [])

  useEffect(() => {
    jobRef.current = null
    startingRef.current = false
    setState(idle)
  }, [workspaceId])

  useEffect(() => {
    const offProgress = backend.onDetectProgress((event) => {
      if (!matches(event.jobId)) {
        return
      }
      setState((current) => ({
        ...current,
        status: 'running',
        videosTotal: event.videosTotal,
        videosDone: event.videosDone,
        currentVideo: event.currentVideo,
        tracksFound: event.tracksFound,
        files: current.files.some((file) => file.id === event.videoId)
          ? current.files.map((file) => file.id === event.videoId ? { id: event.videoId, name: event.videoName, status: event.videoStatus, percent: event.percent } : file)
          : [...current.files, { id: event.videoId, name: event.videoName, status: event.videoStatus, percent: event.percent }],
      }))
    })
    const offFinished = backend.onDetectFinished((event) => {
      if (!matches(event.jobId)) {
        return
      }
      startingRef.current = false
      setState((current) => ({
        ...current,
        status: 'done',
        videosDone: current.videosTotal,
        currentVideo: '',
        tracksFound: event.tracksFound,
        elapsedMs: event.elapsedMs,
      }))
      settledRef.current()
    })
    const offFailed = backend.onDetectFailed((event) => {
      if (!matches(event.jobId)) {
        return
      }
      startingRef.current = false
      setState((current) => ({ ...current, status: 'failed', currentVideo: '', errorMessage: event.error.message, files: current.files.map((file) => ['waiting', 'processing', 'finalizing'].includes(file.status) ? { ...file, status: 'failed' } : file) }))
      showToast('Detection failed', event.error.message, 'error')
      settledRef.current()
    })
    return () => {
      offProgress()
      offFinished()
      offFailed()
    }
  }, [backend, matches, showToast])

  const start = useCallback(
    async (sampleFps: number) => {
      jobRef.current = null
      startingRef.current = true
      setState({ ...idle, status: 'starting' })
      try {
        const response = await backend.startDetection({ workspaceId, sampleFps })
        if (response.error || !response.jobId) {
          startingRef.current = false
          const message = response.error?.message ?? 'Missing job id in response'
          setState({ ...idle, status: 'failed', errorMessage: message })
          showToast('Failed to start detection', message, 'error')
          return
        }
        jobRef.current = response.jobId
      } catch (error) {
        startingRef.current = false
        const message = error instanceof Error ? error.message : 'Unknown error'
        setState({ ...idle, status: 'failed', errorMessage: message })
        showToast('Unexpected error', message, 'error')
      }
    },
    [backend, showToast, workspaceId],
  )

  const cancel = useCallback(async () => {
    const jobId = jobRef.current
    if (!jobId) {
      return
    }
    const response = await backend.cancelDetection(jobId)
    if (response.error) {
      showToast('Unable to cancel', response.error.message, 'error')
      return
    }
    // Cancellation may not emit a finished event; read the final job state instead.
    let job = (await backend.getDetectionJob(jobId)).job
    while (job && (job.status === 'RUNNING' || job.status === 'PENDING')) {
      await new Promise((resolve) => setTimeout(resolve, 300))
      job = (await backend.getDetectionJob(jobId)).job
    }
    if (job && job.status === 'CANCELLED' && jobRef.current === jobId) {
      startingRef.current = false
      const tracksFound = job.tracksFound
      setState((current) => ({ ...current, status: 'cancelled', currentVideo: '', tracksFound, files: current.files.map((file) => ['waiting', 'processing', 'finalizing'].includes(file.status) ? { ...file, status: 'cancelled' } : file) }))
      settledRef.current()
    }
  }, [backend, showToast])

  return { state, start, cancel }
}
