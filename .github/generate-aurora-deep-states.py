#!/usr/bin/env python3
from pathlib import Path

base = Path('/tmp/aurora-final-visual-qa.mjs').read_text()
marker = "await fs.rm(outDir,{recursive:true,force:true});"
if marker not in base:
    raise SystemExit('base fixture runtime marker not found')

prefix = base.split(marker, 1)[0]
append = r'''
const deepOutDir = process.env.QA_OUT || 'qa-artifacts/aurora-deep-states'
const issues = []
const requestLog = []
const unhandled = []
const consoleErrors = []
const pageErrors = []

const featureRoutes = {
  channels: ['/api/channel', '/api/channel/', '/api/channel/search'],
  keys: ['/api/token/', '/api/token/search'],
  logs: ['/api/log/', '/api/log', '/api/log/self'],
  models: ['/api/models/', '/api/models/search'],
  users: ['/api/user/', '/api/user', '/api/user/search'],
}

const featurePages = {
  channels: '/channels',
  keys: '/keys',
  logs: '/usage-logs/common',
  models: '/models/metadata',
  users: '/users',
}

const emptyTitles = {
  channels: ['No Channels Found', '未找到渠道'],
  keys: ['No API Keys Found', '未找到 API 密钥'],
  logs: ['No Logs Found', '未找到日志'],
  models: ['No Models Found', '未找到模型'],
  users: ['No Users Found', '未找到用户'],
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
const esc = (value) => String(value).replace(/[^a-zA-Z0-9_-]+/g, '-')

function addIssue(severity, scenario, message, details = {}) {
  issues.push({ severity, scenario, message, details })
}

function matchesFeature(feature, pathname) {
  return Boolean(featureRoutes[feature]?.includes(pathname))
}

function emptyPayload(feature, url) {
  const q = url.searchParams
  switch (feature) {
    case 'channels':
      return ok({
        items: [],
        total: 0,
        page: 1,
        page_size: Number(q.get('page_size') || 20),
        type_counts: {},
      })
    case 'keys':
      return ok({
        items: [],
        total: 0,
        page: 1,
        page_size: Number(q.get('size') || 20),
      })
    case 'logs':
      return ok({
        items: [],
        total: 0,
        page: 1,
        page_size: Number(q.get('page_size') || 100),
      })
    case 'models':
      return ok({
        items: [],
        total: 0,
        page: 1,
        page_size: Number(q.get('page_size') || 10),
        vendor_counts: { all: 0 },
      })
    case 'users':
      return ok({
        items: [],
        total: 0,
        page: 1,
        page_size: Number(q.get('page_size') || 20),
      })
    default:
      return ok([])
  }
}

function errorPayload(feature) {
  return {
    success: false,
    message: `QA_FORCED_${feature.toUpperCase()}_ERROR`,
    data: null,
  }
}

async function createContext(browser, scenario) {
  const language = scenario.language || 'en-US'
  const theme = scenario.theme || 'light'
  const context = await browser.newContext({
    viewport: scenario.viewport || { width: 1440, height: 1000 },
    deviceScaleFactor: 1,
    locale: language,
    timezoneId: 'Asia/Taipei',
    colorScheme: theme,
  })

  await context.addInitScript(
    ({ user, fixedNow, language, theme }) => {
      localStorage.setItem('user', JSON.stringify(user))
      localStorage.setItem('uid', String(user.id))
      localStorage.setItem('i18nextLng', language)
      localStorage.setItem('theme', theme)
      const NativeDate = Date
      class FixedDate extends NativeDate {
        constructor(...args) {
          super(...(args.length ? args : [fixedNow]))
        }
        static now() {
          return fixedNow
        }
      }
      Date = FixedDate
    },
    { user, fixedNow, language, theme }
  )

  await context.route('**/api/**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    requestLog.push(
      `${scenario.name} ${request.method()} ${url.pathname}${url.search}`
    )

    let payload = null
    if (request.method() === 'GET' && matchesFeature(scenario.feature, url.pathname)) {
      if (scenario.mode === 'loading') {
        await sleep(2500)
      } else if (scenario.mode === 'empty') {
        payload = emptyPayload(scenario.feature, url)
      } else if (scenario.mode === 'error') {
        payload = errorPayload(scenario.feature)
      }
    }

    if (payload == null) {
      payload = responseFor(url)
    }
    if (payload == null) {
      unhandled.push(
        `${scenario.name} ${request.method()} ${url.pathname}${url.search}`
      )
      payload = { success: false, message: 'QA mock missing' }
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(payload),
    })
  })

  return context
}

async function preparePage(context, scenario) {
  const page = await context.newPage()
  page.on('console', (message) => {
    if (message.type() === 'error') {
      consoleErrors.push({ scenario: scenario.name, text: message.text() })
    }
  })
  page.on('pageerror', (error) => {
    pageErrors.push({
      scenario: scenario.name,
      message: error.message,
      stack: error.stack,
    })
  })

  await page.goto(`${APP}${scenario.route || featurePages[scenario.feature]}`, {
    waitUntil: 'domcontentloaded',
  })
  await page.addStyleTag({
    content:
      '*{animation:none!important;transition:none!important;caret-color:transparent!important}',
  })
  return page
}

async function snapshot(page, scenario, suffix = '') {
  await fs.mkdir(path.join(deepOutDir, scenario.theme || 'light'), {
    recursive: true,
  })
  const file = `${esc(scenario.name)}${suffix ? `-${esc(suffix)}` : ''}.png`
  const outputPath = path.join(deepOutDir, scenario.theme || 'light', file)
  await page.screenshot({ path: outputPath, fullPage: false })
  return outputPath
}

async function auditViewport(page, scenario) {
  const metrics = await page.evaluate(() => ({
    width: window.innerWidth,
    height: window.innerHeight,
    scrollWidth: document.documentElement.scrollWidth,
    scrollHeight: document.documentElement.scrollHeight,
  }))
  if (metrics.scrollWidth > metrics.width + 1) {
    addIssue('P1', scenario.name, 'page-level horizontal overflow', metrics)
  }

  const overlays = await page
    .locator('[role="dialog"]:visible, [role="alertdialog"]:visible')
    .evaluateAll((nodes) =>
      nodes.map((node) => {
        const r = node.getBoundingClientRect()
        return {
          left: r.left,
          top: r.top,
          right: r.right,
          bottom: r.bottom,
          width: r.width,
          height: r.height,
        }
      })
    )
  overlays.forEach((box) => {
    if (
      box.left < -1 ||
      box.top < -1 ||
      box.right > metrics.width + 1 ||
      box.bottom > metrics.height + 1
    ) {
      addIssue('P1', scenario.name, 'dialog escapes viewport', box)
    }
  })
}

async function waitForBaseline(page, feature) {
  if (feature === 'models') {
    await page.getByText('gpt-5.6', { exact: true }).first().waitFor({
      state: 'visible',
      timeout: 15000,
    })
    return
  }
  const firstText = {
    channels: 'OpenAI-Main',
    keys: 'Production',
    logs: 'relay completed',
    users: 'alice',
  }[feature]
  if (firstText) {
    await page.getByText(firstText, { exact: true }).first().waitFor({
      state: 'visible',
      timeout: 15000,
    })
  }
}

async function runDataState(browser, feature, mode) {
  const scenario = {
    name: `${feature}-${mode}`,
    feature,
    mode,
    language: 'en-US',
    theme: 'light',
  }
  const context = await createContext(browser, scenario)
  try {
    const page = await preparePage(context, scenario)
    if (mode === 'loading') {
      await sleep(450)
      const skeletons = await page.locator('[data-slot="skeleton"]:visible').count()
      if (skeletons === 0) {
        addIssue('P2', scenario.name, 'loading state has no visible skeleton/progress surface')
      }
    } else {
      await sleep(900)
      const bodyText = await page.locator('body').innerText()
      const titleMatched = emptyTitles[feature].some((title) => bodyText.includes(title))
      if (mode === 'empty' && !titleMatched) {
        addIssue('P2', scenario.name, 'empty result has no explicit empty-state message', {
          expected: emptyTitles[feature],
        })
      }
      if (mode === 'error') {
        const marker = `QA_FORCED_${feature.toUpperCase()}_ERROR`
        if (!bodyText.includes(marker)) {
          addIssue('P2', scenario.name, 'forced API error is indistinguishable from an empty result', {
            marker,
          })
        }
        if (titleMatched) {
          addIssue('P2', scenario.name, 'error state simultaneously renders the normal empty-state message', {
            emptyTitles: emptyTitles[feature],
          })
        }
      }
    }
    await snapshot(page, scenario)
    await auditViewport(page, scenario)
  } finally {
    await context.close()
  }
}

async function openManagement(page) {
  const button = page.getByRole('button', {
    name: /Manage models|Close management/i,
  })
  await button.waitFor({ state: 'visible', timeout: 15000 })
  if ((await button.getAttribute('aria-expanded')) !== 'true') {
    await button.click()
  }
  await page.locator('tbody').first().waitFor({ state: 'visible', timeout: 10000 })
}

async function openRowDelete(page, feature) {
  if (feature === 'models') {
    await openManagement(page)
  }
  const trigger = page.locator('tbody [data-slot="dropdown-menu-trigger"]').first()
  await trigger.waitFor({ state: 'visible', timeout: 10000 })
  await trigger.click()
  const menu = page.locator('[data-slot="dropdown-menu-content"]:visible')
  await menu.waitFor({ state: 'visible', timeout: 5000 })
  let deleteItem = menu.locator('[data-slot="dropdown-menu-item"].text-destructive').last()
  if ((await deleteItem.count()) === 0) {
    deleteItem = menu.locator('[data-slot="dropdown-menu-item"]').last()
  }
  await deleteItem.click()
  const dialog = page.locator('[role="alertdialog"]:visible, [role="dialog"]:visible').last()
  await dialog.waitFor({ state: 'visible', timeout: 5000 })
  return dialog
}

async function runDeleteState(browser, feature, language = 'en-US', theme = 'light') {
  const scenario = {
    name: `${feature}-delete-${language}-${theme}`,
    feature,
    mode: 'normal',
    language,
    theme,
  }
  const context = await createContext(browser, scenario)
  try {
    const page = await preparePage(context, scenario)
    await waitForBaseline(page, feature)
    const dialog = await openRowDelete(page, feature)
    const dialogText = await dialog.innerText()
    if (language.startsWith('zh') && dialogText.includes('Are you sure you want to delete')) {
      addIssue('P2', scenario.name, 'delete confirmation description bypasses i18n', {
        dialogText,
      })
    }
    await snapshot(page, scenario)
    await auditViewport(page, scenario)
  } finally {
    await context.close()
  }
}

async function selectTwoRows(page, feature) {
  if (feature === 'models') {
    await openManagement(page)
  }
  const checkboxes = page.locator('tbody [role="checkbox"]')
  const count = await checkboxes.count()
  if (count < 2) {
    throw new Error(`${feature}: expected at least 2 selectable rows, found ${count}`)
  }
  await checkboxes.nth(0).click()
  await checkboxes.nth(1).click()
  const toolbar = page.getByRole('toolbar')
  await toolbar.waitFor({ state: 'visible', timeout: 5000 })
  return toolbar
}

async function runBulkState(browser, feature, language = 'en-US') {
  const scenario = {
    name: `${feature}-bulk-${language}`,
    feature,
    mode: 'normal',
    language,
    theme: 'light',
  }
  const context = await createContext(browser, scenario)
  try {
    const page = await preparePage(context, scenario)
    await waitForBaseline(page, feature)
    const toolbar = await selectTwoRows(page, feature)
    const ariaLabel = (await toolbar.getAttribute('aria-label')) || ''
    const liveText = await page.locator('[role="status"]').last().innerText().catch(() => '')
    if (language.startsWith('zh')) {
      if (/Bulk actions for/i.test(ariaLabel)) {
        addIssue('P2', scenario.name, 'bulk toolbar accessible name is hard-coded English', {
          ariaLabel,
        })
      }
      if (/selected\. Bulk actions toolbar is available\./i.test(liveText)) {
        addIssue('P2', scenario.name, 'bulk selection live announcement is hard-coded English', {
          liveText,
        })
      }
    }
    await snapshot(page, scenario, 'toolbar')

    const buttons = toolbar.locator('button')
    if ((await buttons.count()) > 1) {
      await buttons.last().click()
      const dialog = page.locator('[role="dialog"]:visible, [role="alertdialog"]:visible').last()
      await dialog.waitFor({ state: 'visible', timeout: 5000 })
      await snapshot(page, scenario, 'confirm')
    }
    await auditViewport(page, scenario)
  } finally {
    await context.close()
  }
}

async function runChannelsSearch(browser) {
  const scenario = {
    name: 'channels-filter-search',
    feature: 'channels',
    mode: 'normal',
    language: 'en-US',
    theme: 'light',
  }
  const context = await createContext(browser, scenario)
  try {
    const page = await preparePage(context, scenario)
    await waitForBaseline(page, 'channels')
    const search = page.getByPlaceholder(/Filter.*name|name.*ID|名称/i).first()
    await search.fill('OpenAI')
    await sleep(850)
    const relevant = requestLog.filter((line) => line.startsWith(`${scenario.name} `))
    if (!relevant.some((line) => line.includes('/api/channel/search'))) {
      addIssue('P2', scenario.name, 'search interaction did not reach channel search API', {
        relevant,
      })
    }
    await snapshot(page, scenario)
    await auditViewport(page, scenario)
  } finally {
    await context.close()
  }
}

async function runLogsPagination(browser) {
  const scenario = {
    name: 'logs-pagination-page-2',
    feature: 'logs',
    mode: 'normal',
    language: 'en-US',
    theme: 'light',
  }
  const context = await createContext(browser, scenario)
  try {
    const page = await preparePage(context, scenario)
    await waitForBaseline(page, 'logs')
    const next = page.getByRole('button', { name: /next page/i })
    await next.click()
    await sleep(650)
    const relevant = requestLog.filter((line) => line.startsWith(`${scenario.name} `))
    if (!relevant.some((line) => /\/api\/log.*[?&]p=2/.test(line))) {
      addIssue('P2', scenario.name, 'pagination did not request page 2', { relevant })
    }
    await snapshot(page, scenario)
    await auditViewport(page, scenario)
  } finally {
    await context.close()
  }
}

async function runModelEdit(browser) {
  const scenario = {
    name: 'models-management-edit',
    feature: 'models',
    mode: 'normal',
    language: 'en-US',
    theme: 'light',
  }
  const context = await createContext(browser, scenario)
  try {
    const page = await preparePage(context, scenario)
    await waitForBaseline(page, 'models')
    await openManagement(page)
    const trigger = page.locator('tbody [data-slot="dropdown-menu-trigger"]').first()
    await trigger.click()
    const menu = page.locator('[data-slot="dropdown-menu-content"]:visible')
    await menu.waitFor({ state: 'visible', timeout: 5000 })
    await menu.locator('[data-slot="dropdown-menu-item"]').first().click()
    const overlay = page
      .locator('[data-slot="sheet-content"]:visible, [role="dialog"]:visible')
      .last()
    await overlay.waitFor({ state: 'visible', timeout: 5000 })
    await snapshot(page, scenario)
    await auditViewport(page, scenario)
  } finally {
    await context.close()
  }
}

async function runLateA11yRegression(browser) {
  const dashboardScenario = {
    name: 'dashboard-request-trend-a11y',
    route: '/dashboard/overview',
    mode: 'normal',
    language: 'en-US',
    theme: 'light',
  }
  let context = await createContext(browser, dashboardScenario)
  try {
    const page = await preparePage(context, dashboardScenario)
    await page.getByText('128,402').first().waitFor({ state: 'visible', timeout: 15000 })
    const trendItems = page.locator('ol.sr-only[aria-label] li')
    const count = await trendItems.count()
    if (count !== 24) {
      addIssue('P2', dashboardScenario.name, 'request trend is not fully exposed to assistive technology', {
        expected: 24,
        actual: count,
      })
    }
    await snapshot(page, dashboardScenario)
  } finally {
    await context.close()
  }

  const keyScenario = {
    name: 'api-key-trigger-localization',
    feature: 'keys',
    mode: 'normal',
    language: 'zh-CN',
    theme: 'light',
  }
  context = await createContext(browser, keyScenario)
  try {
    const page = await preparePage(context, keyScenario)
    await waitForBaseline(page, 'keys')
    const trigger = page.locator('tbody button[aria-label]').filter({ hasText: 'sk-' }).first()
    const label = (await trigger.getAttribute('aria-label')) || ''
    if (!label || label === 'Show full API key') {
      addIssue('P2', keyScenario.name, 'API-key reveal trigger uses an untranslated or empty accessible label', {
        label,
      })
    }
    await snapshot(page, keyScenario)
  } finally {
    await context.close()
  }
}

await fs.rm(deepOutDir, { recursive: true, force: true })
await fs.mkdir(deepOutDir, { recursive: true })
const browser = await chromium.launch({ headless: true })

try {
  for (const feature of ['channels', 'keys', 'logs', 'models', 'users']) {
    for (const mode of ['loading', 'empty', 'error']) {
      await runDataState(browser, feature, mode)
    }
  }

  for (const feature of ['channels', 'keys', 'models', 'users']) {
    await runDeleteState(browser, feature)
  }
  await runDeleteState(browser, 'channels', 'zh-CN')
  await runDeleteState(browser, 'models', 'zh-CN', 'dark')

  for (const feature of ['channels', 'keys', 'models']) {
    await runBulkState(browser, feature)
  }
  await runBulkState(browser, 'channels', 'zh-CN')

  await runChannelsSearch(browser)
  await runLogsPagination(browser)
  await runModelEdit(browser)
  await runLateA11yRegression(browser)
} catch (error) {
  addIssue('P0', 'harness', 'deep-state QA harness crashed', {
    message: error instanceof Error ? error.message : String(error),
    stack: error instanceof Error ? error.stack : null,
  })
} finally {
  await browser.close()
}

const severityRank = { P0: 0, P1: 1, P2: 2, P3: 3 }
issues.sort((a, b) => severityRank[a.severity] - severityRank[b.severity])
const actionable = issues.filter((issue) => ['P0', 'P1', 'P2'].includes(issue.severity))

const report = {
  generatedAt: new Date().toISOString(),
  productHead: process.env.QA_PRODUCT_HEAD || null,
  result: actionable.length === 0 ? 'passed' : 'failed',
  issueCounts: {
    P0: issues.filter((issue) => issue.severity === 'P0').length,
    P1: issues.filter((issue) => issue.severity === 'P1').length,
    P2: issues.filter((issue) => issue.severity === 'P2').length,
    P3: issues.filter((issue) => issue.severity === 'P3').length,
  },
  issues,
  consoleErrors,
  pageErrors,
  unhandled,
}

await fs.writeFile(path.join(deepOutDir, 'report.json'), `${JSON.stringify(report, null, 2)}\n`)
await fs.writeFile(path.join(deepOutDir, 'requests.txt'), requestLog.join('\n'))
await fs.writeFile(path.join(deepOutDir, 'unhandled.txt'), unhandled.join('\n'))
await fs.writeFile(path.join(deepOutDir, 'console-errors.txt'), consoleErrors.map((item) => `${item.scenario}: ${item.text}`).join('\n'))
await fs.writeFile(path.join(deepOutDir, 'page-errors.txt'), pageErrors.map((item) => `${item.scenario}: ${item.message}`).join('\n'))

console.log(JSON.stringify(report, null, 2))
if (consoleErrors.length > 0 || pageErrors.length > 0 || unhandled.length > 0 || actionable.length > 0) {
  process.exitCode = 1
}
'''

Path('/tmp/aurora-deep-states.mjs').write_text(prefix + append)
