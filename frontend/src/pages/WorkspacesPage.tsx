import { useQuery } from '@tanstack/react-query'

import { WorkspaceList } from '../features/workspace-list/WorkspaceList'
import { LoadWorkspaceButton } from '../features/workspace-loader/LoadWorkspaceButton'
import { useBackend } from '../infrastructure/backend/context'

export const WorkspacesPage = () => {
  const backend = useBackend()

  const workspacesQuery = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => backend.listWorkspaces(),
  })

  const workspaces = workspacesQuery.data?.workspaces ?? []

  return (
    <div className="mx-auto max-w-[1400px] p-4 md:p-6">
      <header className="mb-6 flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold tracking-[0.06em]">Workspaces</h1>
          <p className="text-sm text-[var(--muted)]">Choose an existing workspace or import a new one.</p>
        </div>
        <LoadWorkspaceButton />
      </header>

      {workspacesQuery.isLoading ? <p className="text-sm text-[var(--muted)]">Loading workspaces…</p> : null}
      {workspacesQuery.data?.error ? <p className="mb-4 text-sm text-[var(--danger)]">{workspacesQuery.data.error.message}</p> : null}

      <WorkspaceList workspaces={workspaces} />
    </div>
  )
}
