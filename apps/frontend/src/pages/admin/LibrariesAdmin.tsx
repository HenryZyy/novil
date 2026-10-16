import { useState, type FormEvent } from 'react'
import type { Library } from '../../api/client'
import { useCreateLibrary, useLibraries, useScanLibrary, useUpdateLibrary } from '../../api/queries'

export function LibrariesAdmin() {
  const libraries = useLibraries()

  return (
    <>
      <h1>Libraries</h1>
      {libraries.isPending ? (
        <p>Loading…</p>
      ) : libraries.isError ? (
        <p className="error">{libraries.error.message}</p>
      ) : libraries.data.length === 0 ? (
        <p>No libraries yet. Add a folder with your books below.</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Folder</th>
                <th>Uploads</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {libraries.data.map((lib) => (
                <LibraryRow key={lib.id} library={lib} />
              ))}
            </tbody>
          </table>
        </div>
      )}
      <AddLibraryForm />
    </>
  )
}

function LibraryRow({ library }: { library: Library }) {
  const scan = useScanLibrary()
  const update = useUpdateLibrary()

  return (
    <tr>
      <td>{library.name}</td>
      <td>
        <code>{library.path}</code>
      </td>
      <td>
        <label>
          <input
            type="checkbox"
            checked={update.isPending ? update.variables.allowUploads : library.allowUploads}
            disabled={update.isPending}
            onChange={(e) => update.mutate({ id: library.id, allowUploads: e.target.checked })}
          />{' '}
          Allowed
        </label>
        {update.isError && <span className="error"> {update.error.message}</span>}
      </td>
      <td>
        <button onClick={() => scan.mutate(library.id)} disabled={scan.isPending}>
          {scan.isPending ? 'Scanning…' : 'Scan'}
        </button>
        {scan.isSuccess && (
          <span className="muted">
            {' '}
            {scan.data.series} series, {scan.data.books} books
          </span>
        )}
        {scan.isError && <span className="error"> {scan.error.message}</span>}
      </td>
    </tr>
  )
}

function AddLibraryForm() {
  const create = useCreateLibrary()
  const [name, setName] = useState('')
  const [path, setPath] = useState('')

  const submit = (e: FormEvent) => {
    e.preventDefault()
    create.mutate(
      { name, path },
      {
        onSuccess: () => {
          setName('')
          setPath('')
        },
      },
    )
  }

  return (
    <form onSubmit={submit} className="form">
      <h2>Add a library</h2>
      <label>
        Name
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Manga" required />
      </label>
      <label>
        Folder
        <input value={path} onChange={(e) => setPath(e.target.value)} placeholder="/library/manga" required />
        <small className="muted">The path as the server sees it. In Docker, that's the path inside the container.</small>
      </label>
      {create.isError && <p className="error">{create.error.message}</p>}
      <button type="submit" className="primary" disabled={create.isPending}>
        Add library
      </button>
    </form>
  )
}
