import { useState, type FormEvent } from 'react'
import type { User } from '../../api/client'
import {
  useCreateUser,
  useCurrentUser,
  useDeleteUser,
  useDeleteUserAvatar,
  useResetPassword,
  useUpdateUser,
  useUsers,
} from '../../api/queries'
import { Avatar } from '../../components/Avatar'

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
  const update = useUpdateUser()
  const removeAvatar = useDeleteUserAvatar()
  const self = user.id === me.id

  const rename = () => {
    const username = prompt(`New username for ${user.username} (3 to 32 letters, digits, ".", "_" or "-"):`, user.username)
    if (username && username !== user.username) update.mutate({ id: user.id, username })
  }

  const resetPassword = () => {
    const password = prompt(
      `New starting password for ${user.username} (at least 8 characters). They'll have to change it after logging in.`,
    )
    if (password) reset.mutate({ id: user.id, password })
  }

  return (
    <tr>
      <td>
        <span className="user-cell">
          <Avatar user={user} size={28} />
          {user.username}
        </span>
      </td>
      <td>
        {/* Admins can't change their own role, so there's always one left. */}
        <select
          value={user.role}
          disabled={self || update.isPending}
          onChange={(e) => update.mutate({ id: user.id, role: e.target.value as User['role'] })}
          aria-label={`Role of ${user.username}`}
          title={self ? "You can't change your own role" : undefined}
        >
          <option value="user">User</option>
          <option value="admin">Admin</option>
        </select>
      </td>
      <td>{new Date(user.createdAt).toLocaleDateString(undefined, { dateStyle: 'medium' })}</td>
      <td>{user.mustChangePassword ? 'Starting password' : 'Set'}</td>
      <td className="actions">
        <button onClick={rename} disabled={update.isPending}>
          Rename
        </button>
        <button onClick={resetPassword} disabled={reset.isPending}>
          Reset password
        </button>
        {user.hasAvatar && (
          <button
            onClick={() => confirm(`Remove ${user.username}'s profile picture?`) && removeAvatar.mutate(user.id)}
            disabled={removeAvatar.isPending}
          >
            Remove picture
          </button>
        )}
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
        {update.isError && <span className="error">{update.error.message}</span>}
        {removeAvatar.isError && <span className="error">{removeAvatar.error.message}</span>}
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
