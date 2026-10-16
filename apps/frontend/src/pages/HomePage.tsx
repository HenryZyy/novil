import { useState } from 'react'
import { Link } from 'react-router'
import { coverUrl, hasCover, type Book, type Library } from '../api/client'
import { isAdmin, useContinueReading, useCurrentUser, useLibraries, useSeriesList } from '../api/queries'
import { SeriesCard } from '../components/Cards'
import { Cover } from '../components/Cover'
import { Row } from '../components/Row'

// Laid out like Jellyfin's home screen: the libraries, what you're reading,
// then what's new.
export function HomePage() {
  const libraries = useLibraries()
  const continueReading = useContinueReading()
  const recent = useSeriesList({ sort: 'added' })
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

function LibraryCard({ library }: { library: Library }) {
  // The cover of the library's newest series, like Jellyfin's library images.
  const latest = useSeriesList({ library: library.id, sort: 'added' })
  const cover = latest.data?.find((s) => s.coverBookId !== null)?.coverBookId
  const [broken, setBroken] = useState(false)
  return (
    <Link to={`/library?library=${library.id}`} className="library-card">
      {cover != null && !broken && <img src={coverUrl(cover)} alt="" loading="lazy" onError={() => setBroken(true)} />}
      <span>{library.name}</span>
    </Link>
  )
}

function ContinueCard({ book }: { book: Book }) {
  // EPUBs store the percentage read instead of a page.
  const total = book.format === 'epub' ? 100 : book.pageCount
  const progress = total ? book.readPage / total : 0
  const detail =
    book.format === 'epub' ? `${book.title} · ${book.readPage}%` : `${book.title} · Page ${book.readPage} / ${book.pageCount}`
  return (
    <Link to={`/read/${book.id}`} className="card continue-card">
      <div className="continue-thumb">
        <Cover bookId={hasCover(book) ? book.id : null} title={book.seriesName} />
        <div className="continue-progress">
          <div style={{ width: `${Math.max(progress, 0.02) * 100}%` }} />
        </div>
      </div>
      <span className="card-title" title={book.seriesName}>
        {book.seriesName}
      </span>
      <span className="muted card-subtitle" title={detail}>
        {detail}
      </span>
    </Link>
  )
}
