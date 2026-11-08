import { useEffect, useState } from 'react'

import { Button } from './Button'

type PageItem = number | 'gap'

export const pageWindow = (current: number, total: number, siblings: number): PageItem[] => {
  const pages = new Set<number>([1, total, current])

  for (let offset = 1; offset <= siblings; offset += 1) {
    pages.add(current - offset)
    pages.add(current + offset)
  }

  const sorted = [...pages].filter((page) => page >= 1 && page <= total).sort((left, right) => left - right)
  const items: PageItem[] = []

  for (const page of sorted) {
    const previous = items.at(-1)
    if (typeof previous === 'number' && page - previous > 1) {
      items.push('gap')
    }
    items.push(page)
  }

  return items
}

const clampPage = (page: number, total: number) => Math.min(total, Math.max(1, page))

type Props = {
  currentPage: number
  totalPages: number
  onPageChange: (page: number) => void
}

export const Pagination = ({ currentPage, totalPages, onPageChange }: Props) => {
  const [siblings, setSiblings] = useState(1)

  useEffect(() => {
    if (typeof window.matchMedia !== 'function') {
      return
    }

    const query = window.matchMedia('(max-width: 639px)')
    const apply = () => setSiblings(query.matches ? 0 : 1)
    apply()
    query.addEventListener('change', apply)
    return () => query.removeEventListener('change', apply)
  }, [])

  if (totalPages <= 1) {
    return null
  }

  const changePage = (page: number) => {
    const nextPage = clampPage(page, totalPages)
    if (nextPage === currentPage) {
      return
    }
    onPageChange(nextPage)
  }

  return (
    <div className="mt-4 flex flex-col gap-3 border-t border-[var(--border)] pt-4 sm:flex-row sm:items-center sm:justify-between">
      <p className="text-sm text-[var(--muted)]">
        Page {currentPage} of {totalPages}
      </p>
      <div className="flex flex-wrap items-center gap-1.5">
        <Button
          type="button"
          variant="secondary"
          aria-label="Previous"
          disabled={currentPage <= 1}
          onClick={() => changePage(currentPage - 1)}
        >
          Previous
        </Button>
        <div className="flex items-center gap-1.5">
          {pageWindow(currentPage, totalPages, siblings).map((item, index) =>
            item === 'gap' ? (
              <span key={`gap-${index}`} aria-hidden className="select-none px-0.5 text-[var(--muted)]">
                …
              </span>
            ) : (
              <button
                key={item}
                type="button"
                className={`inline-flex h-9 min-w-9 items-center justify-center rounded-md border px-2 text-sm font-semibold transition ${
                  item === currentPage
                    ? 'border-[var(--primary)] bg-[var(--primary)] text-white'
                    : 'border-[var(--border)] bg-[var(--surface)] text-[var(--muted)] hover:bg-[var(--surface-soft)] hover:text-[var(--text)]'
                }`}
                onClick={() => changePage(item)}
                aria-current={item === currentPage ? 'page' : undefined}
              >
                {item}
              </button>
            ),
          )}
        </div>
        <Button
          type="button"
          variant="secondary"
          aria-label="Next"
          disabled={currentPage >= totalPages}
          onClick={() => changePage(currentPage + 1)}
        >
          Next
        </Button>
      </div>
    </div>
  )
}
