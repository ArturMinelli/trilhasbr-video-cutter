import { createHashRouter } from 'react-router-dom'

import { WorkspaceDetailPage } from '../pages/WorkspaceDetailPage'
import { WorkspacesPage } from '../pages/WorkspacesPage'

export const router = createHashRouter([
  {
    path: '/',
    element: <WorkspacesPage />,
  },
  {
    path: '/workspaces/:workspaceId',
    element: <WorkspaceDetailPage />,
  },
])
