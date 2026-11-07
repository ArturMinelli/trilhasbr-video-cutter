import type { VideoPreviewResponse } from '../../infrastructure/backend/types'

const previewCache = new Map<number, VideoPreviewResponse>()
const pendingCache = new Map<number, Promise<VideoPreviewResponse>>()

const maxConcurrentFetches = 3
let activeFetches = 0
const fetchWaitQueue: Array<() => void> = []

function acquireFetchSlot(): Promise<void> {
  if (activeFetches < maxConcurrentFetches) {
    activeFetches++
    return Promise.resolve()
  }

  return new Promise((resolve) => {
    fetchWaitQueue.push(() => {
      activeFetches++
      resolve()
    })
  })
}

function releaseFetchSlot(): void {
  activeFetches--
  const next = fetchWaitQueue.shift()
  if (!next) {
    return
  }
  next()
}

export const fetchVideoPreviewCached = (
  videoId: number,
  fetcher: () => Promise<VideoPreviewResponse>,
): Promise<VideoPreviewResponse> => {
  const cached = previewCache.get(videoId)
  if (cached) {
    return Promise.resolve(cached)
  }

  const pending = pendingCache.get(videoId)
  if (pending) {
    return pending
  }

  const request = acquireFetchSlot()
    .then(() => fetcher())
    .then((response) => {
      previewCache.set(videoId, response)
      pendingCache.delete(videoId)
      releaseFetchSlot()
      return response
    })
    .catch((error) => {
      pendingCache.delete(videoId)
      releaseFetchSlot()
      throw error
    })

  pendingCache.set(videoId, request)
  return request
}
