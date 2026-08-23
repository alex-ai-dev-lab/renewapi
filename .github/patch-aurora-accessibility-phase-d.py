#!/usr/bin/env python3
from pathlib import Path

path = Path('/tmp/aurora-deep-states.mjs')
source = path.read_text()

source = source.replace(
    "    deviceScaleFactor: 1,",
    "    deviceScaleFactor: scenario.deviceScaleFactor || 1,",
    1,
)

arrays_marker = "const pageErrors = []\n"
if arrays_marker not in source:
    raise SystemExit('page error array marker not found')
source = source.replace(
    arrays_marker,
    arrays_marker
    + "const axeViolations = []\n"
    + "const axeIncomplete = []\n"
    + "const ariaSnapshots = []\n"
    + "const zoomEvidence = []\n",
    1,
)

pages_marker = """const featurePages = {
  channels: '/channels',
  keys: '/keys',
  logs: '/usage-logs/common',
  models: '/models/metadata',
  users: '/users',
}
"""
if pages_marker not in source:
    raise SystemExit('feature pages marker not found')
source = source.replace(
    pages_marker,
    pages_marker
    + """
const accessibilityPages = [
  { key: 'dashboard', route: '/dashboard/overview' },
  { key: 'channels', route: '/channels', feature: 'channels' },
  { key: 'keys', route: '/keys', feature: 'keys' },
  { key: 'logs', route: '/usage-logs/common', feature: 'logs' },
  { key: 'models', route: '/models/metadata', feature: 'models' },
  { key: 'users', route: '/users', feature: 'users' },
  { key: 'settings', route: '/system-settings' },
]
""",
    1,
)

issue_marker = """function addIssue(severity, scenario, message, details = {}) {
  issues.push({ severity, scenario, message, details })
}
"""
if issue_marker not in source:
    raise SystemExit('addIssue marker not found')
source = source.replace(
    issue_marker,
    issue_marker
    + r'''

async function runAxeAudit(page, scenarioName) {
  try {
    const hasAxe = await page.evaluate(() => Boolean(window.axe))
    if (!hasAxe) {
      await page.addScriptTag({ path: path.resolve('node_modules/axe-core/axe.min.js') })
    }
    const result = await page.evaluate(async () =>
      window.axe.run(document, {
        runOnly: {
          type: 'tag',
          values: ['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa'],
        },
      })
    )

    for (const violation of result.violations) {
      const details = {
        id: violation.id,
        impact: violation.impact,
        help: violation.help,
        helpUrl: violation.helpUrl,
        nodes: violation.nodes.slice(0, 8).map((node) => ({
          target: node.target,
          html: node.html.slice(0, 280),
          failureSummary: node.failureSummary,
        })),
      }
      axeViolations.push({ scenario: scenarioName, ...details })
      addIssue(
        ['critical', 'serious'].includes(violation.impact) ? 'P1' : 'P2',
        scenarioName,
        `axe ${violation.id}: ${violation.help}`,
        details
      )
    }

    const contrastIncomplete = result.incomplete.filter(
      (entry) => entry.id === 'color-contrast'
    )
    if (contrastIncomplete.length > 0) {
      const details = contrastIncomplete.map((entry) => ({
        id: entry.id,
        impact: entry.impact,
        help: entry.help,
        nodes: entry.nodes.slice(0, 12).map((node) => ({
          target: node.target,
          html: node.html.slice(0, 280),
          failureSummary: node.failureSummary,
        })),
      }))
      axeIncomplete.push({ scenario: scenarioName, entries: details })
      addIssue(
        'P3',
        scenarioName,
        'axe color-contrast requires manual review for one or more nodes',
        { entries: details }
      )
    }
  } catch (error) {
    addIssue('P0', scenarioName, 'axe accessibility audit crashed', {
      message: error instanceof Error ? error.message : String(error),
    })
  }
}

async function captureAriaSnapshot(page, scenarioName) {
  try {
    const main = page.locator('main').first()
    if ((await main.count()) === 0) return
    ariaSnapshots.push({
      scenario: scenarioName,
      snapshot: await main.ariaSnapshot(),
    })
  } catch (error) {
    addIssue('P2', scenarioName, 'ARIA snapshot capture failed', {
      message: error instanceof Error ? error.message : String(error),
    })
  }
}
''',
    1,
)

snapshot_marker = "async function snapshot(page, scenario, suffix = '') {\n"
if snapshot_marker not in source:
    raise SystemExit('snapshot marker not found')
source = source.replace(
    snapshot_marker,
    snapshot_marker
    + "  await runAxeAudit(page, `${scenario.name}${suffix ? `-${suffix}` : ''}`)\n",
    1,
)

