import { useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import type { Book, BookQuery } from '../../api/client'
import { useLibraries, useSearchBooks, useSetHidden, useSetSeriesHidden } from '../../api/queries'
import { Cover } from '../../components/Cover'
import { Dropdown, type Choice } from '../../components/Dropdown'
import { DeleteBookDialog } from '../../components/DeleteBookDialog'
import { EditBookDialog } from '../../components/EditBookDialog'
import { Icon } from '../../components/Icon'
import { formatSize, plural } from '../../format'

const visibilities: Choice<NonNullable<BookQuery['hidden']>>[] = [
  { value: '', label: 'All books' },
  { value: 'visible', label: 'Visible' },
  { value: 'hidden', label: 'Hidden' },
]

// Every book of every library, to find books and choose who sees them.
// The filters live in the URL, like the library page's.
export function BooksAdmin() {
  const [params, setParams] = useSearchParams()
  const libraries = useLibraries()
  const query: BookQuery = {
    library: Number(params.get('library')) || undefined,
    search: params.get('search') ?? '',
    hidden: (params.get('hidden') ?? '') as BookQuery['hidden'],
  }
  const books = useSearchBooks(query)
  const [search, setSearch] = useState(query.search ?? '')
  const searchTimer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const [editing, setEditing] = useState<Book | null>(null)
  const [deleting, setDeleting] = useState<Book | null>(null)

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
    clearTimeout(searchTimer.current)
    searchTimer.current = setTimeout(() => set('search', value.trim()), 300)
  }

  // The books come sorted by series, so each series is one run of them.
  const groups: Book[][] = []
  for (const book of books.data ?? []) {
    const last = groups.at(-1)
    if (last?.[0].seriesId === book.seriesId) last.push(book)
    else groups.push([book])
  }
  const filtered = Boolean(query.search || query.library || query.hidden)

  return (
    <>
      <h1>Books</h1>
      <p className="muted">Hidden books are only shown to admins. Everyone else can't find, open or download them.</p>
      <div className="toolbar">
        <label className="search">
          <Icon name="search" size={18} />
          <input
            type="search"
            placeholder="Search title, author or series"
            value={search}
            onChange={(e) => onSearch(e.target.value)}
            aria-label="Search"
          />
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
          label="Visibility"
          icon="eye"
          value={query.hidden ?? ''}
          defaultValue=""
          choices={visibilities}
          onChange={(v) => set('hidden', v)}
        />
        {books.data && <span className="muted toolbar-count">{plural(books.data.length, 'book')}</span>}
      </div>

      {books.isPending ? (
        <p>Loading…</p>
      ) : books.isError ? (
        <p className="error">{books.error.message}</p>
      ) : groups.length === 0 ? (
        <p className="muted">{filtered ? 'No books match.' : 'No books yet. Add a library or upload some.'}</p>
      ) : (
        <div className="book-admin">
          {groups.map((group) => (
            <SeriesGroup key={group[0].seriesId} books={group} onEdit={setEditing} onDelete={setDeleting} />
          ))}
        </div>
      )}
      {editing && <EditBookDialog key={editing.id} book={editing} onClose={() => setEditing(null)} />}
      {deleting && <DeleteBookDialog key={deleting.id} book={deleting} onClose={() => setDeleting(null)} />}
    </>
  )
}

function SeriesGroup({
  books,
  onEdit,
  onDelete,
}: {
  books: Book[]
  onEdit: (book: Book) => void
  onDelete: (book: Book) => void
}) {
  const setSeriesHidden = useSetSeriesHidden()
  const { seriesId, seriesName } = books[0]
  const hidden = books.filter((b) => b.hidden).length
  const allHidden = hidden === books.length

  return (
    <section className="book-admin-series">
      <header>
        <Link to={`/series/${seriesId}`}>{seriesName}</Link>
        <span className="muted">
          {plural(books.length, 'book')}
          {hidden > 0 && !allHidden && ` · ${hidden} hidden`}
        </span>
        <button
          className="visibility-button"
          onClick={() => setSeriesHidden.mutate({ id: seriesId, hidden: !allHidden })}
          disabled={setSeriesHidden.isPending}
          title={allHidden ? 'Show every book of this series' : 'Hide every book of this series'}
          aria-label={allHidden ? 'Show series' : 'Hide series'}
        >
          <Icon name={allHidden ? 'eye' : 'eyeOff'} size={16} />
          <span>{allHidden ? 'Show series' : 'Hide series'}</span>
        </button>
      </header>
      <ul>
        {books.map((book) => (
          <BookRow key={book.id} book={book} onEdit={() => onEdit(book)} onDelete={() => onDelete(book)} />
        ))}
      </ul>
    </section>
  )
}

function BookRow({ book, onEdit, onDelete }: { book: Book; onEdit: () => void; onDelete: () => void }) {
  const setHidden = useSetHidden()
  // Shows the new state while it's being saved.
  const hidden = setHidden.isPending ? setHidden.variables.hidden : book.hidden
  const details = [book.author, book.format.toUpperCase(), formatSize(book.size)].filter(Boolean).join(' · ')

  return (
    <li className={hidden ? 'is-hidden' : ''}>
      <Cover bookId={book.id} title={book.title} />
      <div className="book-admin-info">
        <strong title={book.title}>{book.title}</strong>
        <span className="muted">{details}</span>
      </div>
      <button className="book-admin-edit" onClick={onEdit} aria-label={`Edit ${book.title}`} title="Edit details">
        <Icon name="edit" size={18} />
      </button>
      <button className="book-admin-edit danger" onClick={onDelete} aria-label={`Delete ${book.title}`} title="Delete">
        <Icon name="trash" size={18} />
      </button>
      <button
        className="visibility-switch"
        role="switch"
        aria-checked={!hidden}
        aria-label={`${book.title} visible to users`}
        onClick={() => setHidden.mutate({ id: book.id, hidden: !hidden })}
        disabled={setHidden.isPending}
      >
        <Icon name={hidden ? 'eyeOff' : 'eye'} size={16} />
        <span>{hidden ? 'Hidden' : 'Visible'}</span>
      </button>
    </li>
  )
}
