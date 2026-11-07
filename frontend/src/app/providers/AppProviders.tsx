import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import { BackendProvider } from '../../infrastructure/backend/context'
import { ToastProvider } from '../../shared/toast/context'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 2_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

export const AppProviders = ({ children }: { children: ReactNode }) => {
  return (
    <QueryClientProvider client={queryClient}>
      <BackendProvider>
        <ToastProvider>{children}</ToastProvider>
      </BackendProvider>
    </QueryClientProvider>
  )
}
