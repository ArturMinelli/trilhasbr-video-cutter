import { useCallback, useEffect, useDeferredValue, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'

import { CutModal, type CutModalInput } from '../features/cut-runner/CutModal'
import { CsvCutImportModal } from '../features/cut-import/CsvCutImportModal'
import { DetectButton } from '../features/person-detection/DetectButton'
import { DetectProgress } from '../features/person-detection/DetectProgress'
import { useDetection } from '../features/person-detection/useDetection'
import { PeopleGallery } from '../features/person-gallery/PeopleGallery'
import { ShiftTimeModal, type ShiftTimeModalInput } from '../features/video-shift/ShiftTimeModal'
import { DeleteWorkspaceModal } from '../features/workspace-manage/DeleteWorkspaceModal'
import { RefreshWorkspaceButton } from '../features/workspace-manage/RefreshWorkspaceButton'
import { FolderTree } from '../features/workspace-explorer/FolderTree'
import { buildExplorerModel, splitFolderPath } from '../features/workspace-explorer/model'
import { fetchVideoPreviewCached } from '../features/workspace-explorer/previewCache'
import { VideoCard } from '../features/workspace-explorer/VideoCard'
import { useBackend } from '../infrastructure/backend/context'
import type { PreviewCutsFromCSVResponse, StartCutsFromCSVCutInput, VideoExplorer } from '../infrastructure/backend/types'
import { useToast } from '../shared/toast/context'
import { Button } from '../shared/ui/Button'

const isShiftableVideo = (video: VideoExplorer): boolean => Boolean(video.startedAt && video.finishedAt)

type WorkspaceDetectionControlsProps = {
  workspaceId: number
  progressHost: HTMLDivElement | null
  onDetectingChange: (detecting: boolean) => void
  onSettled: () => void
  onCompleted: () => void
}

const WorkspaceDetectionControls = ({
  workspaceId,
  progressHost,
  onDetectingChange,
  onSettled,
  onCompleted,
}: WorkspaceDetectionControlsProps) => {
  const detection = useDetection(workspaceId, onSettled, onCompleted)
  const isDetecting = detection.state.status === 'starting' || detection.state.status === 'running'

  useEffect(() => {
    onDetectingChange(isDetecting)
  }, [isDetecting, onDetectingChange])

  return (
    <>
      <DetectButton detection={detection} />
      {progressHost ? createPortal(<DetectProgress detection={detection} />, progressHost) : null}
    </>
  )
}

export const WorkspaceDetailPage = () => {
  const backend = useBackend()
  const queryClient = useQueryClient()
  const { showToast } = useToast()
  const navigate = useNavigate()

  const { workspaceId: workspaceIdParam } = useParams()
  const workspaceId = Number(workspaceIdParam)
  const isWorkspaceIdValid = Number.isFinite(workspaceId) && workspaceId > 0

  const [isModalOpen, setIsModalOpen] = useState(false)
  const [isShiftModalOpen, setIsShiftModalOpen] = useState(false)
  const [isCSVModalOpen, setIsCSVModalOpen] = useState(false)
  const [isDeleteOpen, setIsDeleteOpen] = useState(false)
  const [csvPreview, setCSVPreview] = useState<PreviewCutsFromCSVResponse | null>(null)
  const [view, setView] = useState<'videos' | 'people'>('videos')
  const [isDetecting, setIsDetecting] = useState(false)
  const [progressHost, setProgressHost] = useState<HTMLDivElement | null>(null)
  const [currentFolderPath, setCurrentFolderPath] = useState('')
  const [selectedVideoIds, setSelectedVideoIds] = useState<Set<number>>(new Set())
  const selectAllRef = useRef<HTMLInputElement | null>(null)
  const cutWatchersRef = useRef<Set<() => void>>(new Set())
  const csvWatchersRef = useRef<Set<() => void>>(new Set())

  const workspacesQuery = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => backend.listWorkspaces(),
    staleTime: 30_000,
  })

  const workspace = useMemo(
    () => workspacesQuery.data?.workspaces?.find((candidate) => candidate.id === workspaceId),
    [workspacesQuery.data?.workspaces, workspaceId],
  )

  const explorerQuery = useQuery({
    queryKey: ['workspace-explorer', workspaceId],
    queryFn: () => backend.getWorkspaceExplorer(workspaceId),
    enabled: isWorkspaceIdValid,
    staleTime: 30_000,
  })

  const onDetectionSettled = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ['people', workspaceId] })
  }, [queryClient, workspaceId])

  const onDetectionCompleted = useCallback(() => {
    onDetectionSettled()
    setView('people')
  }, [onDetectionSettled])

  const handleDetectingChange = useCallback((detecting: boolean) => {
    setIsDetecting(detecting)
  }, [])

  const loadPreview = useCallback(
    (videoId: number) => {
      return fetchVideoPreviewCached(videoId, () => backend.getVideoPreview(videoId))
    },
    [backend],
  )

  const deferredExplorerVideos = useDeferredValue(explorerQuery.data?.videos)
  const model = useMemo(() => buildExplorerModel(deferredExplorerVideos ?? []), [deferredExplorerVideos])
  const childFolders = useMemo(() => model.folderChildrenByDir.get(currentFolderPath) ?? [], [model, currentFolderPath])
  const videosInCurrentFolder = useMemo(() => model.videosByDir.get(currentFolderPath) ?? [], [model, currentFolderPath])
  const shiftableVideosInCurrentFolder = useMemo(
    () => videosInCurrentFolder.filter(isShiftableVideo),
    [videosInCurrentFolder],
  )
  const selectedVideos = useMemo(
    () => (explorerQuery.data?.videos ?? []).filter((video) => selectedVideoIds.has(video.id) && isShiftableVideo(video)),
    [explorerQuery.data?.videos, selectedVideoIds],
  )
  const selectedInCurrentFolderCount = useMemo(
    () => shiftableVideosInCurrentFolder.filter((video) => selectedVideoIds.has(video.id)).length,
    [shiftableVideosInCurrentFolder, selectedVideoIds],
  )
  const isCurrentFolderAllSelected =
    shiftableVideosInCurrentFolder.length > 0 && selectedInCurrentFolderCount === shiftableVideosInCurrentFolder.length
  const isCurrentFolderPartiallySelected =
    selectedInCurrentFolderCount > 0 && selectedInCurrentFolderCount < shiftableVideosInCurrentFolder.length

  useEffect(() => {
    setView('videos')
    setCurrentFolderPath('')
    setSelectedVideoIds(new Set())
    setIsShiftModalOpen(false)
    setIsCSVModalOpen(false)
    setCSVPreview(null)
  }, [workspaceId])

  useEffect(() => {
    if (!model.folderChildrenByDir.has(currentFolderPath) && currentFolderPath !== '') {
      setCurrentFolderPath('')
    }
  }, [model, currentFolderPath])

  useEffect(() => {
    const available = new Set(
      (explorerQuery.data?.videos ?? []).filter((video) => isShiftableVideo(video)).map((video) => video.id),
    )
    setSelectedVideoIds((current) => {
      const next = new Set<number>()
      for (const videoID of current) {
        if (available.has(videoID)) {
          next.add(videoID)
        }
      }
      if (next.size === current.size) {
        let changed = false
        for (const videoID of current) {
          if (!next.has(videoID)) {
            changed = true
            break
          }
        }
        if (!changed) {
          return current
        }
      }
      return next
    })
  }, [explorerQuery.data?.videos])

  useEffect(() => {
    if (!selectAllRef.current) {
      return
    }
    selectAllRef.current.indeterminate = isCurrentFolderPartiallySelected
  }, [isCurrentFolderPartiallySelected])

  useEffect(() => {
    return () => {
      for (const cleanup of cutWatchersRef.current) {
        cleanup()
      }
      cutWatchersRef.current.clear()

      for (const cleanup of csvWatchersRef.current) {
        cleanup()
      }
      csvWatchersRef.current.clear()
    }
  }, [])

  const watchCutJob = useCallback(
    (jobId: string) => {
      let unsubscribeFinished = () => {}
      let unsubscribeFailed = () => {}

      const cleanup = () => {
        unsubscribeFinished()
        unsubscribeFailed()
      }

      const removeWatcher = () => {
        cleanup()
        cutWatchersRef.current.delete(removeWatcher)
      }

      unsubscribeFinished = backend.onCutFinished(async (event) => {
        if (event.jobId !== jobId) {
          return
        }

        removeWatcher()
        showToast('Cut finished', 'Your cut was generated successfully', 'success')

        const openFolderResponse = await backend.openFolder(event.result.outputDirectory)
        if (openFolderResponse.error) {
          showToast('Unable to open output folder', openFolderResponse.error.message, 'error')
        }
      })

      unsubscribeFailed = backend.onCutFailed((event) => {
        if (event.jobId !== jobId) {
          return
        }

        removeWatcher()
        showToast('Cut failed', event.error.message, 'error')
      })

      cutWatchersRef.current.add(removeWatcher)
    },
    [backend, showToast],
  )

  const watchCSVCutJob = useCallback(
    (jobId: string) => {
      let unsubscribeFinished = () => {}
      let unsubscribeFailed = () => {}

      const cleanup = () => {
        unsubscribeFinished()
        unsubscribeFailed()
      }

      const removeWatcher = () => {
        cleanup()
        csvWatchersRef.current.delete(removeWatcher)
      }

      unsubscribeFinished = backend.onCutCSVFinished(async (event) => {
        if (event.jobId !== jobId) {
          return
        }

        removeWatcher()
        showToast(
          'CSV cut finished',
          `Generated ${event.completedCuts} cut${event.completedCuts === 1 ? '' : 's'} successfully.`,
          'success',
        )

        const openFolderResponse = await backend.openFolder(event.outputDirectory)
        if (openFolderResponse.error) {
          showToast('Unable to open output folder', openFolderResponse.error.message, 'error')
        }
      })

      unsubscribeFailed = backend.onCutCSVFailed((event) => {
        if (event.jobId !== jobId) {
          return
        }

        removeWatcher()
        const rowPrefix = event.row > 0 ? `Row ${event.row}${event.name ? ` (${event.name})` : ''}: ` : ''
        showToast('CSV cut failed', `${rowPrefix}${event.error.message}`, 'error')
      })

      csvWatchersRef.current.add(removeWatcher)
    },
    [backend, showToast],
  )

  const startCutMutation = useMutation({
    mutationFn: (input: CutModalInput) =>
      backend.startCut({
        workspaceId,
        name: input.name,
        datetimeLocal: input.datetimeLocal,
        secondsBefore: input.secondsBefore,
        secondsAfter: input.secondsAfter,
      }),
    onSuccess: (response) => {
      if (response.error) {
        showToast('Failed to create cut', response.error.message, 'error')
        return
      }
      if (!response.jobId) {
        showToast('Failed to create cut', 'Missing job id in response', 'error')
        return
      }

      setIsModalOpen(false)
      watchCutJob(response.jobId)
      showToast('Cut created', `Job ${response.jobId} is running`, 'info')
    },
    onError: (error: unknown) => {
      showToast('Unexpected error', error instanceof Error ? error.message : 'Unknown error', 'error')
    },
  })

  const previewCSVCutsMutation = useMutation({
    mutationFn: () => backend.previewCutsFromCSV({ workspaceId }),
    onSuccess: (response) => {
      if (response.cancelled) {
        return
      }

      if (response.error && (!response.issues || response.issues.length === 0)) {
        showToast('CSV import failed', response.error.message, 'error')
        return
      }

      setCSVPreview(response)
      setIsCSVModalOpen(true)

      if (response.issues && response.issues.length > 0) {
        showToast('CSV validation failed', `Found ${response.issues.length} issue${response.issues.length === 1 ? '' : 's'}.`, 'error')
      }
    },
    onError: (error: unknown) => {
      showToast('Unexpected error', error instanceof Error ? error.message : 'Unknown error', 'error')
    },
  })

  const startCSVCutsMutation = useMutation({
    mutationFn: (cuts: StartCutsFromCSVCutInput[]) =>
      backend.startCutsFromCSV({
        workspaceId,
        cuts,
      }),
    onSuccess: (response) => {
      if (response.error) {
        showToast('Failed to start CSV cuts', response.error.message, 'error')
        return
      }
      if (!response.jobId) {
        showToast('Failed to start CSV cuts', 'Missing job id in response', 'error')
        return
      }

      setIsCSVModalOpen(false)
      watchCSVCutJob(response.jobId)
      showToast('CSV cut started', `Job ${response.jobId} is running`, 'info')
    },
    onError: (error: unknown) => {
      showToast('Unexpected error', error instanceof Error ? error.message : 'Unknown error', 'error')
    },
  })

  const shiftVideosMutation = useMutation({
    mutationFn: (input: ShiftTimeModalInput) =>
      backend.shiftVideosTime({
        workspaceId,
        videoIds: input.videoIds,
        shiftMilliseconds: input.shiftMilliseconds,
      }),
    onSuccess: async (response) => {
      if (response.error) {
        showToast('Shift failed', response.error.message, 'error')
        return
      }

      setIsShiftModalOpen(false)
      setSelectedVideoIds(new Set())
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['workspace-explorer', workspaceId] }),
        queryClient.invalidateQueries({ queryKey: ['workspaces'] }),
      ])
      showToast('Shift applied', `Updated ${response.updated} video timestamp${response.updated === 1 ? '' : 's'}.`, 'success')
    },
    onError: (error: unknown) => {
      showToast('Unexpected error', error instanceof Error ? error.message : 'Unknown error', 'error')
    },
  })

  const toggleVideoSelection = useCallback((videoId: number, checked: boolean) => {
    setSelectedVideoIds((current) => {
      const next = new Set(current)
      if (checked) {
        next.add(videoId)
      } else {
        next.delete(videoId)
      }
      return next
    })
  }, [])

  const toggleCurrentFolderSelection = useCallback(
    (checked: boolean) => {
      setSelectedVideoIds((current) => {
        const next = new Set(current)
        for (const video of shiftableVideosInCurrentFolder) {
          if (checked) {
            next.add(video.id)
          } else {
            next.delete(video.id)
          }
        }
        return next
      })
    },
    [shiftableVideosInCurrentFolder],
  )

  if (!isWorkspaceIdValid) {
    return (
      <div className="mx-auto max-w-[1500px] p-4 md:p-6">
        <p className="text-sm text-[var(--danger)]">Invalid workspace id.</p>
      </div>
    )
  }

  const breadcrumbSegments = splitFolderPath(currentFolderPath)
  const breadcrumbParts = ['Root', ...breadcrumbSegments]

  return (
    <div className="mx-auto max-w-[1500px] p-4 md:p-6">
      <header className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <Link to="/" className="text-xs text-[var(--primary)] hover:underline">
            ← Back
          </Link>
          <h1 className="mt-1 text-2xl font-extrabold tracking-[0.05em]">{workspace?.name ?? `Workspace ${workspaceId}`}</h1>
          <p className="text-sm text-[var(--muted)]">{workspace?.directory ?? explorerQuery.data?.workspaceDirectory ?? ''}</p>
        </div>

        <div className="flex flex-wrap items-start gap-2">
          <WorkspaceDetectionControls
            workspaceId={workspaceId}
            progressHost={progressHost}
            onDetectingChange={handleDetectingChange}
            onSettled={onDetectionSettled}
            onCompleted={onDetectionCompleted}
          />
          <RefreshWorkspaceButton workspaceId={workspaceId} disabled={isDetecting} />
          <Button variant="secondary" onClick={() => setIsDeleteOpen(true)} disabled={isDetecting}>
            Delete
          </Button>
          <Button variant="secondary" onClick={() => setIsShiftModalOpen(true)} disabled={selectedVideos.length === 0}>
            Shift time {selectedVideos.length > 0 ? `(${selectedVideos.length})` : ''}
          </Button>
          <Button variant="secondary" onClick={() => previewCSVCutsMutation.mutate()} disabled={previewCSVCutsMutation.isPending}>
            {previewCSVCutsMutation.isPending ? 'Loading CSV…' : 'Import CSV'}
          </Button>
          <Button onClick={() => setIsModalOpen(true)}>Cut</Button>
        </div>
      </header>

      <div ref={setProgressHost} />

      <div className="mb-4 flex gap-2" role="tablist">
        {(['videos', 'people'] as const).map((tab) => (
          <Button key={tab} variant={view === tab ? 'primary' : 'secondary'} role="tab" aria-selected={view === tab} onClick={() => setView(tab)}>
            {tab === 'videos' ? 'Videos' : 'People'}
          </Button>
        ))}
      </div>

      {explorerQuery.isLoading ? <p className="mb-3 text-sm text-[var(--muted)]">Loading workspace videos…</p> : null}
      {explorerQuery.data?.error ? <p className="mb-3 text-sm text-[var(--danger)]">{explorerQuery.data.error.message}</p> : null}

      {view === 'people' ? <PeopleGallery workspaceId={workspaceId} onCutStarted={watchCutJob} /> : null}

      {view === 'videos' ? (
        <div className="grid gap-4 lg:grid-cols-[280px_minmax(0,1fr)]">
        <aside className="lg:sticky lg:top-4 lg:max-h-[calc(100vh-2rem)] lg:self-start lg:overflow-y-auto">
          <FolderTree root={model.root} selectedPath={currentFolderPath} onSelect={setCurrentFolderPath} />
        </aside>

        <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
          <div className="mb-4 flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.08em] text-[var(--muted)]">
            {breadcrumbParts.map((part, index) => {
              const folderPath = index === 0 ? '' : breadcrumbSegments.slice(0, index).join('/')
              const isLast = index === breadcrumbParts.length - 1
              return (
                <div key={`${part}-${folderPath}`} className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => setCurrentFolderPath(folderPath)}
                    className={`truncate ${isLast ? 'text-[var(--text)]' : 'hover:text-[var(--text)]'}`}
                    disabled={isLast}
                  >
                    {part}
                  </button>
                  {!isLast ? <span>/</span> : null}
                </div>
              )
            })}
          </div>

          {childFolders.length > 0 ? (
            <div className="mb-5">
              <h2 className="mb-2 text-xs font-bold uppercase tracking-[0.1em] text-[var(--muted)]">Folders</h2>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                {childFolders.map((folder) => (
                  <button
                    key={folder.path}
                    type="button"
                    onClick={() => setCurrentFolderPath(folder.path)}
                    className="rounded-lg border border-[var(--border)] bg-[var(--surface-soft)] p-3 text-left transition hover:border-[var(--primary)]"
                  >
                    <p className="text-sm font-semibold text-[var(--text)]">📁 {folder.name}</p>
                    <p className="mt-1 truncate text-xs text-[var(--muted)]">{folder.path}</p>
                  </button>
                ))}
              </div>
            </div>
          ) : null}

          {videosInCurrentFolder.length > 0 ? (
            <div>
              <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                <h2 className="text-xs font-bold uppercase tracking-[0.1em] text-[var(--muted)]">Videos</h2>
                <div className="flex items-center gap-3 text-xs">
                  <label className="inline-flex items-center gap-2 text-[var(--muted)]">
                    <input
                      ref={selectAllRef}
                      type="checkbox"
                      checked={isCurrentFolderAllSelected}
                      onChange={(event) => toggleCurrentFolderSelection(event.target.checked)}
                      disabled={shiftableVideosInCurrentFolder.length === 0}
                      className="h-4 w-4 accent-[var(--primary)]"
                    />
                    Select all in folder
                  </label>
                  <span className="rounded-full border border-[var(--border)] px-2 py-1 text-[var(--text)]">
                    {selectedInCurrentFolderCount}/{shiftableVideosInCurrentFolder.length} selected
                  </span>
                </div>
              </div>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                {videosInCurrentFolder.map((video) => (
                  <VideoCard
                    key={video.id}
                    video={video}
                    loadPreview={loadPreview}
                    selected={selectedVideoIds.has(video.id)}
                    selectionDisabled={!isShiftableVideo(video)}
                    onSelectionChange={toggleVideoSelection}
                  />
                ))}
              </div>
            </div>
          ) : null}

          {childFolders.length === 0 && videosInCurrentFolder.length === 0 ? (
            <div className="rounded-lg border border-dashed border-[var(--border)] p-5 text-sm text-[var(--muted)]">
              This folder has no indexed videos.
            </div>
          ) : null}
        </section>
      </div>
      ) : null}

      <CutModal
        open={isModalOpen}
        submitting={startCutMutation.isPending}
        onClose={() => setIsModalOpen(false)}
        onSubmit={(input) => startCutMutation.mutate(input)}
      />

      <CsvCutImportModal
        open={isCSVModalOpen}
        fileName={csvPreview?.fileName ?? ''}
        cuts={csvPreview?.cuts ?? []}
        issues={csvPreview?.issues ?? []}
        submitting={startCSVCutsMutation.isPending}
        onClose={() => {
          setIsCSVModalOpen(false)
          setCSVPreview(null)
        }}
        onConfirm={() => {
          const cuts = (csvPreview?.cuts ?? []).map<StartCutsFromCSVCutInput>((cut) => ({
            row: cut.row,
            name: cut.name,
            datetimeLocal: cut.datetimeLocal,
            secondsBefore: cut.secondsBefore,
            secondsAfter: cut.secondsAfter,
          }))
          startCSVCutsMutation.mutate(cuts)
        }}
      />

      <DeleteWorkspaceModal
        workspace={isDeleteOpen && workspace ? workspace : null}
        onClose={() => setIsDeleteOpen(false)}
        onDeleted={() => navigate('/')}
      />

      {isShiftModalOpen ? (
        <ShiftTimeModal
          open={isShiftModalOpen}
          videos={selectedVideos}
          submitting={shiftVideosMutation.isPending}
          onClose={() => setIsShiftModalOpen(false)}
          onSubmit={(input) => shiftVideosMutation.mutate(input)}
        />
      ) : null}
    </div>
  )
}
