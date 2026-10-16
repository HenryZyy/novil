import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router'
import { fileUrl, type Book } from '../api/client'
import type { FoliateView, Location, TocItem } from 'foliate-js/view.js'
import { useProgressSaver } from './useProgressSaver'

type Settings = {
  flow: 'paginated' | 'scrolled'
  theme: 'light' | 'sepia' | 'dark'
  font: 'publisher' | 'serif' | 'sans-serif'
  fontSize: number
}

const themes = {
  light: { bg: '#ffffff', fg: '#1c1917', label: 'Light' },
  sepia: { bg: '#f4ecd8', fg: '#433422', label: 'Sepia' },
  dark: { bg: '#121212', fg: '#e7e5e4', label: 'Dark' },
}
const fonts = { publisher: 'Publisher font', serif: 'Serif', 'sans-serif': 'Sans' }
const defaults: Settings = { flow: 'paginated', theme: 'light', font: 'publisher', fontSize: 100 }
const storageKey = 'novil.epubReader'

function load(): Settings {
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey) ?? '{}')
    return {
      flow: saved.flow === 'scrolled' ? 'scrolled' : 'paginated',
      theme: saved.theme in themes ? saved.theme : defaults.theme,
      font: saved.font in fonts ? saved.font : defaults.font,
      fontSize: typeof saved.fontSize === 'number' ? Math.min(Math.max(saved.fontSize, 60), 250) : defaults.fontSize,
    }
  } catch {
    // Blocked storage or broken JSON: start from the defaults.
    return defaults
  }
}

function styles({ theme, font, fontSize }: Settings) {
  const { bg, fg } = themes[theme]
  return `
    html { color-scheme: ${theme === 'dark' ? 'dark' : 'light'}; --theme-bg-color: ${bg}; }
    html, body { background: ${bg} !important; color: ${fg} !important; }
    html { font-size: ${fontSize}% !important; }
    body * { color: inherit !important; }
    a { text-decoration: underline; }
    p, li, blockquote { line-height: 1.6; }
    ${font === 'publisher' ? '' : `body, body * { font-family: ${font} !important; }`}
  `
}

function flatten(items: TocItem[] | null | undefined, depth = 0): { label: string; href: string; depth: number }[] {
  return (items ?? []).flatMap((item) => [
    { label: item.label, href: item.href, depth },
    ...flatten(item.subitems, depth + 1),
  ])
}

