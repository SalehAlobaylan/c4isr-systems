import { expect, test, type Page } from '@playwright/test'

import {
  ALERT_LIFECYCLE,
  ALERT_SEVERITY,
  ASSET_STATUS,
  COMMAND_STATE,
  FRESHNESS_TONE,
  INCIDENT_STATUS,
  MISSION_STATUS,
  RUN_STATUS,
  TONE_GLYPH,
  TONE_TOKEN,
  TRACK_STATUS,
  presentAlertLifecycle,
  presentCommandState,
  presentIncidentStatus,
  presentSeverity,
  presentTrackStatus,
  presentValue,
  toneRef,
} from '../../src/lib/operationalPresentation'

import { mockOperationalWorkflow } from './fixtures-operational'

/* ============================================================================
   Contract tests for the Alloy Pearl design system.
   ---------------------------------------------------------------------------
   These are not screenshots. They assert the properties DESIGN.md makes
   normative and that a visual review cannot catch reliably: that the palette is
   defined in exactly one place, that the documented contrast pairs actually
   hold as computed by a browser, that no operational label falls below the
   legibility floor, that severity and lifecycle stay separate axes, and that
   the map draws the channels it claims to draw.
   ========================================================================== */

const NO_TOKEN = 'no-token'

/** WCAG 2.2 relative luminance, computed from a computed `rgb()` string. */
function luminance(rgb: string): number {
  const channels = rgb.match(/[\d.]+/g)?.slice(0, 3).map(Number) ?? []
  const [r, g, b] = channels.map((value) => {
    const c = value / 255
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function contrast(foreground: string, background: string): number {
  const a = luminance(foreground)
  const b = luminance(background)
  const [light, dark] = a > b ? [a, b] : [b, a]
  return (light + 0.05) / (dark + 0.05)
}

function hex(rgb: string): string {
  const [r, g, b] = (rgb.match(/[\d.]+/g)?.slice(0, 3).map(Number) ?? []).map((n) =>
    n.toString(16).padStart(2, '0'),
  )
  return `#${r}${g}${b}`
}

async function computed(page: Page, selector: string, property: string): Promise<string> {
  return page.evaluate(
    ([sel, prop]) => {
      const element = document.querySelector(sel)
      if (!element) return ''
      return getComputedStyle(element).getPropertyValue(prop)
    },
    [selector, property] as const,
  )
}

/** The resolved value of a design token, read the way the application reads it. */
async function token(page: Page, name: string): Promise<string> {
  return page.evaluate((token) => {
    const probe = document.createElement('span')
    probe.style.position = 'absolute'
    probe.style.visibility = 'hidden'
    probe.style.color = `var(${token})`
    document.body.appendChild(probe)
    const value = getComputedStyle(probe).color
    probe.remove()
    return value
  }, name)
}

/* ── 1. The semantic resolver ─────────────────────────────────────────────
   Pure, no browser. These assertions are the reason the mapping lives in one
   module: each one is a distinction an operator could otherwise lose. */

test.describe('operational presentation', () => {
  test('severity and lifecycle are separate axes', () => {
    // An ACTIVE alert is a lifecycle state, not a severity. It must never wear
    // severity colour, or an active *low* alert would look like an emergency.
    expect(ALERT_LIFECYCLE.ACTIVE).toBe('info')
    expect(ALERT_LIFECYCLE.ACKNOWLEDGED).toBe('info')
    // Acknowledged is not resolved, and neither is a green "all clear".
    expect(ALERT_LIFECYCLE.RESOLVED).toBe('neutral')

    // Severity keeps its own ramp even while the alert is resolved.
    expect(ALERT_SEVERITY.low).toBe('info')
    expect(ALERT_SEVERITY.medium).toBe('caution')
    expect(ALERT_SEVERITY.high).toBe('high')
    expect(ALERT_SEVERITY.critical).toBe('critical')
    expect(presentSeverity('critical').tone).toBe('critical')
  })

  test('acknowledgement carries its actor rather than implying completion', () => {
    const acknowledged = presentAlertLifecycle('ACKNOWLEDGED', 'operator-01', '12:04')
    expect(acknowledged.tone).toBe('info')
    expect(acknowledged.detail).toContain('operator-01')

    // A command that was received has not been carried out.
    expect(COMMAND_STATE.ACKNOWLEDGED).toBe('info')
    expect(COMMAND_STATE.COMPLETED).toBe('positive')
    expect(COMMAND_STATE.TIMED_OUT).toBe('critical')
    expect(COMMAND_STATE.CANCELLED).toBe('neutral')
    expect(presentCommandState('ACKNOWLEDGED').tone).not.toBe(
      presentCommandState('COMPLETED').tone,
    )
  })

  test('investigating and responding stay distinct', () => {
    expect(INCIDENT_STATUS.INVESTIGATING).toBe('caution')
    expect(INCIDENT_STATUS.RESPONDING).toBe('info')
    expect(INCIDENT_STATUS.INVESTIGATING).not.toBe(INCIDENT_STATUS.RESPONDING)
    expect(presentIncidentStatus('INVESTIGATING').tone).not.toBe(
      presentIncidentStatus('RESPONDING').tone,
    )
  })

  test('brand lime is never an operational tone', () => {
    // DESIGN.md §3.3: lime is a corporate signature and may never mean
    // severity, confidence, connection health, freshness, affiliation or
    // selection. If a tone ever resolved to the lime token, this is where it
    // would be caught — and §2.2's own warning is that lime is unreadable as
    // small text on a light surface, so a badge in lime would fail twice.
    for (const [tone, token] of Object.entries(TONE_TOKEN)) {
      expect(token, `${tone} must not use a brand token`).not.toMatch(/lime|brand/)
    }
    for (const value of Object.values({ ...ALERT_SEVERITY, ...ASSET_STATUS, ...TRACK_STATUS })) {
      expect(TONE_TOKEN[value]).not.toMatch(/lime|brand/)
    }
  })

  test('unrecognised values stay neutral and say so', () => {
    // DESIGN.md §13: never silently treat an unknown status as successful.
    const unknown = presentValue('SOMETHING_NEW', TRACK_STATUS)
    expect(unknown.tone).toBe('neutral')
    expect(unknown.known).toBe(false)

    expect(presentTrackStatus(undefined).known).toBe(false)
    expect(presentTrackStatus('').known).toBe(false)
    expect(presentTrackStatus('active').known).toBe(true)
  })

  test('freshness is independent of administrative status', () => {
    // A stale asset is simultaneously "available" and not current; collapsing
    // the two is the failure this table exists to prevent.
    expect(ASSET_STATUS.available).toBe('positive')
    expect(FRESHNESS_TONE.stale).toBe('critical')
    expect(TONE_TOKEN[ASSET_STATUS.available]).not.toBe(TONE_TOKEN[FRESHNESS_TONE.stale])
    expect(FRESHNESS_TONE.unavailable).toBe('neutral')
    expect(FRESHNESS_TONE.unknown).toBe('neutral')
  })

  test('run and mission states carry the expected tones', () => {
    expect(RUN_STATUS.RUNNING).toBe('positive')
    expect(RUN_STATUS.PAUSED).toBe('caution')
    expect(RUN_STATUS.STOPPED).toBe('neutral')
    expect(MISSION_STATUS.COMPLETED).toBe('positive')
    expect(MISSION_STATUS.ABORTED).toBe('neutral')
  })

  test('every tone has a distinct shape glyph, so hue is never the only cue', () => {
    const glyphs = Object.values(TONE_GLYPH)
    expect(new Set(glyphs).size).toBe(glyphs.length)
    expect(toneRef('critical')).toBe('var(--emad-status-critical)')
  })
})

/* ── 2. The palette, as the browser computes it ─────────────────────────── */

test.describe('design tokens', () => {
  test.beforeEach(async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== NO_TOKEN)
    await mockOperationalWorkflow(page)
    await page.goto('/map')
    await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
  })

  test('every documented primitive resolves to its documented value', async ({ page }) => {
    // The eight foundation colours of DESIGN.md §2.1, plus the functional
    // additions of §2.2 and the operational semantics of §2.3. These are the
    // values a change to the identity must move deliberately: nothing else in
    // the application is allowed to hardcode a colour, so this table is the
    // whole reviewable surface of the palette.
    const expected: Record<string, string> = {
      '--emad-mineral-canvas': '#e3e9e2',
      '--emad-ceramic-surface': '#f2f4ed',
      '--emad-mist-surface': '#d4ded8',
      '--emad-industrial-graphite': '#202b2d',
      '--emad-graphite-ink': '#1d3030',
      '--emad-energized-lime': '#b7d36b',
      '--emad-signal-teal': '#3c7780',
      '--emad-structural-border': '#b4c4b9',

      '--emad-text-primary': '#1d3030',
      '--emad-text-secondary': '#4b6159',
      '--emad-text-on-dark': '#e9f2ea',
      '--emad-text-nav-muted': '#b7cac4',
      '--emad-link': '#2c626b',
      '--emad-lime-on-light': '#355a3f',
      '--emad-surface-hover': '#e8eee7',
      '--emad-surface-selected': '#c7d9c8',
      '--emad-surface-input': '#f7f8f2',
      '--emad-surface-nav': '#202b2d',
      '--emad-surface-nav-raised': '#304044',
      '--emad-border-control': '#607f77',

      '--emad-status-positive': '#276645',
      '--emad-status-info': '#2a5b66',
      '--emad-status-caution': '#7e5319',
      '--emad-status-high': '#91451f',
      '--emad-status-critical': '#9a372f',
      '--emad-status-neutral': '#4b6159',
      '--emad-status-review': '#2c626b',
    }
    for (const [name, value] of Object.entries(expected)) {
      expect(hex(await token(page, name)), name).toBe(value)
    }
  })

  test('no two tokens share a name', async ({ page }) => {
    // A colour token and a size token under the same name is not a style
    // mistake, it is a silent outage: `color: var(--x)` becomes invalid at
    // computed-value time and falls back to `inherit`, so every reference
    // quietly turns into the surrounding text colour. It happened once during
    // this migration, so it is now a test.
    const declarations = await page.evaluate(() => {
      const found: Record<string, string[]> = {}
      for (const sheet of Array.from(document.styleSheets)) {
        let rules: CSSRuleList
        try {
          rules = sheet.cssRules
        } catch {
          continue
        }
        for (const rule of Array.from(rules)) {
          if (!(rule instanceof CSSStyleRule)) continue
          const style = (rule as CSSStyleRule).style
          for (let i = 0; i < style.length; i += 1) {
            const property = style.item(i)
            if (!property.startsWith('--')) continue
            const value = style.getPropertyValue(property).trim()
            if (!value) continue
            found[property] = found[property] ?? []
            if (!found[property].includes(value)) found[property].push(value)
          }
        }
      }
      return found
    })

    const conflicts = Object.entries(declarations).filter(([, values]) => values.length > 1)
    expect(
      conflicts.map(([name, values]) => `${name} → ${values.join('  AND  ')}`),
      'tokens declared with more than one value',
    ).toEqual([])
  })

  test('the type scale matches the documented roles', async ({ page }) => {
    // DESIGN.md §4.2. Asserting the whole scale — not just the sizes in use —
    // is what keeps the tokens and the document from drifting: a reserved top of
    // the scale that nothing renders today is still a promise the next screen
    // will be built against.
    const expected: Record<string, string> = {
      '--emad-type-hero': '30px',
      '--emad-type-leading-hero': '36px',
      '--emad-type-heading': '24px',
      '--emad-type-leading-heading': '30px',
      '--emad-type-panel-heading': '20px',
      '--emad-type-leading-panel-heading': '26px',
      '--emad-type-subheading': '16px',
      '--emad-type-leading-subheading': '22px',
      '--emad-type-body': '14px',
      '--emad-type-leading-body': '21px',
      '--emad-type-secondary': '13px',
      '--emad-type-leading-secondary': '18px',
      '--emad-type-dense': '12px',
      '--emad-type-leading-dense': '17px',
      '--emad-type-meta': '12px',
      '--emad-type-leading-meta': '16px',
      '--emad-type-figure': '26px',
      // §4.2 caps uppercase Latin tracking at 0.04em. The previous identity used
      // 0.08em, which was outside the documented maximum.
      '--emad-type-tracking-micro': '0.03em',
      '--emad-type-tracking-label': '0.04em',
    }
    for (const [name, value] of Object.entries(expected)) {
      const actual = await page.evaluate(
        (token) => getComputedStyle(document.documentElement).getPropertyValue(token).trim(),
        name,
      )
      expect(actual, name).toBe(value)
    }
  })

test('map-domain values resolve through the status tokens, not a second palette', async ({
    page,
  }) => {
    // A marker on the map and a badge in a table must be the same colour, or the
    // same status reads as two things.
    expect(hex(await token(page, '--emad-map-entity-asset'))).toBe(
      hex(await token(page, '--emad-status-positive')),
    )
    expect(hex(await token(page, '--emad-map-entity-track'))).toBe(
      hex(await token(page, '--emad-status-info')),
    )
    expect(hex(await token(page, '--emad-map-area-hazard'))).toBe(
      hex(await token(page, '--emad-status-critical')),
    )
  })

  test('the documented contrast pairs hold as computed', async ({ page }) => {
    const canvas = await token(page, '--emad-surface-canvas')
    const panel = await token(page, '--emad-surface-panel')
    const mist = await token(page, '--emad-surface-inset')
    const ink = await token(page, '--emad-text-primary')
    const secondary = await token(page, '--emad-text-secondary')
    const graphite = await token(page, '--emad-industrial-graphite')
    const onDark = await token(page, '--emad-text-on-dark')
    const navMuted = await token(page, '--emad-text-nav-muted')
    const lime = await token(page, '--emad-energized-lime')
    const link = await token(page, '--emad-link')
    const signal = await token(page, '--emad-signal-teal')
    const critical = await token(page, '--emad-status-critical')
    const control = await token(page, '--emad-border-control')
    const border = await token(page, '--emad-structural-border')

    // DESIGN.md §2.5: normal text AA 4.5:1, large text and meaningful non-text
    // boundaries 3:1. The documented ratios are 11.21, 12.48 and 10.04 for
    // primary text on the three light surfaces.
    expect(contrast(ink, canvas)).toBeGreaterThanOrEqual(4.5)
    expect(contrast(ink, panel)).toBeGreaterThanOrEqual(4.5)
    expect(contrast(ink, mist)).toBeGreaterThanOrEqual(4.5)
    // §2.5 documents secondary-on-mist at 4.83:1 — the tightest text pair in
    // the system, and therefore the one that must not silently regress.
    expect(contrast(secondary, mist)).toBeGreaterThanOrEqual(4.5)
    expect(contrast(link, mist)).toBeGreaterThanOrEqual(4.5)
    expect(contrast(signal, panel)).toBeGreaterThanOrEqual(3)
    expect(contrast(critical, mist)).toBeGreaterThanOrEqual(4.5)
    // §2.5 documents the control border at 3.18:1 on mist — a control edge, not
    // body copy, so it is held to the non-text target.
    expect(contrast(control, mist)).toBeGreaterThanOrEqual(3)
    expect(contrast(control, panel)).toBeGreaterThanOrEqual(3)
    // The decorative divider is explicitly below the essential threshold and
    // must not be mistaken for one: if it ever rises above 3:1 the two roles
    // have stopped being distinguishable.
    expect(contrast(border, panel)).toBeLessThan(3)

    // The graphite anchor carries the navigation and the primary action, so its
    // own two text roles are the ones that must hold.
    expect(contrast(onDark, graphite)).toBeGreaterThanOrEqual(4.5)
    expect(contrast(navMuted, graphite)).toBeGreaterThanOrEqual(4.5)
    // Lime is the brand signature and is 8.68:1 *on graphite* — never on a
    // light surface, which is why the focus ring has a separate dark token.
    expect(contrast(lime, graphite)).toBeGreaterThanOrEqual(4.5)
    expect(contrast(lime, panel)).toBeLessThan(3)
  })

  test('status tones clear the contrast target on every surface they appear on', async ({ page }) => {
    // §12.10 asks for the §2.5 ratios "under actual rendering conditions", and a
    // badge is text sitting on the surface of whatever panel it was rendered
    // into — so each tone is checked against canvas, panel and mist.
    const surfaces = {
      canvas: await token(page, '--emad-surface-canvas'),
      panel: await token(page, '--emad-surface-panel'),
      mist: await token(page, '--emad-surface-inset'),
    }
    for (const tone of ['positive', 'caution', 'high', 'critical', 'info', 'neutral', 'review']) {
      const colour = await token(page, `--emad-status-${tone}`)
      for (const [name, background] of Object.entries(surfaces)) {
        const ratio = contrast(colour, background)
        // `neutral` is the one tone allowed to sit low: it means "not
        // applicable", and it is never the sole carrier of a meaning.
        const floor = tone === 'neutral' ? 3 : 4.5
        expect(ratio, `${tone} on ${name}`).toBeGreaterThanOrEqual(floor)
      }
    }
  })

test('the interface runs on the approved typeface at the approved body size', async ({
    page,
  }) => {
    const family = await computed(page, 'body', 'font-family')
    expect(family).toContain('IBM Plex Sans')
    // The Arabic family is present in the stack for RTL screens, so Arabic text
    // is not rendered with a fallback while Latin is fine.
    expect(family).toContain('IBM Plex Sans Arabic')
    expect(await computed(page, 'body', 'font-size')).toBe('14px')
    expect(hex(await computed(page, 'body', 'background-color'))).toBe(
      hex(await token(page, '--emad-surface-canvas')),
    )

    const mono = await computed(page, '.aegis-clock', 'font-family')
    expect(mono).toContain('IBM Plex Mono')
  })

  test('tabular figures are applied where numbers are compared', async ({ page }) => {
    await page.goto('/board')
    const cell = page.locator('.aegis-board td.cell').first()
    await expect(cell).toBeVisible()
    expect(await cell.evaluate((element) => getComputedStyle(element).fontVariantNumeric)).toContain(
      'tabular-nums',
    )
  })

  test('no operational label renders below the 12px legibility floor', async ({ page }) => {
    for (const path of ['/map', '/board', '/alerts', '/assets', '/tracks', '/timeline', '/scenarios']) {
      await page.goto(path)
      const tooSmall = await page.evaluate(() => {
        const offenders: Array<{ text: string; size: string }> = []
        for (const element of Array.from(document.querySelectorAll('body *'))) {
          // Map attribution is the one documented exception: it is not
          // operational information, but it must stay readable.
          if (element.closest('.maplibregl-ctrl-attrib, .maplibregl-ctrl-scale')) continue
          const text = (element.textContent ?? '').trim()
          if (!text || element.children.length > 0) continue
          const style = getComputedStyle(element)
          if (style.display === 'none' || style.visibility === 'hidden') continue
          const size = Number.parseFloat(style.fontSize)
          if (size < 12) offenders.push({ text: text.slice(0, 40), size: style.fontSize })
        }
        return offenders
      })
      expect(tooSmall, `sub-12px text on ${path}: ${JSON.stringify(tooSmall)}`).toEqual([])
    }
  })

  test('keyboard focus is visible on every surface that carries a control', async ({ page }) => {
    await page.goto('/board')
    const targets = ['.aegis-chip', '.aegis-seg button']
    for (const selector of targets) {
      const element = page.locator(selector).first()
      if ((await element.count()) === 0) continue
      await element.focus()
      const outline = await element.evaluate((node) => {
        const style = getComputedStyle(node)
        return { width: style.outlineWidth, style: style.outlineStyle, color: style.outlineColor }
      })
      expect(Number.parseFloat(outline.width), `${selector} focus width`).toBeGreaterThanOrEqual(2)
      expect(outline.style, `${selector} focus style`).not.toBe('none')
      expect(hex(outline.color)).toBe(hex(await token(page, '--emad-focus-light')))
    }
  })

  test('the graphite rail uses the dark focus indicator, not the light one', async ({ page }) => {
    // The single easiest rule to miss in a two-surface identity: the workspace
    // focus ring is teal, which is roughly 1.9:1 on graphite. A test that only
    // ever inspects the light workspace passes while a keyboard user loses the
    // caret on every navigation item.
    await page.goto('/board')
    const navItem = page.locator('.aegis-nav a').first()
    await navItem.focus()
    const rail = await navItem.evaluate((node) => {
      const style = getComputedStyle(node)
      return {
        outline: style.outlineColor,
        background: getComputedStyle(document.querySelector('.aegis-topbar')!).backgroundColor,
      }
    })
    expect(hex(rail.outline)).toBe(hex(await token(page, '--emad-focus-dark')))
    // And it has to clear 3:1 against the surface it actually sits on.
    const ratio = contrast(rail.outline, rail.background)
    expect(ratio, 'focus ring on graphite').toBeGreaterThanOrEqual(3)
  })

  test('the navigation never clips a view label at a usable width', async ({ page }) => {
    for (const width of [1920, 1440, 1280, 1024]) {
      await page.setViewportSize({ width, height: 900 })
      await page.goto('/board')
      await expect(page.locator('.aegis-nav a').first()).toBeVisible()
      // The fit is measured after mount and re-measured when the webfont
      // lands; one settle frame is enough for both.
      await page.waitForTimeout(150)
      const clipped = await page.locator('.aegis-nav').evaluate((nav) => {
        const bound = nav.getBoundingClientRect()
        return Array.from(nav.querySelectorAll('a'))
          .filter((link) => {
            const box = link.getBoundingClientRect()
            return box.left < bound.left - 1 || box.right > bound.right + 1
          })
          .map((link) => link.textContent)
      })
      expect(clipped, `clipped navigation labels at ${width}px`).toEqual([])
    }
  })

  test('reduced motion removes transitions and animation', async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await page.goto('/map')
    const duration = await page.locator('.aegis-nav a').first().evaluate((element) => {
      const style = getComputedStyle(element)
      return { transition: style.transitionDuration, animation: style.animationDuration }
    })
    expect(Number.parseFloat(duration.transition)).toBeLessThanOrEqual(0.001)
    expect(Number.parseFloat(duration.animation)).toBeLessThanOrEqual(0.001)
  })
})

