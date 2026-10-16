import { Link, useSearchParams } from 'react-router'
import type { BookRequest } from '../../api/client'
import { useDeleteRequest, useRequests, useUpdateRequest } from '../../api/queries'
import { Dropdown, type Choice } from '../../components/Dropdown'
import { RequestCover, RequestStatus } from '../../components/RequestCover'
import { plural } from '../../format'

const statuses: Choice<'' | BookRequest['status']>[] = [
  { value: 'open', label: 'Open' },
  { value: 'added', label: 'Added' },
  { value: 'declined', label: 'Declined' },
  { value: '', label: 'All requests' },
]

// The books users asked for, each once however many asked, the most
// wanted first.
export function RequestsAdmin() {
  const [params, setParams] = useSearchParams()
  // Open by default; "all" in the URL is every status.
  const param = params.get('status') ?? 'open'
  const status = (param === 'all' ? '' : param) as '' | BookRequest['status']
  const requests = useRequests(status)

  return (
    <>
      <h1>Requests</h1>
      <p className="muted">Books users asked for. Upload one to add it; the people who asked see it in their requests.</p>
      <div className="toolbar">
        <Dropdown
          label="Status"
          icon="status"
          value={status}
          defaultValue="open"
          choices={statuses}
          onChange={(v) => setParams(v === 'open' ? {} : { status: v || 'all' }, { replace: true })}
        />
        {requests.data && <span className="muted toolbar-count">{plural(requests.data.length, 'request')}</span>}
      </div>
      {requests.isPending ? (
        <p>Loading…</p>
      ) : requests.isError ? (
        <p className="error">{requests.error.message}</p>
      ) : requests.data.length === 0 ? (
        <p className="muted">{status === 'open' ? 'No open requests. Users ask for books on their Requests page.' : 'None.'}</p>
      ) : (
        <ul className="request-list">
          {requests.data.map((r) => (
            <RequestRow key={r.id} request={r} />
          ))}
        </ul>
      )}
    </>
  )
}

function RequestRow({ request: r }: { request: BookRequest }) {
  const update = useUpdateRequest()
  const remove = useDeleteRequest()
  const busy = update.isPending || remove.isPending
  const upload = `/upload?${new URLSearchParams({ request: String(r.id), series: r.title })}`

  return (
    <li className="request-item">
      <RequestCover url={r.coverUrl} title={r.title} />
      <div className="request-info">
        <strong>{r.title}</strong>
        <span className="muted">{[r.authors, r.published, `ISBN ${r.isbn}`].filter(Boolean).join(' · ')}</span>
        <span className="request-meta">
          <RequestStatus status={r.status} />
          <span className="request-count" title={r.requesters?.join(', ')}>
            {plural(r.requestCount, 'request')}
          </span>
          {r.requesters && <span className="muted">by {r.requesters.join(', ')}</span>}
        </span>
      </div>
      <span className="request-actions">
        {r.status === 'open' && (
          <>
            <Link to={upload} className="button primary">
              Upload
            </Link>
            <button onClick={() => update.mutate({ id: r.id, status: 'added' })} disabled={busy} title="Mark as added without uploading here">
              Mark added
            </button>
            <button onClick={() => update.mutate({ id: r.id, status: 'declined' })} disabled={busy}>
              Decline
            </button>
          </>
        )}
        {r.status === 'added' && r.bookId && (
          <Link to={`/read/${r.bookId}`} className="button">
            Open
          </Link>
        )}
        {r.status !== 'open' && (
          <button onClick={() => update.mutate({ id: r.id, status: 'open' })} disabled={busy}>
            Reopen
          </button>
        )}
        <button
          onClick={() => confirm(`Delete the request for "${r.title}"? The people who asked won't see it any more.`) && remove.mutate(r.id)}
          disabled={busy}
        >
          Delete
        </button>
      </span>
    </li>
  )
}
