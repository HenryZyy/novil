import { Link } from 'react-router'
import { useLibraries, useScanLibrary, useStats } from '../../api/queries'

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
        </dl>
      )}
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

function Stat({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="stat">
      <dt>{label}</dt>
      <dd>{typeof value === 'number' ? value.toLocaleString() : value}</dd>
    </div>
  )
}

function formatSize(bytes: number) {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (bytes >= 1024 && i < units.length - 1) {
    bytes /= 1024
    i++
  }
  return `${bytes.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}
