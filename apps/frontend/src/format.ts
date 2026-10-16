export function formatSize(bytes: number) {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (bytes >= 1024 && i < units.length - 1) {
    bytes /= 1024
    i++
  }
  return `${bytes.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export function plural(n: number, word: string, plural = word + 's') {
  return `${n} ${n === 1 ? word : plural}`
}
