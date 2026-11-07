import { memo } from 'react'

import type { FolderNode } from './model'

type FolderTreeProps = {
  root: FolderNode
  selectedPath: string
  onSelect: (folderPath: string) => void
}

const FolderRow = memo(function FolderRow({
  node,
  depth,
  selectedPath,
  onSelect,
}: {
  node: FolderNode
  depth: number
  selectedPath: string
  onSelect: (folderPath: string) => void
}) {
  const isRoot = node.path === ''
  const label = isRoot ? 'Root' : node.name
  const isSelected = selectedPath === node.path

  return (
    <li>
      <button
        type="button"
        onClick={() => onSelect(node.path)}
        className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition ${
          isSelected
            ? 'bg-[var(--primary-soft)] text-[var(--text)]'
            : 'text-[var(--muted)] hover:bg-[var(--surface-soft)] hover:text-[var(--text)]'
        }`}
        style={{ paddingLeft: `${depth * 12 + 8}px` }}
      >
        <span className="text-xs">📁</span>
        <span className="truncate">{label}</span>
      </button>

      {node.children.length > 0 ? (
        <ul className="space-y-0.5">
          {node.children.map((child) => (
            <FolderRow key={child.path} node={child} depth={depth + 1} selectedPath={selectedPath} onSelect={onSelect} />
          ))}
        </ul>
      ) : null}
    </li>
  )
})

export const FolderTree = ({ root, selectedPath, onSelect }: FolderTreeProps) => {
  return (
    <nav className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-2" aria-label="Workspace folders">
      <ul className="space-y-0.5">
        <FolderRow node={root} depth={0} selectedPath={selectedPath} onSelect={onSelect} />
      </ul>
    </nav>
  )
}
