import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Icon } from './Icon'

// A row of cards that scrolls sideways, with arrow buttons for mouse users
// like Jellyfin's. The arrows only show when there's more to see that way.
export function Row({ children, scrollTo }: { children: ReactNode; scrollTo?: string }) {
  const ref = useRef<HTMLDivElement>(null)
  const [canLeft, setCanLeft] = useState(false)
  const [canRight, setCanRight] = useState(false)

  useEffect(() => {
    const row = ref.current
    if (!row) return
    const update = () => {
      setCanLeft(row.scrollLeft > 4)
      setCanRight(row.scrollLeft + row.clientWidth < row.scrollWidth - 4)
    }
    update()
    row.addEventListener('scroll', update, { passive: true })
    const observer = new ResizeObserver(update)
    observer.observe(row)
    return () => {
      row.removeEventListener('scroll', update)
      observer.disconnect()
    }
  }, [])

  // Start at a given card, e.g. the book to continue with.
  useEffect(() => {
    if (!scrollTo) return
    const card = ref.current?.querySelector<HTMLElement>(`[data-row-id="${scrollTo}"]`)
    if (card && ref.current) ref.current.scrollLeft = card.offsetLeft - ref.current.offsetLeft - 8
  }, [scrollTo])

  const scroll = (direction: 1 | -1) => {
    const row = ref.current
    if (row) row.scrollBy({ left: direction * row.clientWidth * 0.8, behavior: 'smooth' })
  }

  return (
    <div className="row-wrap">
      <div className="row" ref={ref}>
        {children}
      </div>
      {canLeft && (
        <button className="row-arrow left" onClick={() => scroll(-1)} aria-label="Scroll left">
          <Icon name="chevronLeft" size={22} />
        </button>
      )}
      {canRight && (
        <button className="row-arrow right" onClick={() => scroll(1)} aria-label="Scroll right">
          <Icon name="chevronRight" size={22} />
        </button>
      )}
    </div>
  )
}
