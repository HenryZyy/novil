import { useSyncExternalStore } from 'react'

// After an admin changes a cover, its URL gets a new version so the
// browser shows the new image instead of its cached copy of the old one.
const versions = new Map<number, number>()
const listeners = new Set<() => void>()

export function bumpCover(bookId: number) {
  versions.set(bookId, Date.now())
  for (const listener of listeners) listener()
}

export function useCoverVersion(bookId: number | null) {
  return useSyncExternalStore(
    (onChange) => {
      listeners.add(onChange)
      return () => listeners.delete(onChange)
    },
    () => (bookId === null ? 0 : (versions.get(bookId) ?? 0)),
  )
}
