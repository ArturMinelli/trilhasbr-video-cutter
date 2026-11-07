export type AppError = {
  code: string
  message: string
  details?: Record<string, unknown>
}

export type Workspace = {
  id: number
  directory: string
  name: string
  createdAt: string
  updatedAt: string
  videoCount: number
  nullStartedAtCount: number
}

export type Video = {
  id: number
  workspaceId: number
  name: string
  path: string
  startedAt?: string
  finishedAt?: string
  durationSeconds: number
}

export type VideoExplorer = {
  id: number
  workspaceId: number
  name: string
  path: string
  relativePath: string
  relativeDir: string
  startedAt?: string
  finishedAt?: string
  durationSeconds: number
}

export type FileFailure = {
  path: string
  reason: string
}

export type WorkspaceLoadSummary = {
  totalScanned: number
  indexed: number
  nullStartedAtCount: number
  failures: FileFailure[]
}

export type ListWorkspacesResponse = {
  workspaces?: Workspace[]
  error?: AppError
}

export type LoadWorkspaceResponse = {
  workspace?: Workspace
  summary?: WorkspaceLoadSummary
  error?: AppError
}

export type PagedVideosResponse = {
  videos?: Video[]
  page: number
  pageSize: number
  total: number
  error?: AppError
}

export type WorkspaceExplorerResponse = {
  workspaceId: number
  workspaceDirectory: string
  videos?: VideoExplorer[]
  error?: AppError
}

export type VideoPreviewResponse = {
  videoId: number
  dataUrl?: string
  cached: boolean
  error?: AppError
}

export type StartCutRequest = {
  workspaceId: number
  name: string
  datetimeLocal: string
  secondsBefore: number
  secondsAfter: number
}

export type StartCutResponse = {
  jobId?: string
  error?: AppError
}

export type CSVImportIssue = {
  row: number
  column: string
  message: string
  value?: string
}

export type CSVImportCut = {
  row: number
  name: string
  datetimeLocal: string
  secondsBefore: number
  secondsAfter: number
  outputDirectory: string
  totalSegments: number
}

export type PreviewCutsFromCSVRequest = {
  workspaceId: number
}

export type PreviewCutsFromCSVResponse = {
  fileName?: string
  cuts?: CSVImportCut[]
  issues?: CSVImportIssue[]
  cancelled: boolean
  error?: AppError
}

export type StartCutsFromCSVCutInput = {
  row: number
  name: string
  datetimeLocal: string
  secondsBefore: number
  secondsAfter: number
}

export type StartCutsFromCSVRequest = {
  workspaceId: number
  cuts: StartCutsFromCSVCutInput[]
}

export type StartCutsFromCSVResponse = {
  jobId?: string
  error?: AppError
}

export type ShiftVideosTimeRequest = {
  workspaceId: number
  videoIds: number[]
  shiftMilliseconds: number
}

export type ShiftVideosTimeResponse = {
  updated: number
  error?: AppError
}

export type OutputFile = {
  path: string
  durationSeconds: number
}

export type CutResult = {
  jobId: string
  outputDirectory: string
  perSourceOutputs: OutputFile[]
  mergedOutput?: OutputFile
  warnings: string[]
}

export type JobSnapshot = {
  jobId: string
  status: 'PENDING' | 'RUNNING' | 'SUCCEEDED' | 'FAILED'
  totalSegments: number
  completedSegments: number
  currentFile: string
  result?: CutResult
  error?: AppError
  createdAt: string
  updatedAt: string
  finishedAt?: string
}

export type CutJobStatusResponse = {
  job?: JobSnapshot
  error?: AppError
}

export type ActionResponse = {
  success: boolean
  error?: AppError
}

export type CutProgressEvent = {
  jobId: string
  totalSegments: number
  completedSegments: number
  currentFile: string
}

export type CutFinishedEvent = {
  jobId: string
  result: CutResult
}

export type CutFailedEvent = {
  jobId: string
  error: AppError
}

export type CutCSVProgressEvent = {
  jobId: string
  totalCuts: number
  completedCuts: number
  currentRow: number
  currentName: string
}

export type CutCSVFinishedEvent = {
  jobId: string
  outputDirectory: string
  totalCuts: number
  completedCuts: number
}

export type CutCSVFailedEvent = {
  jobId: string
  row: number
  name: string
  error: AppError
}

export type PersonTrackDTO = {
  id: number
  videoId: number
  videoName: string
  startSec: number
  endSec: number
}

export type PersonGroupDTO = {
  id: number
  label?: string // user-given name; absent/empty = unnamed
  thumbnailDataUrl: string
  startSec?: number
  trackCount: number
  videoCount: number
  totalDurationSec: number
  avgConfidence: number
  tracks: PersonTrackDTO[]
}

export type StartDetectionRequest = {
  workspaceId: number
  sampleFps?: number
}

export type StartDetectionResponse = {
  jobId?: string
  error?: AppError
}

export type DetectionJobSnapshot = {
  jobId: string
  status: 'PENDING' | 'RUNNING' | 'SUCCEEDED' | 'FAILED' | 'CANCELLED'
  videosTotal: number
  videosDone: number
  currentVideo: string
  tracksFound: number
  error?: AppError
}

export type DetectionJobResponse = {
  job?: DetectionJobSnapshot
  error?: AppError
}

export type ListPeopleRequest = {
  workspaceId: number
  minDurationSec?: number
  minConfidence?: number
  page: number
  pageSize: number
}

export type ListPeopleResponse = {
  items?: PersonGroupDTO[]
  total: number
  error?: AppError
}

export type PersonGroupResponse = {
  group?: PersonGroupDTO
  error?: AppError
}

// One box row: [t, x1, y1, x2, y2]; t = seconds into the video, coordinates normalized 0..1 of the frame.
export type TrackBoxRow = [number, number, number, number, number]

export type VideoTrackBoxesDTO = {
  trackId: number
  groupId: number
  label?: string
  startSec: number
  endSec: number
  boxes: TrackBoxRow[]
}

export type VideoTrackBoxesResponse = {
  videoId: number
  sampleFps: number // 0 when unknown
  tracks: VideoTrackBoxesDTO[]
  error?: AppError
}

export type StartPeopleCutRequest = {
  workspaceId: number
  groupIds: number[]
  name: string
  secondsBefore: number
  secondsAfter: number
}

export type DetectProgressEvent = {
  jobId: string
  videosTotal: number
  videosDone: number
  currentVideo: string
  tracksFound: number
}

export type DetectFinishedEvent = {
  jobId: string
  tracksFound: number
  elapsedMs: number
}

export type DetectFailedEvent = {
  jobId: string
  error: AppError
}
