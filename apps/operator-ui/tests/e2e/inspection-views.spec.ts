import { expect, test } from '@playwright/test'

import { mockOperationalWorkflow } from './fixtures-operational'

/**
 * Contract tests for the three inspection views: the picture's telemetry dock,
 * the cross-system grid and the track provenance inspector.
 *
 * These assert the things DESIGN.md §16 requires be verified by measurement
 * rather than by reading CSS — the 12px legibility floor, WCAG AA contrast on
 * the pairs actually rendered, a non-colour cue beside every status, and that
 * no raw hex literal escapes the token system.
 */

const MOCKED = 'Runs once against the mocked no-token build'

function relLum(rgb: number[]): number {
  const channel = (value: number) => {
    const c = value / 255
    return c <= 0.03928 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4)
  }
  const [r, g, b] = rgb
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b)
}

function contrast(fg: number[], bg: number[]): number {
  const a = relLum(fg)
  const b = relLum(bg)
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05)
}

/**
 * Every text leaf inside `selector`, with the colour pair it actually renders
 * against. `page.evaluate` runs in the browser, so the colour maths deliberately
 * happens in Node afterwards rather than being captured from module scope.
 */
async function renderedPairs(page: import('@playwright/test').Page, selector: string) {
  return page.evaluate((scope) => {
    // Chromium serialises a `color-mix()` token as `color(srgb r g b)`, whose
    // channels are in the unit range, while `rgb()` is 0-255. Feeding the unit
    // range straight into the luminance maths turns a near-white panel into a
    // near-black one and reports 1.5:1 for black-on-white text — so the two
    // serialisations are normalised to one scale here.
    const parse = (value: string): number[] => {
      const channels = (value.match(/[\d.]+/g) ?? []).slice(0, 3).map(Number)
      return /^color\(/i.test(value) ? channels.map((channel) => channel * 255) : channels
    }
    // Walks up for the first opaque background, because a text colour is only
    // meaningful against what is actually behind it, not against a transparent
    // wrapper.
    const opaqueBg = (el: Element): number[] => {
      let node: Element | null = el
      while (node) {
        const raw = getComputedStyle(node).backgroundColor
        if (parse(raw).length === 3) {
          const alpha = raw.match(/rgba?\([^)]*?,\s*([\d.]+)\)/)
          if (!alpha || Number(alpha[1]) > 0.9) return parse(raw)
        }
        node = node.parentElement
      }
      return [255, 255, 255]
    }
    const out: Array<{ fg: number[]; bg: number[]; size: number; weight: number }> = []
    const seen = new Set<string>()
    for (const el of Array.from(document.querySelectorAll(scope))) {
      if (!el.textContent?.trim()) continue
      if (el.children.length > 0) continue
      const style = getComputedStyle(el)
      const fg = parse(style.color)
      if (fg.length !== 3) continue
      const key = `${style.color}|${style.backgroundColor}`
      if (seen.has(key)) continue
      seen.add(key)
      out.push({
        fg,
        bg: opaqueBg(el),
        size: parseFloat(style.fontSize),
        weight: Number(style.fontWeight),
      })
    }
    return out
  }, selector)
}

async function belowLegibilityFloor(
  page: import('@playwright/test').Page,
  selector: string,
): Promise<Array<{ text: string; size: number }>> {
  return page.evaluate((scope) => {
    const out: Array<{ text: string; size: number }> = []
    for (const el of Array.from(document.querySelectorAll(`${scope} *`))) {
      if (!el.textContent?.trim()) continue
      if (el.children.length > 0) continue
      const size = parseFloat(getComputedStyle(el).fontSize)
      // DESIGN.md §4.2: operational text never goes below 12px.
      if (size < 12) out.push({ text: el.textContent.trim().slice(0, 30), size })
    }
    return out
  }, selector)
}

