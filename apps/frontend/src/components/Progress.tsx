// How much was read: a bar under the cover with the percentage at its end.
// The same everywhere, so a book shows the same progress on every page.
export function Progress({ value }: { value: number }) {
  if (value <= 0) return null
  const percent = Math.min(Math.round(value * 100), 100)
  return (
    <div className="progress" role="progressbar" aria-valuenow={percent} aria-label="Read">
      <div className="progress-track">
        <div style={{ width: `${Math.max(percent, 2)}%` }} />
      </div>
      <span>{percent}%</span>
    </div>
  )
}
