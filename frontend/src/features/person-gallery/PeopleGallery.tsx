import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { useBackend } from '../../infrastructure/backend/context'
import type { ActionResponse, PersonGroupResponse } from '../../infrastructure/backend/types'
import { useToast } from '../../shared/toast/context'
import { Button } from '../../shared/ui/Button'
import { Input } from '../../shared/ui/Input'
import { Pagination } from '../../shared/ui/Pagination'
import { CutSelectedModal, type CutSelectedInput } from './CutSelectedModal'
import { PersonGrid } from './PersonGrid'
import { PersonPreview } from './PersonPreview'

const PAGE_SIZE = 60

export const PeopleGallery = ({
  workspaceId,
  onCutStarted,
}: {
  workspaceId: number
  onCutStarted: (jobId: string) => void
}) => {
  const backend = useBackend()
  const queryClient = useQueryClient()
  const { showToast } = useToast()

  const [minDuration, setMinDuration] = useState('0')
  const [minConfidence, setMinConfidence] = useState('0')
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [previewId, setPreviewId] = useState<number | null>(null)
  const [cutOpen, setCutOpen] = useState(false)
  const [page, setPage] = useState(1)
  const [sort, setSort] = useState<'id' | 'duration'>('id')
  const [selectAllLoading, setSelectAllLoading] = useState(false)
  const selectAllCheckboxRef = useRef<HTMLInputElement>(null)

  const minDurationSec = Math.max(0, Number(minDuration) || 0)
  const minConfidenceValue = Math.min(1, Math.max(0, (Number(minConfidence) || 0) / 100))
  const queryKey = ['people', workspaceId, minDurationSec, minConfidenceValue, sort, page]

  const peopleQuery = useQuery({
    queryKey,
    queryFn: () =>
      backend.listPeople({
        workspaceId,
        minDurationSec,
        minConfidence: minConfidenceValue,
        page,
        pageSize: PAGE_SIZE,
        sort,
      }),
    placeholderData: keepPreviousData,
  })

  const groups = useMemo(() => peopleQuery.data?.items ?? [], [peopleQuery.data?.items])
  const previewGroup = groups.find((group) => group.id === previewId) ?? null
  const total = peopleQuery.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const allSelected = total > 0 && selected.size === total
  const selectAllIndeterminate = selected.size > 0 && selected.size !== total

  useEffect(() => {
    const checkbox = selectAllCheckboxRef.current
    if (!checkbox) {
      return
    }
    checkbox.indeterminate = selectAllIndeterminate
  }, [selectAllIndeterminate])

  const goToPage = (nextPage: number) => {
    if (nextPage === page) {
      return
    }
    setPreviewId(null)
    setPage(nextPage)
  }

  const resetPage = () => {
    setPreviewId(null)
    setPage(1)
  }

  useEffect(() => {
    if (!peopleQuery.data) {
      return
    }
    if (page <= totalPages) {
      return
    }
    setPreviewId(null)
    setPage(totalPages)
  }, [peopleQuery.data, page, totalPages])

  // Merge/split change group ids, which the box overlay labels use.
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: ['videoTrackBoxes'] })
    return queryClient.invalidateQueries({ queryKey: ['people', workspaceId] })
  }

  // Shared handling for merge/split/remove: toast on error, otherwise refresh the list.
  const afterEdit = async (response: ActionResponse | PersonGroupResponse, failTitle: string) => {
    if (response.error) {
      showToast(failTitle, response.error.message, 'error')
      return false
    }
    await refresh()
    return true
  }
  const onError = (error: unknown) => {
    showToast('Unexpected error', error instanceof Error ? error.message : 'Unknown error', 'error')
  }

  const mergeMutation = useMutation({
    mutationFn: (ids: number[]) => backend.mergePeople(ids),
    onSuccess: async (response) => {
      if (await afterEdit(response, 'Merge failed')) {
        setSelected(response.group ? new Set([response.group.id]) : new Set())
      }
    },
    onError,
  })

  const splitMutation = useMutation({
    mutationFn: ({ groupId, trackId }: { groupId: number; trackId: number }) => backend.splitPerson(groupId, trackId),
    onSuccess: (response) => afterEdit(response, 'Split failed'),
    onError,
  })

  const removeMutation = useMutation({
    mutationFn: (trackId: number) => backend.removeTrack(trackId),
    onSuccess: (response) => afterEdit(response, 'Remove failed'),
    onError,
  })

  const cutMutation = useMutation({
    mutationFn: (input: CutSelectedInput) =>
      backend.startPeopleCut({ workspaceId, groupIds: [...selected], ...input }),
    onSuccess: (response) => {
      if (response.error || !response.jobId) {
        showToast('Failed to create cuts', response.error?.message ?? 'Missing job id in response', 'error')
        return
      }
      setCutOpen(false)
      onCutStarted(response.jobId)
      showToast('Cuts created', `Job ${response.jobId} is running`, 'info')
    },
    onError,
  })

  const toggle = useCallback((groupId: number, checked: boolean) => {
    setSelected((current) => {
      const next = new Set(current)
      if (checked) {
        next.add(groupId)
      } else {
        next.delete(groupId)
      }
      return next
    })
  }, [])

  const openPreview = useCallback((groupId: number) => {
    setPreviewId(groupId)
  }, [])

  const handleSelectAll = async () => {
    if (total === 0 || selectAllLoading) {
      return
    }
    if (allSelected) {
      setSelected(new Set())
      return
    }

    setSelectAllLoading(true)
    try {
      const response = await backend.listPeopleIDs({
        workspaceId,
        minDurationSec,
        minConfidence: minConfidenceValue,
      })
      if (response.error) {
        showToast('Select all failed', response.error.message, 'error')
        return
      }
      setSelected(new Set(response.ids ?? []))
    } catch (error) {
      showToast('Select all failed', error instanceof Error ? error.message : 'Unknown error', 'error')
    } finally {
      setSelectAllLoading(false)
    }
  }

  const editing = splitMutation.isPending || removeMutation.isPending

  return (
    <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
      <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
        <div className="flex flex-wrap items-end gap-3">
          <label className="flex flex-col gap-1 text-xs uppercase tracking-[0.08em] text-[var(--muted)]">
            Min duration (s)
            <Input
              value={minDuration}
              onChange={(event) => {
                setMinDuration(event.target.value)
                resetPage()
              }}
              type="number"
              min={0}
              className="w-28"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs uppercase tracking-[0.08em] text-[var(--muted)]">
            Min confidence (%)
            <Input
              value={minConfidence}
              onChange={(event) => {
                setMinConfidence(event.target.value)
                resetPage()
              }}
              type="number"
              min={0}
              max={100}
              className="w-28"
            />
          </label>
          <label className="flex flex-col gap-1 text-xs uppercase tracking-[0.08em] text-[var(--muted)]">
            Order
            <select
              value={sort}
              onChange={(event) => {
                setSort(event.target.value as 'id' | 'duration')
                resetPage()
              }}
              aria-label="Order people"
              className="w-44 rounded-md border border-[var(--border)] bg-[var(--surface)] px-3 py-2 text-sm text-[var(--text)] outline-none transition focus:border-[var(--primary)] focus:ring-2 focus:ring-[var(--primary-soft)]"
            >
              <option value="id">Person 1, 2, 3…</option>
              <option value="duration">Longest first</option>
            </select>
          </label>
        </div>

        <div className="flex items-center gap-2">
          <span className="rounded-full border border-[var(--border)] px-2 py-1 text-xs text-[var(--text)]">
            {selected.size} selected
          </span>
          <span className="inline-flex items-center gap-1.5 rounded-full border border-[var(--border)] px-2 py-1 text-xs text-[var(--text)]">
            <input
              ref={selectAllCheckboxRef}
              id="people-gallery-select-all"
              type="checkbox"
              checked={allSelected}
              aria-checked={selectAllIndeterminate ? 'mixed' : allSelected}
              disabled={total === 0 || selectAllLoading}
              onChange={() => {
                void handleSelectAll()
              }}
              className="h-3.5 w-3.5 accent-[var(--primary)]"
            />
            <label htmlFor="people-gallery-select-all" className="cursor-pointer">
              Select all
            </label>
          </span>
          <Button
            variant="secondary"
            onClick={() => mergeMutation.mutate([...selected])}
            disabled={selected.size < 2 || mergeMutation.isPending}
          >
            Merge selected
          </Button>
          <Button onClick={() => setCutOpen(true)} disabled={selected.size === 0}>
            Cut selected
          </Button>
        </div>
      </div>

      {peopleQuery.isLoading ? <p className="text-sm text-[var(--muted)]">Loading people…</p> : null}
      {peopleQuery.data?.error ? <p className="mb-3 text-sm text-[var(--danger)]">{peopleQuery.data.error.message}</p> : null}
      <PersonGrid groups={groups} selectedIds={selected} onToggle={toggle} onOpen={openPreview} />

      {total > 0 ? <Pagination currentPage={page} totalPages={totalPages} onPageChange={goToPage} /> : null}

      {previewGroup ? (
        <PersonPreview
          key={previewGroup.id}
          group={previewGroup}
          busy={editing}
          onClose={() => setPreviewId(null)}
          onSplit={(trackId) => splitMutation.mutate({ groupId: previewGroup.id, trackId })}
          onRemove={(trackId) => removeMutation.mutate(trackId)}
        />
      ) : null}

      <CutSelectedModal
        open={cutOpen}
        count={selected.size}
        submitting={cutMutation.isPending}
        onClose={() => setCutOpen(false)}
        onSubmit={(input) => cutMutation.mutate(input)}
      />
    </section>
  )
}
