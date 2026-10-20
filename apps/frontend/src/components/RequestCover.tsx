import { useState } from 'react'

// A requested book's cover: novil's saved copy, or for a lookup not yet
// requested, the book database's.
export function RequestCover({ url, title }: { url: string; title: string }) {
  const [failed, setFailed] = useState(false)
  if (!url || failed) return <div className="cover cover-empty">{title.charAt(0).toUpperCase()}</div>
  return <img className="cover" src={url} alt="" loading="lazy" referrerPolicy="no-referrer" onError={() => setFailed(true)} />
}

const statusLabels = { open: 'Requested', added: 'Added', declined: 'Declined' }

export function RequestStatus({ status }: { status: keyof typeof statusLabels }) {
  return <span className={`request-status ${status}`}>{statusLabels[status]}</span>
}