/* ── 3. The map's visual channels ───────────────────────────────────────── */

test.describe('operational picture', () => {
  test.beforeEach(async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== NO_TOKEN)
    await mockOperationalWorkflow(page)
  })

  test('draws one layer per documented channel, selection on top', async ({ page }) => {
    const errors: string[] = []
    // The realtime socket has no backend under this harness, so a refused
    // WebSocket is an environment artefact rather than a styling fault. A
    // rejected style expression is not.
    const isHarnessNoise = (text: string) => /WebSocket|ws:\/\/|ERR_CONNECTION/i.test(text)
    page.on('pageerror', (error) => {
      if (!isHarnessNoise(error.message)) errors.push(error.message)
    })
    page.on('console', (message) => {
      if (message.type() === 'error' && !isHarnessNoise(message.text())) {
        errors.push(message.text())
      }
    })

    await page.goto('/map?selected=track%3Atrk-demo')
    await expect(page.locator('[data-map-ready="true"]')).toBeVisible()

    // MapLibre drops a layer whose style expression it cannot parse, and says
    // so asynchronously. The component reads the layer ids back, so a missing
    // channel shows up as a missing id rather than as silence.
    const root = page.locator('[data-map-ready="true"]')
    await expect
      .poll(async () => (await root.getAttribute('data-map-layers')) ?? '', { timeout: 5_000 })
      .toContain('track-selected-ring')

    const ids = ((await root.getAttribute('data-map-layers')) ?? '').split(' ').filter(Boolean)

    for (const id of [
      'geofence-fill',
      'geofence-outline',
      'area-fill',
      'area-outline',
      'track-circle',
      'track-freshness-ring',
      'track-selected-ring',
      'asset-circle',
      'asset-core',
      'asset-freshness-ring',
      'asset-selected-ring',
      'measure-line',
      'measure-points',
      'track-history-line',
    ]) {
      expect(ids, `map layer ${id}`).toContain(id)
    }

    // Selection must paint above the freshness ring and the entity body, or a
    // ring or a cluster can hide it (DESIGN.md §9.2).
    expect(ids.indexOf('track-selected-ring')).toBeGreaterThan(ids.indexOf('track-circle'))
    expect(ids.indexOf('asset-selected-ring')).toBeGreaterThan(ids.indexOf('asset-freshness-ring'))
    expect(ids.indexOf('asset-core')).toBeGreaterThan(ids.indexOf('asset-circle'))

    expect(errors).toEqual([])
  })

  /** Moves the picture back in time and waits until the replay is established. */
  async function enterReplay(page: Page) {
    const timeline = page.getByLabel('Picture timeline position')
    await expect(timeline).toBeEnabled()
    // The window comes from the server, so the bounds are read from the element
    // rather than hardcoded: a hardcoded instant can silently land outside the
    // recorded evidence and quietly stop testing anything.
    await expect
      .poll(async () => {
        const min = Number(await timeline.getAttribute('min'))
        const max = Number(await timeline.getAttribute('max'))
        return max > min
      }, { timeout: 5_000 })
      .toBe(true)

    const min = Number(await timeline.getAttribute('min'))
    await timeline.fill(String(min + 60_000))
    await expect(page.getByTestId('scrub-mode')).toHaveText('Replay')
  }

  test('the freshness legend states what is not current, in words', async ({ page }) => {
    await page.goto('/map')
    await expect(page.locator('[data-map-ready="true"]')).toBeVisible()

    const legend = page.getByLabel('Freshness legend')
    await expect(legend).toContainText('Stale (1)')
    await expect(legend).toContainText('1 not current')
    // The ring is the shape cue; the word is what an operator actually reads.
    await expect(legend.locator('i[data-shape="ring"]').first()).toBeVisible()

    const entity = page.getByLabel('Event area legend')
    await expect(entity).toContainText('Hazard area (1)')
  })

  test('live and replay never share an appearance', async ({ page }) => {
    await page.goto('/map')
    const live = page.getByTestId('scrub-mode')
    await expect(live).toHaveText('Live')
    const liveColour = await live.evaluate((element) => getComputedStyle(element).color)

    await enterReplay(page)
    const replayColour = await page
      .getByTestId('scrub-mode')
      .evaluate((element) => getComputedStyle(element).color)
    expect(replayColour).not.toBe(liveColour)
    await expect(page.getByTestId('replay-banner')).toBeVisible()
  })

  test('the replay control is never covered by the board overlay', async ({ page }) => {
    await page.goto('/map')
    await enterReplay(page)
    await expect(page.getByTestId('replay-banner')).toBeVisible()

    // Every control the operator needs while reviewing a past picture must be
    // hit-testable. This is the regression that shipped once already: a floating
    // panel sat over "Return to live".
    //
    // "Return to live" goes last, because using it leaves replay and relabels
    // the board — clicking it first would make the next lookup miss.
    await page.getByRole('button', { name: /Board · replayed plot/ }).first().click({ timeout: 4_000 })
    await page.getByRole('button', { name: 'Layers', exact: true }).click({ timeout: 4_000 })
    await page.getByRole('checkbox', { name: 'Legend' }).uncheck()
    await page.getByRole('button', { name: 'Layers', exact: true }).click()
    await page.getByRole('button', { name: 'Return to live', exact: true }).click({ timeout: 4_000 })
    await expect(page.getByTestId('scrub-mode')).toHaveText('Live')
  })

  test('selection is marked by an outline, not by recolouring the entity', async ({ page }) => {
    await page.goto('/map?selected=asset%3A' + 'scn_run_demo__asset__patrol-01')
    await expect(page.locator('[data-map-ready="true"]')).toBeVisible()
    const card = page.locator('.aegis-mbcard[aria-pressed="true"]')
    await expect(card).toHaveCount(1)
    // The selection outline is chrome; the entity's own status colour is untouched.
    // Graphite ink is the chrome channel under Alloy Pearl — §7.1 requires
    // selection to be readable without implying a domain state.
    const shadow = await card.locator('.inner').evaluate((node) => getComputedStyle(node).boxShadow)
    expect(shadow).toContain('rgb(29, 48, 48)')
  })
})

