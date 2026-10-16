import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from './client'

export const useLibraries = () => useQuery({ queryKey: ['libraries'], queryFn: api.libraries })

export const useSeriesList = (libraryId?: number) =>
  useQuery({ queryKey: ['series', 'list', libraryId ?? 'all'], queryFn: () => api.seriesList(libraryId) })

export const useSeries = (id: number) => useQuery({ queryKey: ['series', id], queryFn: () => api.series(id) })

export const useBooks = (seriesId: number | undefined) =>
  useQuery({
    queryKey: ['books', seriesId],
    queryFn: () => api.books(seriesId!),
    enabled: seriesId !== undefined,
  })

export const useBook = (id: number) => useQuery({ queryKey: ['book', id], queryFn: () => api.book(id) })

export function useCreateLibrary() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ name, path }: { name: string; path: string }) => api.createLibrary(name, path),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['libraries'] }),
  })
}

export function useUpdateLibrary() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, allowUploads }: { id: number; allowUploads: boolean }) =>
      api.updateLibrary(id, { allowUploads }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['libraries'] }),
  })
}

export function useScanLibrary() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: api.scanLibrary,
    onSuccess: () => queryClient.invalidateQueries(),
  })
}
