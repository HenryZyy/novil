import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import { api } from '../api/client'
import { useLibraries } from '../api/queries'

export function UploadPage() {
  const libraries = useLibraries()
  const [params] = useSearchParams()

  if (libraries.isPending) return <p>Loading…</p>
  if (libraries.isError) return <p className="error">{libraries.error.message}</p>

  const uploadable = libraries.data.filter((l) => l.allowUploads)
  if (uploadable.length === 0) {
    return (
      <p>
        Turn on uploads for a library in the <Link to="/admin/libraries">dashboard</Link> first.
      </p>
    )
  }

  const requested = uploadable.find((l) => l.id === Number(params.get('library')))
  return (
    <UploadForm
      libraries={uploadable}
      initialLibraryId={(requested ?? uploadable[0]).id}
      initialSeries={params.get('series') ?? ''}
      requestId={Number(params.get('request')) || undefined}
    />
  )
}

function UploadForm({
  libraries,
  initialLibraryId,
  initialSeries,
  requestId,
}: {
  libraries: { id: number; name: string }[]
  initialLibraryId: number
  initialSeries: string
  // A book request this upload answers; it's marked as added with the book.
  requestId?: number
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
    let first: number | undefined
    for (const file of files) {
      try {
        const book = await api.uploadBook(libraryId, series.trim(), file)
        first ??= book.id
        setResults((r) => [...r, `${file.name}: done`])
      } catch (err) {
        setResults((r) => [...r, `${file.name}: ${(err as Error).message}`])
      }
    }
    if (requestId && first !== undefined) {
      try {
        await api.updateRequest(requestId, { status: 'added', bookId: first })
        setResults((r) => [...r, 'The request is marked as added; the people who asked can read it now.'])
      } catch (err) {
        setResults((r) => [...r, `Couldn't mark the request as added: ${(err as Error).message}`])
      }
    }
    setUploading(false)
    queryClient.invalidateQueries()
  }

  return (
    <div className="form">
      <h1>Upload</h1>
      {requestId && <p className="notice">Uploading for a book request. When it's done, the request is marked as added.</p>}
      <label>
        Library
        <select value={libraryId} onChange={(e) => setLibraryId(Number(e.target.value))}>
          {libraries.map((l) => (
            <option key={l.id} value={l.id}>
              {l.name}
            </option>
          ))}
        </select>
      </label>
      <label>
        Series
        <input value={series} onChange={(e) => setSeries(e.target.value)} placeholder="New or existing series name" />
      </label>
      <label>
        Files
        <input type="file" multiple accept=".cbz,.epub,.pdf" onChange={(e) => setFiles(Array.from(e.target.files ?? []))} />
      </label>
      <button className="primary" onClick={upload} disabled={uploading || files.length === 0 || !series.trim()}>
        {uploading ? 'Uploading…' : 'Upload'}
      </button>
      <ul>
        {results.map((r, i) => (
          <li key={i}>{r}</li>
        ))}
      </ul>
    </div>
  )
}
