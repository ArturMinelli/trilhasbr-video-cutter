import type { VideoExplorer } from '../../infrastructure/backend/types'

export type FolderNode = {
  name: string
  path: string
  children: FolderNode[]
}

export type ExplorerModel = {
  root: FolderNode
  videosByDir: Map<string, VideoExplorer[]>
  folderChildrenByDir: Map<string, FolderNode[]>
}

type MutableFolderNode = {
  name: string
  path: string
  children: Set<string>
}

const normalizeRelativeDir = (input: string): string => {
  if (!input) {
    return ''
  }
  const normalized = input
    .replaceAll('\\', '/')
    .split('/')
    .map((segment) => segment.trim())
    .filter(Boolean)
    .join('/')
  return normalized
}

const compareAlpha = (a: string, b: string): number => {
  return a.localeCompare(b, undefined, { sensitivity: 'base' })
}

const compareFolderNode = (a: FolderNode, b: FolderNode): number => {
  return compareAlpha(a.name, b.name)
}

const compareVideoName = (a: VideoExplorer, b: VideoExplorer): number => {
  const byName = compareAlpha(a.name || '', b.name || '')
  if (byName !== 0) {
    return byName
  }
  return compareAlpha(a.relativePath, b.relativePath)
}

const buildFolderNode = (folderMap: Map<string, MutableFolderNode>, currentPath: string): FolderNode => {
  const current = folderMap.get(currentPath)
  if (!current) {
    return { name: currentPath, path: currentPath, children: [] }
  }

  const children = Array.from(current.children)
    .map((childPath) => buildFolderNode(folderMap, childPath))
    .sort(compareFolderNode)

  return {
    name: current.name,
    path: current.path,
    children,
  }
}

const collectFolderChildren = (node: FolderNode, result: Map<string, FolderNode[]>): void => {
  result.set(node.path, [...node.children])
  for (const child of node.children) {
    collectFolderChildren(child, result)
  }
}

export const buildExplorerModel = (videos: VideoExplorer[]): ExplorerModel => {
  const folderMap = new Map<string, MutableFolderNode>()
  folderMap.set('', { name: '', path: '', children: new Set<string>() })

  const videosByDir = new Map<string, VideoExplorer[]>()

  for (const video of videos) {
    const normalizedDir = normalizeRelativeDir(video.relativeDir)
    if (!videosByDir.has(normalizedDir)) {
      videosByDir.set(normalizedDir, [])
    }
    videosByDir.get(normalizedDir)?.push(video)

    if (!normalizedDir) {
      continue
    }

    const segments = normalizedDir.split('/').filter(Boolean)
    let currentPath = ''
    for (const segment of segments) {
      const nextPath = currentPath ? `${currentPath}/${segment}` : segment
      if (!folderMap.has(nextPath)) {
        folderMap.set(nextPath, { name: segment, path: nextPath, children: new Set<string>() })
      }
      const parent = folderMap.get(currentPath)
      parent?.children.add(nextPath)
      currentPath = nextPath
    }
  }

  for (const [dir, list] of videosByDir.entries()) {
    list.sort(compareVideoName)
    videosByDir.set(dir, list)
  }

  const root = buildFolderNode(folderMap, '')
  const folderChildrenByDir = new Map<string, FolderNode[]>()
  collectFolderChildren(root, folderChildrenByDir)

  return {
    root,
    videosByDir,
    folderChildrenByDir,
  }
}

export const splitFolderPath = (folderPath: string): string[] => {
  return normalizeRelativeDir(folderPath)
    .split('/')
    .filter(Boolean)
}
