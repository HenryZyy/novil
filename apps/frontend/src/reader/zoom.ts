// Zoom for the comic reader: scale, plus how far the zoomed page is moved.
export type Zoom = { scale: number; x: number; y: number }

export const noZoom: Zoom = { scale: 1, x: 0, y: 0 }
export const maxZoom = 5

const clamp = (v: number, lo: number, hi: number) => Math.min(Math.max(v, lo), hi)

// Keeps the zoomed page covering its w×h box, so it can't be dragged away.
export function clampPan(z: Zoom, w: number, h: number): Zoom {
  return { scale: z.scale, x: clamp(z.x, w - w * z.scale, 0), y: clamp(z.y, h - h * z.scale, 0) }
}

// Zooms by factor around the point (px, py) of the box, which stays put
// under the cursor or fingers.
export function zoomAt(z: Zoom, factor: number, px: number, py: number, w: number, h: number): Zoom {
  const scale = clamp(z.scale * factor, 1, maxZoom)
  if (scale === 1) return noZoom
  const r = scale / z.scale
  return clampPan({ scale, x: px - (px - z.x) * r, y: py - (py - z.y) * r }, w, h)
}
