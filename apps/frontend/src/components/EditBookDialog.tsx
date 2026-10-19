import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { api, metadataFields, type Book, type IsbnBook, type MetadataField } from '../api/client'
import { bumpCover } from '../coverVersion'
import { Cover } from './Cover'
import { RequestCover } from './RequestCover'
import { useUpdateBook } from '../api/queries'

const labels: Record<MetadataField, string> = {
  title: 'Title',
  author: 'Author',
  language: 'Language',
  released: 'Release date',
  description: 'Description',
  isbn: 'ISBN',
}

// A lookup's date as novil stores dates: "2005", "2005-03" or
// "2005-03-01", from e.g. "2005-03-01" or "Mar 1, 2005".
function toReleased(published: string) {
  return published.match(/^\d{4}(-\d{2}(-\d{2})?)?/)?.[0] ?? published.match(/\b\d{4}\b/)?.[0] ?? ''
}

// Lets admins correct what a book file says about itself. Fields they
// changed can go back to the file's value.
export function EditBookDialog({ book, onClose }: { book: Book; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null)
  const update = useUpdateBook()
  const [values, setValues] = useState(() => Object.fromEntries(metadataFields.map((f) => [f, book[f]])))
  const [reset, setReset] = useState<MetadataField[]>([])
  // Set when "Use these details" took a lookup's cover too; saved with the rest.
  const [coverFrom, setCoverFrom] = useState('')
  const [coverError, setCoverError] = useState('')
  const queryClient = useQueryClient()

  useEffect(() => {
    dialog.current?.showModal()
  }, [])

  const set = (field: MetadataField, value: string) => {
    setValues((v) => ({ ...v, [field]: value }))
    setReset((r) => r.filter((f) => f !== field))
  }

  // Fills in what a lookup found, over what's there.
  const applyDetails = (found: IsbnBook, withCover: boolean) => {
    const filled: Partial<Record<MetadataField, string>> = {
      isbn: found.isbn,
      title: found.title,
      author: found.authors,
      released: toReleased(found.published),
      description: found.description,
    }
    for (const [field, value] of Object.entries(filled) as [MetadataField, string][]) if (value) set(field, value)
    setCoverFrom(withCover && found.coverUrl ? found.isbn : '')
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const changes: Partial<Record<MetadataField, string | null>> = {}
    for (const field of metadataFields) {
      if (reset.includes(field)) changes[field] = null
      else if (values[field].trim() !== book[field]) changes[field] = values[field].trim()
    }
    if (Object.keys(changes).length > 0) {
      try {
        await update.mutateAsync({ id: book.id, changes })
      } catch {
        return
      }
    }
    if (coverFrom) {
      setCoverError('')
      try {
        await api.setCoverFromIsbn(book.id, coverFrom)
        bumpCover(book.id)
        await queryClient.invalidateQueries()
      } catch (err) {
        setCoverError(`The details are saved, but the cover couldn't be set: ${(err as Error).message}`)
        setCoverFrom('')
        return
      }
    }
    onClose()
  }

  return (
    <dialog ref={dialog} className="dialog" onClose={onClose} aria-labelledby="edit-book-title">
      <form onSubmit={submit} className="form">
        <h2 id="edit-book-title">Edit details</h2>
        <p className="muted">{book.title}</p>
        <CoverField book={book} />
        <IsbnField
          value={values.isbn}
          onChange={(v) => set('isbn', v)}
          edited={book.edited.includes('isbn') && !reset.includes('isbn')}
          onReset={() => setReset((r) => [...r, 'isbn'])}
          resetting={reset.includes('isbn')}
          onUse={applyDetails}
          coverPending={Boolean(coverFrom)}
        />
        {metadataFields.filter((f) => f !== 'isbn').map((field) => {
          const edited = book.edited.includes(field) && !reset.includes(field)
          const id = `book-${field}`
          const input =
            field === 'description' ? (
              <textarea id={id} value={values[field]} onChange={(e) => set(field, e.target.value)} rows={6} />
            ) : (
              <input
                id={id}
                value={values[field]}
                onChange={(e) => set(field, e.target.value)}
                required={field === 'title'}
                placeholder={field === 'released' ? '2006, 2006-01 or 2006-01-02' : field === 'language' ? 'e.g. en, ja' : undefined}
                pattern={field === 'released' ? '\\d{4}(-\\d{2}(-\\d{2})?)?' : undefined}
              />
            )
          // The reset button sits beside the label, not in it: a label belongs
          // to the first control inside it, which would then be the button.
          return (
            <div key={field} className="field">
              <div className="field-label">
                <label htmlFor={id}>{labels[field]}</label>
                {edited && (
                  <button
                    type="button"
                    className="link-button"
                    onClick={() => setReset((r) => [...r, field])}
                    title="Use the value from the book file again"
                  >
                    Reset to file
                  </button>
                )}
                {reset.includes(field) && <span className="muted">Uses the file's value after saving</span>}
              </div>
              {input}
            </div>
          )
        })}
        {update.isError && <p className="error">{update.error.message}</p>}
        {coverError && <p className="error">{coverError}</p>}
        <p className="actions">
          <button type="submit" className="primary" disabled={update.isPending}>
            Save
          </button>
          <button type="button" onClick={onClose}>
            Cancel
          </button>
        </p>
      </form>
    </dialog>
  )
}

// The ISBN, and a lookup that fills in the other fields from it: most
// useful for PDFs, which carry no details of their own.
function IsbnField({
  value,
  onChange,
  edited,
  onReset,
  resetting,
  onUse,
  coverPending,
}: {
  value: string
  onChange: (value: string) => void
  edited: boolean
  onReset: () => void
  resetting: boolean
  onUse: (found: IsbnBook, withCover: boolean) => void
  coverPending: boolean
}) {
  const [found, setFound] = useState<IsbnBook | null>(null)
  const [withCover, setWithCover] = useState(true)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const lookUp = async () => {
    setBusy(true)
    setError('')
    setFound(null)
    try {
      const result = await api.lookupIsbn(value.trim())
      setFound(result.book)
      setWithCover(Boolean(result.book.coverUrl))
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="field">
      <div className="field-label">
        <label htmlFor="book-isbn">ISBN</label>
        {edited && (
          <button type="button" className="link-button" onClick={onReset} title="Use the value from the book file again">
            Reset to file
          </button>
        )}
        {resetting && <span className="muted">Uses the file's value after saving</span>}
      </div>
      <div className="isbn-row">
        <input
          id="book-isbn"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={(e) => {
            // Enter looks up instead of saving the form.
            if (e.key === 'Enter') {
              e.preventDefault()
              if (value.trim()) lookUp()
            }
          }}
          placeholder="e.g. 978-0-441-01359-3"
          inputMode="numeric"
          autoComplete="off"
        />
        <button type="button" onClick={lookUp} disabled={busy || !value.trim()}>
          {busy ? 'Looking up…' : 'Look up'}
        </button>
      </div>
      {error && <span className="error">{error.charAt(0).toUpperCase() + error.slice(1)}.</span>}
      {coverPending && !found && <span className="muted">The cover from the ISBN is set when you save.</span>}
      {found && (
        <div className="isbn-found">
          <RequestCover url={found.coverUrl} title={found.title} />
          <div>
            <strong>{found.title}</strong>
            <span className="muted">{[found.authors, found.published].filter(Boolean).join(' · ')}</span>
            {found.coverUrl && (
              <label className="checkbox">
                <input type="checkbox" checked={withCover} onChange={(e) => setWithCover(e.target.checked)} /> Use its cover too
              </label>
            )}
            <span className="actions">
              <button
                type="button"
                className="primary"
                onClick={() => {
                  onUse(found, withCover)
                  setFound(null)
                }}
              >
                Use these details
              </button>
              <button type="button" onClick={() => setFound(null)}>
                Not this one
              </button>
            </span>
          </div>
        </div>
      )}
    </div>
  )
}

// Changing the cover takes effect right away, separately from Save.
function CoverField({ book }: { book: Book }) {
  const queryClient = useQueryClient()
  const input = useRef<HTMLInputElement>(null)
  const [custom, setCustom] = useState(book.customCover)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const run = async (action: () => Promise<Book>) => {
    setBusy(true)
    setError('')
    try {
      const updated = await action()
      setCustom(updated.customCover)
      bumpCover(book.id)
      await queryClient.invalidateQueries()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="field cover-field">
      <span className="field-label">Cover</span>
      <div className="cover-field-row">
        <div className="cover-field-preview">
          <Cover bookId={book.id} title={book.title} />
        </div>
        <div className="cover-field-actions">
          <button type="button" onClick={() => input.current?.click()} disabled={busy}>
            {busy ? 'Saving…' : 'Upload cover'}
          </button>
          {custom && (
            <button type="button" className="link-button" onClick={() => run(() => api.deleteCover(book.id))} disabled={busy}>
              Use the cover from the file
            </button>
          )}
          <span className="muted">JPEG, PNG, GIF or WebP.</span>
          {error && <span className="error">{error}</span>}
        </div>
      </div>
      <input
        ref={input}
        type="file"
        accept="image/jpeg,image/png,image/gif,image/webp"
        hidden
        onChange={(e) => {
          const file = e.target.files?.[0]
          e.target.value = ''
          if (file) run(() => api.setCover(book.id, file))
        }}
      />
    </div>
  )
}
