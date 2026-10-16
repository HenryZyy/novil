import { createBrowserRouter } from 'react-router'
import { RequireAuth } from './components/RequireAuth'
import { Layout, LayoutUnlessPending } from './Layout'
import { ProfilePage } from './pages/ProfilePage'
import { AdminLayout } from './pages/admin/AdminLayout'
import { LibrariesAdmin } from './pages/admin/LibrariesAdmin'
import { Overview } from './pages/admin/Overview'
import { BooksAdmin } from './pages/admin/BooksAdmin'
import { LogsAdmin } from './pages/admin/LogsAdmin'
import { RequestsAdmin } from './pages/admin/RequestsAdmin'
import { UsersAdmin } from './pages/admin/UsersAdmin'
import { HomePage } from './pages/HomePage'
import { LibraryPage } from './pages/LibraryPage'
import { LoginPage } from './pages/LoginPage'
import { RequestsPage } from './pages/RequestsPage'
import { SeriesPage } from './pages/SeriesPage'
import { UploadPage } from './pages/UploadPage'
import { ReaderPage } from './reader/ReaderPage'

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    element: <RequireAuth allowPendingPassword />,
    children: [{ element: <LayoutUnlessPending />, children: [{ path: '/profile', element: <ProfilePage /> }] }],
  },
  {
    element: <RequireAuth />,
    children: [
      {
        element: <Layout />,
        children: [
          { path: '/', element: <HomePage /> },
          { path: '/library', element: <LibraryPage /> },
          { path: '/series/:seriesId', element: <SeriesPage /> },
          { path: '/requests', element: <RequestsPage /> },
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
                  { path: 'books', element: <BooksAdmin /> },
                  { path: 'requests', element: <RequestsAdmin /> },
                  { path: 'users', element: <UsersAdmin /> },
                  { path: 'logs', element: <LogsAdmin /> },
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
