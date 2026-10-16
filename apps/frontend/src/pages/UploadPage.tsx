import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { api } from '../api/client'
import { useLibraries } from '../api/queries'

export function UploadPage() {
  const libraries = useLibraries()
  const [params] = useSearchParams()

  if (libraries.isPending) return <p>Loading…</p>
  if (libraries.isError) return <p>Error: {libraries.error.message}</p>

  const uploadable = libraries.data.filter((l) => l.allowUploads)
  if (uploadable.length === 0) {
    return (
      <p>
        Turn on "Allow uploads" for a library on the <Link to="/libraries">Libraries</Link> page first.
      </p>
    )
  }

  const requested = uploadable.find((l) => l.id === Number(params.get('library')))
  return <UploadForm libraries={uploadable} initialLibraryId={(requested ?? uploadable[0]).id} initialSeries={params.get('series') ?? ''} />
}

function UploadForm({
  libraries,
  initialLibraryId,
  initialSeries,
}: {
  libraries: { id: number; name: string }[]
  initialLibraryId: number
  initialSeries: string
}) {
  const queryClient = useQueryClient()
  const [libraryId, setLibraryId] = useState(initialLibraryId)
  const [series, setSeries] = useState(initialSeries)
  const [files, setFiles] = useState<File[]>([])
  const [results, setResults] = useState<string[]>([])
  const [uploading, setUploading] = useState(false)

  const upload = async () => {
    setUploading(true)
    setResults([])
    for (const file of files) {
      try {
        await api.uploadBook(libraryId, series.trim(), file)
        setResults((r) => [...r, `${file.name}: done`])
      } catch (err) {
        setResults((r) => [...r, `${file.name}: ${(err as Error).message}`])
      }
    }
    setUploading(false)
    queryClient.invalidateQueries()
  }

  return (
    <>
      <h1>Upload</h1>
      <p>
        <label>
          Library{' '}
          <select value={libraryId} onChange={(e) => setLibraryId(Number(e.target.value))}>
            {libraries.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
        </label>
      </p>
      <p>
        <label>
          Series <input value={series} onChange={(e) => setSeries(e.target.value)} />
        </label>
      </p>
      <p>
        <input type="file" multiple accept=".cbz,.epub,.pdf" onChange={(e) => setFiles(Array.from(e.target.files ?? []))} />
      </p>
      <button onClick={upload} disabled={uploading || files.length === 0 || !series.trim()}>
        {uploading ? 'Uploading…' : 'Upload'}
      </button>
      <ul>
        {results.map((r, i) => (
          <li key={i}>{r}</li>
        ))}
      </ul>
    </>
  )
}
