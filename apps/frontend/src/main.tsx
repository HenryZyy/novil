import { MutationCache, QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router'
import { ApiError } from './api/client'
import './index.css'
import { router } from './router'

// A 401 means the session ended, and password_change_required that an
// admin reset the password. Re-checking who is logged in lets the router
// send the user to the right page.
const onError = (error: Error) => {
  if (error instanceof ApiError && (error.status === 401 || error.code === 'password_change_required')) {
    queryClient.invalidateQueries({ queryKey: ['me'] })
  }
}

const queryClient = new QueryClient({
  queryCache: new QueryCache({ onError }),
  mutationCache: new MutationCache({ onError }),
  defaultOptions: { queries: { retry: false } },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
