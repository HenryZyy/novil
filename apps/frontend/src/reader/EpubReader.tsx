import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router'
import { fileUrl, type Book } from '../api/client'
import type { FoliateView, Location, TocItem } from 'foliate-js/view.js'
import { Icon } from '../components/Icon'
import { useIsDark } from '../theme'
import { MenuSection, MenuStepper, ReaderMenuPanel, SettingRow, ThemeRow } from './MenuParts'
import { addFontFaces, fontFaces, fonts, type FontName } from './fonts'
import { useProgressSaver } from './useProgressSaver'

type Settings = {
  flow: 'paginated' | 'scrolled'
  // The book's colours. "auto" follows the site's light or dark theme.
  theme: 'auto' | 'light' | 'sepia' | 'dark'
  font: FontName
  fontSize: number
  header: boolean
}

// Light and dark match the site's own colours.
const themes = {
  light: { bg: '#faf7f6', fg: '#2b2226', label: 'Light' },
  sepia: { bg: '#f4ecd8', fg: '#433422', label: 'Sepia' },
  dark: { bg: '#242426', fg: '#ececee', label: 'Dark' },
}
const themeChoices: Settings['theme'][] = ['auto', 'light', 'sepia', 'dark']

const resolve = (theme: Settings['theme'], dark: boolean) => (theme === 'auto' ? (dark ? 'dark' : 'light') : theme)
const defaults: Settings = { flow: 'paginated', theme: 'auto', font: 'publisher', fontSize: 100, header: true }
const storageKey = 'novil.epubReader'

function load(): Settings {
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey) ?? '{}')
    return {
      flow: saved.flow === 'scrolled' ? 'scrolled' : 'paginated',
      theme: themeChoices.includes(saved.theme) ? saved.theme : defaults.theme,
      font: Object.hasOwn(fonts, saved.font) ? saved.font : defaults.font,
      fontSize: typeof saved.fontSize === 'number' ? Math.min(Math.max(saved.fontSize, 60), 250) : defaults.fontSize,
      header: saved.header !== false,
    }
  } catch {
    // Blocked storage or broken JSON: start from the defaults.
    return defaults
  }
}