test.describe('picture telemetry dock', () => {
  test('collapsed, it still answers whether the picture is now and what is out of date', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name !== 'no-token', MOCKED)
    await mockOperationalWorkflow(page)
    await page.goto('/map')
    await expect(page.locator('[data-map-ready="true"]')).toBeVisible()

    await expect(page.getByTestId('tray-mode')).toHaveText('Live')
    await expect(page.locator('.aegis-tray')).toBeVisible()

    // Collapsed by default: the plot is height-constrained, so the dock must not
    // take map away from an operator who did not ask for it.
    await expect(page.locator('#pane-tray')).toHaveCount(0)

    await page.getByRole('button', { name: /telemetry/i }).first().click()
    await expect(page.locator('#pane-tray')).toBeVisible()

    expect(await belowLegibilityFloor(page, '.aegis-tray')).toEqual([])
    const pairs = await renderedPairs(page, '.aegis-tray p, .aegis-tray span, .aegis-tray a')
    const failures = pairs
      .map((p) => ({ ...p, ratio: contrast(p.fg, p.bg) }))
      .filter((p) => p.ratio + 0.005 < 4.5)
      .map((p) => `${p.ratio.toFixed(2)}:1  fg=${p.fg} bg=${p.bg} ${p.size}px/${p.weight}`)
    expect(failures, 'dock contrast below AA').toEqual([])
  })

  test('every status in the dock carries a label and a shape, never colour alone', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name !== 'no-token', MOCKED)
    await mockOperationalWorkflow(page)
    await page.goto('/map')
    await page.getByRole('button', { name: /telemetry/i }).first().click()
    await expect(page.locator('#pane-tray')).toBeVisible()

    const badges = await page.evaluate(() =>
      Array.from(document.querySelectorAll('.aegis-tray .aegis-sev')).map((el) => ({
        text: el.textContent?.trim() ?? '',
        glyph: el.querySelector('.tone-glyph')?.getAttribute('data-glyph') ?? null,
      })),
    )
    expect(badges.length).toBeGreaterThan(0)
    for (const badge of badges) {
      expect(badge.glyph, `badge "${badge.text}" has no shape glyph`).not.toBeNull()
      expect(badge.text.length, `badge "${badge.text}" has no label`).toBeGreaterThan(0)
    }
  })
})

test.describe('chain of custody', () => {
  test('the footer states what is at each stage, not a slogan about the pipeline', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name !== 'no-token', MOCKED)
    await mockOperationalWorkflow(page)
    await page.goto('/map')

    const ribbon = page.getByRole('navigation', { name: 'Chain of custody' })
    await expect(ribbon).toBeVisible()
    // The counts are what the API returned: three registered sources, one
    // observation. A chain printed as a string is a claim about how the
    // system is built; a chain printed as counts is a reading of it.
    await expect(ribbon).toContainText('3 sources')
    await expect(ribbon).toContainText('1 observation')
    // A count of one is one.
    await expect(ribbon).not.toContainText('1 observations')
    // The stages that have a view are links, so the footer is also the
    // shortest route to them.
    await expect(ribbon.getByRole('link', { name: /tracks/i })).toBeVisible()
    await expect(ribbon.getByRole('link', { name: /entities/i })).toBeVisible()
  })
})

test.describe('picture panel', () => {
  test('is organised into named regions, and the transport never scrolls away', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name !== 'no-token', MOCKED)
    await mockOperationalWorkflow(page)
    await page.goto('/map')
    await expect(page.locator('[data-map-ready="true"]')).toBeVisible()

    const rail = page.getByRole('complementary', { name: 'Operations rail' })
    // The panel's name is a name, not a button that happens to be named.
    await expect(rail.getByRole('heading', { name: 'Picture' })).toBeVisible()
    // Layers and view tools are different kinds of control and are labelled as
    // such, rather than sharing one line and one anonymous group.
    await expect(rail.getByRole('group', { name: 'Map layers' })).toBeVisible()
    await expect(rail.getByRole('group', { name: 'View tools' })).toBeVisible()
    // The selection is a named section, not just whatever came below.
    await expect(rail.getByText('Selection', { exact: true })).toBeVisible()
    // The scenario transport is a side feature and says so.
    await expect(
      rail.locator('section[aria-label="Scenario run"]').locator('.aegis-railsection'),
    ).toHaveText('Scenario')

    // The transport is outside the scrolling region, so reading a long
    // selection never costs the operator the ability to scrub the picture.
    const transportScrolls = await page.evaluate(() => {
      const transport = document.querySelector('[data-testid="scrub-mode"]')
      const scroll = document.querySelector('.aegis-railscroll')
      return Boolean(transport && scroll && scroll.contains(transport))
    })
    expect(transportScrolls, 'the timeline scrolls out of the panel').toBe(false)
  })

  test('the collapse control says what it does', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== 'no-token', MOCKED)
    await mockOperationalWorkflow(page)
    await page.goto('/map')

    // A button labelled "Picture" that collapses the panel promises a view
    // and does something else.
    const collapse = page.getByRole('button', { name: 'Collapse the picture drawer' })
    await expect(collapse).toBeVisible()
    await collapse.click()
    // Collapsing it leaves a way back that says what it opens.
    await expect(page.getByRole('button', { name: /picture/i }).first()).toBeVisible()
  })
})

