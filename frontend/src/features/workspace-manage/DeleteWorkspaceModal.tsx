import { useMutation, useQueryClient } from '@tanstack/react-query'

import type { WorkspaceEntity } from '../../entities/workspace/model'
import { useBackend } from '../../infrastructure/backend/context'
import { useToast } from '../../shared/toast/context'
import { Button } from '../../shared/ui/Button'

type Props = {
  workspace: WorkspaceEntity | null
  onClose: () => void
  onDeleted?: () => void
}

export const DeleteWorkspaceModal = ({ workspace, onClose, onDeleted }: Props) => {
  const backend = useBackend()
  const queryClient = useQueryClient()
  const { showToast } = useToast()

  const mutation = useMutation({
    mutationFn: (id: number) => backend.deleteWorkspace(id),
    onSuccess: async (response) => {
      if (response.error) {
        showToast('Failed to delete workspace', response.error.message, 'error')
        return
      }
      await queryClient.invalidateQueries({ queryKey: ['workspaces'] })
      showToast('Workspace deleted', `${workspace?.name ?? 'Workspace'} was removed from the app`, 'success')
      onClose()
      onDeleted?.()
    },
    onError: (error: unknown) => {
      showToast('Unexpected error', error instanceof Error ? error.message : 'Unknown error', 'error')
    },
  })

  if (!workspace) {
    return null
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" role="dialog" aria-modal="true">
      <div className="w-full max-w-md rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
        <h2 className="mb-2 text-lg font-bold">Delete workspace?</h2>
        <p className="text-sm text-[var(--text)]">
          <strong>{workspace.name}</strong> and its detected people will be removed from the app.
        </p>
        <p className="mt-2 text-sm text-[var(--muted)]">
          Your video files and exported cuts on disk are not touched. You can import the folder again later.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="secondary" onClick={onClose} disabled={mutation.isPending}>
            Cancel
          </Button>
          <Button variant="danger" onClick={() => mutation.mutate(workspace.id)} disabled={mutation.isPending}>
            {mutation.isPending ? 'Deleting…' : 'Delete'}
          </Button>
        </div>
      </div>
    </div>
  )
}
