import { Link } from 'react-router'
import { useSeriesList } from '../api/queries'

export function HomePage() {
  const series = useSeriesList()

  if (series.isPending) return <p>Loading…</p>
  if (series.isError) return <p>Error: {series.error.message}</p>

  return (
    <>
      <h1>Series</h1>
      {series.data.length === 0 ? (
        <p>
          Nothing here yet. Add and scan a library on the <Link to="/libraries">Libraries</Link> page.
        </p>
      ) : (
        <ul>
          {series.data.map((s) => (
            <li key={s.id}>
              <Link to={`/series/${s.id}`}>{s.name}</Link> ({s.bookCount})
            </li>
          ))}
        </ul>
      )}
    </>
  )
}
