import { useInfiniteQuery, useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { api, ApiError, type Book, type BookQuery, type BookRequest, type MetadataField, type SeriesList, type SeriesQuery } from './client'

// The logged-in user, or null when nobody is.
export const useMe = () =>
  useQuery({
    queryKey: ['me'],
    queryFn: async () => {
      try {
        return await api.me()
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) return null
        throw err
      }
    },
  })

export function useCurrentUser() {
  const user = useMe().data
  if (!user) throw new Error('useCurrentUser is only available on pages that require login')
  return user
}

export const isAdmin = (user: { role: string }) => user.role === 'admin'

// Drops everything cached for the previous user and reloads who is logged in.
export async function resetSession(queryClient: QueryClient) {
  queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== 'me' })
  await queryClient.refetchQueries({ queryKey: ['me'] })
}

export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation({ mutationFn: api.logout, onSuccess: () => resetSession(queryClient) })
}

export const useLibraries = () => useQuery({ queryKey: ['libraries'], queryFn: api.libraries })

export const useSeriesList = (query: SeriesQuery = {}) =>
  useQuery({ queryKey: ['series', 'list', query], queryFn: () => api.seriesList(query) })

// enabled false waits, e.g. until a menu that needs the series opens.
export const useSeries = (id: number, enabled = true) =>
  useQuery({ queryKey: ['series', id], queryFn: () => api.series(id), enabled })

export const useContinueReading = () => useQuery({ queryKey: ['books', 'continue'], queryFn: api.continueReading })

export const useBooks = (seriesId: number | undefined) =>
  useQuery({
    queryKey: ['books', seriesId],
    queryFn: () => api.books(seriesId!),
    enabled: seriesId !== undefined,
  })

export const useSearchBooks = (query: BookQuery) =>
  useQuery({ queryKey: ['books', 'search', query], queryFn: () => api.searchBooks(query), placeholderData: (previous) => previous })

export const useBook = (id: number) => useQuery({ queryKey: ['book', id], queryFn: () => api.book(id) })

export const useUsers = () => useQuery({ queryKey: ['users'], queryFn: api.users })

export const useStats = () => useQuery({ queryKey: ['stats'], queryFn: api.stats })

// Mutations refresh everything they could affect; the data is small enough.
function useInvalidatingMutation<T, R>(mutationFn: (vars: T) => Promise<R>) {
  const queryClient = useQueryClient()
  return useMutation({ mutationFn, onSuccess: () => queryClient.invalidateQueries() })
}

export const useCreateLibrary = () =>
  useInvalidatingMutation(({ name, path }: { name: string; path: string }) => api.createLibrary(name, path))

export const useUpdateLibrary = () =>
  useInvalidatingMutation(({ id, allowUploads }: { id: number; allowUploads: boolean }) =>
    api.updateLibrary(id, { allowUploads }),
  )

export const useScanLibrary = () => useInvalidatingMutation(api.scanLibrary)

export const useSetSeriesRead = () =>
  useInvalidatingMutation(({ id, read }: { id: number; read: boolean }) => api.setSeriesRead(id, read))

// Marks a book read, or unread by clearing its progress.
export const useSetBookRead = () =>
  useInvalidatingMutation(({ book, read }: { book: Book; read: boolean }) =>
    api.saveProgress(book.id, read ? (book.format === 'epub' ? 100 : book.pageCount) : 0, read),
  )

export const useDeleteBook = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.deleteBook,
    onSuccess: (_, id) => {
      queryClient.removeQueries({ queryKey: ['book', id] })
      return queryClient.invalidateQueries()
    },
  })
}

export const useDismissContinue = () => useInvalidatingMutation(api.dismissContinue)

export const useSetHidden = () =>
  useInvalidatingMutation(({ id, hidden }: { id: number; hidden: boolean }) => api.setHidden(id, hidden))

export const useSetSeriesHidden = () =>
  useInvalidatingMutation(({ id, hidden }: { id: number; hidden: boolean }) => api.setSeriesHidden(id, hidden))

export const useSetListed = () =>
  useInvalidatingMutation(({ id, list, listed }: { id: number; list: SeriesList; listed: boolean }) =>
    api.setListed(id, list, listed),
  )

export const useAnnotations = (bookId: number) =>
  useQuery({ queryKey: ['annotations', bookId], queryFn: () => api.annotations(bookId) })

// Only the book's annotations change, so only they are reloaded; the reader
// stays as it is.
function useAnnotationMutation<T, R>(bookId: number, mutationFn: (vars: T) => Promise<R>) {
  const queryClient = useQueryClient()
  return useMutation({ mutationFn, onSuccess: () => queryClient.invalidateQueries({ queryKey: ['annotations', bookId] }) })
}

export const useCreateAnnotation = (bookId: number) =>
  useAnnotationMutation(bookId, (a: Parameters<typeof api.createAnnotation>[1]) => api.createAnnotation(bookId, a))

export const useUpdateAnnotation = (bookId: number) =>
  useAnnotationMutation(bookId, ({ id, ...changes }: { id: number } & Parameters<typeof api.updateAnnotation>[1]) =>
    api.updateAnnotation(id, changes),
  )

export const useDeleteAnnotation = (bookId: number) => useAnnotationMutation(bookId, api.deleteAnnotation)

export const useMyRequests = () => useQuery({ queryKey: ['requests', 'mine'], queryFn: api.myRequests })

export const useRequests = (status: '' | BookRequest['status']) =>
  useQuery({ queryKey: ['requests', 'all', status], queryFn: () => api.requests(status) })

export const useRequestBook = () => useInvalidatingMutation(api.requestBook)

export const useWithdrawRequest = () => useInvalidatingMutation(api.withdrawRequest)

export const useUpdateRequest = () =>
  useInvalidatingMutation(({ id, ...changes }: { id: number; status: BookRequest['status']; bookId?: number }) =>
    api.updateRequest(id, changes),
  )

export const useDeleteRequest = () => useInvalidatingMutation(api.deleteRequest)

// Refreshed every 30 seconds while shown.
export const useReading = () => useQuery({ queryKey: ['reading'], queryFn: api.reading, refetchInterval: 30_000 })

export const useLogs = (query: { event?: string; search?: string }) =>
  useInfiniteQuery({
    queryKey: ['logs', query],
    queryFn: ({ pageParam }) => api.logs({ ...query, before: pageParam }),
    initialPageParam: 0,
    // Pages hold 100; a shorter one is the last.
    getNextPageParam: (last) => (last.length === 100 ? last[last.length - 1].id : undefined),
  })

export const useCreateUser = () => useInvalidatingMutation(api.createUser)

export const useUpdateUser = () =>
  useInvalidatingMutation(({ id, ...changes }: { id: number; username?: string; role?: 'admin' | 'user' }) =>
    api.updateUser(id, changes),
  )

export const useDeleteUserAvatar = () => useInvalidatingMutation(api.deleteUserAvatar)

export const useUpdateBook = () =>
  useInvalidatingMutation(({ id, changes }: { id: number; changes: Partial<Record<MetadataField, string | null>> }) =>
    api.updateBook(id, changes),
  )

export const useResetPassword = () =>
  useInvalidatingMutation(({ id, password }: { id: number; password: string }) => api.resetPassword(id, password))

export const useDeleteUser = () => useInvalidatingMutation(api.deleteUser)
