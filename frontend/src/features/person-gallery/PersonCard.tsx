import { formatDuration } from '../../entities/video/model'
import { formatPersonRange, personName, plural } from '../../entities/person/model'
import type { PersonGroupDTO } from '../../infrastructure/backend/types'
import { PersonName } from './PersonName'

type Props = {
  group: PersonGroupDTO
  selected: boolean
  onSelectionChange: (checked: boolean) => void
  onOpen: () => void
}

export const PersonCard = ({ group, selected, onSelectionChange, onOpen }: Props) => {
  const first = group.tracks[0]
  const name = personName(group.label, group.id)
  return (
    <article
      className={`rounded-xl border p-3 transition ${
        selected ? 'border-[var(--primary)] bg-[var(--surface-soft)]' : 'border-[var(--border)] bg-[var(--surface)]'
      }`}
    >
      <div className="relative mb-3 aspect-[3/4] overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--surface-soft)]">
        <label className="absolute left-2 top-2 z-10 inline-flex cursor-pointer items-center gap-1 rounded-full border border-[var(--border)] bg-black/40 px-2 py-1 text-[11px] font-semibold text-[var(--text)] backdrop-blur">
          <input
            type="checkbox"
            checked={selected}
            onChange={(event) => onSelectionChange(event.target.checked)}
            className="h-3.5 w-3.5 accent-[var(--primary)]"
            aria-label={`Select ${name}`}
          />
          <span>Select</span>
        </label>
        <button type="button" onClick={onOpen} className="h-full w-full" aria-label={`Preview ${name}`}>
          {group.thumbnailDataUrl ? (
            <img src={group.thumbnailDataUrl} alt={name} className="h-full w-full object-cover" loading="lazy" />
          ) : (
            <span className="text-xs text-[var(--muted)]">No thumbnail</span>
          )}
        </button>
      </div>

      <PersonName group={group} className="text-sm" />
      {first ? (
        <p className="mt-1 truncate text-xs text-[var(--muted)]" title={first.videoName}>
          {first.videoName}
        </p>
      ) : null}
      <p className="mt-1 text-xs text-[var(--muted)]">{formatPersonRange(group)}</p>
      <div className="mt-2 grid grid-cols-2 gap-1 text-xs text-[var(--muted)]">
        <span>{plural(group.trackCount, 'appearance')}</span>
        <span className="text-right">{plural(group.videoCount, 'video')}</span>
        <span>{formatDuration(group.totalDurationSec)}</span>
        <span className="text-right">{Math.round(group.avgConfidence * 100)}% conf.</span>
      </div>
    </article>
  )
}
