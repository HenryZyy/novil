import { Link } from 'react-router'
import type { ReadingSession } from '../../api/client'
import { useLibraries, useReading, useScanLibrary, useStats } from '../../api/queries'
import { Avatar } from '../../components/Avatar'
import { Cover } from '../../components/Cover'
import { Device } from '../../components/Device'
import { Progress } from '../../components/Progress'
import { formatSize, timeAgo } from '../../format'

export function Overview() {
  const stats = useStats()
  const libraries = useLibraries()
  const scan = useScanLibrary()

  const scanAll = async () => {
    for (const lib of libraries.data ?? []) await scan.mutateAsync(lib.id)
  }

  return (
    <>
      <h1>Dashboard</h1>
      {stats.isError && <p className="error">{stats.error.message}</p>}
      {stats.data && (
        <dl className="stats">
          <Stat label="Libraries" value={stats.data.libraries} />
          <Stat label="Series" value={stats.data.series} />
          <Stat label="Books" value={stats.data.books} />
          <Stat label="Pages" value={stats.data.pages} />
          <Stat label="Storage" value={formatSize(stats.data.size)} />
          <Stat label="Users" value={stats.data.users} />
          <Stat label="Open requests" value={stats.data.requests} to="/admin/requests" />
        </dl>
      )}
      <ReadingNow />

      <h2>Quick actions</h2>
      <p className="actions">
        <button onClick={scanAll} disabled={scan.isPending || !libraries.data?.length}>
          {scan.isPending ? 'Scanning…' : 'Scan all libraries'}
        </button>
        <Link to="/admin/libraries" className="button">
          Add a library
        </Link>
        <Link to="/admin/users" className="button">
          Add a user
        </Link>
      </p>
      {scan.isError && <p className="error">{scan.error.message}</p>}
    </>
  )
}

// Who has a reader open, refreshed every 30 seconds.
function ReadingNow() {
  const reading = useReading()
  return (
    <section className="reading-now">
      <h2>
        Reading now
        {reading.data && reading.data.length > 0 && <span className="live-dot" title="Updates by itself" />}
      </h2>
      {reading.isError ? (
        <p className="error">{reading.error.message}</p>
      ) : !reading.data ? (
        <p>Loading…</p>
      ) : reading.data.length === 0 ? (
        <p className="muted">Nobody is reading right now.</p>
      ) : (
        <ul className="reading-list">
          {reading.data.map((r) => (
            <ReadingRow key={`${r.user.id}-${r.device}-${r.startedAt}`} session={r} />
          ))}
        </ul>
      )}
    </section>
  )
}

function ReadingRow({ session: r }: { session: ReadingSession }) {
  const { book } = r
  // EPUBs store the percentage read instead of a page.
  const total = book.format === 'epub' ? 100 : book.pageCount
  const progress = book.completed ? 1 : total ? book.readPage / total : 0
  const where = book.format === 'epub' ? `${Math.round(progress * 100)}%` : `Page ${book.readPage} / ${book.pageCount}`
  return (
    <li className="reading-item">
      <Link to={`/series/${book.seriesId}`} className="reading-cover" title={book.title}>
        <Cover bookId={book.id} title={book.title} />
      </Link>
      <div className="reading-info">
        <span className="reading-user">
          <Avatar user={r.user} size={22} />
          <strong>@{r.user.username}</strong>
        </span>
        <span className="reading-book" title={`${book.seriesName} · ${book.title}`}>
          {book.title === book.seriesName ? book.title : `${book.seriesName} · ${book.title}`}
        </span>
        <span className="reading-progress">
          <Progress value={Math.max(progress, 0.001)} />
          <span className="muted">{where}</span>
        </span>
        <span className="reading-meta">
          <Device kind={r.deviceKind} name={r.device} />
          <span className="muted" title={new Date(r.lastSeen).toLocaleString()}>
            since {new Date(r.startedAt).toLocaleTimeString(undefined, { timeStyle: 'short' })} · {timeAgo(r.lastSeen)}
          </span>
        </span>
      </div>
    </li>
  )
}

function Stat({ label, value, to }: { label: string; value: number | string; to?: string }) {
  const shown = typeof value === 'number' ? value.toLocaleString() : value
  return (
    <div className={`stat${to ? ' stat-link' : ''}`}>
      <dt>{label}</dt>
      <dd>{to ? <Link to={to}>{shown}</Link> : shown}</dd>
    </div>
  )
}

