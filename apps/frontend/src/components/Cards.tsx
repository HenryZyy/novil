import { Link } from 'react-router'
import { fileUrl, isReadable, type Book, type Series } from '../api/client'
import { plural } from '../format'
import { Cover } from './Cover'
import { Progress } from './Progress'

export function SeriesCard({ series }: { series: Series }) {
  const done = series.bookCount > 0 && series.readCount === series.bookCount
  return (
    <Link to={`/series/${series.id}`} className="card">
      <Cover bookId={series.coverBookId} title={series.name} />
      <Progress value={series.bookCount ? (series.readCount + series.readingCount / 2) / series.bookCount : 0} />
      <span className="card-title" title={series.name}>
        {series.name}
      </span>
      <span className="muted">
        {done ? 'Read' : series.readCount > 0 ? `${series.readCount} / ${series.bookCount} read` : plural(series.bookCount, 'book')}
      </span>
    </Link>
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
      <Cover bookId={book.format === 'cbz' ? book.id : null} title={book.title} />
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
