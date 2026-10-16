import atkinson from '@fontsource-variable/atkinson-hyperlegible-next/wght.css?inline'
import atkinsonItalic from '@fontsource-variable/atkinson-hyperlegible-next/wght-italic.css?inline'
import garamond from '@fontsource-variable/eb-garamond/wght.css?inline'
import garamondItalic from '@fontsource-variable/eb-garamond/wght-italic.css?inline'
import literata from '@fontsource-variable/literata/wght.css?inline'
import literataItalic from '@fontsource-variable/literata/wght-italic.css?inline'
import dyslexic from '@fontsource/opendyslexic/400.css?inline'
import dyslexicItalic from '@fontsource/opendyslexic/400-italic.css?inline'
import dyslexicBold from '@fontsource/opendyslexic/700.css?inline'
import dyslexicBoldItalic from '@fontsource/opendyslexic/700-italic.css?inline'

export const fonts = {
  publisher: { label: 'Original', family: '' },
  literata: { label: 'Literata', family: "'Literata Variable', serif" },
  garamond: { label: 'Garamond', family: "'EB Garamond Variable', serif" },
  atkinson: { label: 'Atkinson', family: "'Atkinson Hyperlegible Next Variable', sans-serif" },
  dyslexic: { label: 'Dyslexic', family: "'OpenDyslexic', sans-serif" },
  serif: { label: 'System serif', family: 'Georgia, serif' },
  'sans-serif': { label: 'System sans', family: 'system-ui, sans-serif' },
}

export type FontName = keyof typeof fonts

// Books are shown from blob: URLs, where the font files' root-relative URLs
// don't resolve, so they're made absolute.
export const fontFaces = [
  literata,
  literataItalic,
  garamond,
  garamondItalic,
  atkinson,
  atkinsonItalic,
  dyslexic,
  dyslexicItalic,
  dyslexicBold,
  dyslexicBoldItalic,
]
  .join('\n')
  .replace(/url\((['"]?)(\/[^)'"]+)\1\)/g, (_, quote, path) => `url(${quote}${location.origin}${path}${quote})`)

// The menu shows each font in itself.
let added = false
export function addFontFaces() {
  if (added) return
  added = true
  const style = document.createElement('style')
  style.textContent = fontFaces
  document.head.append(style)
}
