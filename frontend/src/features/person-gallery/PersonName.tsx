import { useRef, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'

import { personName } from '../../entities/person/model'
import { useBackend } from '../../infrastructure/backend/context'
import type { PersonGroupDTO } from '../../infrastructure/backend/types'
import { useToast } from '../../shared/toast/context'
import { Input } from '../../shared/ui/Input'

const MAX_LABEL = 60

// Person title with an inline rename control (empty name clears it; the id stays visible when named).
export const PersonName = ({ group, className = '' }: { group: PersonGroupDTO; className?: string }) => {
  const backend = useBackend()
  const queryClient = useQueryClient()
  const { showToast } = useToast()
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const settled = useRef(false) // Enter/Esc/blur must only act once
  const name = personName(group.label, group.id)

  const rename = useMutation({
    mutationFn: (label: string) => backend.renamePerson(group.id, label),
    onSuccess: async (response) => {
      if (response.error) {
        showToast('Rename failed', response.error.message, 'error')
        return
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['people'] }),
        queryClient.invalidateQueries({ queryKey: ['videoTrackBoxes'] }),
      ])
    },
    onError: (error) => showToast('Unexpected error', error instanceof Error ? error.message : 'Unknown error', 'error'),
    onSettled: () => setEditing(false),
  })

  const start = () => {
    settled.current = false
    setDraft(group.label ?? '')
    setEditing(true)
  }
  const commit = () => {
    if (settled.current) {
      return
    }
    settled.current = true
    const next = draft.trim()
    if (next === (group.label ?? '')) {
      setEditing(false)
      return
    }
    rename.mutate(next)
  }
  const cancel = () => {
    settled.current = true
    setEditing(false)
  }

  if (editing) {
    return (
      <Input
        autoFocus
        value={draft}
        maxLength={MAX_LABEL}
        disabled={rename.isPending}
        placeholder={`Person ${group.id}`}
        aria-label={`Name for person ${group.id}`}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={commit}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            commit()
          } else if (event.key === 'Escape') {
            cancel()
          }
        }}
        className={`py-1 ${className}`}
      />
    )
  }

  return (
    <div className={`flex min-w-0 items-center gap-1 ${className}`}>
      <span className="min-w-0 truncate font-semibold text-[var(--text)]" title={name}>
        {name}
      </span>
      {group.label?.trim() ? <span className="shrink-0 text-xs text-[var(--muted)]">#{group.id}</span> : null}
      <button
        type="button"
        onClick={start}
        title="Rename"
        aria-label={`Rename ${name}`}
        className="shrink-0 rounded px-1 text-xs text-[var(--muted)] hover:text-[var(--text)]"
      >
        ✎
      </button>
    </div>
  )
}
