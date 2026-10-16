import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useRef } from 'react'
import { api, type Book } from '../api/client'

// Saves shortly after the page stops changing, and when the reader closes.
export function useProgressSaver(book: Book, page: number) {
  const { id: bookId, pageCount, completed } = book
  const queryClient = useQueryClient()
  const latest = useRef(page)
  const saved = useRef<number | null>(null)

  const flush = useCallback(() => {
    const current = latest.current
    if (current === saved.current) return
    saved.current = current
    api
      .saveProgress(bookId, current, completed || current >= pageCount)
      .then(() => queryClient.invalidateQueries({ queryKey: ['books'] }))
      .catch(() => {
        saved.current = null
      })
  }, [bookId, pageCount, completed, queryClient])

  useEffect(() => {
    latest.current = page
    const timer = setTimeout(flush, 500)
    return () => clearTimeout(timer)
  }, [page, flush])

  useEffect(() => {
    window.addEventListener('pagehide', flush)
    return () => {
      window.removeEventListener('pagehide', flush)
      flush()
    }
  }, [flush])
}
