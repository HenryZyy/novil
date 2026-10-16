import { useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Navigate, useNavigate, useSearchParams } from 'react-router'
import { api } from '../api/client'
import { resetSession, useMe } from '../api/queries'

// Only same-site paths, so a link can't send people elsewhere after logging in.
function safeNext(next: string | null) {
  return next?.startsWith('/') && !next.startsWith('//') ? next : '/'
}

export function LoginPage() {
  const me = useMe()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  if (me.isPending) return <p className="page">Loading…</p>
  if (me.data) return <Navigate to={safeNext(params.get('next'))} replace />

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await api.login(username, password)
      await resetSession(queryClient)
      navigate(safeNext(params.get('next')), { replace: true })
    } catch (err) {
      setError((err as Error).message)
      setBusy(false)
    }
  }

  return (
    <main className="login">
      <form onSubmit={submit} className="form">
        <h1 className="brand">
          <span className="logo" aria-hidden="true" />
          novil
        </h1>
        <label>
          Username
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required autoFocus />
        </label>
        <label>
          Password
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            required
          />
        </label>
        {error && <p className="error">{error}</p>}
        <button type="submit" className="primary" disabled={busy}>
          Log in
        </button>
      </form>
    </main>
  )
}
