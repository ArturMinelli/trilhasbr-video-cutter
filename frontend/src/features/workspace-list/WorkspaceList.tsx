import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

import type { WorkspaceEntity } from '../../entities/workspace/model'
import { DeleteWorkspaceModal } from '../workspace-manage/DeleteWorkspaceModal'

export const WorkspaceList = ({ workspaces }: { workspaces: WorkspaceEntity[] }) => {
  const navigate = useNavigate()
  const [pendingDelete, setPendingDelete] = useState<WorkspaceEntity | null>(null)

  if (workspaces.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-[var(--border)] bg-[var(--surface)] p-8 text-center text-sm text-[var(--muted)]">
        No workspaces yet. Click <strong>Import</strong> to add one.
      </div>
    )
  }

  return (
    <>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {workspaces.map((workspace) => (
          <div key={workspace.id} className="group relative">
            <button
              type="button"
              onClick={() => navigate(`/workspaces/${workspace.id}`)}
              className="w-full rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4 text-left transition hover:border-[var(--primary)] hover:bg-[var(--surface-soft)]"
            >
              <div className="mb-3 inline-flex h-10 w-10 items-center justify-center rounded-md bg-[var(--primary-soft)] text-[var(--primary)]">
                <span className="text-xl">📁</span>
              </div>
              <p className="truncate pr-8 text-sm font-semibold text-[var(--text)]">{workspace.name}</p>
              <p className="mt-1 truncate text-xs text-[var(--muted)]">{workspace.directory}</p>
              <p className="mt-2 text-xs text-[var(--muted)]">{workspace.videoCount} videos</p>
            </button>
            <button
              type="button"
              aria-label={`Delete workspace ${workspace.name}`}
              title="Delete workspace"
              onClick={() => setPendingDelete(workspace)}
              className="absolute right-2 top-2 rounded-md p-1.5 text-sm text-[var(--muted)] opacity-60 transition hover:text-[var(--danger)] hover:opacity-100 focus:opacity-100 group-hover:opacity-100"
            >
              🗑
            </button>
          </div>
        ))}
      </div>
      <DeleteWorkspaceModal workspace={pendingDelete} onClose={() => setPendingDelete(null)} />
    </>
  )
}
