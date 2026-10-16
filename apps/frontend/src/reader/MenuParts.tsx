import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { Icon, type IconName } from '../components/Icon'
import { setTheme, useIsDark } from '../theme'

// The pieces both readers build their menus from, styled like the site's
// navigation drawer.

export function ReaderMenuPanel({
  seriesId,
  seriesName,
  title,
  pinned,
  onPin,
  onClose,
  children,
}: {
  seriesId: number
  seriesName: string
  title: string
  pinned?: boolean
  onPin?: () => void
  onClose: () => void
  children: ReactNode
}) {
  return (
    <aside className="reader-menu" aria-label="Reader menu">
      <div className="reader-menu-top">
        <div className="reader-menu-title">
          <Link to={`/series/${seriesId}`}>{seriesName}</Link>
          {/* A single book's series is usually named like the book. */}
          {title !== seriesName && <span>{title}</span>}
        </div>
        {onPin && (
          <button
            className="reader-icon"
            onClick={onPin}
            aria-pressed={pinned}
            title={pinned ? 'Unpin menu' : 'Keep the menu open'}
            aria-label={pinned ? 'Unpin menu' : 'Pin menu'}
          >
            <Icon name="pin" />
          </button>
        )}
        <button className="reader-icon" onClick={onClose} aria-label="Close menu">
          <Icon name="close" />
        </button>
      </div>
      {children}
    </aside>
  )
}

export function MenuSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="reader-menu-section">
      <h3>{title}</h3>
      {children}
    </section>
  )
}

export function MenuStepper({
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
      <button className="reader-icon" onClick={onPrev} disabled={!onPrev} aria-label={`Previous ${label}`}>
        <Icon name="chevronLeft" />
      </button>
      {children}
      <button className="reader-icon" onClick={onNext} disabled={!onNext} aria-label={`Next ${label}`}>
        <Icon name="chevronRight" />
      </button>
    </div>
  )
}

// A setting that switches to its next option when clicked.
export function SettingRow({
  icon,
  label,
  value,
  onClick,
}: {
  icon: IconName
  label: string
  value: string
  onClick: () => void
}) {
  return (
    <button className="setting-row" onClick={onClick}>
      <Icon name={icon} />
      <span>{label}</span>
      <span className="setting-value">{value}</span>
    </button>
  )
}

export function BookmarkButton({ bookmarked, onClick }: { bookmarked: boolean; onClick: () => void }) {
  return (
    <button
      className={`reader-icon bookmark-button${bookmarked ? ' on' : ''}`}
      onClick={onClick}
      aria-pressed={bookmarked}
      aria-label={bookmarked ? 'Remove bookmark' : 'Bookmark this page'}
      title={bookmarked ? 'Remove bookmark' : 'Bookmark this page (B)'}
    >
      <Icon name="bookmark" filled={bookmarked} />
    </button>
  )
}

export function ThemeRow() {
  const dark = useIsDark()
  return (
    <SettingRow
      icon={dark ? 'moon' : 'sun'}
      label="Theme"
      value={dark ? 'Dark' : 'Light'}
      onClick={() => setTheme(dark ? 'light' : 'dark')}
    />
  )
}
