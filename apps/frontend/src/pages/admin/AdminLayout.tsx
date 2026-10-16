import { NavLink, Outlet } from 'react-router'

export function AdminLayout() {
  return (
    <div className="admin">
      <nav className="admin-nav">
        <NavLink to="/admin" end>
          Overview
        </NavLink>
        <NavLink to="/admin/libraries">Libraries</NavLink>
        <NavLink to="/admin/users">Users</NavLink>
      </nav>
      <div className="admin-content">
        <Outlet />
      </div>
    </div>
  )
}
