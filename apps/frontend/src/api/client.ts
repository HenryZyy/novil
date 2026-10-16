export type Library = {
  id: number
  name: string
  path: string
  allowUploads: boolean
}

export type Series = {
  id: number
  libraryId: number
  name: string
  bookCount: number
  coverBookId: number | null
}

export type Book = {
  id: number
  seriesId: number
  title: string
  format: 'cbz' | 'epub' | 'pdf'
  size: number
  pageCount: number
  readPage: number
  completed: boolean
}

export type ScanResult = {
  series: number
  books: number
}

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: typeof init.body === 'string' ? { 'Content-Type': 'application/json' } : undefined,
  })
  const body = await res.json().catch(() => null)
  if (!res.ok) {
    throw new ApiError(res.status, body?.error ?? res.statusText)
  }
  return body as T
}

export const api = {
  libraries: () => request<Library[]>('/api/libraries'),
  createLibrary: (name: string, path: string) =>
    request<Library>('/api/libraries', { method: 'POST', body: JSON.stringify({ name, path }) }),
  updateLibrary: (id: number, changes: { allowUploads: boolean }) =>
    request<Library>(`/api/libraries/${id}`, { method: 'PATCH', body: JSON.stringify(changes) }),
  scanLibrary: (id: number) => request<ScanResult>(`/api/libraries/${id}/scan`, { method: 'POST' }),

  seriesList: (libraryId?: number) =>
    request<Series[]>(libraryId ? `/api/series?library=${libraryId}` : '/api/series'),
  series: (id: number) => request<Series>(`/api/series/${id}`),
  books: (seriesId: number) => request<Book[]>(`/api/series/${seriesId}/books`),
  book: (id: number) => request<Book>(`/api/books/${id}`),

  uploadBook: (libraryId: number, series: string, file: File) =>
    request<Book>(`/api/libraries/${libraryId}/books?${new URLSearchParams({ series, filename: file.name })}`, {
      method: 'POST',
      body: file,
    }),

  saveProgress: (bookId: number, page: number, completed: boolean) =>
    request(`/api/books/${bookId}/progress`, {
      method: 'PUT',
      body: JSON.stringify({ page, completed }),
      keepalive: true,
    }),
}

export const pageUrl = (bookId: number, page: number) => `/api/books/${bookId}/pages/${page}`
export const fileUrl = (bookId: number) => `/api/books/${bookId}/file`
