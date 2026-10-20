import { useLocation, useNavigate } from 'react-router'
import type { Series } from '../api/client'
import { useDeleteSeries } from '../api/queries'
import { plural } from '../format'
import { ConfirmDialog } from './ConfirmDialog'

export function DeleteSeriesDialog({ series, onClose }: { series: Series; onClose: () => void }) {
  const remove = useDeleteSeries()
  const navigate = useNavigate()
  const location = useLocation()

  const confirm = async () => {
    await remove.mutateAsync(series.id)
    if (location.pathname === `/series/${series.id}`) navigate('/library', { replace: true })
  }

  const single = series.bookCount === 1
  return (
    <ConfirmDialog
      title={`Are you sure you want to delete “${series.name}”?`}
      confirmLabel={single ? 'Delete' : `Delete ${plural(series.bookCount, 'book')}`}
      onConfirm={confirm}
      onClose={onClose}
    >
      <p>
        {single ? 'Its file is' : `All ${series.bookCount} files are`} deleted from the library folder, together with
        everyone's progress, bookmarks and notes in {single ? 'it' : 'them'}.
      </p>
      <p className="muted">This can't be undone.</p>
    </ConfirmDialog>
  )
}
