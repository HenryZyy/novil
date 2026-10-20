import { useState } from 'react'
import { fileUrl, isReadable, type Book, type Series } from '../api/client'
import {
  isAdmin,
  useCurrentUser,
  useDismissContinue,
  useSeries,
  useSetBookRead,
  useSetHidden,
  useSetListed,
  useSetSeriesHidden,
  useSetSeriesRead,
} from '../api/queries'
import { CardMenu, type MenuItem } from './CardMenu'
import { DeleteBookDialog } from './DeleteBookDialog'
import { DeleteSeriesDialog } from './DeleteSeriesDialog'
import { EditBookDialog } from './EditBookDialog'

function listItems(series: Series | undefined, setListed: ReturnType<typeof useSetListed>): MenuItem[] {
  if (!series) return []
  return [
    {
      label: series.wantToRead ? 'Remove from Want to read' : 'Want to read',
      icon: 'bookmark',
      onSelect: () => setListed.mutate({ id: series.id, list: 'want', listed: !series.wantToRead }),
    },
    {
      label: series.favorite ? 'Remove from favorites' : 'Add to favorites',
      icon: 'heart',
      onSelect: () => setListed.mutate({ id: series.id, list: 'favorites', listed: !series.favorite }),
    },
  ]
}

// inSeries: the card is on its series' page, which has the series' own
// buttons, so the menu leaves them out. inContinue: it's in Continue
// reading, which the menu can take it out of.
export function BookMenu({ book, inSeries = false, inContinue = false }: { book: Book; inSeries?: boolean; inContinue?: boolean }) {
  const user = useCurrentUser()
  const [opened, setOpened] = useState(false)
  // The book's series, for the list items; only loaded once the menu opens.
  const series = useSeries(book.seriesId, opened && !inSeries)
  const setRead = useSetBookRead()
  const setListed = useSetListed()
  const setHidden = useSetHidden()
  const dismiss = useDismissContinue()
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const started = book.readPage > 0 && !book.completed

  const items: MenuItem[] = [
    isReadable(book)
      ? { label: started ? 'Continue reading' : 'Read', icon: 'open', to: `/read/${book.id}` }
      : { label: 'Download', icon: 'download', download: fileUrl(book.id) },
    ...(inContinue
      ? [{ label: 'Remove from Continue reading', icon: 'close', onSelect: () => dismiss.mutate(book.id) } as const]
      : []),
    ...(inSeries ? [] : [{ label: 'Go to series', icon: 'folder', to: `/series/${book.seriesId}` } as const]),
    {
      label: book.completed ? 'Mark as unread' : 'Mark as read',
      icon: 'check',
      onSelect: () => setRead.mutate({ book, read: !book.completed }),
    },
    ...(inSeries ? [] : listItems(series.data, setListed)),
    ...(isReadable(book) ? [{ label: 'Download', icon: 'download', download: fileUrl(book.id) } as const] : []),
    ...(isAdmin(user)
      ? ([
          'divider',
          { label: 'Edit details', icon: 'edit', onSelect: () => setEditing(true) },
          {
            label: book.hidden ? 'Show to users' : 'Hide from users',
            icon: book.hidden ? 'eye' : 'eyeOff',
            onSelect: () => setHidden.mutate({ id: book.id, hidden: !book.hidden }),
          },
          { label: 'Delete', icon: 'trash', danger: true, onSelect: () => setDeleting(true) },
        ] as MenuItem[])
      : []),
  ]

  return (
    <>
      <CardMenu label={book.title} items={items} onOpen={() => setOpened(true)} />
      {editing && <EditBookDialog book={book} onClose={() => setEditing(false)} />}
      {deleting && <DeleteBookDialog book={book} onClose={() => setDeleting(false)} />}
    </>
  )
}

export function SeriesMenu({ series }: { series: Series }) {
  const user = useCurrentUser()
  const setRead = useSetSeriesRead()
  const setListed = useSetListed()
  const setHidden = useSetSeriesHidden()
  const allRead = series.bookCount > 0 && series.readCount === series.bookCount
  const allHidden = series.hiddenCount > 0 && series.hiddenCount === series.bookCount
  const [deleting, setDeleting] = useState(false)

  const items: MenuItem[] = [
    { label: 'Open', icon: 'folder', to: `/series/${series.id}` },
    {
      label: allRead ? 'Mark as unread' : 'Mark as read',
      icon: 'check',
      onSelect: () => setRead.mutate({ id: series.id, read: !allRead }),
    },
    ...listItems(series, setListed),
    ...(isAdmin(user)
      ? ([
          'divider',
          {
            label: allHidden ? 'Show to users' : 'Hide from users',
            icon: allHidden ? 'eye' : 'eyeOff',
            onSelect: () => setHidden.mutate({ id: series.id, hidden: !allHidden }),
          },
          {
            label: series.bookCount === 1 ? 'Delete' : 'Delete series',
            icon: 'trash',
            danger: true,
            onSelect: () => setDeleting(true),
          },
        ] as MenuItem[])
      : []),
  ]
  return (
    <>
      <CardMenu label={series.name} items={items} />
      {deleting && <DeleteSeriesDialog series={series} onClose={() => setDeleting(false)} />}
    </>
  )
}
