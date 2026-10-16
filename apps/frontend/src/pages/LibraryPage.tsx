import { useRef, useState } from 'react'
import { useSearchParams } from 'react-router'
import type { SeriesQuery, SeriesSort } from '../api/client'
import { useLibraries, useSeriesList } from '../api/queries'
import { SeriesCard } from '../components/Cards'
import { Dropdown, type Choice } from '../components/Dropdown'
import { Icon } from '../components/Icon'
import { plural } from '../format'

const statuses: Choice<NonNullable<SeriesQuery['status']>>[] = [
  { value: '', label: 'Any status' },
  { value: 'unread', label: 'Unread' },
  { value: 'reading', label: 'Reading' },
  { value: 'read', label: 'Read' },
]

const sorts: Choice<SeriesSort>[] = [
  { value: '', label: 'Name, A to Z' },
  { value: 'name-desc', label: 'Name, Z to A' },
  { value: 'added', label: 'Recently added' },
  { value: 'added-asc', label: 'Oldest added' },
  { value: 'released', label: 'Newest release' },
  { value: 'released-asc', label: 'Oldest release' },
  { value: 'read', label: 'Recently read' },
]

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
        <label className="search">
          <Icon name="search" size={18} />
          <input type="search" placeholder="Search series" value={search} onChange={(e) => onSearch(e.target.value)} aria-label="Search" />
        </label>
        <Dropdown
          label="Library"
          icon="library"
          value={String(query.library ?? '')}
          defaultValue=""
          choices={[{ value: '', label: 'All libraries' }, ...(libraries.data ?? []).map((l) => ({ value: String(l.id), label: l.name }))]}
          onChange={(v) => set('library', v)}
        />
        <Dropdown
          label="Status"
          icon="status"
          value={query.status ?? ''}
          defaultValue=""
          choices={statuses}
          onChange={(v) => set('status', v)}
        />
        <Dropdown label="Sort by" icon="sort" iconOnly value={query.sort ?? ''} defaultValue="" choices={sorts} onChange={(v) => set('sort', v)} />
        {series.data && <span className="muted toolbar-count">{plural(series.data.length, 'series', 'series')}</span>}
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
