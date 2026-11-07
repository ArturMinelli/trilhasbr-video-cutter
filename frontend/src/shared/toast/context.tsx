import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import type { ReactNode } from 'react'

export type ToastKind = 'info' | 'success' | 'error'

type ToastMessage = {
  id: string
  title: string
  description?: string
  kind: ToastKind
}

type ToastContextValue = {
  showToast: (title: string, description?: string, kind?: ToastKind) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

export const ToastProvider = ({ children }: { children: ReactNode }) => {
  const [messages, setMessages] = useState<ToastMessage[]>([])

  const showToast = useCallback((title: string, description?: string, kind: ToastKind = 'info') => {
    const id = `${Date.now()}-${Math.random().toString(16).slice(2)}`
    setMessages((current) => [...current, { id, title, description, kind }])

    window.setTimeout(() => {
      setMessages((current) => current.filter((msg) => msg.id !== id))
    }, 4500)
  }, [])

  const value = useMemo<ToastContextValue>(
    () => ({
      showToast,
    }),
    [showToast],
  )

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="pointer-events-none fixed bottom-4 right-4 z-50 flex w-[320px] flex-col gap-2">
        {messages.map((msg) => {
          const palette =
            msg.kind === 'success'
              ? 'border-[var(--success)] bg-[var(--success-soft)] text-[var(--success)]'
              : msg.kind === 'error'
              ? 'border-[var(--danger)] bg-[var(--danger-soft)] text-[var(--danger)]'
              : 'border-[var(--primary)] bg-[var(--primary-soft)] text-[var(--primary)]'

          return (
            <div key={msg.id} className={`pointer-events-auto rounded-md border p-3 shadow-lg ${palette}`}>
              <p className="text-sm font-semibold">{msg.title}</p>
              {msg.description ? <p className="mt-1 text-xs opacity-90">{msg.description}</p> : null}
            </div>
          )
        })}
      </div>
    </ToastContext.Provider>
  )
}

export const useToast = () => {
  const context = useContext(ToastContext)
  if (!context) {
    throw new Error('ToastProvider is missing in app tree')
  }
  return context
}
