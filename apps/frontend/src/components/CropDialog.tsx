import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent, type WheelEvent } from 'react'

const outputSize = 256
const maxZoom = 4

type Crop = { zoom: number; x: number; y: number }

// Lets the user pick which part of an image becomes their round profile
// picture: drag to move, wheel, pinch or the slider to zoom.
export function CropDialog({
  file,
  onSave,
  onCancel,
}: {
  file: File
  onSave: (dataUrl: string) => void
  onCancel: () => void
}) {
  const dialog = useRef<HTMLDialogElement>(null)
  const frame = useRef<HTMLDivElement>(null)
  const [image, setImage] = useState<HTMLImageElement | null>(null)
  const [error, setError] = useState('')
  const [frameSize, setFrameSize] = useState(280)
  // x and y move the image's centre away from the frame's, in screen pixels.
  const [crop, setCrop] = useState<Crop>({ zoom: 1, x: 0, y: 0 })
  const pointers = useRef(new Map<number, { x: number; y: number }>())
  const pinch = useRef<number | null>(null)

  useEffect(() => {
    dialog.current?.showModal()
    const url = URL.createObjectURL(file)
    const img = new Image()
    img.onload = () => setImage(img)
    img.onerror = () => setError("This image can't be read.")
    img.src = url
    return () => URL.revokeObjectURL(url)
  }, [file])

  useEffect(() => {
    const el = frame.current
    if (!el) return
    const observer = new ResizeObserver(() => setFrameSize(el.clientWidth))
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  // At zoom 1 the image's shorter side just fills the frame.
  const base = image ? frameSize / Math.min(image.naturalWidth, image.naturalHeight) : 1

  // Keeps the image covering the whole frame, so the picture has no gaps.
  const clamp = (c: Crop): Crop => {
    if (!image) return c
    const zoom = Math.min(Math.max(c.zoom, 1), maxZoom)
    const k = base * zoom
    const maxX = (image.naturalWidth * k - frameSize) / 2
    const maxY = (image.naturalHeight * k - frameSize) / 2
    return { zoom, x: Math.min(Math.max(c.x, -maxX), maxX), y: Math.min(Math.max(c.y, -maxY), maxY) }
  }
  const zoomTo = (zoom: number) => setCrop((c) => clamp({ ...c, zoom, x: (c.x * zoom) / c.zoom, y: (c.y * zoom) / c.zoom }))
  const move = (dx: number, dy: number) => setCrop((c) => clamp({ ...c, x: c.x + dx, y: c.y + dy }))

  const onPointerDown = (e: PointerEvent<HTMLDivElement>) => {
    e.currentTarget.setPointerCapture(e.pointerId)
    pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY })
  }
  const onPointerMove = (e: PointerEvent<HTMLDivElement>) => {
    const last = pointers.current.get(e.pointerId)
    if (!last) return
    pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY })
    const all = [...pointers.current.values()]
    if (all.length === 2) {
      const d = Math.hypot(all[0].x - all[1].x, all[0].y - all[1].y)
      if (pinch.current) zoomTo(crop.zoom * (d / pinch.current))
      pinch.current = d
    } else if (all.length === 1) {
      move(e.clientX - last.x, e.clientY - last.y)
    }
  }
  const onPointerUp = (e: PointerEvent<HTMLDivElement>) => {
    pointers.current.delete(e.pointerId)
    pinch.current = null
  }
  const onWheel = (e: WheelEvent<HTMLDivElement>) => zoomTo(crop.zoom * Math.exp(-Math.sign(e.deltaY) * 0.12))
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const step = e.shiftKey ? 40 : 10
    const moves: Record<string, [number, number]> = {
      ArrowLeft: [step, 0],
      ArrowRight: [-step, 0],
      ArrowUp: [0, step],
      ArrowDown: [0, -step],
    }
    if (moves[e.key]) {
      e.preventDefault()
      move(...moves[e.key])
    } else if (e.key === '+' || e.key === '=') zoomTo(crop.zoom * 1.1)
    else if (e.key === '-') zoomTo(crop.zoom / 1.1)
  }

  const save = () => {
    if (!image) return
    // What the frame shows, in the image's own pixels.
    const k = base * crop.zoom
    const side = frameSize / k
    const sx = image.naturalWidth / 2 - (frameSize / 2 + crop.x) / k
    const sy = image.naturalHeight / 2 - (frameSize / 2 + crop.y) / k
    const canvas = document.createElement('canvas')
    canvas.width = canvas.height = outputSize
    canvas.getContext('2d')!.drawImage(image, sx, sy, side, side, 0, 0, outputSize, outputSize)
    onSave(canvas.toDataURL('image/webp', 0.85))
  }

  const k = base * crop.zoom
  return (
    <dialog ref={dialog} className="dialog crop-dialog" onClose={onCancel} aria-labelledby="crop-title">
      <h2 id="crop-title">Crop your picture</h2>
      <p className="muted">Drag to move, scroll or pinch to zoom.</p>
      <div
        ref={frame}
        className="crop-frame"
        tabIndex={0}
        role="application"
        aria-label="Picture position. Arrow keys move it, plus and minus zoom."
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
        onWheel={onWheel}
        onKeyDown={onKeyDown}
      >
        {image && (
          <img
            src={image.src}
            alt=""
            draggable={false}
            style={{
              width: image.naturalWidth * k,
              height: image.naturalHeight * k,
              transform: `translate(calc(-50% + ${crop.x}px), calc(-50% + ${crop.y}px))`,
            }}
          />
        )}
        <div className="crop-circle" aria-hidden="true" />
      </div>
      {error && <p className="error">{error}</p>}
      <label className="crop-zoom">
        <span className="muted">Zoom</span>
        <input
          type="range"
          min={1}
          max={maxZoom}
          step={0.01}
          value={crop.zoom}
          onChange={(e) => zoomTo(Number(e.target.value))}
          disabled={!image}
        />
      </label>
      <p className="actions">
        <button className="primary" onClick={save} disabled={!image}>
          Save picture
        </button>
        <button onClick={onCancel}>Cancel</button>
      </p>
    </dialog>
  )
}
