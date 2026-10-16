import { useRef, useState } from 'react'
import { useSearchParams } from 'react-router'
import type { SeriesQuery } from '../api/client'
import { useLibraries, useSeriesList } from '../api/queries'
import { SeriesCard } from '../components/Cards'
import { plural } from '../format'

// The filters live in the URL, so they survive a reload and can be shared.
export function LibraryPage() {
  const [params, setParams] = useSearchParams()
  const libraries = useLibraries()
  const query: SeriesQuery = {
    library: Number(params.get('library')) || undefined,
    search: params.get('search') ?? '',
    status: (params.get('status') ?? '') as SeriesQuery['status'],
    sort: (params.get('sort') ?? '') as SeriesQuery['sort'],
  }
  const series = useSeriesList(query)
  const [search, setSearch] = useState(query.search ?? '')
  const searchTimer = useRef<ReturnType<typeof setTimeout>>(undefined)

  const set = (key: string, value: string) =>
    setParams(
      (p) => {
        if (value) p.set(key, value)
        else p.delete(key)
        return p
      },
      { replace: true },
    )

  // Search after typing stops, not on every key.
  const onSearch = (value: string) => {
    setSearch(value)
    clearTimeout(searchTimer.current)
    searchTimer.current = setTimeout(() => set('search', value.trim()), 300)
  }

  return (
    <>
      <div className="toolbar">
        <input type="search" placeholder="Search" value={search} onChange={(e) => onSearch(e.target.value)} aria-label="Search" />
        <select value={query.library ?? ''} onChange={(e) => set('library', e.target.value)} aria-label="Library">
          <option value="">All libraries</option>
          {libraries.data?.map((l) => (
            <option key={l.id} value={l.id}>
              {l.name}
            </option>
          ))}
        </select>
        <select value={query.status} onChange={(e) => set('status', e.target.value)} aria-label="Read status">
          <option value="">Any status</option>
          <option value="unread">Unread</option>
          <option value="reading">Reading</option>
          <option value="read">Read</option>
        </select>
        <select value={query.sort} onChange={(e) => set('sort', e.target.value)} aria-label="Sort">
          <option value="">Name</option>
          <option value="added">Recently added</option>
          <option value="read">Recently read</option>
        </select>
        {series.data && <span className="muted">{plural(series.data.length, 'series', 'series')}</span>}
      </div>

      {series.isPending ? (
        <p>Loading…</p>
      ) : series.isError ? (
        <p className="error">{series.error.message}</p>
      ) : series.data.length === 0 ? (
        <p>No series match these filters.</p>
      ) : (
        <div className="grid">
          {series.data.map((s) => (
            <SeriesCard key={s.id} series={s} />
          ))}
        </div>
      )}
    </>
  )
}
