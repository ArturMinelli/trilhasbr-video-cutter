import { useMutation, useQueryClient } from '@tanstack/react-query'

import { useBackend } from '../../infrastructure/backend/context'
import { useToast } from '../../shared/toast/context'
import { Button } from '../../shared/ui/Button'

export const RefreshWorkspaceButton = ({ workspaceId, disabled = false }: { workspaceId: number; disabled?: boolean }) => {
  const backend = useBackend()
  const queryClient = useQueryClient()
  const { showToast } = useToast()

  const mutation = useMutation({
    mutationFn: () => backend.refreshWorkspace(workspaceId),
    onSuccess: async (response) => {
      if (response.error) {
        showToast('Failed to refresh workspace', response.error.message, 'error')
        return
      }

      // Videos may have been added/removed, so everything derived from them is stale.
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['workspaces'] }),
        queryClient.invalidateQueries({ queryKey: ['workspace-explorer', workspaceId] }),
        queryClient.invalidateQueries({ queryKey: ['people', workspaceId] }),
      ])

      const summary = response.summary
      if (summary) {
        const notes = [`Indexed ${summary.indexed} of ${summary.totalScanned} videos`]
        if (summary.failures?.length) notes.push(`${summary.failures.length} failed`)
        if (summary.nullStartedAtCount) notes.push(`${summary.nullStartedAtCount} without start time`)
        showToast('Workspace refreshed', notes.join(' · '), summary.failures?.length ? 'info' : 'success')
      }
    },
    onError: (error: unknown) => {
      showToast('Unexpected error', error instanceof Error ? error.message : 'Unknown error', 'error')
    },
  })

  return (
    <Button variant="secondary" onClick={() => mutation.mutate()} disabled={disabled || mutation.isPending}>
      {mutation.isPending ? 'Refreshing…' : 'Refresh'}
    </Button>
  )
}
