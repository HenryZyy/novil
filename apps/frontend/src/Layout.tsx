import { NavLink, Outlet } from 'react-router'
import { isAdmin, useCurrentUser, useLogout } from './api/queries'

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
          {isAdmin(user) && <NavLink to="/upload">Upload</NavLink>}
          {isAdmin(user) && <NavLink to="/admin">Dashboard</NavLink>}
        </nav>
        <span className="user">
          <NavLink to="/account">{user.username}</NavLink>
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

// Users who must change their password get that page on its own, without
// the rest of the app.
export function LayoutUnlessPending() {
  return useCurrentUser().mustChangePassword ? <Outlet /> : <Layout />
}
