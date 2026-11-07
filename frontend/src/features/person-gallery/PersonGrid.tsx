import type { PersonGroupDTO } from '../../infrastructure/backend/types'
import { PersonCard } from './PersonCard'

type Props = {
  groups: PersonGroupDTO[]
  selectedIds: Set<number>
  onToggle: (groupId: number, checked: boolean) => void
  onOpen: (group: PersonGroupDTO) => void
}

export const PersonGrid = ({ groups, selectedIds, onToggle, onOpen }: Props) => {
  if (groups.length === 0) {
    return (
      <div className="rounded-lg border border-dashed border-[var(--border)] p-5 text-sm text-[var(--muted)]">
        No people to show. Run “Detect people”, or loosen the filters.
      </div>
    )
  }
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-5 2xl:grid-cols-6">
      {groups.map((group) => (
        <PersonCard
          key={group.id}
          group={group}
          selected={selectedIds.has(group.id)}
          onSelectionChange={(checked) => onToggle(group.id, checked)}
          onOpen={() => onOpen(group)}
        />
      ))}
    </div>
  )
}
