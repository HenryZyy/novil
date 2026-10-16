import { NavLink, Outlet } from 'react-router'
import { useCurrentUser } from './api/queries'
import { Avatar } from './components/Avatar'
import { NavMenu } from './components/NavMenu'
import { ThemeButton } from './components/ThemeButton'

export function Layout() {
  const user = useCurrentUser()

  return (
    <>
      <header className="header">
        <NavMenu />
        <NavLink to="/" className="brand">
          <span className="logo" aria-hidden="true" />
          novil
        </NavLink>
        <span className="header-end">
          <ThemeButton />
          <NavLink to="/profile" className="user" title="Your profile">
            <Avatar user={user} />
            <span className="username">@{user.username}</span>
          </NavLink>
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