/* ── 4. The board ───────────────────────────────────────────────────────── */

test.describe('entity board', () => {
  test.beforeEach(async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== NO_TOKEN)
    await mockOperationalWorkflow(page)
  })

  test('freshness rides the column edge and status rides the labelled chip', async ({ page }) => {
    await page.goto('/board')
    const headers = page.locator('th.ent-col')
    await expect(headers.first()).toBeVisible()

    const tone = await headers.first().getAttribute('data-tone')
    const freshness = await headers.first().getAttribute('data-freshness')
    expect(['positive', 'info', 'caution', 'high', 'critical', 'neutral']).toContain(tone)
    expect(['live', 'delayed', 'stale', 'unavailable', 'unknown']).toContain(freshness)

    // Administrative status is words, not a coloured background.
    const chip = headers.first().locator('.st')
    await expect(chip).not.toBeEmpty()
    await expect(chip.locator('.tone-glyph')).toHaveCount(1)
  })

  test('a stale column is distinguished from one with no data', async ({ page }) => {
    await page.goto('/board')
    const stale = page.locator('th.ent-col[data-freshness="stale"]')
    await expect(stale).toHaveCount(1)
    const staleEdge = await stale.locator('.inner').evaluate((node) => getComputedStyle(node).borderLeftColor)

    const fresh = page.locator('th.ent-col[data-freshness="live"]').first()
    const freshEdge = await fresh.locator('.inner').evaluate((node) => getComputedStyle(node).borderLeftColor)

    expect(staleEdge).not.toBe(freshEdge)
    expect(hex(staleEdge)).toBe(hex(await token(page, '--emad-status-critical')))
    // Live is unmarked: a ring appears only when trust is in question.
    expect(hex(freshEdge)).not.toBe(hex(staleEdge))
  })

  test('compact density stays legible', async ({ page }) => {
    await page.goto('/board')
    await page.getByRole('button', { name: 'Compact', exact: true }).click()
    const cell = page.locator('.aegis-board td.cell').first()
    await expect(cell).toBeVisible()
    const size = await cell.evaluate((node) => Number.parseFloat(getComputedStyle(node).fontSize))
    expect(size).toBeGreaterThanOrEqual(12)
  })
})

