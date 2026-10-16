import type { DeviceKind } from '../api/client'
import { Icon } from './Icon'

const kinds = { phone: 'Phone', tablet: 'Tablet', desktop: 'Computer' }

// What a request came from: an icon for the kind and e.g. "Firefox on Windows".
export function Device({ kind, name }: { kind: DeviceKind; name: string }) {
  if (!kind && !name) return <span className="muted">Unknown</span>
  return (
    <span className="device" title={kind ? `${kinds[kind]} · ${name}` : name}>
      <Icon name={kind || 'device'} size={16} />
      <span>{name || kinds[kind as Exclude<DeviceKind, ''>]}</span>
    </span>
  )
}
