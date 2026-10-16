import { Navigate, Outlet, useLocation } from 'react-router'
import { isAdmin, useMe } from '../api/queries'

// Only lets logged-in users through. Unless allowPendingPassword is set,
// users who must change their password are sent to do that first.
export function RequireAuth({ admin = false, allowPendingPassword = false }) {
  const me = useMe()
  const location = useLocation()

  if (me.isPending) return <p className="page">Loading…</p>
  if (me.isError) return <p className="page error">Can't reach the server: {me.error.message}</p>
  if (!me.data) {
    return <Navigate to={`/login?${new URLSearchParams({ next: location.pathname + location.search })}`} replace />
  }
  if (me.data.mustChangePassword && !allowPendingPassword) return <Navigate to="/profile" replace />
  if (admin && !isAdmin(me.data)) return <p className="page">Only admins can see this page.</p>
  return <Outlet />
}
