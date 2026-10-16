import { createBrowserRouter } from 'react-router'
import { RequireAuth } from './components/RequireAuth'
import { Layout } from './Layout'
import { AdminLayout } from './pages/admin/AdminLayout'
import { LibrariesAdmin } from './pages/admin/LibrariesAdmin'
import { Overview } from './pages/admin/Overview'
import { UsersAdmin } from './pages/admin/UsersAdmin'
import { HomePage } from './pages/HomePage'
import { LibraryPage } from './pages/LibraryPage'
import { LoginPage } from './pages/LoginPage'
import { SeriesPage } from './pages/SeriesPage'
import { UploadPage } from './pages/UploadPage'
import { ReaderPage } from './reader/ReaderPage'

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  { path: '/setup', element: <LoginPage setup /> },
  {
    element: <RequireAuth />,
    children: [
      {
        element: <Layout />,
        children: [
          { path: '/', element: <HomePage /> },
          { path: '/library', element: <LibraryPage /> },
          { path: '/series/:seriesId', element: <SeriesPage /> },
          {
            element: <RequireAuth admin />,
            children: [
              { path: '/upload', element: <UploadPage /> },
              {
                path: '/admin',
                element: <AdminLayout />,
                children: [
                  { index: true, element: <Overview /> },
                  { path: 'libraries', element: <LibrariesAdmin /> },
                  { path: 'users', element: <UsersAdmin /> },
                ],
              },
            ],
          },
          { path: '*', element: <p>Page not found.</p> },
        ],
      },
      { path: '/read/:bookId', element: <ReaderPage /> },
    ],
  },
])
