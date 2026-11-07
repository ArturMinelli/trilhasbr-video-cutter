import type { CutResult, JobSnapshot } from '../../infrastructure/backend/types'

export type CutResultEntity = CutResult
export type CutJobEntity = JobSnapshot

export const progressPercent = (job: CutJobEntity): number => {
  if (job.totalSegments <= 0) {
    return job.status === 'SUCCEEDED' ? 100 : 0
  }
  return Math.min(100, Math.round((job.completedSegments / job.totalSegments) * 100))
}
