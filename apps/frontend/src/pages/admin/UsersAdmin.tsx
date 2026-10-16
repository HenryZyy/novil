import { useState, type FormEvent } from 'react'
import { useCreateUser, useCurrentUser, useDeleteUser, useUsers } from '../../api/queries'

export function UsersAdmin() {
  const users = useUsers()
  const me = useCurrentUser()
  const remove = useDeleteUser()

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
                <th>Last login</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {users.data.map((u) => (
                <tr key={u.id}>
                  <td>{u.username}</td>
                  <td>{u.isAdmin ? 'Admin' : 'User'}</td>
                  <td>{formatDate(u.createdAt)}</td>
                  <td>{u.lastLoginAt ? formatDate(u.lastLoginAt) : 'Never'}</td>
                  <td>
                    {u.id !== me.id && (
                      <button
                        onClick={() => confirm(`Delete ${u.username}? Their reading progress is deleted too.`) && remove.mutate(u.id)}
                        disabled={remove.isPending}
                      >
                        Delete
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {remove.isError && <p className="error">{remove.error.message}</p>}
      <AddUserForm />
    </>
  )
}

function AddUserForm() {
  const create = useCreateUser()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [isAdmin, setIsAdmin] = useState(false)

  const submit = (e: FormEvent) => {
    e.preventDefault()
    create.mutate(
      { username, password, isAdmin },
      {
        onSuccess: () => {
          setUsername('')
          setPassword('')
          setIsAdmin(false)
        },
      },
    )
  }

  return (
    <form onSubmit={submit} className="form">
      <h2>Add a user</h2>
      <label>
        Username
        <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="off" required />
      </label>
      <label>
        Password
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="new-password"
          minLength={8}
          required
        />
      </label>
      <label className="checkbox">
        <input type="checkbox" checked={isAdmin} onChange={(e) => setIsAdmin(e.target.checked)} /> Admin (can manage
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

function formatDate(iso: string) {
  return new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}