function styles({ theme, font, fontSize }: Settings, dark: boolean) {
  const resolved = resolve(theme, dark)
  const { bg, fg } = themes[resolved]
  const family = fonts[font].family
  return `
    ${family ? fontFaces : ''}
    html { color-scheme: ${resolved === 'dark' ? 'dark' : 'light'}; --theme-bg-color: ${bg}; }
    html, body { background: ${bg} !important; color: ${fg} !important; }
    html { font-size: ${fontSize}% !important; }
    body * { color: inherit !important; }
    a { text-decoration: underline; }
    p, li, blockquote { line-height: 1.6; }
    ${family ? `body, body * { font-family: ${family} !important; }` : ''}
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
  const dark = useIsDark()
  // Read when the book opens, like the settings.
  const darkRef = useRef(dark)
  const [menuOpen, setMenuOpen] = useState(false)
  useEffect(() => {
    if (menuOpen) addFontFaces()
  }, [menuOpen])
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
        view.renderer.setStyles(styles(settingsRef.current, darkRef.current))
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
    darkRef.current = dark
    const view = viewRef.current
    if (!view?.renderer) return
    view.renderer.setAttribute('flow', settings.flow)
    view.renderer.setStyles(styles(settings, dark))
  }, [settings, dark])

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
  const chapterIndex = chapters.findIndex((c) => c.href === currentChapter)
  const prevChapter = chapterIndex > 0 ? chapters[chapterIndex - 1] : undefined
  const nextChapter = chapterIndex >= 0 ? chapters[chapterIndex + 1] : undefined
  const resolved = resolve(settings.theme, dark)
  const colors = themes[resolved]
  const cycle = <T,>(list: T[], current: T) => list[(list.indexOf(current) + 1) % list.length]

  return (
    <div
      className={`reader epub-reader${settings.header ? '' : ' reader-no-header'}`}
      style={{ background: colors.bg, color: colors.fg }}
    >
      {settings.header ? (
        <header className="reader-bar" style={{ background: colors.bg, color: colors.fg }}>
          <Link to={`/series/${book.seriesId}`} className="reader-icon" aria-label="Back to series" title="Back to series">
            <Icon name="back" />
          </Link>
          <span className="reader-title">
            <strong>{book.title}</strong>
            {location?.tocItem?.label && <span>{location.tocItem.label}</span>}
          </span>
          <span className="reader-count">{percent}%</span>
          <button className="reader-icon" onClick={() => setMenuOpen((o) => !o)} aria-label="Menu" aria-expanded={menuOpen}>
            <Icon name="menu" />
          </button>
        </header>
      ) : (
        <button
          className="reader-icon reader-menu-button"
          style={{ background: colors.bg, color: colors.fg }}
          onClick={() => setMenuOpen(true)}
          aria-label="Menu"
        >
          <Icon name="menu" />
        </button>
      )}

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
          <ReaderMenuPanel
            seriesId={book.seriesId}
            seriesName={book.seriesName}
            title={book.title}
            onClose={() => setMenuOpen(false)}
          >
            {chapters.length > 0 && (
              <MenuSection title="Chapter">
                <MenuStepper
                  label="chapter"
                  onPrev={prevChapter ? () => viewRef.current?.goTo(prevChapter.href) : undefined}
                  onNext={nextChapter ? () => viewRef.current?.goTo(nextChapter.href) : undefined}
                >
                  <select value={currentChapter} onChange={(e) => viewRef.current?.goTo(e.target.value)} aria-label="Chapter">
                    {chapterIndex < 0 && <option value="">—</option>}
                    {chapters.map((c, i) => (
                      <option key={i} value={c.href}>
                        {'\u00a0'.repeat(c.depth * 3)}
                        {c.label}
                      </option>
                    ))}
                  </select>
                </MenuStepper>
              </MenuSection>
            )}

            <MenuSection title={`Progress · ${percent}%`}>
              <input
                className="epub-slider"
                type="range"
                min={0}
                max={100}
                value={percent}
                onChange={(e) => viewRef.current?.goToFraction(Number(e.target.value) / 100)}
                aria-label="Progress"
              />
            </MenuSection>

            <MenuSection title="Reading">
              <SettingRow
                icon="flow"
                label="Layout"
                value={settings.flow === 'paginated' ? 'Pages' : 'Scrolling'}
                onClick={() => update({ flow: settings.flow === 'paginated' ? 'scrolled' : 'paginated' })}
              />
              <div className="setting-row">
                <Icon name="textSize" />
                <span>Text size</span>
                <span className="size-controls">
                  <button
                    className="reader-icon"
                    onClick={() => update({ fontSize: Math.max(settings.fontSize - 10, 60) })}
                    aria-label="Smaller text"
                  >
                    <Icon name="minus" size={16} />
                  </button>
                  <span className="setting-value">{settings.fontSize}%</span>
                  <button
                    className="reader-icon"
                    onClick={() => update({ fontSize: Math.min(settings.fontSize + 10, 250) })}
                    aria-label="Larger text"
                  >
                    <Icon name="plus" size={16} />
                  </button>
                </span>
              </div>
            </MenuSection>

            <MenuSection title="Font">
              <div className="font-choices" role="radiogroup" aria-label="Font">
                {(Object.keys(fonts) as FontName[]).map((name) => (
                  <button
                    key={name}
                    role="radio"
                    aria-checked={settings.font === name}
                    style={{ fontFamily: fonts[name].family || undefined }}
                    onClick={() => update({ font: name })}
                  >
                    <span className="font-sample" aria-hidden>Aa</span>
                    <span className="font-name">{fonts[name].label}</span>
                  </button>
                ))}
              </div>
            </MenuSection>

            <MenuSection title="Display">
              <ThemeRow />
              <SettingRow
                icon="header"
                label="Header"
                value={settings.header ? 'Shown' : 'Hidden'}
                onClick={() => update({ header: !settings.header })}
              />
              <SettingRow
                icon="palette"
                label="Book colours"
                value={settings.theme === 'auto' ? `Auto (${colors.label})` : themes[settings.theme].label}
                onClick={() => update({ theme: cycle(themeChoices, settings.theme) })}
              />
            </MenuSection>

            <p className="reader-menu-hint">← → turn pages · M opens this menu</p>
          </ReaderMenuPanel>
        </>
      )}
    </div>
  )
}
