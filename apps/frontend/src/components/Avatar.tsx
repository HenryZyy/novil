import { useAvatar } from '../avatar'

export function Avatar({ user, size = 32 }: { user: { id: number; username: string }; size?: number }) {
  const picture = useAvatar(user.id)
  const style = { width: size, height: size, fontSize: size * 0.45 }
  if (picture) return <img className="avatar" src={picture} alt="" style={style} />
  return (
    <span className="avatar avatar-empty" style={style} aria-hidden="true">
      {user.username.charAt(0).toUpperCase()}
    </span>
  )
}
