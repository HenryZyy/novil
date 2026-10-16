export function plural(n: number, word: string, plural = word + 's') {
  return `${n} ${n === 1 ? word : plural}`
}
