import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { highlightColors, type Annotation, type HighlightColor } from '../api/client'
import { useDeleteAnnotation, useUpdateAnnotation } from '../api/queries'
import { Icon } from '../components/Icon'
import { colorValues } from './highlightColors'
import { MenuSection } from './MenuParts'

export function ColorSwatches({ value, onPick }: { value?: string; onPick: (color: HighlightColor) => void }) {
  return (
    <span className="swatches" role="radiogroup" aria-label="Highlight colour">
      {highlightColors.map((color) => (
        <button
          key={color}
          role="radio"
          aria-checked={value === color}
          aria-label={color}
          title={color[0].toUpperCase() + color.slice(1)}
          style={{ background: colorValues[color] }}
          onClick={() => onPick(color)}
        />
      ))}
    </span>
  )
}

export function NoteEditor({
  initial,
  onSave,
  onCancel,
}: {
  initial: string
  onSave: (note: string) => void
  onCancel: () => void
}) {
  const [note, setNote] = useState(initial)
  return (
    <div className="note-editor">
      <textarea
        value={note}
        onChange={(e) => setNote(e.target.value)}
        placeholder="Write a note…"
        rows={3}
        maxLength={10000}
        autoFocus
        aria-label="Note"
        onKeyDown={(e) => {
          // Keys typed here must not turn pages or close the menu.
          e.stopPropagation()
          if (e.key === 'Escape') onCancel()
          if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) onSave(note.trim())
        }}
      />
      <span className="note-editor-actions">
        <button onClick={onCancel}>Cancel</button>
        <button className="primary" onClick={() => onSave(note.trim())}>
          Save
        </button>
      </span>
    </div>
  )
}

// The menu's list of a book's bookmarks and highlights, in reading order.
export function AnnotationList({
  bookId,
  annotations,
  where,
  onGo,
  canHighlight = false,
  children,
}: {
  bookId: number
  annotations: Annotation[]
  canHighlight?: boolean
  // Where an annotation is, e.g. "Page 3" or "12%".
  where: (a: Annotation) => string
  onGo: (a: Annotation) => void
  // Shown above the list, like a button to bookmark the current page.
  children?: ReactNode
}) {
  return (
    <MenuSection title={`Bookmarks & notes${annotations.length ? ` · ${annotations.length}` : ''}`}>
      {children}
      {annotations.length === 0 ? (
        <p className="annotation-empty">
          Nothing yet. Bookmark a page with <Icon name="bookmark" size={14} />
          {canHighlight && ' or select text to highlight it'}.
        </p>
      ) : (
        <ul className="annotation-list">
          {annotations.map((a) => (
            <AnnotationItem key={a.id} bookId={bookId} annotation={a} where={where(a)} onGo={() => onGo(a)} />
          ))}
        </ul>
      )}
    </MenuSection>
  )
}

function AnnotationItem({
  bookId,
  annotation: a,
  where,
  onGo,
}: {
  bookId: number
  annotation: Annotation
  where: string
  onGo: () => void
}) {
  const update = useUpdateAnnotation(bookId)
  const remove = useDeleteAnnotation(bookId)
  const [editing, setEditing] = useState(false)
  const highlight = a.kind === 'highlight'

  return (
    <li className="annotation-item">
      <button className="annotation-go" onClick={onGo} title="Go there">
        <span className="annotation-where">
          {highlight ? (
            <span className="color-dot" style={{ background: colorValues[a.color || 'yellow'] }} />
          ) : (
            <Icon name="bookmark" size={14} filled />
          )}
          {highlight ? 'Highlight' : 'Bookmark'} · {where}
        </span>
        {a.text && <span className={highlight ? 'annotation-quote' : 'annotation-excerpt'}>{a.text}</span>}
      </button>
      {editing ? (
        <NoteEditor
          initial={a.note}
          onCancel={() => setEditing(false)}
          onSave={(note) => {
            update.mutate({ id: a.id, note })
            setEditing(false)
          }}
        />
      ) : (
        a.note && <p className="annotation-note">{a.note}</p>
      )}
      {!editing && (
        <span className="annotation-actions">
          <button className="reader-icon" onClick={() => setEditing(true)} aria-label={a.note ? 'Edit note' : 'Add note'} title={a.note ? 'Edit note' : 'Add note'}>
            <Icon name="note" size={16} />
          </button>
          <button
            className="reader-icon"
            onClick={() => remove.mutate(a.id)}
            disabled={remove.isPending}
            aria-label={`Delete ${a.kind}`}
            title="Delete"
          >
            <Icon name="trash" size={16} />
          </button>
        </span>
      )}
    </li>
  )
}

// Where the text a popup belongs to is on the screen.
export type PopupRect = { left: number; top: number; bottom: number; width: number }

// The popup over selected text or a clicked highlight.
export function HighlightPopup({
  popup,
  onColor,
  onNote,
  onSaveNote,
  onDelete,
  onClose,
}: {
  popup: { rect: PopupRect; annotation?: Annotation; editingNote?: boolean }
  onColor: (color: HighlightColor) => void
  onNote: () => void
  onSaveNote: (note: string) => void
  onDelete: () => void
  onClose: () => void
}) {
  const ref = useRef<HTMLDivElement>(null)
  const [position, setPosition] = useState<{ left: number; top: number } | null>(null)
  const { annotation, editingNote, rect } = popup

  // Above the text, or below it when there's no room; always on screen.
  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const margin = 8
    const left = Math.min(Math.max(rect.left + rect.width / 2 - el.offsetWidth / 2, margin), innerWidth - el.offsetWidth - margin)
    const above = rect.top - el.offsetHeight - margin
    const top = above > 56 ? above : Math.min(rect.bottom + margin, innerHeight - el.offsetHeight - margin)
    setPosition({ left, top })
  }, [rect, editingNote])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div
      ref={ref}
      className={`highlight-popup${editingNote ? ' editing' : ''}`}
      style={position ?? { visibility: 'hidden', left: 0, top: 0 }}
      role="dialog"
      aria-label={annotation ? 'Highlight' : 'Highlight selection'}
    >
      {editingNote ? (
        <NoteEditor initial={annotation?.note ?? ''} onSave={onSaveNote} onCancel={onClose} />
      ) : (
        <>
          <ColorSwatches value={annotation?.color} onPick={onColor} />
          <span className="highlight-popup-divider" />
          <button className="reader-icon" onClick={onNote} aria-label={annotation?.note ? 'Edit note' : 'Add note'} title={annotation?.note ? 'Edit note' : 'Add note'}>
            <Icon name="note" size={18} />
          </button>
          {annotation && (
            <button className="reader-icon" onClick={onDelete} aria-label="Delete highlight" title="Delete highlight">
              <Icon name="trash" size={18} />
            </button>
          )}
        </>
      )}
      {!editingNote && annotation?.note && <p className="highlight-popup-note">{annotation.note}</p>}
    </div>
  )
}
