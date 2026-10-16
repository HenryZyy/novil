import { useState } from 'react'

export type ReaderSettings = {
  layout: 'single' | 'double' | 'strip'
  fit: 'both' | 'width' | 'height' | 'none'
  direction: 'ltr' | 'rtl'
  header: boolean
  progress: 'normal' | 'light' | 'hidden'
  pinned: boolean
}

type Choices<K extends keyof ReaderSettings> = [ReaderSettings[K], string][]

export const layouts: Choices<'layout'> = [
  ['single', 'Single page'],
  ['double', 'Double page'],
  ['strip', 'Long strip'],
]
export const fits: Choices<'fit'> = [
  ['both', 'Both'],
  ['width', 'Width'],
  ['height', 'Height'],
  ['none', 'No limit'],
]
export const directions: Choices<'direction'> = [
  ['ltr', 'Left to right'],
  ['rtl', 'Right to left'],
]
export const progressStyles: Choices<'progress'> = [
  ['normal', 'Normal'],
  ['light', 'Thin'],
  ['hidden', 'Hidden'],
]

const defaults: ReaderSettings = {
  layout: 'single',
  fit: 'both',
  direction: 'ltr',
  header: true,
  progress: 'normal',
  pinned: false,
}

const storageKey = 'novil.reader'

function pick<K extends keyof ReaderSettings>(choices: Choices<K>, value: unknown, fallback: ReaderSettings[K]) {
  return choices.some(([v]) => v === value) ? (value as ReaderSettings[K]) : fallback
}

function load(): ReaderSettings {
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey) ?? '{}')
    return {
      layout: pick(layouts, saved.layout, defaults.layout),
      fit: pick(fits, saved.fit, defaults.fit),
      direction: pick(directions, saved.direction, defaults.direction),
      header: typeof saved.header === 'boolean' ? saved.header : defaults.header,
      progress: pick(progressStyles, saved.progress, defaults.progress),
      pinned: typeof saved.pinned === 'boolean' ? saved.pinned : defaults.pinned,
    }
  } catch {
    // Blocked storage or broken JSON: start from the defaults.
    return defaults
  }
}

export function useReaderSettings() {
  const [settings, setSettings] = useState(load)
  const update = (changes: Partial<ReaderSettings>) =>
    setSettings((current) => {
      const next = { ...current, ...changes }
      try {
        localStorage.setItem(storageKey, JSON.stringify(next))
      } catch {
        // The settings just aren't remembered then.
      }
      return next
    })
  return [settings, update] as const
}

// The choice after current, wrapping around, like MangaDex's toggle buttons.
export function nextChoice<T>(choices: [T, string][], current: T) {
  const i = choices.findIndex(([v]) => v === current)
  return choices[(i + 1) % choices.length][0]
}

export function label<T>(choices: [T, string][], current: T) {
  return choices.find(([v]) => v === current)?.[1] ?? ''
}
