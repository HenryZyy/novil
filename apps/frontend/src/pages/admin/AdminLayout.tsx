import type { ReactNode } from 'react'
import { NavLink, Outlet } from 'react-router'
import { Icon, type IconName } from '../../components/Icon'

function Item({ to, icon, end, children }: { to: string; icon: IconName; end?: boolean; children: ReactNode }) {
  return (
    <NavLink to={to} end={end}>
      <Icon name={icon} />
      <span>{children}</span>
    </NavLink>
  )
}

export function AdminLayout() {
  return (
    <div className="admin">
      <nav className="admin-nav" aria-label="Administration">
        <h3>Administration</h3>
        <Item to="/admin" icon="dashboard" end>
          Overview
        </Item>
        <Item to="/admin/libraries" icon="library">
          Libraries
        </Item>
        <Item to="/admin/books" icon="book">
          Books
        </Item>
        <Item to="/admin/users" icon="users">
          Users
        </Item>
      </nav>
      <div className="admin-content">
        <Outlet />
      </div>
    </div>
  )
}