export function EpubReader({ book }: { book: Book }) {
  const container = useRef<HTMLDivElement>(null)
  const viewRef = useRef<FoliateView | null>(null)
  const [settings, setSettings] = useState(load)
  const [menuOpen, setMenuOpen] = useState(false)
  const [location, setLocation] = useState<Location | null>(null)
  const [toc, setToc] = useState<TocItem[]>([])
  const [error, setError] = useState('')
  // Read when the book opens, which can be after a setting changed.
  const settingsRef = useRef(settings)
  // Where to open the book; later saves must not reopen it.
  const startPosition = useRef(book.completed ? undefined : book.readPosition || undefined)

  const update = (changes: Partial<Settings>) =>
    setSettings((current) => {
      const next = { ...current, ...changes }
      try {
        localStorage.setItem(storageKey, JSON.stringify(next))
      } catch {
        // The settings just aren't remembered then.
      }
      return next
    })

  useEffect(() => {
    let cancelled = false
    let opened = false
    let view: FoliateView | undefined
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLSelectElement || e.target instanceof HTMLInputElement) return
      if (e.key === 'm' || e.key === 'M') setMenuOpen((open) => !open)
      else if (e.key === 'Escape') setMenuOpen(false)
      else if (e.key === 'ArrowLeft') view?.goLeft()
      else if (e.key === 'ArrowRight') view?.goRight()
    }

    ;(async () => {
      await import('foliate-js/view.js')
      if (cancelled || !container.current) return
      view = document.createElement('foliate-view') as FoliateView
      viewRef.current = view
      container.current.append(view)
      view.addEventListener('relocate', (e) => setLocation((e as CustomEvent<Location>).detail))
      // Keys pressed while the book has focus happen inside its iframe.
      view.addEventListener('load', (e) => (e as CustomEvent<{ doc: Document }>).detail.doc.addEventListener('keydown', onKey))
      try {
        await view.open(fileUrl(book.id))
        opened = true
        if (cancelled) return
        view.renderer.setAttribute('flow', settingsRef.current.flow)
        view.renderer.setAttribute('max-inline-size', '720px')
        view.renderer.setAttribute('gap', '6%')
        view.renderer.setStyles(styles(settingsRef.current))
        setToc(view.book.toc ?? [])
        await view.init({ lastLocation: startPosition.current, showTextStart: true })
      } catch (err) {
        if (!cancelled) setError(`This book couldn't be opened: ${(err as Error).message}`)
      }
    })()

    window.addEventListener('keydown', onKey)
    return () => {
      cancelled = true
      window.removeEventListener('keydown', onKey)
      // foliate-js throws when closing a book that is still opening.
      try {
        if (opened) view?.close()
      } catch {
        // Nothing to clean up then.
      }
      view?.remove()
      viewRef.current = null
    }
  }, [book.id])

  useEffect(() => {
    settingsRef.current = settings
    const view = viewRef.current
    if (!view?.renderer) return
    view.renderer.setAttribute('flow', settings.flow)
    view.renderer.setStyles(styles(settings))
  }, [settings])

  const percent = location ? Math.round(location.fraction * 100) : 0
  const progress = useMemo(
    () =>
      location
        ? { page: percent, position: location.cfi, completed: book.completed || location.fraction >= 0.995 }
        : null,
    [location, percent, book.completed],
  )
  useProgressSaver(book.id, progress)

  const chapters = flatten(toc)
  const currentChapter = location?.tocItem?.href ?? ''
  const theme = themes[settings.theme]

  return (
    <div className="reader epub-reader" style={{ background: theme.bg, color: theme.fg }}>
      <header className="reader-bar" style={{ background: theme.bg, color: theme.fg }}>
        <Link to={`/series/${book.seriesId}`}>← Back</Link>
        <span className="card-title">
          {book.title}
          {location?.tocItem?.label ? ` · ${location.tocItem.label}` : ''}
        </span>
        <span className="muted">{percent}%</span>
        <button className="icon-button" onClick={() => setMenuOpen((o) => !o)} aria-label="Menu">
          ☰
        </button>
      </header>

      {error ? <p className="page error">{error}</p> : <div ref={container} className="epub-view" />}

      {settings.flow === 'paginated' && !error && (
        <>
          <button className="epub-edge left" onClick={() => viewRef.current?.goLeft()} aria-label="Previous page" />
          <button className="epub-edge right" onClick={() => viewRef.current?.goRight()} aria-label="Next page" />
        </>
      )}

      <div className="reader-lightbar">
        <div style={{ width: `${percent}%` }} />
      </div>

      {menuOpen && (
        <>
          <div className="reader-backdrop" onClick={() => setMenuOpen(false)} />
          <aside className="reader-menu" aria-label="Reader menu">
            <div className="reader-menu-top">
              <button className="icon-button" onClick={() => setMenuOpen(false)} aria-label="Close menu">
                ✕
              </button>
            </div>
            <div className="reader-menu-title">
              <Link to={`/series/${book.seriesId}`}>{book.seriesName}</Link>
              <span>{book.title}</span>
            </div>

            {chapters.length > 0 && (
              <div className="stepper">
                <label>
                  <span className="muted">Chapter</span>
                  <select value={currentChapter} onChange={(e) => viewRef.current?.goTo(e.target.value)} aria-label="Chapter">
                    {!chapters.some((c) => c.href === currentChapter) && <option value="">—</option>}
                    {chapters.map((c, i) => (
                      <option key={i} value={c.href}>
                        {' '.repeat(c.depth * 3)}
                        {c.label}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
            )}

            <label className="epub-slider">
              <span className="muted">Progress: {percent}%</span>
              <input
                type="range"
                min={0}
                max={100}
                value={percent}
                onChange={(e) => viewRef.current?.goToFraction(Number(e.target.value) / 100)}
              />
            </label>

            <hr />

            <div className="reader-menu-settings">
              <button onClick={() => update({ flow: settings.flow === 'paginated' ? 'scrolled' : 'paginated' })}>
                {settings.flow === 'paginated' ? 'Paginated' : 'Scrolled'}
              </button>
              <button
                onClick={() => {
                  const keys = Object.keys(themes) as Settings['theme'][]
                  update({ theme: keys[(keys.indexOf(settings.theme) + 1) % keys.length] })
                }}
              >
                Theme: {theme.label}
              </button>
              <button
                onClick={() => {
                  const keys = Object.keys(fonts) as Settings['font'][]
                  update({ font: keys[(keys.indexOf(settings.font) + 1) % keys.length] })
                }}
              >
                {fonts[settings.font]}
              </button>
              <div className="stepper">
                <button
                  className="icon-button"
                  onClick={() => update({ fontSize: Math.max(settings.fontSize - 10, 60) })}
                  aria-label="Smaller text"
                >
                  A−
                </button>
                <label>
                  <span className="muted">Text size</span>
                  <span>{settings.fontSize}%</span>
                </label>
                <button
                  className="icon-button"
                  onClick={() => update({ fontSize: Math.min(settings.fontSize + 10, 250) })}
                  aria-label="Larger text"
                >
                  A+
                </button>
              </div>
            </div>

            <p className="muted">← → turn pages · M opens this menu</p>
          </aside>
        </>
      )}
    </div>
  )
}
