import { useSyncExternalStore } from 'react'

export type Theme = 'system' | 'light' | 'dark'

export const themes: { value: Theme; label: string }[] = [
  { value: 'system', label: 'System' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
]

// Must match the script in index.html, which applies the theme before the
// page draws so it doesn't flash.
const storageKey = 'novil.theme'
const changed = 'novil-theme-changed'

function read(): Theme {
  try {
    const saved = localStorage.getItem(storageKey)
    if (saved === 'light' || saved === 'dark') return saved
  } catch {
    // Blocked storage: follow the system.
  }
  return 'system'
}

function subscribe(onChange: () => void) {
  window.addEventListener('storage', onChange)
  window.addEventListener(changed, onChange)
  return () => {
    window.removeEventListener('storage', onChange)
    window.removeEventListener(changed, onChange)
  }
}

export function useTheme() {
  return useSyncExternalStore(subscribe, read)
}

export function setTheme(theme: Theme) {
  try {
    if (theme === 'system') localStorage.removeItem(storageKey)
    else localStorage.setItem(storageKey, theme)
  } catch {
    // The choice just isn't remembered then.
  }
  if (theme === 'system') delete document.documentElement.dataset.theme
  else document.documentElement.dataset.theme = theme
  window.dispatchEvent(new Event(changed))
}

const darkQuery = '(prefers-color-scheme: dark)'

function subscribeSystem(onChange: () => void) {
  const query = matchMedia(darkQuery)
  query.addEventListener('change', onChange)
  return () => query.removeEventListener('change', onChange)
}

// Whether the site is dark right now, following the system when the theme
// is "system", also when the system switches while the page is open.
export function useIsDark() {
  const theme = useTheme()
  const systemDark = useSyncExternalStore(subscribeSystem, () => matchMedia(darkQuery).matches)
  return theme === 'dark' || (theme === 'system' && systemDark)
}
