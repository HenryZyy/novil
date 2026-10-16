import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { api, ApiError, type SeriesQuery } from './client'

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

export const useSeries = (id: number) => useQuery({ queryKey: ['series', id], queryFn: () => api.series(id) })

export const useContinueReading = () => useQuery({ queryKey: ['books', 'continue'], queryFn: api.continueReading })

export const useBooks = (seriesId: number | undefined) =>
  useQuery({
    queryKey: ['books', seriesId],
    queryFn: () => api.books(seriesId!),
    enabled: seriesId !== undefined,
  })

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

export const useCreateUser = () => useInvalidatingMutation(api.createUser)

export const useResetPassword = () =>
  useInvalidatingMutation(({ id, password }: { id: number; password: string }) => api.resetPassword(id, password))

export const useDeleteUser = () => useInvalidatingMutation(api.deleteUser)