test.describe('systems grid', () => {
  test('reports every subsystem and admits which of them failed', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== 'no-token', MOCKED)
    await mockOperationalWorkflow(page)
    await page.goto('/systems')

    await expect(page.getByRole('heading', { name: 'Systems' })).toBeVisible()
    await expect(page.locator('.aegis-xsystem')).toHaveCount(6)

    expect(await belowLegibilityFloor(page, '.aegis-xsystems')).toEqual([])
    const pairs = await renderedPairs(page, '.aegis-xsystems p, .aegis-xsystems span, .aegis-xsystems a')
    const failures = pairs
      .map((p) => ({ ...p, ratio: contrast(p.fg, p.bg) }))
      .filter((p) => p.ratio + 0.005 < 4.5)
      .map((p) => `${p.ratio.toFixed(2)}:1  fg=${p.fg} bg=${p.bg} ${p.size}px/${p.weight}`)
    expect(failures, 'systems grid contrast below AA').toEqual([])
  })

  test('names the devices a subsystem is made of, not only how many', async ({
    page,
  }, testInfo) => {
    // The count says one feed is degraded. The count does not say which, and
    // "which" is the whole question an operator has about a degraded feed.
    test.skip(testInfo.project.name !== 'no-token', MOCKED)
    await mockOperationalWorkflow(page)
    await page.goto('/systems')

    const inventory = page.locator('.aegis-xsystem-inventory')
    await expect(inventory).toHaveCount(1)
    await expect(inventory.getByRole('columnheader', { name: 'Device' })).toBeVisible()
    await expect(inventory.getByRole('columnheader', { name: 'Type' })).toBeVisible()
    await expect(inventory.getByRole('columnheader', { name: 'Status' })).toBeVisible()
    await expect(inventory.getByText('radar-01')).toBeVisible()
    await expect(inventory.getByText('ais-shore-04')).toBeVisible()
    // The degraded device says so in words, not by colour alone.
    const degraded = inventory.locator('tr', { hasText: 'ais-shore-04' })
    await expect(degraded.getByText('DEGRADED')).toBeVisible()
  })

  test('no raw hex literal reaches the new views', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== 'no-token', MOCKED)
    await mockOperationalWorkflow(page)
    await page.goto('/systems')

    const literals = await page.evaluate(() => {
      const found = new Set<string>()
      const walk = (node: Element) => {
        const style = getComputedStyle(node)
        for (const prop of ['color', 'backgroundColor', 'borderTopColor'] as const) {
          const value = style.getPropertyValue(prop).trim()
          if (/^#[0-9a-f]{3,8}$/i.test(value)) found.add(value)
        }
        for (const child of Array.from(node.children)) walk(child)
      }
      for (const node of Array.from(document.querySelectorAll('.aegis-xsystems'))) walk(node)
      return Array.from(found)
    })
    expect(literals, 'raw hex colours in the systems grid').toEqual([])
  })
})

