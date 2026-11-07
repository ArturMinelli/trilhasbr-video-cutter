import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { useBackend } from '../../infrastructure/backend/context'
import type { ActionResponse, PersonGroupResponse } from '../../infrastructure/backend/types'
import { useToast } from '../../shared/toast/context'
import { Button } from '../../shared/ui/Button'
import { Input } from '../../shared/ui/Input'
import { CutSelectedModal, type CutSelectedInput } from './CutSelectedModal'
import { PersonGrid } from './PersonGrid'
import { PersonPreview } from './PersonPreview'

// ponytail: one page of 500 groups, no pager; add pagination if a workspace exceeds this.
const PAGE_SIZE = 500

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

  const minDurationSec = Math.max(0, Number(minDuration) || 0)
  const minConfidenceValue = Math.min(1, Math.max(0, (Number(minConfidence) || 0) / 100))
  const queryKey = ['people', workspaceId, minDurationSec, minConfidenceValue]

  const peopleQuery = useQuery({
    queryKey,
    queryFn: () =>
      backend.listPeople({
        workspaceId,
        minDurationSec,
        minConfidence: minConfidenceValue,
        page: 1,
        pageSize: PAGE_SIZE,
      }),
  })

  const groups = useMemo(() => peopleQuery.data?.items ?? [], [peopleQuery.data?.items])
  const selectedIds = useMemo(() => new Set(groups.filter((g) => selected.has(g.id)).map((g) => g.id)), [groups, selected])
  const previewGroup = groups.find((group) => group.id === previewId) ?? null

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
      backend.startPeopleCut({ workspaceId, groupIds: [...selectedIds], ...input }),
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

  const toggle = (groupId: number, checked: boolean) =>
    setSelected((current) => {
      const next = new Set(current)
      if (checked) {
        next.add(groupId)
      } else {
        next.delete(groupId)
      }
      return next
    })

  const total = peopleQuery.data?.total ?? 0
  const editing = splitMutation.isPending || removeMutation.isPending

  return (
    <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
      <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
        <div className="flex flex-wrap items-end gap-3">
          <label className="flex flex-col gap-1 text-xs uppercase tracking-[0.08em] text-[var(--muted)]">
            Min duration (s)
            <Input value={minDuration} onChange={(event) => setMinDuration(event.target.value)} type="number" min={0} className="w-28" />
          </label>
          <label className="flex flex-col gap-1 text-xs uppercase tracking-[0.08em] text-[var(--muted)]">
            Min confidence (%)
            <Input
              value={minConfidence}
              onChange={(event) => setMinConfidence(event.target.value)}
              type="number"
              min={0}
              max={100}
              className="w-28"
            />
          </label>
        </div>

        <div className="flex items-center gap-2">
          <span className="rounded-full border border-[var(--border)] px-2 py-1 text-xs text-[var(--text)]">
            {selectedIds.size} selected
          </span>
          <Button
            variant="secondary"
            onClick={() => mergeMutation.mutate([...selectedIds])}
            disabled={selectedIds.size < 2 || mergeMutation.isPending}
          >
            Merge selected
          </Button>
          <Button onClick={() => setCutOpen(true)} disabled={selectedIds.size === 0}>
            Cut selected
          </Button>
        </div>
      </div>

      {peopleQuery.isLoading ? <p className="text-sm text-[var(--muted)]">Loading people…</p> : null}
      {peopleQuery.data?.error ? <p className="mb-3 text-sm text-[var(--danger)]">{peopleQuery.data.error.message}</p> : null}
      {total > groups.length ? (
        <p className="mb-3 text-xs text-[var(--muted)]">
          Showing {groups.length} of {total}. Raise the filters to narrow the list.
        </p>
      ) : null}

      <PersonGrid groups={groups} selectedIds={selectedIds} onToggle={toggle} onOpen={(group) => setPreviewId(group.id)} />

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
        count={selectedIds.size}
        submitting={cutMutation.isPending}
        onClose={() => setCutOpen(false)}
        onSubmit={(input) => cutMutation.mutate(input)}
      />
    </section>
  )
}
