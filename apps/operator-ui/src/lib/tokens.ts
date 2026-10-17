/**
 * Single bridge from the canonical design tokens to JavaScript consumers.
 *
 * The only consumer that genuinely needs this is MapLibre, whose style
 * expressions require literal colour strings rather than CSS custom
 * properties. Everything else in the interface should keep using `var(--emad-*)`
 * in CSS or the Tailwind aliases, and must not route its colours through here.
 *
 * Two implementation notes that are load-bearing:
 *
 * 1. `getPropertyValue('--a')` on an element whose `--a: var(--b)` returns the
 *    token text, not necessarily a resolved value, and it degrades silently.
 *    Reading the value through a `color` property instead forces the engine to
 *    substitute the whole chain and compute it, so what comes back is a real
 *    colour no matter how many tokens it passes through.
 * 2. MapLibre's colour parser is stricter than the CSS parser and does not
 *    accept `color-mix()`, so results are normalised to `#rrggbb` (or
 *    `#rrggbbaa`) before they are handed over.
 */

const resolved = new Map<string, string>()

/** Reads a token and returns it as a MapLibre-safe colour string. */
export function resolveTokenColor(token: string): string | undefined {
  const cached = resolved.get(token)
  if (cached !== undefined) return cached === '' ? undefined : cached

  const value = computeTokenColor(token)
  resolved.set(token, value ?? '')
  return value
}

/**
 * Computes the resolved value of one token.
 *
 * Exported separately from the caching wrapper so tests can assert that the
 * stylesheet and the resolver agree without depending on the cache being empty.
 */
export function computeTokenColor(token: string): string | undefined {
  if (typeof document === 'undefined') return undefined
  const raw = getComputedStyle(document.documentElement).getPropertyValue(token).trim()
  if (raw === '') return undefined

  // `--x` may itself be `var(--y)`. Assigning it to `color` makes the engine
  // resolve the chain and produce a usable colour.
  const probe = document.createElement('span')
  probe.style.position = 'absolute'
  probe.style.visibility = 'hidden'
  probe.style.pointerEvents = 'none'
  probe.style.color = raw
  document.body.appendChild(probe)
  const computed = getComputedStyle(probe).color
  probe.remove()

  return toMapLibreColor(computed) ?? undefined
}

/**
 * Normalises `rgb()`/`rgba()` to hex. Named colours and the handful of forms
 * this stylesheet can produce are passed through unchanged.
 */
function toMapLibreColor(value: string): string | undefined {
  const trimmed = value.trim()
  if (trimmed === '') return undefined
  if (trimmed.startsWith('#')) return trimmed

  const match = /^rgba?\(([^)]+)\)$/i.exec(trimmed)
  if (!match) return trimmed

  const parts = match[1]
    .split(/[,/\s]+/)
    .map((part) => part.trim())
    .filter((part) => part !== '')
  if (parts.length < 3) return trimmed

  const channel = (raw: string): number => {
    if (raw.endsWith('%')) return Math.round((Number.parseFloat(raw) / 100) * 255)
    return Math.round(Number.parseFloat(raw))
  }
  const hex = (raw: string): string =>
    Math.min(255, Math.max(0, channel(raw))).toString(16).padStart(2, '0')

  const alphaRaw = parts[3]
  if (alphaRaw === undefined) return `#${hex(parts[0])}${hex(parts[1])}${hex(parts[2])}`

  const alpha =
    alphaRaw.endsWith('%') ? Number.parseFloat(alphaRaw) / 100 : Number.parseFloat(alphaRaw)
  if (alpha >= 1) return `#${hex(parts[0])}${hex(parts[1])}${hex(parts[2])}`
  const alphaHex = Math.round(Math.min(1, Math.max(0, alpha)) * 255)
    .toString(16)
    .padStart(2, '0')
  return `#${hex(parts[0])}${hex(parts[1])}${hex(parts[2])}${alphaHex}`
}