runtime_marker = "await fs.rm(deepOutDir, { recursive: true, force: true })"
if runtime_marker not in source:
    raise SystemExit('runtime marker not found')
accessibility_runtime = r'''
async function waitForAccessibilityBaseline(page, key) {
  if (key === 'dashboard') {
    await page.getByText('128,402').first().waitFor({
      state: 'visible',
      timeout: 15000,
    })
  } else if (key === 'settings') {
    await page.locator('main').first().waitFor({ state: 'visible', timeout: 15000 })
    await page.waitForTimeout(1200)
  } else {
    await waitForBaseline(page, key)
  }
  await page.waitForTimeout(350)
}

async function runCoreAccessibilityBaseline(browser, theme) {
  for (const entry of accessibilityPages) {
    const scenario = {
      name: `a11y-baseline-${entry.key}-${theme}`,
      route: entry.route,
      feature: entry.feature,
      mode: 'normal',
      language: 'en-US',
      theme,
    }
    const context = await createContext(browser, scenario)
    try {
      const page = await preparePage(context, scenario)
      await waitForAccessibilityBaseline(page, entry.key)
      await captureAriaSnapshot(page, scenario.name)
      await snapshot(page, scenario)
      await auditViewport(page, scenario)
    } finally {
      await context.close()
    }
  }
}

async function runZoomEquivalent(browser, theme) {
  for (const entry of accessibilityPages) {
    const scenario = {
      name: `zoom200-equivalent-${entry.key}-${theme}`,
      route: entry.route,
      feature: entry.feature,
      mode: 'normal',
      language: 'en-US',
      theme,
      viewport: { width: 720, height: 500 },
      deviceScaleFactor: 2,
    }
    const context = await createContext(browser, scenario)
    try {
      const page = await preparePage(context, scenario)
      await waitForAccessibilityBaseline(page, entry.key)
      const metrics = await page.evaluate(() => ({
        innerWidth: window.innerWidth,
        innerHeight: window.innerHeight,
        devicePixelRatio: window.devicePixelRatio,
        scrollWidth: document.documentElement.scrollWidth,
        scrollHeight: document.documentElement.scrollHeight,
      }))
      zoomEvidence.push({ scenario: scenario.name, ...metrics })
      if (metrics.innerWidth !== 720 || metrics.devicePixelRatio < 1.9) {
        addIssue('P1', scenario.name, '200% high-DPI layout equivalence was not established', {
          expectedInnerWidth: 720,
          expectedDevicePixelRatio: 2,
          ...metrics,
        })
      }
      await captureAriaSnapshot(page, scenario.name)
      await snapshot(page, scenario)
      await auditViewport(page, scenario)
    } finally {
      await context.close()
    }
  }
}

'''
source = source.replace(runtime_marker, accessibility_runtime + runtime_marker, 1)

call_marker = "  await runLateA11yRegression(browser)\n"
if call_marker not in source:
    raise SystemExit('late a11y call marker not found')
source = source.replace(
    call_marker,
    call_marker
    + "  for (const theme of ['light', 'dark']) {\n"
    + "    await runCoreAccessibilityBaseline(browser, theme)\n"
    + "    await runZoomEquivalent(browser, theme)\n"
    + "  }\n",
    1,
)

report_marker = """  issues,
  consoleErrors,
  pageErrors,
  unhandled,
}
"""
if report_marker not in source:
    raise SystemExit('report marker not found')
source = source.replace(
    report_marker,
    """  issues,
  consoleErrors,
  pageErrors,
  unhandled,
  axeViolations,
  axeIncomplete,
  zoomEvidence,
  ariaSnapshotCount: ariaSnapshots.length,
}
""",
    1,
)

write_marker = "console.log(JSON.stringify(report, null, 2))\n"
if write_marker not in source:
    raise SystemExit('console report marker not found')
source = source.replace(
    write_marker,
    """await fs.writeFile(path.join(deepOutDir, 'axe-violations.json'), `${JSON.stringify(axeViolations, null, 2)}\\n`)
await fs.writeFile(path.join(deepOutDir, 'axe-incomplete.json'), `${JSON.stringify(axeIncomplete, null, 2)}\\n`)
await fs.writeFile(path.join(deepOutDir, 'zoom-evidence.json'), `${JSON.stringify(zoomEvidence, null, 2)}\\n`)
await fs.writeFile(path.join(deepOutDir, 'aria-snapshots.json'), `${JSON.stringify(ariaSnapshots, null, 2)}\\n`)

console.log(JSON.stringify(report, null, 2))
""",
    1,
)

path.write_text(source)
