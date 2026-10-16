import { useState } from 'react'
import { coverUrl } from '../api/client'

export function Cover({ bookId, title }: { bookId: number | null; title: string }) {
  const [failed, setFailed] = useState(false)
  if (bookId === null || failed) {
    return <div className="cover cover-empty">{title.charAt(0).toUpperCase()}</div>
  }
  return <img className="cover" src={coverUrl(bookId)} alt="" loading="lazy" onError={() => setFailed(true)} />
}
