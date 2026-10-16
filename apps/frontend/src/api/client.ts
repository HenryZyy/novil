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
  readCount: number
  readingCount: number
  addedAt: string
  lastReadAt: string | null
}

export type SeriesQuery = {
  library?: number
  search?: string
  status?: '' | 'unread' | 'reading' | 'read'
  sort?: '' | 'added' | 'read'
}

export type User = {
  id: number
  username: string
  role: 'admin' | 'user'
  // New accounts and admin resets must pick their own password first.
  mustChangePassword: boolean
  createdAt: string
}

export type Stats = {
  libraries: number
  series: number
  books: number
  pages: number
  size: number
  users: number
}

export type Book = {
  id: number
  seriesId: number
  seriesName: string
  title: string
  format: 'cbz' | 'epub' | 'pdf'
  size: number
  pageCount: number
  readPage: number
  readPosition: string
  completed: boolean
  // When the book was uploaded or first found by a scan.
  addedAt: string
  // From the file, or set by an admin; edited lists the fields an admin set.
  author: string
  language: string
  // "2006", "2006-01" or "2006-01-02", or empty.
  released: string
  description: string
  edited: MetadataField[]
}

export const metadataFields = ['title', 'author', 'language', 'released', 'description'] as const
export type MetadataField = (typeof metadataFields)[number]

export type ScanResult = {
  series: number
  books: number
}

export class ApiError extends Error {
  status: number
  // Machine-readable, e.g. "password_change_required".
  code?: string

  constructor(status: number, message: string, code?: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: typeof init.body === 'string' ? { 'Content-Type': 'application/json' } : undefined,
  })
  if (res.status === 204) return undefined as T
  const body = await res.json().catch(() => null)
  if (!res.ok) {
    throw new ApiError(res.status, body?.error ?? res.statusText, body?.code)
  }
  return body as T
}

const json = (method: string, body: unknown): RequestInit => ({ method, body: JSON.stringify(body) })

export const api = {
  // The server also sets the token as a cookie, which is all the browser
  // needs, so the token in the response isn't kept.
  me: () => request<User>('/api/me'),
  login: (username: string, password: string) =>
    request<{ user: User }>('/api/auth/login', json('POST', { username, password })),
  changePassword: (currentPassword: string, newPassword: string) =>
    request<{ user: User }>('/api/me/password', json('PUT', { currentPassword, newPassword })),
  logout: () => request<void>('/api/auth/logout', { method: 'POST' }),

  users: () => request<User[]>('/api/users'),
  createUser: (user: { username: string; password: string; role: User['role'] }) =>
    request<User>('/api/users', json('POST', user)),
  resetPassword: (id: number, password: string) => request<User>(`/api/users/${id}/password`, json('PUT', { password })),
  deleteUser: (id: number) => request<void>(`/api/users/${id}`, { method: 'DELETE' }),
  stats: () => request<Stats>('/api/stats'),

  libraries: () => request<Library[]>('/api/libraries'),
  createLibrary: (name: string, path: string) => request<Library>('/api/libraries', json('POST', { name, path })),
  updateLibrary: (id: number, changes: { allowUploads: boolean }) =>
    request<Library>(`/api/libraries/${id}`, json('PATCH', changes)),
  scanLibrary: (id: number) => request<ScanResult>(`/api/libraries/${id}/scan`, { method: 'POST' }),

  seriesList: (query: SeriesQuery = {}) => {
    const params = new URLSearchParams()
    for (const [key, value] of Object.entries(query)) if (value) params.set(key, String(value))
    return request<Series[]>(`/api/series?${params}`)
  },
  series: (id: number) => request<Series>(`/api/series/${id}`),
  setSeriesRead: (id: number, read: boolean) => request<Series>(`/api/series/${id}/read`, json('PUT', { read })),
  continueReading: () => request<Book[]>('/api/continue'),
  books: (seriesId: number) => request<Book[]>(`/api/series/${seriesId}/books`),
  book: (id: number) => request<Book>(`/api/books/${id}`),
  // A string replaces the file's value, null goes back to it.
  updateBook: (id: number, changes: Partial<Record<MetadataField, string | null>>) =>
    request<Book>(`/api/books/${id}`, json('PATCH', changes)),

  uploadBook: (libraryId: number, series: string, file: File) =>
    request<Book>(`/api/libraries/${libraryId}/books?${new URLSearchParams({ series, filename: file.name })}`, {
      method: 'POST',
      body: file,
    }),

  saveProgress: (bookId: number, page: number, completed: boolean, position = '') =>
    request(`/api/books/${bookId}/progress`, {
      method: 'PUT',
      body: JSON.stringify({ page, completed, position }),
      keepalive: true,
    }),
}

export const isReadable = (book: Book) => book.format === 'cbz' || book.format === 'epub'

export const pageUrl = (bookId: number, page: number) => `/api/books/${bookId}/pages/${page}`
export const fileUrl = (bookId: number) => `/api/books/${bookId}/file`
