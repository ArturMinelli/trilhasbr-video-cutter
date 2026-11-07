import { formatDuration } from '../video/model'
import type { PersonGroupDTO } from '../../infrastructure/backend/types'

export type PersonGroupEntity = PersonGroupDTO

export const formatPersonRange = (group: PersonGroupDTO): string => {
  const first = group.tracks[0]
  if (!first) {
    return '—'
  }
  return `${formatDuration(first.startSec)} – ${formatDuration(first.endSec)}`
}

// Display name: the user's label if set, otherwise "Person <id>".
export const personName = (label: string | undefined, id: number): string => label?.trim() || `Person ${id}`

export const plural = (count: number, word: string): string => `${count} ${word}${count === 1 ? '' : 's'}`
