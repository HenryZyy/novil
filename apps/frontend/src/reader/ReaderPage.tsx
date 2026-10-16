import { useEffect, useRef, useState } from 'react'
import { Link, useParams } from 'react-router'
import { fileUrl, pageUrl, type Book } from '../api/client'
import { useBook, useBooks } from '../api/queries'
import { useProgressSaver } from './useProgressSaver'

type Mode = 'ltr' | 'rtl' | 'vertical'

function loadMode(): Mode {
  try {
    const mode = localStorage.getItem('novil.readerMode')
    if (mode === 'ltr' || mode === 'rtl' || mode === 'vertical') return mode
  } catch {
    // localStorage is blocked in some private windows.
  }
  return 'ltr'
}

function saveMode(mode: Mode) {
  try {
    localStorage.setItem('novil.readerMode', mode)
  } catch {
    // The mode just isn't remembered then.
  }
}

export function ReaderPage() {
  const book = useBook(Number(useParams().bookId))

  if (book.isPending) return <p>Loading…</p>
  if (book.isError) return <p>Error: {book.error.message}</p>
  if (book.data.format !== 'cbz' || book.data.pageCount === 0) {
    return (
      <p>
        This book can't be read in the browser yet. <a href={fileUrl(book.data.id)}>Download it</a>
      </p>
    )
  }
  return <Reader key={book.data.id} book={book.data} />
}

function Reader({ book }: { book: Book }) {
  const [mode, setMode] = useState(loadMode)
  const startPage = book.completed ? 1 : Math.min(Math.max(book.readPage, 1), book.pageCount)
  const [page, setPage] = useState(startPage)
  const books = useBooks(book.seriesId)
  useProgressSaver(book, page)

  const index = books.data?.findIndex((b) => b.id === book.id) ?? -1
  const nextBook = index >= 0 ? books.data?.slice(index + 1).find((b) => b.format === 'cbz') : undefined

  const next = () => setPage((p) => Math.min(p + 1, book.pageCount))
  const prev = () => setPage((p) => Math.max(p - 1, 1))
  const left = mode === 'rtl' ? next : prev
  const right = mode === 'rtl' ? prev : next

  useEffect(() => {
    if (mode === 'vertical') return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'ArrowLeft') left()
      if (e.key === 'ArrowRight') right()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  return (
    <>
      <p>
        <Link to={`/series/${book.seriesId}`}>Back</Link> | {book.title} | {page} / {book.pageCount} |{' '}
        <select
          value={mode}
          onChange={(e) => {
            setMode(e.target.value as Mode)
            saveMode(e.target.value as Mode)
          }}
        >
          <option value="ltr">Left to right</option>
          <option value="rtl">Right to left</option>
          <option value="vertical">Vertical</option>
        </select>
      </p>

      {mode === 'vertical' ? (
        <VerticalPages book={book} startPage={page} onPageChange={setPage} />
      ) : (
        <>
          <p>
            <button onClick={left}>←</button> <button onClick={right}>→</button>
          </p>
          <img
            src={pageUrl(book.id, page)}
            alt={`Page ${page}`}
            onClick={next}
            style={{ maxWidth: '100%', maxHeight: '90vh' }}
          />
        </>
      )}

      {page === book.pageCount && (
        <p>
          End of {book.title}.{' '}
          {nextBook ? (
            <Link to={`/read/${nextBook.id}`}>Next: {nextBook.title}</Link>
          ) : (
            <Link to={`/series/${book.seriesId}`}>Back to series</Link>
          )}
        </p>
      )}
    </>
  )
}

function VerticalPages({
  book,
  startPage,
  onPageChange,
}: {
  book: Book
  startPage: number
  onPageChange: (page: number) => void
}) {
  const pages = useRef<(HTMLImageElement | null)[]>([])
  const initialPage = useRef(startPage)

  useEffect(() => {
    pages.current[initialPage.current - 1]?.scrollIntoView()
  }, [])

  // The current page is the one crossing the middle of the screen.
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) onPageChange(Number((entry.target as HTMLElement).dataset.page))
        }
      },
      { rootMargin: '-50% 0px -50% 0px' },
    )
    for (const img of pages.current) if (img) observer.observe(img)
    return () => observer.disconnect()
  }, [book.id, onPageChange])

  return (
    <div style={{ maxWidth: 800 }}>
      {Array.from({ length: book.pageCount }, (_, i) => (
        <img
          key={i}
          ref={(el) => {
            pages.current[i] = el
          }}
          data-page={i + 1}
          src={pageUrl(book.id, i + 1)}
          alt={`Page ${i + 1}`}
          loading="lazy"
          style={{ display: 'block', width: '100%', minHeight: 200 }}
        />
      ))}
    </div>
  )
}
