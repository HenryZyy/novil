import { useSyncExternalStore } from 'react'

// Profile pictures are kept in this browser's localStorage until the backend
// can store them, so they only show on the device where they were set.

const key = (userId: number) => `novil.avatar.${userId}`
const changed = 'novil-avatar-changed'

function read(userId: number) {
  try {
    return localStorage.getItem(key(userId))
  } catch {
    return null
  }
}

function subscribe(onChange: () => void) {
  // "storage" fires when another tab changes it, our own event for this tab.
  window.addEventListener('storage', onChange)
  window.addEventListener(changed, onChange)
  return () => {
    window.removeEventListener('storage', onChange)
    window.removeEventListener(changed, onChange)
  }
}

export function useAvatar(userId: number) {
  return useSyncExternalStore(subscribe, () => read(userId))
}

export function setAvatar(userId: number, dataUrl: string | null) {
  try {
    if (dataUrl) localStorage.setItem(key(userId), dataUrl)
    else localStorage.removeItem(key(userId))
  } catch {
    throw new Error("This browser doesn't allow saving the picture.")
  }
  window.dispatchEvent(new Event(changed))
}
