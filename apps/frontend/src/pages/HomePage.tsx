import { useState } from 'react'
import { Link } from 'react-router'
import { coverUrl, type Book, type Library, type Series } from '../api/client'
import { isAdmin, useContinueReading, useCurrentUser, useLibraries, useSeriesList } from '../api/queries'
import { SeriesCard } from '../components/Cards'
import { BookMenu } from '../components/CardMenus'
import { Cover } from '../components/Cover'
import { Progress } from '../components/Progress'
import { Row } from '../components/Row'

// Laid out like Jellyfin's home screen: the libraries, what you're reading,
// your lists, then what's new.
export function HomePage() {
  const libraries = useLibraries()
  const continueReading = useContinueReading()
  const recent = useSeriesList({ sort: 'added' })
  const want = useSeriesList({ list: 'want', sort: 'listed' })
  const favorites = useSeriesList({ list: 'favorites', sort: 'listed' })
  const user = useCurrentUser()

  if (libraries.isPending) return <p>Loading…</p>
  if (libraries.isError) return <p className="error">{libraries.error.message}</p>
  if (libraries.data.length === 0) {
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
      <section>
        <h2>My libraries</h2>
        <Row>
          {libraries.data.map((library) => (
            <LibraryCard key={library.id} library={library} />
          ))}
        </Row>
      </section>

      {continueReading.data && continueReading.data.length > 0 && (
        <section>
          <h2>Continue reading</h2>
          <Row>
            {continueReading.data.map((book) => (
              <ContinueCard key={book.id} book={book} />
            ))}
          </Row>
        </section>
      )}

      <SeriesRow title="Want to read" link="/library?list=want&sort=listed" series={want.data} />
      <SeriesRow title="Favorites" link="/library?list=favorites&sort=listed" series={favorites.data} />

      {recent.data && recent.data.length > 0 && (
        <section>
          <h2>
            <Link to="/library?sort=added" className="section-link">
              Recently added
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="m9 6 6 6-6 6" />
              </svg>
            </Link>
          </h2>
          <Row>
            {recent.data.slice(0, 20).map((s) => (
              <SeriesCard key={s.id} series={s} />
            ))}
          </Row>
        </section>
      )}
    </>
  )
}

function SeriesRow({ title, link, series }: { title: string; link: string; series: Series[] | undefined }) {
  if (!series?.length) return null
  return (
    <section>
      <h2>
        <Link to={link} className="section-link">
          {title}
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="m9 6 6 6-6 6" />
          </svg>
        </Link>
      </h2>
      <Row>
        {series.slice(0, 20).map((s) => (
          <SeriesCard key={s.id} series={s} />
        ))}
      </Row>
    </section>
  )
}

function LibraryCard({ library }: { library: Library }) {
  // The cover of the library's newest series, like Jellyfin's library images.
  // If a cover can't be loaded, the next series' cover is tried.
  const latest = useSeriesList({ library: library.id, sort: 'added' })
  const candidates = latest.data?.flatMap((s) => (s.coverBookId === null ? [] : [s.coverBookId])) ?? []
  const [failed, setFailed] = useState(0)
  const cover = candidates[failed]
  return (
    <Link to={`/library?library=${library.id}`} className="library-card">
      {cover !== undefined && (
        <img key={cover} src={coverUrl(cover)} alt="" loading="lazy" onError={() => setFailed((n) => n + 1)} />
      )}
      <span>{library.name}</span>
    </Link>
  )
}

function ContinueCard({ book }: { book: Book }) {
  // EPUBs store the percentage read instead of a page.
  const total = book.format === 'epub' ? 100 : book.pageCount
  const percent = total ? Math.min(Math.round((book.readPage / total) * 100), 100) : 0
  // A single book's series has the same name, so say what it is instead.
  const name = book.title === book.seriesName ? book.format.toUpperCase() : book.title
  const detail = book.format === 'epub' ? name : `${name} · Page ${book.readPage} / ${book.pageCount}`
  return (
    <div className="card-wrap">
      <Link to={`/read/${book.id}`} className="card continue-card">
        <Cover bookId={book.id} title={book.seriesName} />
        <Progress value={percent / 100} />
        <span className="card-title" title={book.seriesName}>
          {book.seriesName}
        </span>
        <span className="muted card-subtitle" title={detail}>
          {detail}
        </span>
      </Link>
      <BookMenu book={book} />
    </div>
  )
}
