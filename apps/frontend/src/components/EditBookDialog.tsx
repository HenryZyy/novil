import { useEffect, useRef, useState, type FormEvent } from 'react'
import { metadataFields, type Book, type MetadataField } from '../api/client'
import { useUpdateBook } from '../api/queries'

const labels: Record<MetadataField, string> = {
  title: 'Title',
  author: 'Author',
  language: 'Language',
  released: 'Release date',
  description: 'Description',
}

// Lets admins correct what a book file says about itself. Fields they
// changed can go back to the file's value.
export function EditBookDialog({ book, onClose }: { book: Book; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null)
  const update = useUpdateBook()
  const [values, setValues] = useState(() => Object.fromEntries(metadataFields.map((f) => [f, book[f]])))
  const [reset, setReset] = useState<MetadataField[]>([])

  useEffect(() => {
    dialog.current?.showModal()
  }, [])

  const set = (field: MetadataField, value: string) => {
    setValues((v) => ({ ...v, [field]: value }))
    setReset((r) => r.filter((f) => f !== field))
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const changes: Partial<Record<MetadataField, string | null>> = {}
    for (const field of metadataFields) {
      if (reset.includes(field)) changes[field] = null
      else if (values[field].trim() !== book[field]) changes[field] = values[field].trim()
    }
    if (Object.keys(changes).length === 0) return onClose()
    update.mutate({ id: book.id, changes }, { onSuccess: onClose })
  }

  return (
    <dialog ref={dialog} className="dialog" onClose={onClose} aria-labelledby="edit-book-title">
      <form onSubmit={submit} className="form">
        <h2 id="edit-book-title">Edit details</h2>
        <p className="muted">{book.title}</p>
        {metadataFields.map((field) => {
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