/* ── 5. Alerts ──────────────────────────────────────────────────────────── */

test.describe('alert center', () => {
  test('severity, lifecycle and provenance are three separate readings', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== NO_TOKEN)
    await mockOperationalWorkflow(page)
    await page.goto('/alerts')

    const row = page.getByRole('row', { name: /Restricted Zone A/ })
    await expect(row).toBeVisible()

    const badges = row.locator('.aegis-sev')
    await expect(badges).toHaveCount(2)
    // Severity first, lifecycle second, and they are not the same colour.
    expect(await badges.nth(0).getAttribute('data-tone')).toBe('high')
    expect(await badges.nth(1).getAttribute('data-tone')).toBe('info')

    // Acknowledgement adds who and when; it does not turn the alert green.
    await page.getByRole('button', { name: 'Ack', exact: true }).click()
    const acknowledged = page.getByRole('row', { name: /Restricted Zone A/ }).locator('.aegis-sev').nth(1)
    await expect(acknowledged).toContainText('ACKNOWLEDGED')
    expect(await acknowledged.getAttribute('data-tone')).toBe('info')
    await expect(acknowledged).toContainText('operator-01')
  })

  test('an empty filter result is distinguishable from a failure', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== NO_TOKEN)
    await mockOperationalWorkflow(page)
    await page.goto('/alerts?severity=low')
    await expect(page.getByText('No alerts match the current filter.')).toBeVisible()
    // "No data" is not an error: no failure indicator is shown.
    await expect(page.getByText('Request failed')).toHaveCount(0)
  })

  test('a failed fetch names the failure, never a JavaScript exception', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== NO_TOKEN)
    await mockOperationalWorkflow(page)
    // A non-envelope failure body: the response body is already consumed by
    // the API client, and an error state that says "Failed to execute 'clone'
    // on 'Response'" tells the operator nothing about what actually failed.
    await page.route('**/api/v1/alerts*', (route) =>
      route.fulfill({ status: 502, contentType: 'text/plain', body: 'upstream connect error' }),
    )
    await page.goto('/alerts')
    const failure = page.getByRole('alert').first()
    await expect(failure).toContainText(/upstream connect error|status 502/)
    await expect(failure).not.toContainText(/clone|Response body/)
  })
})