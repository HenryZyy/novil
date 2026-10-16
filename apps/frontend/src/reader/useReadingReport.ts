import { useEffect } from 'react'
import { api } from '../api/client'

// Tells the server the book is open, so admins see who is reading what:
// at once, every minute while the tab is visible, and that it closed.
export function useReadingReport(bookId: number) {
  useEffect(() => {
    const report = () => {
      if (document.visibilityState === 'visible') api.reportReading(bookId).catch(() => {})
    }
    report()
    const timer = setInterval(report, 60_000)
    document.addEventListener('visibilitychange', report)
    const stop = () => api.stopReading().catch(() => {})
    window.addEventListener('pagehide', stop)
    return () => {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', report)
      window.removeEventListener('pagehide', stop)
      stop()
    }
  }, [bookId])
}
