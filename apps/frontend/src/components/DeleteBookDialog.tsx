import { useLocation, useNavigate } from 'react-router'
import { api, type Book } from '../api/client'
import { useDeleteBook } from '../api/queries'
import { ConfirmDialog } from './ConfirmDialog'

export function DeleteBookDialog({ book, onClose }: { book: Book; onClose: () => void }) {
  const remove = useDeleteBook()
  const navigate = useNavigate()
  const location = useLocation()

  const confirm = async () => {
    await remove.mutateAsync(book.id)
    // Off a page that's gone with it: the book's own, or its series' if
    // this was the last book.
    if (location.pathname === `/series/${book.seriesId}`) {
      await api.series(book.seriesId).catch(() => navigate('/library', { replace: true }))
    }
  }

  return (
    <ConfirmDialog title={`Are you sure you want to delete “${book.title}”?`} confirmLabel="Delete book" onConfirm={confirm} onClose={onClose}>
      <p>
        Its file is deleted from the library folder, together with everyone's progress, bookmarks and notes in it.
        {book.title !== book.seriesName && ' If it is the last book of its series, the series goes too.'}
      </p>
      <p className="muted">This can't be undone.</p>
    </ConfirmDialog>
  )
}
