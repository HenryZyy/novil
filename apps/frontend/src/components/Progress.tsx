export function Progress({ value }: { value: number }) {
  if (value <= 0) return null
  return (
    <div className="progress" role="progressbar" aria-valuenow={Math.round(value * 100)}>
      <div style={{ width: `${Math.min(value, 1) * 100}%` }} />
    </div>
  )
}
