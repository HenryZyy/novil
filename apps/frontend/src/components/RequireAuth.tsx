import { Navigate, Outlet, useLocation } from 'react-router'
import { useAuthStatus } from '../api/queries'

export function RequireAuth({ admin = false }: { admin?: boolean }) {
  const status = useAuthStatus()
  const location = useLocation()

  if (status.isPending) return <p className="page">Loading…</p>
  if (status.isError) return <p className="page error">Can't reach the server: {status.error.message}</p>
  if (status.data.setupRequired) return <Navigate to="/setup" replace />
  if (!status.data.user) {
    return <Navigate to={`/login?${new URLSearchParams({ next: location.pathname + location.search })}`} replace />
  }
  if (admin && !status.data.user.isAdmin) return <p className="page">Only admins can see this page.</p>
  return <Outlet />
}
