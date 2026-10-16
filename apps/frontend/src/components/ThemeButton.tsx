import { setTheme, useIsDark } from '../theme'
import { Icon } from './Icon'

// Switches between light and dark. "Follow the system" is on the profile page.
export function ThemeButton() {
  const dark = useIsDark()
  const label = dark ? 'Switch to light mode' : 'Switch to dark mode'
  return (
    <button className="theme-button" onClick={() => setTheme(dark ? 'light' : 'dark')} title={label} aria-label={label}>
      <Icon name={dark ? 'sun' : 'moon'} size={18} />
    </button>
  )
}
