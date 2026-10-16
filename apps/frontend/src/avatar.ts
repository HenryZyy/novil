// Profile pictures live on the server, so every device shows the same one.

type AvatarUser = { id: number; hasAvatar?: boolean; avatarUpdatedAt?: string }

export const avatarUrl = (user: AvatarUser) =>
  user.hasAvatar ? `/api/users/${user.id}/avatar?v=${encodeURIComponent(user.avatarUpdatedAt ?? '')}` : null

// Pictures set before the server stored them were kept in this browser.
// Returns such a picture (as a data URL) once and forgets it.
export function takeLocalAvatar(userId: number): string | null {
  try {
    const key = `novil.avatar.${userId}`
    const picture = localStorage.getItem(key)
    localStorage.removeItem(key)
    return picture
  } catch {
    return null
  }
}