test.describe('track provenance inspector', () => {
  test('walks the evidence chain from source to recorded action', async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== 'no-token', MOCKED)
    await mockOperationalWorkflow(page)
    await page.goto('/tracks/trk-demo')

    // The page is named by the track's own external reference, not its id.
    await expect(page.getByRole('heading', { name: /patrol-01/i })).toBeVisible()
    // The chain runs source → recorded action, and every stage states a count
    // rather than a reassuring adjective.
    await expect(page.locator('.aegis-chain-cell')).toHaveCount(6)
    const chain = page.locator('.aegis-chain')
    for (const stage of [
      'Sources',
      'Observations',
      'Track history',
      'Classifications',
      'Assessments',
      'Recorded action',
    ]) {
      await expect(chain.getByText(stage, { exact: false })).toBeVisible()
    }

    // Lifecycle and freshness are different axes and both are stated on the page,
    // which is the separation DESIGN.md §7.1 requires.
    await expect(page.locator('.aegis-chain')).toBeVisible()
    await expect(page.locator('.aegis-listhead').getByText('ACTIVE')).toBeVisible()
    await expect(page.locator('.aegis-listhead').getByText('LIVE')).toBeVisible()

    expect(await belowLegibilityFloor(page, '.aegis-xgrid')).toEqual([])
    const pairs = await renderedPairs(
      page,
      '.aegis-xgrid p, .aegis-xgrid dt, .aegis-xgrid dd, .aegis-xgrid span, .aegis-xgrid td, .aegis-xgrid th',
    )
    const failures = pairs
      .map((p) => ({ ...p, ratio: contrast(p.fg, p.bg) }))
      .filter((p) => p.ratio + 0.005 < 4.5)
      .map((p) => `${p.ratio.toFixed(2)}:1  fg=${p.fg} bg=${p.bg} ${p.size}px/${p.weight}`)
    expect(failures, 'inspector contrast below AA').toEqual([])
  })

  test('lets an operator record their own reading beside the engine hypothesis', async ({
    page,
  }, testInfo) => {
    // The system states what it believes and why. An operator who disagrees
    // must be able to say so in the same ledger rather than only in their head.
    test.skip(testInfo.project.name !== 'no-token', MOCKED)
    await mockOperationalWorkflow(page)
    await page.goto('/tracks/trk-demo')

    const panel = page.locator('section[aria-labelledby="x-override"]')
    await expect(panel).toBeVisible()
    await panel.getByLabel('Your reading').fill('unarmed civilian craft')
    await panel.getByLabel('Confidence').fill('0.4')
    await panel.getByLabel('Source').fill('village report 14:02')
    await panel.getByRole('button', { name: 'Record classification' }).click()

    // It joins the hypotheses rather than replacing them: the engine's reading
    // is still standing, and both are listed.
    const hypotheses = page.locator('section[aria-labelledby="x-classifications"]')
    await expect(hypotheses.getByText('unarmed civilian craft')).toBeVisible()
    await expect(hypotheses.getByText('vehicle')).toBeVisible()
  })

  test('states the provenance integrity checks in words, not as a coloured bar', async ({
    page,
  }, testInfo) => {
    test.skip(testInfo.project.name !== 'no-token', MOCKED)
    await mockOperationalWorkflow(page)
    await page.goto('/tracks/trk-demo')

    const checks = page.locator('.aegis-xcheck')
    await expect(checks).toHaveCount(4)
    // Each row carries a verdict in text; the left bar only repeats it.
    const texts = await checks.allInnerTexts()
    for (const text of texts) {
      expect(text.trim().length, `integrity check with no verdict: "${text}"`).toBeGreaterThan(0)
      expect(text).toMatch(/\d|yes|no|all|none/i)
    }
  })
})

test.describe('count agreement', () => {
  test.beforeEach(async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== 'no-token', MOCKED)
    await mockOperationalWorkflow(page)
  })

  test('a count of one reads in the singular wherever it is quoted to the operator', async ({ page }) => {
    await page.goto('/map')
    await expect(page.locator('[data-map-ready="true"]')).toBeVisible()

    // The demo picture carries a single asset. The dock quotes that count in words,
    // and "1 assets" in a status line reads as a defect in the number itself
    // rather than in the wording - so every place that quotes it has to agree.
    const dock = page.locator('.aegis-tray', { hasText: 'asset' })
    await expect(dock.first()).toContainText('1 asset')
    await expect(dock.first()).not.toContainText('1 assets')

    // The console quotes the same figures as numerals beside their layer name.
    // It is the surface the operator scans fastest, so it is the one where a
    // count that disagrees with the dock is most likely to be believed.
    const console_ = page.locator('.aegis-console')
    await expect(console_.getByRole('button', { name: /^Tracks/ })).toContainText('2')
    await expect(console_.getByRole('button', { name: /^Assets/ })).toContainText('1')
    await expect(console_).not.toContainText('1 assets')
  })
})
