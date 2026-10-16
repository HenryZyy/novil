import { useState } from 'react'
import { coverUrl } from '../api/client'
import { useCoverVersion } from '../coverVersion'

export function Cover({ bookId, title }: { bookId: number | null; title: string }) {
  const version = useCoverVersion(bookId)
  const url = bookId === null ? null : coverUrl(bookId, version)
  // Remembers which URL failed, so a new version is tried again.
  const [failed, setFailed] = useState<string | null>(null)
  if (url === null || failed === url) {
    return <div className="cover cover-empty">{title.charAt(0).toUpperCase()}</div>
  }
  return <img className="cover" src={url} alt="" loading="lazy" onError={() => setFailed(url)} />
}
