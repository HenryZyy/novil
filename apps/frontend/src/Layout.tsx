import { NavLink, Outlet } from 'react-router'
import { useCurrentUser, useLogout } from './api/queries'

export function Layout() {
  const user = useCurrentUser()
  const logout = useLogout()

  return (
    <>
      <header className="header">
        <NavLink to="/" className="brand">
          <span className="logo" aria-hidden="true" />
          novil
        </NavLink>
        <nav className="nav">
          <NavLink to="/" end>
            Home
          </NavLink>
          <NavLink to="/library">Library</NavLink>
          {user.isAdmin && <NavLink to="/upload">Upload</NavLink>}
          {user.isAdmin && <NavLink to="/admin">Dashboard</NavLink>}
        </nav>
        <span className="user">
          {user.username}
          <button onClick={() => logout.mutate()} disabled={logout.isPending}>
            Log out
          </button>
        </span>
      </header>
      <main className="page">
        <Outlet />
      </main>
    </>
  )
}
