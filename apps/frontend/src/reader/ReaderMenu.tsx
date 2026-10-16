import { useNavigate } from 'react-router'
import { Icon } from '../components/Icon'
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
import { MenuSection, MenuStepper, ReaderMenuPanel, SettingRow, ThemeRow } from './MenuParts'

export function ReaderMenu({
  book,
  chapters,
  page,
  onPageChange,
  settings,
  onChange,
  zoom,
  onZoom,
  onClose,
}: {
  book: Book
  chapters: Book[]
  page: number
  onPageChange: (page: number) => void
  settings: ReaderSettings
  onChange: (changes: Partial<ReaderSettings>) => void
  zoom: number
  onZoom: (factor: number) => void
  onClose: () => void
}) {
  const navigate = useNavigate()
  const index = chapters.findIndex((c) => c.id === book.id)
  const prevChapter = index > 0 ? chapters[index - 1] : undefined
  const nextChapter = index >= 0 ? chapters[index + 1] : undefined

  return (
    <ReaderMenuPanel
      seriesId={book.seriesId}
      seriesName={book.seriesName}
      title={book.title}
      pinned={settings.pinned}
      onPin={() => onChange({ pinned: !settings.pinned })}
      onClose={onClose}
    >
      <MenuSection title="Page">
        <MenuStepper
          label="page"
          onPrev={page > 1 ? () => onPageChange(page - 1) : undefined}
          onNext={page < book.pageCount ? () => onPageChange(page + 1) : undefined}
        >
          <select value={page} onChange={(e) => onPageChange(Number(e.target.value))} aria-label="Page">
            {Array.from({ length: book.pageCount }, (_, i) => (
              <option key={i} value={i + 1}>
                {i + 1} / {book.pageCount}
              </option>
            ))}
          </select>
        </MenuStepper>
      </MenuSection>

      <MenuSection title="Chapter">
        <MenuStepper
          label="chapter"
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
        </MenuStepper>
      </MenuSection>

      <MenuSection title="Reading">
        <SettingRow
          icon="layout"
          label="Layout"
          value={label(layouts, settings.layout)}
          onClick={() => onChange({ layout: nextChoice(layouts, settings.layout) })}
        />
        <SettingRow
          icon="fit"
          label="Fit"
          value={label(fits, settings.fit)}
          onClick={() => onChange({ fit: nextChoice(fits, settings.fit) })}
        />
        <SettingRow
          icon="direction"
          label="Direction"
          value={label(directions, settings.direction)}
          onClick={() => onChange({ direction: nextChoice(directions, settings.direction) })}
        />
      </MenuSection>

      <MenuSection title="Display">
        <div className="setting-row">
          <Icon name="zoom" />
          <span>Zoom</span>
          <span className="size-controls">
            <button className="reader-icon" onClick={() => onZoom(0.8)} disabled={zoom <= 1} aria-label="Zoom out">
              <Icon name="minus" size={16} />
            </button>
            <span className="setting-value">{Math.round(zoom * 100)}%</span>
            <button className="reader-icon" onClick={() => onZoom(1.25)} disabled={zoom >= 5} aria-label="Zoom in">
              <Icon name="plus" size={16} />
            </button>
          </span>
        </div>
        <ThemeRow />
        <SettingRow
          icon="header"
          label="Header"
          value={settings.header ? 'Shown' : 'Hidden'}
          onClick={() => onChange({ header: !settings.header })}
        />
        <SettingRow
          icon="progress"
          label="Progress bar"
          value={label(progressStyles, settings.progress)}
          onClick={() => onChange({ progress: nextChoice(progressStyles, settings.progress) })}
        />
      </MenuSection>

      <p className="reader-menu-hint">
        ← → turn pages · Ctrl + scroll or pinch to zoom · M opens this menu
        {settings.layout === 'strip' ? '' : ' · click the left or right half of a page'}
      </p>
    </ReaderMenuPanel>
  )
}
