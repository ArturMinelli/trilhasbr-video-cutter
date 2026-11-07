import { EventsOn } from '../../../wailsjs/runtime/runtime'
import type { BackendClient } from './backendClient'
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

type GoAppInvoker = {
  ListWorkspaces: () => Promise<ListWorkspacesResponse>
  LoadWorkspaceByDialog: () => Promise<LoadWorkspaceResponse>
  RefreshWorkspace: (workspaceId: number) => Promise<LoadWorkspaceResponse>
  DeleteWorkspace: (workspaceId: number) => Promise<ActionResponse>
  GetWorkspaceVideos: (workspaceId: number, page: number, pageSize: number) => Promise<PagedVideosResponse>
  GetWorkspaceExplorer: (workspaceId: number) => Promise<WorkspaceExplorerResponse>
  GetVideoPreview: (videoId: number) => Promise<VideoPreviewResponse>
  ShiftVideosTime: (request: ShiftVideosTimeRequest) => Promise<ShiftVideosTimeResponse>
  StartCut: (request: StartCutRequest) => Promise<StartCutResponse>
  PreviewCutsFromCSV: (request: PreviewCutsFromCSVRequest) => Promise<PreviewCutsFromCSVResponse>
  StartCutsFromCSV: (request: StartCutsFromCSVRequest) => Promise<StartCutsFromCSVResponse>
  GetCutJob: (jobId: string) => Promise<CutJobStatusResponse>
  OpenFolder: (path: string) => Promise<ActionResponse>
  StartDetection: (request: StartDetectionRequest) => Promise<StartDetectionResponse>
  CancelDetection: (jobId: string) => Promise<ActionResponse>
  GetDetectionJob: (jobId: string) => Promise<DetectionJobResponse>
  ListPeople: (request: ListPeopleRequest) => Promise<ListPeopleResponse>
  MergePeople: (groupIds: number[]) => Promise<PersonGroupResponse>
  SplitPerson: (groupId: number, trackId: number) => Promise<PersonGroupResponse>
  RemoveTrack: (trackId: number) => Promise<ActionResponse>
  RenamePerson: (groupId: number, label: string) => Promise<ActionResponse>
  StartPeopleCut: (request: StartPeopleCutRequest) => Promise<StartCutResponse>
  GetVideoTrackBoxes: (videoId: number) => Promise<VideoTrackBoxesResponse>
}

const getGoApp = (): GoAppInvoker => {
  const app = (window as unknown as { go?: { main?: { App?: GoAppInvoker } } }).go?.main?.App
  if (!app) {
    throw new Error('Wails backend is not available')
  }
  return app
}

export class WailsBackendClient implements BackendClient {
  async listWorkspaces(): Promise<ListWorkspacesResponse> {
    return getGoApp().ListWorkspaces()
  }

  async loadWorkspaceByDialog(): Promise<LoadWorkspaceResponse> {
    return getGoApp().LoadWorkspaceByDialog()
  }

  async refreshWorkspace(workspaceId: number): Promise<LoadWorkspaceResponse> {
    return getGoApp().RefreshWorkspace(workspaceId)
  }

  async deleteWorkspace(workspaceId: number): Promise<ActionResponse> {
    return getGoApp().DeleteWorkspace(workspaceId)
  }

  async getWorkspaceVideos(workspaceId: number, page: number, pageSize: number): Promise<PagedVideosResponse> {
    return getGoApp().GetWorkspaceVideos(workspaceId, page, pageSize)
  }

  async getWorkspaceExplorer(workspaceId: number): Promise<WorkspaceExplorerResponse> {
    return getGoApp().GetWorkspaceExplorer(workspaceId)
  }

  async getVideoPreview(videoId: number): Promise<VideoPreviewResponse> {
    return getGoApp().GetVideoPreview(videoId)
  }

  async shiftVideosTime(request: ShiftVideosTimeRequest): Promise<ShiftVideosTimeResponse> {
    return getGoApp().ShiftVideosTime(request)
  }

