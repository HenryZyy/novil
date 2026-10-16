import { useEffect, useMemo, useRef, useState } from 'react'
import { collapse, compare } from 'foliate-js/epubcfi.js'
import { Overlayer } from 'foliate-js/overlayer.js'
import type { DrawAnnotationDetail, FoliateView, Location, ShowAnnotationDetail } from 'foliate-js/view.js'
import type { Annotation, Book, HighlightColor } from '../api/client'
import { useAnnotations, useCreateAnnotation, useDeleteAnnotation, useUpdateAnnotation } from '../api/queries'
import { HighlightPopup, type PopupRect } from './Annotations'
import { colorValues } from './highlightColors'


// The popup over selected text or a clicked highlight.
type Popup = {
  rect: PopupRect
  // A new selection has a range but no annotation yet.
  annotation?: Annotation
  selection?: { cfi: string; text: string; doc: Document }
  editingNote?: boolean
}

// Where a range in the book's iframe is on the screen.
function screenRect(range: Range): PopupRect {
  const r = range.getBoundingClientRect()
  const frame = range.startContainer.ownerDocument?.defaultView?.frameElement?.getBoundingClientRect()
  const left = (frame?.left ?? 0) + r.left
  const top = (frame?.top ?? 0) + r.top
  return { left, top, bottom: top + r.height, width: r.width }
}

const excerpt = (text: string, length = 160) => {
  const t = text.trim().replace(/\s+/g, ' ')
  return t.length > length ? t.slice(0, length).trimEnd() + '…' : t
}

// Bookmarks and highlights for the EPUB reader. attach() hooks into the view
// once it is created; popup is the element to render.
export function useEpubAnnotations(book: Book, location: Location | null, percent: number) {
  const annotations = useAnnotations(book.id)
  const create = useCreateAnnotation(book.id)
  const update = useUpdateAnnotation(book.id)
  const remove = useDeleteAnnotation(book.id)
  const [popup, setPopup] = useState<Popup | null>(null)
  const viewRef = useRef<FoliateView | null>(null)
  const list = useMemo(() => annotations.data ?? [], [annotations.data])
  const highlights = useMemo(() => list.filter((a) => a.kind === 'highlight'), [list])
  // Read by the view's events, which are attached once.
  const highlightsRef = useRef(highlights)
  // What is drawn, CFI to colour, to tell what changed.
  const drawn = useRef(new Map<string, string>())

  useEffect(() => {
    highlightsRef.current = highlights
    const view = viewRef.current
    // Until the book is open, create-overlay draws them.
    if (!view?.book) return
    const wanted = new Map(highlights.map((a) => [a.position, a.color || 'yellow']))
    for (const value of drawn.current.keys()) if (!wanted.has(value)) view.deleteAnnotation({ value })
    for (const [value, color] of wanted) if (drawn.current.get(value) !== color) view.addAnnotation({ value, color })
    drawn.current = wanted
  }, [highlights])

  const attach = (view: FoliateView) => {
    viewRef.current = view
    // Each section gets its highlights when it's shown.
    view.addEventListener('create-overlay', () => {
      for (const a of highlightsRef.current) {
        const color = a.color || 'yellow'
        view.addAnnotation({ value: a.position, color })
        drawn.current.set(a.position, color)
      }
    })
    view.addEventListener('draw-annotation', (e) => {
      const { draw, annotation } = (e as CustomEvent<DrawAnnotationDetail>).detail
      draw(Overlayer.highlight, { color: colorValues[(annotation.color || 'yellow') as HighlightColor] })
    })
    view.addEventListener('show-annotation', (e) => {
      const { value, range } = (e as CustomEvent<ShowAnnotationDetail>).detail
      const annotation = highlightsRef.current.find((a) => a.position === value)
      if (annotation) setPopup({ rect: screenRect(range), annotation })
    })
    view.addEventListener('relocate', () => setPopup((p) => (p?.editingNote ? p : null)))
    view.addEventListener('load', (e) => {
      const { doc, index } = (e as CustomEvent<{ doc: Document; index: number }>).detail
      let timer: ReturnType<typeof setTimeout> | undefined
      const check = () => {
        const selection = doc.getSelection()
        if (!selection || selection.isCollapsed || !selection.rangeCount) {
          // A popup for a selection goes away with it.
          setPopup((p) => (p?.selection && !p.editingNote ? null : p))
          return
        }
        const range = selection.getRangeAt(0)
        const text = range.toString().trim()
        if (!text) return
        setPopup({ rect: screenRect(range), selection: { cfi: view.getCFI(index, range), text, doc } })
      }
      // Touch screens select without a pointerup, so selectionchange too,
      // once the selection stops changing.
      doc.addEventListener('pointerup', () => setTimeout(check, 10))
      doc.addEventListener('selectionchange', () => {
        clearTimeout(timer)
        timer = setTimeout(check, 500)
      })
      // Clicking elsewhere closes a highlight's popup; clicking a highlight
      // opens it again right after.
      doc.addEventListener('pointerdown', () => setPopup((p) => (p?.annotation && !p.editingNote ? null : p)))
    })
  }

  const bookmarks = list.filter((a) => a.kind === 'bookmark')
  const bookmark = location
    ? bookmarks.find((b) => compare(b.position, collapse(location.cfi)) >= 0 && compare(b.position, collapse(location.cfi, true)) <= 0)
    : undefined

  const toggleBookmark = () => {
    if (!location) return
    if (bookmark) remove.mutate(bookmark.id)
    else
      create.mutate({
        kind: 'bookmark',
        page: percent,
        position: collapse(location.cfi),
        text: excerpt(location.range?.toString() ?? ''),
      })
  }

  const close = () => {
    popup?.selection?.doc.getSelection()?.removeAllRanges()
    setPopup(null)
  }

  const highlight = async (color: HighlightColor, note = '') => {
    if (!popup?.selection) return undefined
    const { cfi, text } = popup.selection
    return create.mutateAsync({ kind: 'highlight', page: percent, position: cfi, text: excerpt(text, 2000), color, note })
  }

  const popupElement = popup && (
    <HighlightPopup
      popup={popup}
      onColor={async (color) => {
        if (popup.annotation) update.mutate({ id: popup.annotation.id, color })
        else await highlight(color)
        close()
      }}
      onNote={() => setPopup({ ...popup, editingNote: true })}
      onSaveNote={async (note) => {
        if (popup.annotation) update.mutate({ id: popup.annotation.id, note })
        else await highlight('yellow', note)
        close()
      }}
      onDelete={() => {
        if (popup.annotation) remove.mutate(popup.annotation.id)
        close()
      }}
      onClose={close}
    />
  )

  return { annotations: list, bookmark, toggleBookmark, attach, popup: popupElement }
}
