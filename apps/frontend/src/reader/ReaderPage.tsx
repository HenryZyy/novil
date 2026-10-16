import { useEffect, useLayoutEffect, useMemo, useRef, useState, type MouseEvent, type PointerEvent, type ReactNode, type RefObject } from 'react'
import { Link, useParams } from 'react-router'
import { fileUrl, isReadable, pageUrl, type Book } from '../api/client'
import { useAnnotations, useBook, useBooks, useCreateAnnotation, useDeleteAnnotation } from '../api/queries'
import { EpubReader } from './EpubReader'
import { Icon } from '../components/Icon'
import { BookmarkButton } from './MenuParts'
import { ReaderMenu } from './ReaderMenu'
import { useReaderSettings, type ReaderSettings } from './settings'
import { useProgressSaver } from './useProgressSaver'
import { useReadingReport } from './useReadingReport'
import { clampPan, noZoom, zoomAt, type Zoom } from './zoom'

export function ReaderPage() {
  const book = useBook(Number(useParams().bookId))

  if (book.isPending) return <p className="page">Loading…</p>
  if (book.isError) return <p className="page error">{book.error.message}</p>
  if (book.data.format === 'epub') return <EpubReader key={book.data.id} book={book.data} />
  if (!isReadable(book.data)) {
    return (
      <p className="page">
        This book can't be read in the browser yet. <a href={fileUrl(book.data.id)}>Download it</a>
      </p>
    )
  }
  return <Reader key={book.data.id} book={book.data} />
}

