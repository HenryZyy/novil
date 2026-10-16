import { useState, type FormEvent } from 'react'
import type { Library } from '../api/client'
import { useCreateLibrary, useLibraries, useScanLibrary, useUpdateLibrary } from '../api/queries'

export function LibrariesPage() {
  const libraries = useLibraries()

  return (
    <>
      <h1>Libraries</h1>
      {libraries.isPending ? (
        <p>Loading…</p>
      ) : libraries.isError ? (
        <p>Error: {libraries.error.message}</p>
      ) : (
        <ul>
          {libraries.data.map((lib) => (
            <LibraryRow key={lib.id} library={lib} />
          ))}
        </ul>
      )}
      <AddLibraryForm />
    </>
  )
}

function LibraryRow({ library }: { library: Library }) {
  const scan = useScanLibrary()
  const update = useUpdateLibrary()

  return (
    <li>
      {library.name} ({library.path}){' '}
      <button onClick={() => scan.mutate(library.id)} disabled={scan.isPending}>
        {scan.isPending ? 'Scanning…' : 'Scan'}
      </button>{' '}
      <label>
        <input
          type="checkbox"
          checked={update.isPending ? update.variables.allowUploads : library.allowUploads}
          disabled={update.isPending}
          onChange={(e) => update.mutate({ id: library.id, allowUploads: e.target.checked })}
        />
        Allow uploads
      </label>
      {scan.isSuccess && ` Found ${scan.data.series} series, ${scan.data.books} books.`}
      {scan.isError && ` Error: ${scan.error.message}`}
      {update.isError && ` Error: ${update.error.message}`}
    </li>
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
    <form onSubmit={submit}>
      <h2>Add a library</h2>
      <p>
        <label>
          Name <input value={name} onChange={(e) => setName(e.target.value)} required />
        </label>
      </p>
      <p>
        <label>
          Folder <input value={path} onChange={(e) => setPath(e.target.value)} placeholder="/library" required />
        </label>
      </p>
      {create.isError && <p>Error: {create.error.message}</p>}
      <button type="submit" disabled={create.isPending}>
        Add
      </button>
    </form>
  )
}
