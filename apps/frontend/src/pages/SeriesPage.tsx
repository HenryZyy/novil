import { Link, useParams } from 'react-router'
import { fileUrl, type Book } from '../api/client'
import { useBooks, useSeries } from '../api/queries'

export function SeriesPage() {
  const seriesId = Number(useParams().seriesId)
  const series = useSeries(seriesId)
  const books = useBooks(seriesId)

  if (series.isPending || books.isPending) return <p>Loading…</p>
  if (series.isError) return <p>Error: {series.error.message}</p>
  if (books.isError) return <p>Error: {books.error.message}</p>

  return (
    <>
      <h1>{series.data.name}</h1>
      <p>
        <Link to={`/upload?${new URLSearchParams({ library: String(series.data.libraryId), series: series.data.name })}`}>
          Upload to this series
        </Link>
      </p>
      <ul>
        {books.data.map((book) => (
          <li key={book.id}>
            {book.format === 'cbz' ? (
              <Link to={`/read/${book.id}`}>{book.title}</Link>
            ) : (
              <a href={fileUrl(book.id)}>{book.title}</a>
            )}{' '}
            ({progress(book)})
          </li>
        ))}
      </ul>
    </>
  )
}

function progress(book: Book) {
  if (book.format !== 'cbz') return `${book.format}, download`
  if (book.completed) return 'read'
  if (book.readPage > 0) return `page ${book.readPage} of ${book.pageCount}`
  return `${book.pageCount} pages`
}
