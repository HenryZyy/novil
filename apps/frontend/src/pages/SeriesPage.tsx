import { Link, useParams } from 'react-router'
import { isAdmin, useBooks, useCurrentUser, useLibraries, useSeries, useSetSeriesRead } from '../api/queries'
import { isReadable } from '../api/client'
import { BookCard } from '../components/Cards'
import { plural } from '../format'
import { Cover } from '../components/Cover'

export function SeriesPage() {
  const seriesId = Number(useParams().seriesId)
  const series = useSeries(seriesId)
  const books = useBooks(seriesId)
  const libraries = useLibraries()
  const user = useCurrentUser()
  const setRead = useSetSeriesRead()

  if (series.isPending || books.isPending) return <p>Loading…</p>
  if (series.isError) return <p className="error">{series.error.message}</p>
  if (books.isError) return <p className="error">{books.error.message}</p>

  const s = series.data
  const next = books.data.find((b) => isReadable(b) && !b.completed)
  const allRead = s.bookCount > 0 && s.readCount === s.bookCount
  const canUpload = isAdmin(user) && libraries.data?.some((l) => l.id === s.libraryId && l.allowUploads)

  return (
    <>
      <div className="series-header">
        <div className="series-cover">
          <Cover bookId={s.coverBookId} title={s.name} />
        </div>
        <div>
          <h1>{s.name}</h1>
          <p className="muted">
            {plural(s.bookCount, 'book')} · {s.readCount} read
          </p>
          <p className="actions">
            {next && (
              <Link to={`/read/${next.id}`} className="button primary">
                {next.readPage > 0 ? `Continue ${next.title}` : `Start ${next.title}`}
              </Link>
            )}
            <button onClick={() => setRead.mutate({ id: s.id, read: !allRead })} disabled={setRead.isPending}>
              {allRead ? 'Mark as unread' : 'Mark as read'}
            </button>
            {canUpload && (
              <Link to={`/upload?${new URLSearchParams({ library: String(s.libraryId), series: s.name })}`} className="button">
                Upload to this series
              </Link>
            )}
          </p>
          {setRead.isError && <p className="error">{setRead.error.message}</p>}
        </div>
      </div>
      <div className="grid">
        {books.data.map((book) => (
          <BookCard key={book.id} book={book} />
        ))}
      </div>
    </>
  )
}
