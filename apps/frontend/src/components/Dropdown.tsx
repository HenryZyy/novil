import { useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react'
import { Icon, type IconName } from './Icon'

// The style shows a choice as it would look, like a font in itself.
export type Choice<T extends string> = { value: T; label: string; style?: CSSProperties }

// A button that opens a menu of choices, for filters and sorting. Arrow keys
// move through the choices, Enter picks one, Escape or a click outside closes.
export function Dropdown<T extends string>({
  label,
  icon,
  value,
  choices,
  onChange,
  iconOnly = false,
  defaultValue,
}: {
  label: string
  icon?: IconName
  value: T
  choices: Choice<T>[]
  onChange: (value: T) => void
  // Shows only the icon, with the choice in its tooltip.
  iconOnly?: boolean
  // When the value differs from this, the button is highlighted as active.
  defaultValue?: T
}) {
  const [open, setOpen] = useState(false)
  // Which edge of the button the menu lines up with, so it stays on screen.
  const [alignLeft, setAlignLeft] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const menu = useRef<HTMLDivElement>(null)
  const button = useRef<HTMLButtonElement>(null)
  const menuId = useId()
  const current = choices.find((c) => c.value === value) ?? choices[0]

  useLayoutEffect(() => {
    if (!open || !menu.current || !root.current) return
    const button = root.current.getBoundingClientRect()
    setAlignLeft(button.right - menu.current.offsetWidth < 8)
  }, [open])

  useEffect(() => {
    if (!open) return
    root.current?.querySelector<HTMLButtonElement>('[aria-checked="true"]')?.focus()
    const onPointerDown = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [open])

  const close = () => {
    setOpen(false)
    button.current?.focus()
  }

  const onMenuKey = (e: KeyboardEvent<HTMLDivElement>) => {
    const items = [...(root.current?.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]') ?? [])]
    const i = items.indexOf(document.activeElement as HTMLButtonElement)
    if (e.key === 'ArrowDown') items[(i + 1) % items.length]?.focus()
    else if (e.key === 'ArrowUp') items[(i - 1 + items.length) % items.length]?.focus()
    else if (e.key === 'Home') items[0]?.focus()
    else if (e.key === 'End') items.at(-1)?.focus()
    else if (e.key === 'Escape') close()
    else if (e.key === 'Tab') setOpen(false)
    else return
    if (e.key !== 'Tab') e.preventDefault()
  }

  const active = defaultValue !== undefined && value !== defaultValue
  return (
    <div className="dropdown" ref={root}>
      <button
        ref={button}
        className={`toolbar-button${iconOnly ? ' icon-only' : ''}${active ? ' active' : ''}`}
        onClick={() => setOpen((o) => !o)}
        onKeyDown={(e) => {
          if (e.key === 'ArrowDown' && !open) {
            e.preventDefault()
            setOpen(true)
          }
        }}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={menuId}
        aria-label={`${label}: ${current.label}`}
        title={`${label}: ${current.label}`}
      >
        {icon && <Icon name={icon} size={18} />}
        {!iconOnly && <span style={current.style}>{current.label}</span>}
        {!iconOnly && <Icon name="chevronDown" size={16} />}
      </button>
      {open && (
        <div
          id={menuId}
          ref={menu}
          className={`dropdown-menu${alignLeft ? ' align-left' : ''}`}
          role="menu"
          aria-label={label}
          onKeyDown={onMenuKey}
        >
          <div className="dropdown-heading">{label}</div>
          {choices.map((c) => (
            <button
              key={c.value}
              role="menuitemradio"
              aria-checked={c.value === value}
              onClick={() => {
                onChange(c.value)
                close()
              }}
            >
              <span style={c.style}>{c.label}</span>
              {c.value === value && <Icon name="check" size={16} />}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
