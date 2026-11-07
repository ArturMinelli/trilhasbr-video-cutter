import type { Workspace } from '../../infrastructure/backend/types'

export type WorkspaceEntity = Workspace

export const workspaceLabel = (workspace: WorkspaceEntity): string => {
  return `${workspace.name}`
}
