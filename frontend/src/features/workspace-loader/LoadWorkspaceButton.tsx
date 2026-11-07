import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'

import { useBackend } from '../../infrastructure/backend/context'
import { useToast } from '../../shared/toast/context'
import { Button } from '../../shared/ui/Button'

export const LoadWorkspaceButton = ({ className = '' }: { className?: string }) => {
  const backend = useBackend()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { showToast } = useToast()

  const mutation = useMutation({
    mutationFn: () => backend.loadWorkspaceByDialog(),
    onSuccess: async (response) => {
      if (response.error) {
        showToast('Failed to import workspace', response.error.message, 'error')
        return
      }

      if (response.workspace?.id) {
        navigate(`/workspaces/${response.workspace.id}`)
      }
      void queryClient.invalidateQueries({ queryKey: ['workspaces'] })

      if (response.summary) {
        showToast('Workspace imported', `Indexed ${response.summary.indexed} videos`, 'success')
      }
    },
    onError: (error: unknown) => {
      showToast('Unexpected error', error instanceof Error ? error.message : 'Unknown error', 'error')
    },
  })

  return (
    <Button className={className} onClick={() => mutation.mutate()} disabled={mutation.isPending}>
      {mutation.isPending ? 'Importing…' : 'Import'}
    </Button>
  )
}
