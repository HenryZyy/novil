import { useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Navigate, useNavigate, useSearchParams } from 'react-router'
import { api } from '../api/client'
import { resetSession, useAuthStatus } from '../api/queries'

export function LoginPage({ setup = false }: { setup?: boolean }) {
  const status = useAuthStatus()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  if (status.isPending) return <p className="page">Loading…</p>
  if (status.data?.user) return <Navigate to="/" replace />
  if (status.data && status.data.setupRequired !== setup) return <Navigate to={setup ? '/login' : '/setup'} replace />

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (setup && password !== confirm) {
      setError("The passwords don't match.")
      return
    }
    setBusy(true)
    setError('')
    try {
      await (setup ? api.setup(username, password) : api.login(username, password))
      await resetSession(queryClient)
      // Only same-site paths, so a link can't send people elsewhere after logging in.
      const next = params.get('next')
      navigate(next?.startsWith('/') && !next.startsWith('//') ? next : '/', { replace: true })
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
        {setup && <p>Welcome! Create the admin account to get started.</p>}
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
            autoComplete={setup ? 'new-password' : 'current-password'}
            minLength={setup ? 8 : undefined}
            required
          />
        </label>
        {setup && (
          <label>
            Confirm password
            <input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" required />
          </label>
        )}
        {error && <p className="error">{error}</p>}
        <button type="submit" className="primary" disabled={busy}>
          {setup ? 'Create account' : 'Log in'}
        </button>
      </form>
    </main>
  )
}
