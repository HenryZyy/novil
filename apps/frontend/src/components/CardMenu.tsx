import { useEffect, useId, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import { Link } from 'react-router'
import { Icon, type IconName } from './Icon'

export type MenuItem =
  | {
      label: string
      icon?: IconName
      // One of: an action, a page in the app, or a file to download.
      onSelect?: () => void
      to?: string
      download?: string
      danger?: boolean
    }
  | 'divider'

// The ⋮ button on a card and the menu it opens. The menu is drawn over the
// whole page, so a scrolling row can't cut it off.
export function CardMenu({ label, items, onOpen }: { label: string; items: MenuItem[]; onOpen?: () => void }) {
  const [open, setOpen] = useState(false)
  const [position, setPosition] = useState<{ top: number; left: number } | null>(null)
  const button = useRef<HTMLButtonElement>(null)
  const menu = useRef<HTMLDivElement>(null)
  const id = useId()

  const close = (focus = true) => {
    setOpen(false)
    setPosition(null)
    if (focus) button.current?.focus()
  }

  // Under the button, right-aligned with it; above it when there's no room.
  // It follows the button when the page scrolls, and closes once the
  // button is gone from view.
  useLayoutEffect(() => {
    if (!open) return
    const place = () => {
      if (!button.current || !menu.current) return
      const b = button.current.getBoundingClientRect()
      if (b.bottom < 0 || b.top > innerHeight) return close(false)
      const m = menu.current
      const margin = 8
      const left = Math.min(Math.max(b.right - m.offsetWidth, margin), innerWidth - m.offsetWidth - margin)
      const below = b.bottom + 4
      const top = below + m.offsetHeight > innerHeight - margin ? Math.max(b.top - m.offsetHeight - 4, margin) : below
      setPosition({ top, left })
    }
    place()
    addEventListener('scroll', place, true)
    addEventListener('resize', place)
    return () => {
      removeEventListener('scroll', place, true)
      removeEventListener('resize', place)
    }
  }, [open])

  // Once placed: hidden, it couldn't take the focus.
  const placed = position !== null
  useEffect(() => {
    if (placed) menu.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus({ preventScroll: true })
  }, [placed])

  useEffect(() => {
    if (!open) return
    const onPointerDown = (e: PointerEvent) => {
      const target = e.target as Node
      if (!menu.current?.contains(target) && !button.current?.contains(target)) close(false)
    }
    const onEscape = (e: globalThis.KeyboardEvent) => e.key === 'Escape' && close()
    document.addEventListener('pointerdown', onPointerDown)
    addEventListener('keydown', onEscape)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      removeEventListener('keydown', onEscape)
    }
  }, [open])

  const onKey = (e: KeyboardEvent) => {
    const all = [...(menu.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])]
    const i = all.indexOf(document.activeElement as HTMLElement)
    if (e.key === 'ArrowDown') all[(i + 1) % all.length]?.focus()
    else if (e.key === 'ArrowUp') all[(i - 1 + all.length) % all.length]?.focus()
    else if (e.key === 'Home') all[0]?.focus()
    else if (e.key === 'End') all.at(-1)?.focus()
    else if (e.key === 'Escape') close()
    else if (e.key === 'Tab') close(false)
    else return
    if (e.key !== 'Tab') e.preventDefault()
  }

  return (
    <>
      <button
        ref={button}
        className={`card-menu-button${open ? ' open' : ''}`}
        onClick={() => {
          if (open) return close()
          onOpen?.()
          setOpen(true)
        }}
        aria-label={`More for ${label}`}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? id : undefined}
        title="More"
      >
        <Icon name="more" size={18} />
      </button>
      {open &&
        createPortal(
          <div
            id={id}
            ref={menu}
            className="card-menu"
            role="menu"
            aria-label={label}
            onKeyDown={onKey}
            style={position ?? { visibility: 'hidden', top: 0, left: 0 }}
          >
            {items.map((item, i) => {
              if (item === 'divider') return <div key={i} className="card-menu-divider" role="separator" />
              const content = (
                <>
                  {item.icon && <Icon name={item.icon} size={18} />}
                  <span>{item.label}</span>
                </>
              )
              const className = item.danger ? 'danger' : undefined
              if (item.to)
                return (
                  <Link key={i} role="menuitem" to={item.to} className={className} onClick={() => close(false)}>
                    {content}
                  </Link>
                )
              if (item.download)
                return (
                  <a key={i} role="menuitem" href={item.download} download className={className} onClick={() => close(false)}>
                    {content}
                  </a>
                )
              return (
                <button
                  key={i}
                  role="menuitem"
                  className={className}
                  onClick={() => {
                    close()
                    item.onSelect?.()
                  }}
                >
                  {content}
                </button>
              )
            })}
          </div>,
          document.body,
        )}
    </>
  )
}