function Reader({ book }: { book: Book }) {
  const [settings, update] = useReaderSettings(book.format === 'pdf' ? 'pdf' : 'comic')
  const [menuOpen, setMenuOpen] = useState(settings.pinned)
  const startPage = book.completed ? 1 : Math.min(Math.max(book.readPage, 1), book.pageCount)
  const [page, setPage] = useState(startPage)
  const annotations = useAnnotations(book.id)
  const createAnnotation = useCreateAnnotation(book.id)
  const deleteAnnotation = useDeleteAnnotation(book.id)
  const bookmark = annotations.data?.find((a) => a.kind === 'bookmark' && a.page === page)
  const toggleBookmark = () => {
    if (bookmark) deleteAnnotation.mutate(bookmark.id)
    else createAnnotation.mutate({ kind: 'bookmark', page })
  }
  const books = useBooks(book.seriesId)
  const progress = useMemo(
    () => ({ page, completed: book.completed || page >= book.pageCount }),
    [page, book.completed, book.pageCount],
  )
  useProgressSaver(book.id, progress)
  useReadingReport(book.id)

  // The series' other books that open in this reader.
  const chapters = books.data?.filter((b) => b.format === book.format && isReadable(b)) ?? []
  const nextChapter = chapters[chapters.findIndex((c) => c.id === book.id) + 1]
  const rtl = settings.direction === 'rtl'
  const strip = settings.layout === 'strip'
  const step = settings.layout === 'double' ? 2 : 1

  // Zooming the page, not the browser: see zoom.ts. In page layouts the page
  // is scaled and can be panned; the long strip just gets wider.
  const [zoom, setZoom] = useState(noZoom)
  const zoomBox = useRef<HTMLDivElement>(null)
  const zoomAnchor = useRef<{ x: number; y: number } | null>(null)
  const zoomBy = (factor: number, clientX?: number, clientY?: number) => {
    const box = zoomBox.current?.getBoundingClientRect()
    if (!box) return
    const x = clientX ?? box.left + box.width / 2
    const y = clientY ?? innerHeight / 2
    zoomAnchor.current = { x, y }
    setZoom((z) => zoomAt(z, factor, x - box.left, y - box.top, box.width, box.height))
  }
  const pan = (dx: number, dy: number) => {
    const box = zoomBox.current?.getBoundingClientRect()
    if (box) setZoom((z) => clampPan({ ...z, x: z.x + dx, y: z.y + dy }, box.width, box.height))
  }
  const resetZoom = () => setZoom(noZoom)

  const goTo = (p: number) => {
    setPage(Math.min(Math.max(p, 1), book.pageCount))
    if (!strip) resetZoom()
  }
  const changeSettings = (changes: Partial<ReaderSettings>) => {
    update(changes)
    resetZoom()
  }
  const next = () => goTo(page + step)
  const prev = () => goTo(page - step)
  const left = rtl ? next : prev
  const right = rtl ? prev : next
  const atEnd = page + step > book.pageCount

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLSelectElement || e.target instanceof HTMLTextAreaElement) return
      const mod = e.ctrlKey || e.metaKey
      if (mod && (e.key === '+' || e.key === '=')) zoomBy(1.25)
      else if (mod && e.key === '-') zoomBy(0.8)
      else if (mod && e.key === '0') resetZoom()
      else if (e.key === 'm' || e.key === 'M') setMenuOpen((open) => !open)
      else if (!mod && (e.key === 'b' || e.key === 'B')) toggleBookmark()
      else if (e.key === 'Escape') setMenuOpen(false)
      else if (settings.layout !== 'strip' && e.key === 'ArrowLeft') left()
      else if (settings.layout !== 'strip' && e.key === 'ArrowRight') right()
      else return
      e.preventDefault()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  // Ctrl + wheel (and trackpad pinch, which browsers send as one), Safari's
  // pinch gestures and two-finger pinches on touch screens zoom the page.
  useEffect(() => {
    const inMenu = (e: Event) => e.target instanceof Element && !!e.target.closest('.reader-menu')
    const onWheel = (e: WheelEvent) => {
      if (e.ctrlKey || e.metaKey) {
        e.preventDefault()
        // About 25% per mouse wheel notch; trackpads send many small steps.
        zoomBy(Math.exp(-Math.min(Math.max(e.deltaY, -100), 100) * 0.0022), e.clientX, e.clientY)
      } else if (!strip && zoom.scale > 1 && !inMenu(e)) {
        e.preventDefault()
        pan(-e.deltaX, -e.deltaY)
      }
    }
    let gesture = 1
    const onGestureStart = (e: Event) => {
      e.preventDefault()
      gesture = 1
    }
    const onGestureChange = (e: Event) => {
      e.preventDefault()
      const g = e as Event & { scale: number; clientX: number; clientY: number }
      zoomBy(g.scale / gesture, g.clientX, g.clientY)
      gesture = g.scale
    }
    let pinch: { d: number; x: number; y: number } | null = null
    const measure = (t: TouchList) => ({
      d: Math.hypot(t[0].clientX - t[1].clientX, t[0].clientY - t[1].clientY),
      x: (t[0].clientX + t[1].clientX) / 2,
      y: (t[0].clientY + t[1].clientY) / 2,
    })
    const onTouchStart = (e: TouchEvent) => {
      if (e.touches.length === 2) pinch = measure(e.touches)
    }
    const onTouchMove = (e: TouchEvent) => {
      if (e.touches.length !== 2 || !pinch) return
      e.preventDefault()
      const m = measure(e.touches)
      zoomBy(m.d / pinch.d, m.x, m.y)
      if (!strip) pan(m.x - pinch.x, m.y - pinch.y)
      pinch = m
    }
    const onTouchEnd = (e: TouchEvent) => {
      if (e.touches.length < 2) pinch = null
    }
    const opts = { passive: false }
    window.addEventListener('wheel', onWheel, opts)
    window.addEventListener('gesturestart', onGestureStart, opts)
    window.addEventListener('gesturechange', onGestureChange, opts)
    window.addEventListener('touchstart', onTouchStart, opts)
    window.addEventListener('touchmove', onTouchMove, opts)
    window.addEventListener('touchend', onTouchEnd)
    return () => {
      window.removeEventListener('wheel', onWheel)
      window.removeEventListener('gesturestart', onGestureStart)
      window.removeEventListener('gesturechange', onGestureChange)
      window.removeEventListener('touchstart', onTouchStart)
      window.removeEventListener('touchmove', onTouchMove)
      window.removeEventListener('touchend', onTouchEnd)
    }
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
          <Link to={`/series/${book.seriesId}`} className="reader-icon" aria-label="Back to series" title="Back to series">
            <Icon name="back" />
          </Link>
          <span className="reader-title">
            <strong>{book.seriesName}</strong>
            <span>{book.title}</span>
          </span>
          <span className="reader-count">
            {page} / {book.pageCount}
          </span>
          <BookmarkButton bookmarked={Boolean(bookmark)} onClick={toggleBookmark} />
          <button className="reader-icon" onClick={() => setMenuOpen((o) => !o)} aria-label="Menu" aria-expanded={menuOpen}>
            <Icon name="menu" />
          </button>
        </header>
      ) : (
        <button className="reader-icon reader-menu-button" onClick={() => setMenuOpen(true)} aria-label="Menu">
          <Icon name="menu" />
        </button>
      )}

      <main className={`reader-main fit-${settings.fit}${strip ? ' strip-main' : ''}`}>
        {strip ? (
          <StripView
            book={book}
            startPage={page}
            onPageChange={setPage}
            footer={endOfChapter}
            scale={zoom.scale}
            anchor={zoomAnchor}
            boxRef={zoomBox}
          />
        ) : (
          <>
            <PagedView
              book={book}
              page={page}
              settings={settings}
              onLeft={left}
              onRight={right}
              zoom={zoom}
              onPan={pan}
              boxRef={zoomBox}
            />
            {atEnd && endOfChapter}
          </>
        )}
      </main>

      {zoom.scale > 1 && (
        <div className="zoom-badge" role="status">
          <span>{Math.round(zoom.scale * 100)}%</span>
          <button className="reader-icon" onClick={resetZoom} aria-label="Reset zoom" title="Reset zoom (Ctrl+0)">
            <Icon name="close" size={16} />
          </button>
        </div>
      )}

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
            onChange={changeSettings}
            zoom={zoom.scale}
            onZoom={zoomBy}
            onClose={() => setMenuOpen(false)}
            annotations={annotations.data ?? []}
            bookmarked={Boolean(bookmark)}
            onToggleBookmark={toggleBookmark}
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
  zoom,
  onPan,
  boxRef,
}: {
  book: Book
  page: number
  settings: ReaderSettings
  onLeft: () => void
  onRight: () => void
  zoom: Zoom
  onPan: (dx: number, dy: number) => void
  boxRef: RefObject<HTMLDivElement | null>
}) {
  const pages = settings.layout === 'double' && page < book.pageCount ? [page, page + 1] : [page]
  const zoomed = zoom.scale > 1
  // Dragging pans a zoomed page. Pointers are counted so a two-finger pinch
  // doesn't also pan, and a drag doesn't count as a click.
  const drag = useRef({ pointers: 0, x: 0, y: 0, moved: false })
  const [dragging, setDragging] = useState(false)

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    drag.current.pointers++
    drag.current = { ...drag.current, x: e.clientX, y: e.clientY, moved: false }
    if (zoomed) {
      e.currentTarget.setPointerCapture(e.pointerId)
      setDragging(true)
    }
  }
  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    const d = drag.current
    if (!zoomed || !dragging || d.pointers !== 1) return
    const dx = e.clientX - d.x
    const dy = e.clientY - d.y
    if (Math.abs(dx) + Math.abs(dy) > 3) d.moved = true
    d.x = e.clientX
    d.y = e.clientY
    onPan(dx, dy)
  }
  const onPointerUp = () => {
    drag.current.pointers = Math.max(0, drag.current.pointers - 1)
    setDragging(false)
  }

  // Clicking the left or right half turns the page, but not while zoomed in.
  const onClick = (e: MouseEvent<HTMLDivElement>) => {
    if (zoomed || drag.current.moved) return
    const { left, width } = e.currentTarget.getBoundingClientRect()
    if (e.clientX - left < width / 2) onLeft()
    else onRight()
  }

  return (
    <div
      ref={boxRef}
      className={`reader-pages${zoomed ? ' zoomed' : ''}${dragging ? ' dragging' : ''}`}
      onClick={onClick}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
    >
      <div
        className={`zoom-layer${settings.direction === 'rtl' ? ' rtl' : ''}${pages.length > 1 ? ' double' : ''}`}
        style={zoomed ? { transform: `translate(${zoom.x}px, ${zoom.y}px) scale(${zoom.scale})` } : undefined}
      >
        {pages.map((p) => (
          <img key={p} src={pageUrl(book.id, p)} alt={`Page ${p}`} draggable={false} />
        ))}
      </div>
    </div>
  )
}

function StripView({
  book,
  startPage,
  onPageChange,
  footer,
  scale,
  anchor,
  boxRef,
}: {
  book: Book
  startPage: number
  onPageChange: (page: number) => void
  footer: ReactNode
  scale: number
  anchor: RefObject<{ x: number; y: number } | null>
  boxRef: RefObject<HTMLDivElement | null>
}) {
  const pages = useRef<(HTMLImageElement | null)[]>([])
  const initialPage = useRef(startPage)
  const lastScale = useRef(scale)

  // Zooming makes the strip wider and longer; scrolling by the same ratio
  // keeps the spot under the cursor or fingers in place.
  useLayoutEffect(() => {
    const ratio = scale / lastScale.current
    lastScale.current = scale
    if (ratio === 1) return
    const a = anchor.current ?? { x: innerWidth / 2, y: innerHeight / 2 }
    window.scrollTo({ top: (window.scrollY + a.y) * ratio - a.y })
    const main = boxRef.current?.parentElement
    main?.scrollTo({ left: (main.scrollLeft + a.x) * ratio - a.x })
  }, [scale, anchor, boxRef])

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
    <div className="reader-strip" ref={boxRef} style={scale > 1 ? { zoom: scale } : undefined}>
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
