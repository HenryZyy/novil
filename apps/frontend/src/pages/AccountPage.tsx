import { useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { api } from '../api/client'
import { resetSession, useCurrentUser, useLogout } from '../api/queries'

export function AccountPage() {
  const user = useCurrentUser()
  const logout = useLogout()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)
  const [busy, setBusy] = useState(false)
  const forced = user.mustChangePassword

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setDone(false)
    if (next !== confirm) {
      setError("The new passwords don't match.")
      return
    }
    setBusy(true)
    try {
      await api.changePassword(current, next)
      await resetSession(queryClient)
      setCurrent('')
      setNext('')
      setConfirm('')
      if (forced) navigate('/', { replace: true })
      else setDone(true)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const form = (
    <form onSubmit={submit} className="form">
      <h1>{forced ? 'Choose your password' : 'Account'}</h1>
      {forced ? (
        <p>
          Welcome, {user.username}! Your account has a starting password. Pick your own to continue.
        </p>
      ) : (
        <p className="muted">
          Logged in as {user.username} ({user.role}). Changing your password logs you out on your other devices.
        </p>
      )}
      <label>
        {forced ? 'Starting password' : 'Current password'}
        <input type="password" value={current} onChange={(e) => setCurrent(e.target.value)} autoComplete="current-password" required />
      </label>
      <label>
        New password
        <input
          type="password"
          value={next}
          onChange={(e) => setNext(e.target.value)}
          autoComplete="new-password"
          minLength={8}
          required
        />
        <small className="muted">At least 8 characters.</small>
      </label>
      <label>
        Confirm new password
        <input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" required />
      </label>
      {error && <p className="error">{error}</p>}
      {done && <p className="muted">Password changed.</p>}
      <p className="actions">
        <button type="submit" className="primary" disabled={busy}>
          Change password
        </button>
        {forced ? (
          <button type="button" onClick={() => logout.mutate()}>
            Log out
          </button>
        ) : (
          <Link to="/" className="button">
            Back
          </Link>
        )}
      </p>
    </form>
  )

  return forced ? <main className="login">{form}</main> : form
}
