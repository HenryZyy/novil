import { Link } from 'react-router'
import { isAdmin, useContinueReading, useCurrentUser, useSeriesList } from '../api/queries'
import { BookCard, SeriesCard } from '../components/Cards'

export function HomePage() {
  const continueReading = useContinueReading()
  const recent = useSeriesList({ sort: 'added' })
  const user = useCurrentUser()

  if (recent.isPending) return <p>Loading…</p>
  if (recent.isError) return <p className="error">{recent.error.message}</p>
  if (recent.data.length === 0) {
    return (
      <p>
        Nothing here yet.{' '}
        {isAdmin(user) ? (
          <>
            Add and scan a library in the <Link to="/admin/libraries">dashboard</Link>.
          </>
        ) : (
          'Ask an admin to add a library.'
        )}
      </p>
    )
  }

  return (
    <>
      {continueReading.data && continueReading.data.length > 0 && (
        <section>
          <h2>Continue reading</h2>
          <div className="row">
            {continueReading.data.map((book) => (
              <BookCard key={book.id} book={book} showSeries />
            ))}
          </div>
        </section>
      )}
      <section>
        <h2>
          Recently added <Link to="/library?sort=added">See all</Link>
        </h2>
        <div className="row">
          {recent.data.slice(0, 12).map((s) => (
            <SeriesCard key={s.id} series={s} />
          ))}
        </div>
      </section>
    </>
  )
}