  async startCut(request: StartCutRequest): Promise<StartCutResponse> {
    return getGoApp().StartCut(request)
  }

  async previewCutsFromCSV(request: PreviewCutsFromCSVRequest): Promise<PreviewCutsFromCSVResponse> {
    return getGoApp().PreviewCutsFromCSV(request)
  }

  async startCutsFromCSV(request: StartCutsFromCSVRequest): Promise<StartCutsFromCSVResponse> {
    return getGoApp().StartCutsFromCSV(request)
  }

  async getCutJob(jobId: string): Promise<CutJobStatusResponse> {
    return getGoApp().GetCutJob(jobId)
  }

  async openFolder(path: string): Promise<ActionResponse> {
    return getGoApp().OpenFolder(path)
  }

  async startDetection(request: StartDetectionRequest): Promise<StartDetectionResponse> {
    return getGoApp().StartDetection(request)
  }

  async cancelDetection(jobId: string): Promise<ActionResponse> {
    return getGoApp().CancelDetection(jobId)
  }

  async getDetectionJob(jobId: string): Promise<DetectionJobResponse> {
    return getGoApp().GetDetectionJob(jobId)
  }

  async listPeople(request: ListPeopleRequest): Promise<ListPeopleResponse> {
    return getGoApp().ListPeople(request)
  }

  async mergePeople(groupIds: number[]): Promise<PersonGroupResponse> {
    return getGoApp().MergePeople(groupIds)
  }

  async splitPerson(groupId: number, trackId: number): Promise<PersonGroupResponse> {
    return getGoApp().SplitPerson(groupId, trackId)
  }

  async removeTrack(trackId: number): Promise<ActionResponse> {
    return getGoApp().RemoveTrack(trackId)
  }

  async renamePerson(groupId: number, label: string): Promise<ActionResponse> {
    return getGoApp().RenamePerson(groupId, label)
  }

  async startPeopleCut(request: StartPeopleCutRequest): Promise<StartCutResponse> {
    return getGoApp().StartPeopleCut(request)
  }

  async getVideoTrackBoxes(videoId: number): Promise<VideoTrackBoxesResponse> {
    return getGoApp().GetVideoTrackBoxes(videoId)
  }

  onCutProgress(handler: (event: CutProgressEvent) => void): () => void {
    return EventsOn('cut:progress', (event: CutProgressEvent) => {
      handler(event)
    })
  }

  onCutFinished(handler: (event: CutFinishedEvent) => void): () => void {
    return EventsOn('cut:finished', (event: CutFinishedEvent) => {
      handler(event)
    })
  }

  onCutFailed(handler: (event: CutFailedEvent) => void): () => void {
    return EventsOn('cut:failed', (event: CutFailedEvent) => {
      handler(event)
    })
  }

  onCutCSVProgress(handler: (event: CutCSVProgressEvent) => void): () => void {
    return EventsOn('cut:csv:progress', (event: CutCSVProgressEvent) => {
      handler(event)
    })
  }

  onCutCSVFinished(handler: (event: CutCSVFinishedEvent) => void): () => void {
    return EventsOn('cut:csv:finished', (event: CutCSVFinishedEvent) => {
      handler(event)
    })
  }

  onCutCSVFailed(handler: (event: CutCSVFailedEvent) => void): () => void {
    return EventsOn('cut:csv:failed', (event: CutCSVFailedEvent) => {
      handler(event)
    })
  }

  onDetectProgress(handler: (event: DetectProgressEvent) => void): () => void {
    return EventsOn('detect:progress', (event: DetectProgressEvent) => {
      handler(event)
    })
  }

  onDetectFinished(handler: (event: DetectFinishedEvent) => void): () => void {
    return EventsOn('detect:finished', (event: DetectFinishedEvent) => {
      handler(event)
    })
  }

  onDetectFailed(handler: (event: DetectFailedEvent) => void): () => void {
    return EventsOn('detect:failed', (event: DetectFailedEvent) => {
      handler(event)
    })
  }
}
