import { Link, Outlet } from 'react-router'

export function Layout() {
  return (
    <>
      <nav>
        <Link to="/">Home</Link> | <Link to="/libraries">Libraries</Link> | <Link to="/upload">Upload</Link>
      </nav>
      <Outlet />
    </>
  )
}
