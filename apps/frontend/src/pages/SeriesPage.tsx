import { useState } from 'react'
import { Link, useParams } from 'react-router'
import { isAdmin, useBooks, useCurrentUser, useLibraries, useSeries, useSetSeriesRead } from '../api/queries'
import { isReadable, type Book } from '../api/client'
import { BookCard } from '../components/Cards'
import { Row } from '../components/Row'
import { plural } from '../format'
import { Cover } from '../components/Cover'
import { EditBookDialog } from '../components/EditBookDialog'
import { Icon } from '../components/Icon'

export function SeriesPage() {
  const seriesId = Number(useParams().seriesId)
  const series = useSeries(seriesId)
  const books = useBooks(seriesId)
  const libraries = useLibraries()
  const user = useCurrentUser()
  const setRead = useSetSeriesRead()
  const [editing, setEditing] = useState<Book | null>(null)
  const [expanded, setExpanded] = useState(false)

  if (series.isPending || books.isPending) return <p>Loading…</p>
  if (series.isError) return <p className="error">{series.error.message}</p>
  if (books.isError) return <p className="error">{books.error.message}</p>

  const s = series.data
  const next = books.data.find((b) => isReadable(b) && !b.completed)
  const allRead = s.bookCount > 0 && s.readCount === s.bookCount
  const canUpload = isAdmin(user) && libraries.data?.some((l) => l.id === s.libraryId && l.allowUploads)
  const details = seriesDetails(books.data)

  return (
    <>
      <div className="series-header">
        <div className="series-cover">
          <Cover bookId={s.coverBookId} title={s.name} />
        </div>
        <div>
          <h1>{s.name}</h1>
          {details.facts.length > 0 && <p className="series-facts">{details.facts.join(' · ')}</p>}
          <p className="muted">
            {plural(s.bookCount, 'book')} · {s.readCount} read
          </p>
          {details.description && (
            <div className="series-description">
              <p className={expanded ? '' : 'clamped'}>{details.description}</p>
              {details.description.length > 280 && (
                <button className="link-button" onClick={() => setExpanded((e) => !e)}>
                  {expanded ? 'Less' : 'More'}
                </button>
              )}
            </div>
          )}
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
      <Row scrollTo={next ? String(next.id) : undefined}>
        {books.data.map((book) => (
          <div key={book.id} className="card-wrap" data-row-id={book.id}>
            <BookCard book={book} />
            {isAdmin(user) && (
              <button className="card-edit" onClick={() => setEditing(book)} aria-label={`Edit details of ${book.title}`} title="Edit details">
                <Icon name="edit" size={16} />
              </button>
            )}
          </div>
        ))}
      </Row>
      {editing && <EditBookDialog key={editing.id} book={editing} onClose={() => setEditing(null)} />}
    </>
  )
}

// What the books say about the series: their authors, release years and
// languages, and the first description.
function seriesDetails(books: Book[]) {
  const unique = (values: string[]) => [...new Set(values.filter(Boolean))]
  const authors = unique(books.map((b) => b.author))
  const years = unique(books.map((b) => b.released.slice(0, 4))).sort()
  const languages = unique(books.map((b) => b.language))
  const facts = [
    authors.length > 2 ? `${authors.slice(0, 2).join(', ')} and others` : authors.join(', '),
    years.length > 1 ? `${years[0]}–${years[years.length - 1]}` : years[0],
    languages.map(languageName).join(', '),
  ].filter(Boolean) as string[]
  return { facts, description: books.find((b) => b.description)?.description ?? '' }
}

function languageName(code: string) {
  try {
    return new Intl.DisplayNames(undefined, { type: 'language' }).of(code) ?? code
  } catch {
    return code
  }
}
