import { useState, type FormEvent } from 'react'
import type { User } from '../../api/client'
import { useCreateUser, useCurrentUser, useDeleteUser, useResetPassword, useUsers } from '../../api/queries'

export function UsersAdmin() {
  const users = useUsers()

  return (
    <>
      <h1>Users</h1>
      {users.isPending ? (
        <p>Loading…</p>
      ) : users.isError ? (
        <p className="error">{users.error.message}</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Username</th>
                <th>Role</th>
                <th>Created</th>
                <th>Password</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {users.data.map((u) => (
                <UserRow key={u.id} user={u} />
              ))}
            </tbody>
          </table>
        </div>
      )}
      <AddUserForm />
    </>
  )
}

function UserRow({ user }: { user: User }) {
  const me = useCurrentUser()
  const remove = useDeleteUser()
  const reset = useResetPassword()

  const resetPassword = () => {
    const password = prompt(
      `New starting password for ${user.username} (at least 8 characters). They'll have to change it after logging in.`,
    )
    if (password) reset.mutate({ id: user.id, password })
  }

  return (
    <tr>
      <td>{user.username}</td>
      <td>{user.role === 'admin' ? 'Admin' : 'User'}</td>
      <td>{new Date(user.createdAt).toLocaleDateString(undefined, { dateStyle: 'medium' })}</td>
      <td>{user.mustChangePassword ? 'Starting password' : 'Set'}</td>
      <td className="actions">
        <button onClick={resetPassword} disabled={reset.isPending}>
          Reset password
        </button>
        {user.id !== me.id && (
          <button
            onClick={() => confirm(`Delete ${user.username}? Their reading progress is deleted too.`) && remove.mutate(user.id)}
            disabled={remove.isPending}
          >
            Delete
          </button>
        )}
        {reset.isSuccess && <span className="muted">Password reset.</span>}
        {reset.isError && <span className="error">{reset.error.message}</span>}
        {remove.isError && <span className="error">{remove.error.message}</span>}
      </td>
    </tr>
  )
}

function AddUserForm() {
  const create = useCreateUser()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [admin, setAdmin] = useState(false)

  const submit = (e: FormEvent) => {
    e.preventDefault()
    create.mutate(
      { username, password, role: admin ? 'admin' : 'user' },
      {
        onSuccess: () => {
          setUsername('')
          setPassword('')
          setAdmin(false)
        },
      },
    )
  }

  return (
    <form onSubmit={submit} className="form">
      <h2>Add a user</h2>
      <label>
        Username
        <input
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoComplete="off"
          pattern="[a-zA-Z0-9._\-]{3,32}"
          required
        />
        <small className="muted">3 to 32 letters, digits, ".", "_" or "-".</small>
      </label>
      <label>
        Starting password
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="new-password"
          minLength={8}
          required
        />
        <small className="muted">They'll pick their own password after logging in.</small>
      </label>
      <label className="checkbox">
        <input type="checkbox" checked={admin} onChange={(e) => setAdmin(e.target.checked)} /> Admin (can manage
        libraries, uploads and users)
      </label>
      {create.isError && <p className="error">{create.error.message}</p>}
      {create.isSuccess && <p className="muted">Added {create.data.username}.</p>}
      <button type="submit" className="primary" disabled={create.isPending}>
        Add user
      </button>
    </form>
  )
}
