import { createContext, useContext, useMemo } from 'react'
import type { ReactNode } from 'react'

import type { BackendClient } from './backendClient'
import { WailsBackendClient } from './wailsBackendClient'

const BackendContext = createContext<BackendClient | null>(null)

export const BackendProvider = ({ children }: { children: ReactNode }) => {
  const client = useMemo<BackendClient>(() => new WailsBackendClient(), [])
  return <BackendContext.Provider value={client}>{children}</BackendContext.Provider>
}

export const useBackend = (): BackendClient => {
  const backend = useContext(BackendContext)
  if (!backend) {
    throw new Error('BackendProvider is missing in app tree')
  }
  return backend
}
