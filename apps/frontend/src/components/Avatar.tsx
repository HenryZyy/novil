import { useState } from 'react'
import { avatarUrl } from '../avatar'

export function Avatar({
  user,
  size = 32,
}: {
  user: { id: number; username: string; hasAvatar?: boolean; avatarUpdatedAt?: string }
  size?: number
}) {
  const url = avatarUrl(user)
  const [failed, setFailed] = useState<string | null>(null)
  const style = { width: size, height: size, fontSize: size * 0.45 }
  if (url && failed !== url) return <img className="avatar" src={url} alt="" style={style} onError={() => setFailed(url)} />
  return (
    <span className="avatar avatar-empty" style={style} aria-hidden="true">
      {user.username.charAt(0).toUpperCase()}
    </span>
  )
}
