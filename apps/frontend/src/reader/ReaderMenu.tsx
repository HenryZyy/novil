import type { ReactNode } from 'react'
import { Link, useNavigate } from 'react-router'
import type { Book } from '../api/client'
import {
  directions,
  fits,
  label,
  layouts,
  nextChoice,
  progressStyles,
  type ReaderSettings,
} from './settings'

export function ReaderMenu({
  book,
  chapters,
  page,
  onPageChange,
  settings,
  onChange,
  onClose,
}: {
  book: Book
  chapters: Book[]
  page: number
  onPageChange: (page: number) => void
  settings: ReaderSettings
  onChange: (changes: Partial<ReaderSettings>) => void
  onClose: () => void
}) {
  const navigate = useNavigate()
  const index = chapters.findIndex((c) => c.id === book.id)
  const prevChapter = index > 0 ? chapters[index - 1] : undefined
  const nextChapter = index >= 0 ? chapters[index + 1] : undefined

  return (
    <aside className="reader-menu" aria-label="Reader menu">
      <div className="reader-menu-top">
        <button className="icon-button" onClick={onClose} aria-label="Close menu">
          ✕
        </button>
        <button
          className="icon-button"
          onClick={() => onChange({ pinned: !settings.pinned })}
          aria-pressed={settings.pinned}
          title={settings.pinned ? 'Unpin menu' : 'Pin menu'}
        >
          {settings.pinned ? 'Unpin' : 'Pin'}
        </button>
      </div>

      <div className="reader-menu-title">
        <Link to={`/series/${book.seriesId}`}>{book.seriesName}</Link>
        <span>{book.title}</span>
      </div>

      <Stepper
        label="Page"
        onPrev={page > 1 ? () => onPageChange(page - 1) : undefined}
        onNext={page < book.pageCount ? () => onPageChange(page + 1) : undefined}
      >
        <select value={page} onChange={(e) => onPageChange(Number(e.target.value))} aria-label="Page">
          {Array.from({ length: book.pageCount }, (_, i) => (
            <option key={i} value={i + 1}>
              {i + 1}
            </option>
          ))}
        </select>
      </Stepper>

      <Stepper
        label="Chapter"
        onPrev={prevChapter ? () => navigate(`/read/${prevChapter.id}`) : undefined}
        onNext={() => navigate(nextChapter ? `/read/${nextChapter.id}` : `/series/${book.seriesId}`)}
      >
        <select value={book.id} onChange={(e) => navigate(`/read/${e.target.value}`)} aria-label="Chapter">
          {chapters.map((c) => (
            <option key={c.id} value={c.id}>
              {c.title}
            </option>
          ))}
        </select>
      </Stepper>

      <hr />

      <div className="reader-menu-settings">
        <button onClick={() => onChange({ layout: nextChoice(layouts, settings.layout) })}>
          {label(layouts, settings.layout)}
        </button>
        <button onClick={() => onChange({ fit: nextChoice(fits, settings.fit) })}>{label(fits, settings.fit)}</button>
        <button onClick={() => onChange({ direction: nextChoice(directions, settings.direction) })}>
          {label(directions, settings.direction)}
        </button>
        <button onClick={() => onChange({ header: !settings.header })}>
          {settings.header ? 'Header Shown' : 'Header Hidden'}
        </button>
        <button onClick={() => onChange({ progress: nextChoice(progressStyles, settings.progress) })}>
          {label(progressStyles, settings.progress)}
        </button>
      </div>

      <p className="muted">
        ← → turn pages · M opens this menu
        {settings.layout === 'strip' ? '' : ' · click the left or right side of a page'}
      </p>
    </aside>
  )
}

function Stepper({
  label,
  onPrev,
  onNext,
  children,
}: {
  label: string
  onPrev?: () => void
  onNext?: () => void
  children: ReactNode
}) {
  return (
    <div className="stepper">
      <button className="icon-button" onClick={onPrev} disabled={!onPrev} aria-label={`Previous ${label.toLowerCase()}`}>
        ‹
      </button>
      <label>
        <span className="muted">{label}</span>
        {children}
      </label>
      <button className="icon-button" onClick={onNext} disabled={!onNext} aria-label={`Next ${label.toLowerCase()}`}>
        ›
      </button>
    </div>
  )
}
