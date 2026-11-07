import type { VideoPreviewResponse } from '../../infrastructure/backend/types'

const previewCache = new Map<number, VideoPreviewResponse>()
const pendingCache = new Map<number, Promise<VideoPreviewResponse>>()

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

  const request = fetcher()
    .then((response) => {
      previewCache.set(videoId, response)
      pendingCache.delete(videoId)
      return response
    })
    .catch((error) => {
      pendingCache.delete(videoId)
      throw error
    })

  pendingCache.set(videoId, request)
  return request
}
