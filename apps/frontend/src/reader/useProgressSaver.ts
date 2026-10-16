import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useRef } from 'react'
import { api } from '../api/client'

export type ReadingProgress = {
  page: number
  position?: string
  completed: boolean
}

// Saves shortly after the progress stops changing, and when the reader closes.
export function useProgressSaver(bookId: number, progress: ReadingProgress | null) {
  const queryClient = useQueryClient()
  const latest = useRef(progress)
  const saved = useRef('')

  const flush = useCallback(() => {
    const current = latest.current
    if (!current) return
    const key = JSON.stringify(current)
    if (key === saved.current) return
    saved.current = key
    api
      .saveProgress(bookId, current.page, current.completed, current.position)
      .then(() => queryClient.invalidateQueries({ queryKey: ['books'] }))
      .catch(() => {
        saved.current = ''
      })
  }, [bookId, queryClient])

  useEffect(() => {
    latest.current = progress
    const timer = setTimeout(flush, 500)
    return () => clearTimeout(timer)
  }, [progress, flush])

  useEffect(() => {
    window.addEventListener('pagehide', flush)
    return () => {
      window.removeEventListener('pagehide', flush)
      flush()
    }
  }, [flush])
}
