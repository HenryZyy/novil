import { useEffect, useRef, useState, type ReactNode } from 'react'
import { NavLink, useLocation } from 'react-router'
import { isAdmin, useCurrentUser, useLibraries } from '../api/queries'
import { Icon, type IconName } from './Icon'

function Item({
  to,
  icon,
  end,
  library,
  children,
}: {
  to: string
  icon: IconName
  end?: boolean
  // For /library links: which library it shows. NavLink ignores the query,
  // so without this every library item would light up together.
  library?: string
  children: ReactNode
}) {
  const location = useLocation()
  const current = new URLSearchParams(location.search).get('library') ?? ''
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) => (isActive && (library === undefined || library === current) ? 'active' : '')}
    >
      <Icon name={icon} />
      <span>{children}</span>
    </NavLink>
  )
}

export function NavMenu() {
  const user = useCurrentUser()
  const libraries = useLibraries()
  // The menu is open for the page it was opened on, so navigating closes it,
  // also with the browser's back button.
  const location = useLocation()
  const [openOn, setOpenOn] = useState<string | null>(null)
  const open = openOn === location.key
  const panel = useRef<HTMLElement>(null)
  const button = useRef<HTMLButtonElement>(null)

  // Lets the page make room for the menu, like Kavita's side nav.
  useEffect(() => {
    if (!open) return
    document.body.dataset.nav = 'open'
    return () => {
      delete document.body.dataset.nav
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    panel.current?.querySelector('a')?.focus()
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpenOn(null)
        button.current?.focus()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open])

  return (
    <>
      <button
        ref={button}
        className="menu-button"
        onClick={() => setOpenOn(open ? null : location.key)}
        aria-label="Menu"
        aria-expanded={open}
        aria-controls="nav-menu"
      >
        <Icon name={open ? 'close' : 'menu'} size={22} />
      </button>
      <nav id="nav-menu" ref={panel} className={`menu${open ? ' open' : ''}`} aria-label="Main" inert={!open}>
        <Item to="/" icon="home" end>
          Home
        </Item>

        <h3>Libraries</h3>
        {libraries.data?.map((library) => (
          <Item key={library.id} to={`/library?library=${library.id}`} icon="book" library={String(library.id)}>
            {library.name}
          </Item>
        ))}
        <Item to="/library" icon="library" library="">
          All series
        </Item>

        {isAdmin(user) && (
          <>
            <h3>Administration</h3>
            <Item to="/admin" icon="dashboard">
              Dashboard
            </Item>
            <Item to="/upload" icon="upload">
              Upload
            </Item>
          </>
        )}
      </nav>
    </>
  )
}
