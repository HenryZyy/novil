import { useState, type FormEvent } from 'react'
import { Link } from 'react-router'
import { ApiError, api, requestCoverUrl, type BookRequest, type IsbnBook } from '../api/client'
import { useMyRequests, useRequestBook, useWithdrawRequest } from '../api/queries'
import { Icon } from '../components/Icon'
import { RequestCover, RequestStatus } from '../components/RequestCover'
import { plural } from '../format'

// Ask for a book by ISBN, and see what became of the books you asked for.
export function RequestsPage() {
  const mine = useMyRequests()

  return (
    <>
      <section className="request-intro">
        <h1>Request a book</h1>
        <p className="muted">
          Missing a book? Enter its ISBN, found on the back cover or the copyright page. An admin will add it to the library.
        </p>
        <RequestForm />
      </section>

      <section>
        <h2>My requests</h2>
        {mine.isPending ? (
          <p>Loading…</p>
        ) : mine.isError ? (
          <p className="error">{mine.error.message}</p>
        ) : mine.data.length === 0 ? (
          <p className="muted">You haven't requested any books yet.</p>
        ) : (
          <ul className="request-list">
            {mine.data.map((r) => (
              <MyRequest key={r.id} request={r} />
            ))}
          </ul>
        )}
      </section>
    </>
  )
}

function RequestForm() {
  const [isbn, setIsbn] = useState('')
  const [found, setFound] = useState<{ book: IsbnBook; request: BookRequest | null } | null>(null)
  const [error, setError] = useState('')
  const [looking, setLooking] = useState(false)
  const [done, setDone] = useState<BookRequest | null>(null)
  const requestBook = useRequestBook()

  const lookUp = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setFound(null)
    setDone(null)
    setLooking(true)
    try {
      setFound(await api.lookupIsbn(isbn.trim()))
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLooking(false)
    }
  }

  const request = () =>
    found &&
    requestBook.mutate(found.book.isbn, {
      onSuccess: (r) => {
        setDone(r)
        setFound(null)
        setIsbn('')
      },
      onError: (err) => setError(err instanceof ApiError ? err.message : String(err)),
    })

  const existing = found?.request
  return (
    <>
      <form className="isbn-form" onSubmit={lookUp}>
        <label className="search">
          <Icon name="search" size={18} />
          <input
            value={isbn}
            onChange={(e) => setIsbn(e.target.value)}
            placeholder="ISBN, e.g. 978-0-06-294150-3"
            aria-label="ISBN"
            inputMode="numeric"
            autoComplete="off"
            required
          />
        </label>
        <button className="primary" disabled={looking || !isbn.trim()}>
          {looking ? 'Looking up…' : 'Look up'}
        </button>
      </form>
      {error && <p className="error">{capitalize(error)}.</p>}
      {done && (
        <p className="notice">
          <Icon name="check" size={18} />
          <span>
            Requested <strong>{done.title}</strong>.
            {done.requestCount > 1 && ` ${plural(done.requestCount - 1, 'other person', 'other people')} asked for it too.`}
          </span>
        </p>
      )}
      {found && (
        <div className="isbn-result">
          {/* Already requested: novil has the cover. Else the database's, for a look. */}
          <RequestCover url={found.request ? requestCoverUrl(found.request) : found.book.coverUrl} title={found.book.title} />
          <div>
            <h3>{found.book.title}</h3>
            <p className="muted">{[found.book.authors, found.book.published].filter(Boolean).join(' · ')}</p>
            <p className="muted small">ISBN {found.book.isbn}{found.book.pages > 0 && ` · ${plural(found.book.pages, 'page')}`}</p>
            {existing?.requestedByMe ? (
              <p className="muted">You already requested this book.</p>
            ) : existing?.status === 'added' ? (
              <p>
                It's already in the library.{' '}
                {existing.bookId && <Link to={`/read/${existing.bookId}`}>Read it</Link>}
              </p>
            ) : (
              <>
                {existing && (
                  <p className="muted">{plural(existing.requestCount, 'person', 'people')} asked for it already. Requesting it too shows the admins it's wanted.</p>
                )}
                <button className="primary" onClick={request} disabled={requestBook.isPending}>
                  Request this book
                </button>
              </>
            )}
          </div>
        </div>
      )}
    </>
  )
}

function MyRequest({ request: r }: { request: BookRequest }) {
  const withdraw = useWithdrawRequest()
  return (
    <li className="request-item">
      <RequestCover url={requestCoverUrl(r)} title={r.title} />
      <div className="request-info">
        <strong>{r.title}</strong>
        <span className="muted">{[r.authors, r.published].filter(Boolean).join(' · ')}</span>
        <span className="request-meta">
          <RequestStatus status={r.status} />
          {r.status === 'open' && r.requestCount > 1 && <span className="muted">{plural(r.requestCount, 'person', 'people')} want it</span>}
          {r.requestedAt && <span className="muted">since {new Date(r.requestedAt).toLocaleDateString(undefined, { dateStyle: 'medium' })}</span>}
        </span>
      </div>
      {r.status === 'added' && r.bookId ? (
        <Link to={`/read/${r.bookId}`} className="button primary">
          Read
        </Link>
      ) : (
        r.status === 'open' && (
          <button onClick={() => withdraw.mutate(r.id)} disabled={withdraw.isPending}>
            Withdraw
          </button>
        )
      )}
    </li>
  )
}

const capitalize = (s: string) => s.charAt(0).toUpperCase() + s.slice(1)
