import { useEffect, useRef, useState, type ReactNode } from 'react'

// Asks before something that can't be undone. onConfirm may be async; the
// dialog stays open, with its error, if it fails.
export function ConfirmDialog({
  title,
  children,
  confirmLabel,
  onConfirm,
  onClose,
}: {
  title: string
  children: ReactNode
  confirmLabel: string
  onConfirm: () => Promise<unknown>
  onClose: () => void
}) {
  const dialog = useRef<HTMLDialogElement>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    dialog.current?.showModal()
  }, [])

  const confirm = async () => {
    setBusy(true)
    setError('')
    try {
      await onConfirm()
      onClose()
    } catch (err) {
      setError((err as Error).message)
      setBusy(false)
    }
  }

  return (
    <dialog ref={dialog} className="dialog confirm-dialog" onClose={onClose} aria-labelledby="confirm-title">
      <h2 id="confirm-title">{title}</h2>
      <div className="confirm-body">{children}</div>
      {error && <p className="error">{error}</p>}
      <p className="actions">
        {/* Cancel first, so Enter on the open dialog doesn't delete. */}
        <button onClick={onClose} autoFocus>
          Cancel
        </button>
        <button className="danger-button" onClick={confirm} disabled={busy}>
          {busy ? 'Deleting…' : confirmLabel}
        </button>
      </p>
    </dialog>
  )
}
