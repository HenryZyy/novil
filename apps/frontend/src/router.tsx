import { createBrowserRouter } from 'react-router'
import { Layout } from './Layout'
import { HomePage } from './pages/HomePage'
import { LibrariesPage } from './pages/LibrariesPage'
import { SeriesPage } from './pages/SeriesPage'
import { UploadPage } from './pages/UploadPage'
import { ReaderPage } from './reader/ReaderPage'

export const router = createBrowserRouter([
  {
    element: <Layout />,
    children: [
      { path: '/', element: <HomePage /> },
      { path: '/libraries', element: <LibrariesPage /> },
      { path: '/series/:seriesId', element: <SeriesPage /> },
      { path: '/upload', element: <UploadPage /> },
      { path: '*', element: <p>Page not found.</p> },
    ],
  },
  { path: '/read/:bookId', element: <ReaderPage /> },
])
