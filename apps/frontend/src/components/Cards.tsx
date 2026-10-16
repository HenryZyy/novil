import { Link } from 'react-router'
import { fileUrl, isReadable, type Book, type Series } from '../api/client'
import { plural } from '../format'
import { Cover } from './Cover'
import { Icon } from './Icon'
import { Progress } from './Progress'

export function SeriesCard({ series }: { series: Series }) {
  const done = series.bookCount > 0 && series.readCount === series.bookCount
  return (
    <Link to={`/series/${series.id}`} className="card">
      <Cover bookId={series.coverBookId} title={series.name} />
      {series.hiddenCount > 0 && series.hiddenCount === series.bookCount && <HiddenBadge />}
      <Progress value={series.progress} />
      <span className="card-title" title={series.name}>
        {series.name}
      </span>
      <span className="muted">
        {done ? 'Read' : series.readCount > 0 ? `${series.readCount} / ${series.bookCount} read` : plural(series.bookCount, 'book')}
      </span>
    </Link>
  )
}

// Only admins get hidden books, so only they see this.
function HiddenBadge() {
  return (
    <span className="hidden-badge" title="Hidden from users">
      <Icon name="eyeOff" size={14} />
      Hidden
    </span>
  )
}

function status(book: Book) {
  if (!isReadable(book)) return `${book.format.toUpperCase()}, download`
  if (book.completed) return 'Read'
  if (book.format === 'epub') return book.readPage > 0 ? `${book.readPage}% read` : 'EPUB'
  return book.readPage > 0 ? `Page ${book.readPage} / ${book.pageCount}` : plural(book.pageCount, 'page')
}

export function BookCard({ book, showSeries = false }: { book: Book; showSeries?: boolean }) {
  const readable = isReadable(book)
  // EPUBs store the percentage read instead of a page.
  const total = book.format === 'epub' ? 100 : book.pageCount
  const progress = book.completed ? 1 : total ? book.readPage / total : 0
  const content = (
    <>
      <Cover bookId={book.id} title={book.title} />
      {book.hidden && <HiddenBadge />}
      <Progress value={progress} />
      <span className="card-title" title={book.title}>
        {showSeries ? book.seriesName : book.title}
      </span>
      <span className="muted">{showSeries ? book.title : status(book)}</span>
    </>
  )
  return readable ? (
    <Link to={`/read/${book.id}`} className="card">
      {content}
    </Link>
  ) : (
    <a href={fileUrl(book.id)} className="card">
      {content}
    </a>
  )
}
