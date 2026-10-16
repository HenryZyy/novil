import { useQueryClient } from '@tanstack/react-query'
import { useRef, useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { api } from '../api/client'
import { resetSession, useCurrentUser, useLogout } from '../api/queries'
import { resizeAvatar, setAvatar, useAvatar } from '../avatar'
import { Avatar } from '../components/Avatar'
import { setTheme, themes, useTheme } from '../theme'

export function ProfilePage() {
  const user = useCurrentUser()
  const logout = useLogout()

  // Users with a starting password only get the password form, on its own.
  if (user.mustChangePassword) {
    return (
      <main className="login">
        <div className="form">
          <h1>Choose your password</h1>
          <p>Welcome, {user.username}! Your account has a starting password. Pick your own to continue.</p>
          <PasswordForm forced />
          <button onClick={() => logout.mutate()}>Log out</button>
        </div>
      </main>
    )
  }

  return (
    <>
      <section className="profile">
        <Avatar user={user} size={112} />
        <h1>@{user.username}</h1>
        <p className="muted">
          {user.role === 'admin' ? 'Admin' : 'User'} · member since{' '}
          {new Date(user.createdAt).toLocaleDateString(undefined, { dateStyle: 'long' })}
        </p>
        <PictureButtons userId={user.id} />
      </section>

      <section className="form">
        <h2>Appearance</h2>
        <ThemeChoice />
      </section>

      <section className="form">
        <h2>Password</h2>
        <p className="muted">Changing your password logs you out on your other devices.</p>
        <PasswordForm />
      </section>

      <section>
        <button onClick={() => logout.mutate()} disabled={logout.isPending}>
          Log out
        </button>
      </section>
    </>
  )
}

function ThemeChoice() {
  const theme = useTheme()
  return (
    <div className="segmented" role="radiogroup" aria-label="Theme">
      {themes.map((t) => (
        <button key={t.value} role="radio" aria-checked={theme === t.value} onClick={() => setTheme(t.value)}>
          {t.label}
        </button>
      ))}
    </div>
  )
}

function PictureButtons({ userId }: { userId: number }) {
  const picture = useAvatar(userId)
  const input = useRef<HTMLInputElement>(null)
  const [error, setError] = useState('')

  const choose = async (file: File | undefined) => {
    if (!file) return
    setError('')
    try {
      setAvatar(userId, await resizeAvatar(file))
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <>
      <p className="actions">
        <button onClick={() => input.current?.click()}>{picture ? 'Change picture' : 'Add picture'}</button>
        {picture && <button onClick={() => setAvatar(userId, null)}>Remove</button>}
      </p>
      <input
        ref={input}
        type="file"
        accept="image/*"
        hidden
        onChange={(e) => {
          choose(e.target.files?.[0])
          e.target.value = ''
        }}
      />
      {error && <p className="error">{error}</p>}
      <p className="muted">Your picture is saved in this browser only.</p>
    </>
  )
}

function PasswordForm({ forced = false }: { forced?: boolean }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [done, setDone] = useState(false)
  const [busy, setBusy] = useState(false)

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

  return (
    <form onSubmit={submit} className="form" style={{ marginTop: 0 }}>
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
      <button type="submit" className="primary" disabled={busy}>
        Change password
      </button>
    </form>
  )
}
