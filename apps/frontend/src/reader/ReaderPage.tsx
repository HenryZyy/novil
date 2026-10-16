import { useEffect, useMemo, useRef, useState, type MouseEvent, type ReactNode } from 'react'
import { Link, useParams } from 'react-router'
import { fileUrl, pageUrl, type Book } from '../api/client'
import { useBook, useBooks } from '../api/queries'
import { EpubReader } from './EpubReader'
import { ReaderMenu } from './ReaderMenu'
import { useReaderSettings, type ReaderSettings } from './settings'
import { useProgressSaver } from './useProgressSaver'

export function ReaderPage() {
  const book = useBook(Number(useParams().bookId))

  if (book.isPending) return <p className="page">Loading…</p>
  if (book.isError) return <p className="page error">{book.error.message}</p>
  if (book.data.format === 'epub') return <EpubReader key={book.data.id} book={book.data} />
  if (book.data.format !== 'cbz' || book.data.pageCount === 0) {
    return (
      <p className="page">
        This book can't be read in the browser yet. <a href={fileUrl(book.data.id)}>Download it</a>
      </p>
    )
  }
  return <Reader key={book.data.id} book={book.data} />
}

function Reader({ book }: { book: Book }) {
  const [settings, update] = useReaderSettings()
  const [menuOpen, setMenuOpen] = useState(settings.pinned)
  const startPage = book.completed ? 1 : Math.min(Math.max(book.readPage, 1), book.pageCount)
  const [page, setPage] = useState(startPage)
  const books = useBooks(book.seriesId)
  const progress = useMemo(
    () => ({ page, completed: book.completed || page >= book.pageCount }),
    [page, book.completed, book.pageCount],
  )
  useProgressSaver(book.id, progress)

  const chapters = books.data?.filter((b) => b.format === 'cbz') ?? []
  const nextChapter = chapters[chapters.findIndex((c) => c.id === book.id) + 1]
  const rtl = settings.direction === 'rtl'
  const step = settings.layout === 'double' ? 2 : 1
  const goTo = (p: number) => setPage(Math.min(Math.max(p, 1), book.pageCount))
  const next = () => goTo(page + step)
  const prev = () => goTo(page - step)
  const left = rtl ? next : prev
  const right = rtl ? prev : next
  const atEnd = page + step > book.pageCount

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLSelectElement) return
      if (e.key === 'm' || e.key === 'M') setMenuOpen((open) => !open)
      else if (e.key === 'Escape') setMenuOpen(false)
      else if (settings.layout !== 'strip' && e.key === 'ArrowLeft') left()
      else if (settings.layout !== 'strip' && e.key === 'ArrowRight') right()
      else return
      e.preventDefault()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  useEffect(() => {
    for (let p = page + 1; p <= Math.min(page + 2 * step, book.pageCount); p++) new Image().src = pageUrl(book.id, p)
  }, [book.id, book.pageCount, page, step])

  const endOfChapter = (
    <p className="reader-end">
      End of {book.title}.{' '}
      {nextChapter ? (
        <Link to={`/read/${nextChapter.id}`}>Next: {nextChapter.title}</Link>
      ) : (
        <Link to={`/series/${book.seriesId}`}>Back to series</Link>
      )}
    </p>
  )

  const pinned = menuOpen && settings.pinned
  return (
    <div className={`reader${pinned ? ' reader-pinned' : ''}${settings.header ? '' : ' reader-no-header'}`}>
      {settings.header ? (
        <header className="reader-bar">
          <Link to={`/series/${book.seriesId}`}>← Back</Link>
          <span className="card-title">
            {book.seriesName} · {book.title}
          </span>
          <span className="muted">
            {page} / {book.pageCount}
          </span>
          <button className="icon-button" onClick={() => setMenuOpen((o) => !o)} aria-label="Menu">
            ☰
          </button>
        </header>
      ) : (
        <button className="icon-button reader-menu-button" onClick={() => setMenuOpen(true)} aria-label="Menu">
          ☰
        </button>
      )}

      <main className={`reader-main fit-${settings.fit}`}>
        {settings.layout === 'strip' ? (
          <StripView book={book} startPage={page} onPageChange={setPage} footer={endOfChapter} />
        ) : (
          <>
            <PagedView book={book} page={page} settings={settings} onLeft={left} onRight={right} />
            {atEnd && endOfChapter}
          </>
        )}
      </main>

      <ProgressBar book={book} page={page} settings={settings} onPageChange={goTo} />

      {menuOpen && (
        <>
          {!settings.pinned && <div className="reader-backdrop" onClick={() => setMenuOpen(false)} />}
          <ReaderMenu
            book={book}
            chapters={chapters}
            page={page}
            onPageChange={goTo}
            settings={settings}
            onChange={update}
            onClose={() => setMenuOpen(false)}
          />
        </>
      )}
    </div>
  )
}

function PagedView({
  book,
  page,
  settings,
  onLeft,
  onRight,
}: {
  book: Book
  page: number
  settings: ReaderSettings
  onLeft: () => void
  onRight: () => void
}) {
  const pages = settings.layout === 'double' && page < book.pageCount ? [page, page + 1] : [page]

  const onClick = (e: MouseEvent<HTMLDivElement>) => {
    const { left, width } = e.currentTarget.getBoundingClientRect()
    if (e.clientX - left < width / 2) onLeft()
    else onRight()
  }

  return (
    <div className={`reader-pages${settings.direction === 'rtl' ? ' rtl' : ''}${pages.length > 1 ? ' double' : ''}`} onClick={onClick}>
      {pages.map((p) => (
        <img key={p} src={pageUrl(book.id, p)} alt={`Page ${p}`} draggable={false} />
      ))}
    </div>
  )
}

function StripView({
  book,
  startPage,
  onPageChange,
  footer,
}: {
  book: Book
  startPage: number
  onPageChange: (page: number) => void
  footer: ReactNode
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
    <div className="reader-strip">
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
        />
      ))}
      {footer}
    </div>
  )
}

function ProgressBar({
  book,
  page,
  settings,
  onPageChange,
}: {
  book: Book
  page: number
  settings: ReaderSettings
  onPageChange: (page: number) => void
}) {
  if (settings.progress === 'hidden') return null
  const rtl = settings.direction === 'rtl'
  if (settings.progress === 'light') {
    return (
      <div className={`reader-lightbar${rtl ? ' rtl' : ''}`}>
        <div style={{ width: `${(page / book.pageCount) * 100}%` }} />
      </div>
    )
  }
  return (
    <div className={`reader-progress${rtl ? ' rtl' : ''}`}>
      {Array.from({ length: book.pageCount }, (_, i) => (
        <button
          key={i}
          className={i + 1 === page ? 'current' : i + 1 < page ? 'read' : ''}
          onClick={() => onPageChange(i + 1)}
          aria-label={`Page ${i + 1}`}
          title={`Page ${i + 1}`}
        />
      ))}
    </div>
  )
}
