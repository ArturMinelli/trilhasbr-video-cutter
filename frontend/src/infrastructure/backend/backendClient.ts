import type {
  ActionResponse,
  CutCSVFailedEvent,
  CutCSVFinishedEvent,
  CutCSVProgressEvent,
  CutFailedEvent,
  CutFinishedEvent,
  CutJobStatusResponse,
  CutProgressEvent,
  DetectFailedEvent,
  DetectFinishedEvent,
  DetectProgressEvent,
  DetectionJobResponse,
  ListPeopleIDsRequest,
  ListPeopleIDsResponse,
  ListPeopleRequest,
  ListPeopleResponse,
  ListWorkspacesResponse,
  LoadWorkspaceResponse,
  PagedVideosResponse,
  PersonGroupResponse,
  PreviewCutsFromCSVRequest,
  PreviewCutsFromCSVResponse,
  ShiftVideosTimeRequest,
  ShiftVideosTimeResponse,
  StartCutRequest,
  StartCutResponse,
  StartCutsFromCSVRequest,
  StartCutsFromCSVResponse,
  StartDetectionRequest,
  StartDetectionResponse,
  StartPeopleCutRequest,
  VideoPreviewResponse,
  VideoTrackBoxesResponse,
  WorkspaceExplorerResponse,
} from './types'

export interface BackendClient {
  listWorkspaces(): Promise<ListWorkspacesResponse>
  loadWorkspaceByDialog(): Promise<LoadWorkspaceResponse>
  refreshWorkspace(workspaceId: number): Promise<LoadWorkspaceResponse>
  deleteWorkspace(workspaceId: number): Promise<ActionResponse>
  getWorkspaceVideos(workspaceId: number, page: number, pageSize: number): Promise<PagedVideosResponse>
  getWorkspaceExplorer(workspaceId: number): Promise<WorkspaceExplorerResponse>
  getVideoPreview(videoId: number): Promise<VideoPreviewResponse>
  shiftVideosTime(request: ShiftVideosTimeRequest): Promise<ShiftVideosTimeResponse>
  startCut(request: StartCutRequest): Promise<StartCutResponse>
  previewCutsFromCSV(request: PreviewCutsFromCSVRequest): Promise<PreviewCutsFromCSVResponse>
  startCutsFromCSV(request: StartCutsFromCSVRequest): Promise<StartCutsFromCSVResponse>
  getCutJob(jobId: string): Promise<CutJobStatusResponse>
  openFolder(path: string): Promise<ActionResponse>
  startDetection(request: StartDetectionRequest): Promise<StartDetectionResponse>
  cancelDetection(jobId: string): Promise<ActionResponse>
  getDetectionJob(jobId: string): Promise<DetectionJobResponse>
  listPeople(request: ListPeopleRequest): Promise<ListPeopleResponse>
  listPeopleIDs(request: ListPeopleIDsRequest): Promise<ListPeopleIDsResponse>
  mergePeople(groupIds: number[]): Promise<PersonGroupResponse>
  splitPerson(groupId: number, trackId: number): Promise<PersonGroupResponse>
  removeTrack(trackId: number): Promise<ActionResponse>
  renamePerson(groupId: number, label: string): Promise<ActionResponse>
  startPeopleCut(request: StartPeopleCutRequest): Promise<StartCutResponse>
  getVideoTrackBoxes(videoId: number): Promise<VideoTrackBoxesResponse>
  onCutProgress(handler: (event: CutProgressEvent) => void): () => void
  onCutFinished(handler: (event: CutFinishedEvent) => void): () => void
  onCutFailed(handler: (event: CutFailedEvent) => void): () => void
  onCutCSVProgress(handler: (event: CutCSVProgressEvent) => void): () => void
  onCutCSVFinished(handler: (event: CutCSVFinishedEvent) => void): () => void
  onCutCSVFailed(handler: (event: CutCSVFailedEvent) => void): () => void
  onDetectProgress(handler: (event: DetectProgressEvent) => void): () => void
  onDetectFinished(handler: (event: DetectFinishedEvent) => void): () => void
  onDetectFailed(handler: (event: DetectFailedEvent) => void): () => void
}
