import { useRef, useState } from 'react'
import { useSearchParams } from 'react-router'
import type { LogEntry, LogEvent } from '../../api/client'
import { useLogs } from '../../api/queries'
import { Device } from '../../components/Device'
import { Dropdown, type Choice } from '../../components/Dropdown'
import { Icon } from '../../components/Icon'
import { timeAgo } from '../../format'

const labels: Record<LogEvent, string> = {
  login: 'Logged in',
  login_failed: 'Failed login',
  logout: 'Logged out',
  password_changed: 'Changed password',
  user_created: 'Created user',
  user_updated: 'Changed user',
  user_deleted: 'Deleted user',
  password_reset: 'Reset password',
  book_uploaded: 'Uploaded book',
  library_scanned: 'Scanned library',
  request_updated: 'Answered request',
}

// The filter's groups of events, as sent to the API.
const filters: Choice<string>[] = [
  { value: '', label: 'Everything' },
  { value: 'login,logout', label: 'Logins and logouts' },
  { value: 'login_failed', label: 'Failed logins' },
  { value: 'password_changed,user_created,user_updated,user_deleted,password_reset', label: 'Accounts' },
  { value: 'book_uploaded,library_scanned,request_updated', label: 'Library' },
]

// What happened, newest first: logins, account changes and admin actions,
// kept for 90 days.
export function LogsAdmin() {
  const [params, setParams] = useSearchParams()
  const event = params.get('event') ?? ''
  const query = { event, search: params.get('search') ?? '' }
  const logs = useLogs(query)
  const [search, setSearch] = useState(query.search)
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)

  const set = (key: string, value: string) =>
    setParams(
      (p) => {
        if (value) p.set(key, value)
        else p.delete(key)
        return p
      },
      { replace: true },
    )

  const onSearch = (value: string) => {
    setSearch(value)
    clearTimeout(timer.current)
    timer.current = setTimeout(() => set('search', value.trim()), 300)
  }

  const entries = logs.data?.pages.flat() ?? []
  return (
    <>
      <h1>Logs</h1>
      <p className="muted">Logins, account changes and admin actions of the last 90 days.</p>
      <div className="toolbar">
        <label className="search">
          <Icon name="search" size={18} />
          <input type="search" placeholder="Search user, IP, device or detail" value={search} onChange={(e) => onSearch(e.target.value)} aria-label="Search" />
        </label>
        <Dropdown label="Show" icon="log" value={event} defaultValue="" choices={filters} onChange={(v) => set('event', v)} />
      </div>
      {logs.isPending ? (
        <p>Loading…</p>
      ) : logs.isError ? (
        <p className="error">{logs.error.message}</p>
      ) : entries.length === 0 ? (
        <p className="muted">Nothing logged{query.search || event ? ' that matches' : ' yet'}.</p>
      ) : (
        <>
          <div className="table-wrap">
            <table className="log-table">
              <thead>
                <tr>
                  <th>When</th>
                  <th>What</th>
                  <th>User</th>
                  <th>Device</th>
                  <th>IP</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((e) => (
                  <LogRow key={e.id} entry={e} />
                ))}
              </tbody>
            </table>
          </div>
          {logs.hasNextPage && (
            <p>
              <button onClick={() => logs.fetchNextPage()} disabled={logs.isFetchingNextPage}>
                {logs.isFetchingNextPage ? 'Loading…' : 'Load older'}
              </button>
            </p>
          )}
        </>
      )}
    </>
  )
}

function LogRow({ entry: e }: { entry: LogEntry }) {
  const at = new Date(e.at)
  return (
    <tr className={e.event === 'login_failed' ? 'log-failed' : undefined}>
      <td title={at.toLocaleString()}>
        <span className="log-when">
          <span>{timeAgo(e.at)}</span>
          <span className="muted">{at.toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' })}</span>
        </span>
      </td>
      <td>
        <span className={`log-event ${e.event}`}>{labels[e.event] ?? e.event}</span>
        {e.detail && <span className="log-detail">{e.detail}</span>}
      </td>
      <td>{e.username ? <span className={e.userId === null ? 'muted' : undefined}>@{e.username}</span> : <span className="muted">—</span>}</td>
      <td>
        <Device kind={e.deviceKind} name={e.device} />
      </td>
      <td>
        <code>{e.ip}</code>
      </td>
    </tr>
  )
}
