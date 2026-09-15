import type { Point } from '@/lib/api'

const dateTimeFormatter = new Intl.DateTimeFormat('en-GB', {
  day: '2-digit',
  month: 'short',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
})

const timeFormatter = new Intl.DateTimeFormat('en-GB', {
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
})

const dateFormatter = new Intl.DateTimeFormat('en-GB', {
  day: '2-digit',
  month: 'short',
})

/** Absolute timestamp: "17:05:32" for today, "13 Sep 17:05:32" otherwise. */
export function formatTimestamp(iso?: string | null): string {
  if (!iso) return '—'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '—'
  const now = new Date()
  const sameDay =
    date.getFullYear() === now.getFullYear() &&
    date.getMonth() === now.getMonth() &&
    date.getDate() === now.getDate()
  return sameDay
    ? timeFormatter.format(date)
    : `${dateFormatter.format(date)} ${timeFormatter.format(date)}`
}

export function formatDateTime(iso?: string | null): string {
  if (!iso) return '—'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '—'
  return dateTimeFormatter.format(date)
}

/** Compact relative time: "now", "12s ago", "4m ago", "2h ago", otherwise a date. */
export function formatRelative(iso?: string | null, now: number = Date.now()): string {
  if (!iso) return '—'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '—'
  const diffMs = now - date.getTime()
  if (diffMs < -5_000) return formatTimestamp(iso)
  const seconds = Math.max(0, Math.floor(diffMs / 1000))
  if (seconds < 10) return 'now'
  if (seconds < 60) return `${seconds}s ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  return dateTimeFormatter.format(date)
}

export function formatCoord(point?: Point | null): string {
  if (!point) return '—'
  return `${point.lat.toFixed(5)}, ${point.lng.toFixed(5)}`
}

export function formatSpeed(speed?: number | null): string {
  if (speed === undefined || speed === null) return '—'
  return `${speed.toFixed(1)} m/s`
}

const COMPASS = ['N', 'NNE', 'NE', 'ENE', 'E', 'ESE', 'SE', 'SSE', 'S', 'SSW', 'SW', 'WSW', 'W', 'WNW', 'NW', 'NNW']

export function formatHeading(heading?: number | null): string {
  if (heading === undefined || heading === null) return '—'
  const index = Math.round((((heading % 360) + 360) % 360) / 22.5) % 16
  return `${Math.round(heading)}° ${COMPASS[index]}`
}

export function formatConfidence(confidence?: number | null): string {
  if (confidence === undefined || confidence === null) return '—'
  return `${Math.round(confidence * 100)}%`
}

export function formatPercent(value: number): string {
  return `${Math.round(value * 100)}%`
}

/** Virtual scenario time in milliseconds rendered as minutes:seconds. */
export function formatVirtualTime(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000))
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${minutes}:${String(seconds).padStart(2, '0')}`
}

export function formatDistanceMeters(meters: number): string {
  if (meters < 1000) return `${Math.round(meters)} m`
  return `${(meters / 1000).toFixed(2)} km`
}

export function titleCase(value?: string | null): string {
  if (!value) return '—'
  return value
    .replace(/[._]/g, ' ')
    .replace(/\b\w/g, (character) => character.toUpperCase())
}

export function truncateId(id?: string | null, length = 18): string {
  if (!id) return '—'
  if (id.length <= length) return id
  return `${id.slice(0, length)}…`
}

export function parseCommaList(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

export function parseGeojson(value: string): unknown | null {
  try {
    return JSON.parse(value) as unknown
  } catch {
    return null
  }
}
