/* Local, disposable full-stack QA. Never makes paid model, payment or email calls. */
const { spawn, spawnSync } = require('node:child_process')
const { createRequire } = require('node:module')
const crypto = require('node:crypto')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const appRequire = createRequire(path.join(root, 'web/default/package.json'))
const { chromium } = appRequire('playwright')
const { default: AxeBuilder } = appRequire('@axe-core/playwright')
const binary = process.env.QA_BINARY || path.join(os.tmpdir(), 'renewapi-obsidian-qa.exe')
const port = Number(process.env.QA_PORT || 39001)
const baseURL = `http://127.0.0.1:${port}`
const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'renewapi-obsidian-'))
const output = path.resolve(root, process.env.QA_OUT || 'qa-artifacts/obsidian')
const username = 'obsidian_qa'
const password = `Qa!${crypto.randomBytes(8).toString('hex')}`
const userName = 'obsidian_reader'
const userPassword = `Qa!${crypto.randomBytes(8).toString('hex')}`
const report = { screenshots: false, backend: 'real Go + disposable SQLite', checks: [], issues: [], pageErrors: [], consoleErrors: [], networkFailures: [], skipped: [] }
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms))
const routes = {
  public: ['/', '/sign-in', '/sign-up', '/forgot-password', '/about', '/pricing', '/privacy-policy', '/user-agreement', '/rankings', '/401', '/403', '/404', '/500', '/503'],
  admin: ['/dashboard/overview', '/dashboard/models', '/dashboard/channels', '/dashboard/users', '/keys', '/usage-logs/common', '/usage-logs/drawing', '/usage-logs/task', '/channels', '/channels/new', '/users', '/models/metadata', '/profile', '/wallet', '/subscriptions', '/redemption-codes', '/playground', '/system-settings/site', '/system-settings/auth', '/system-settings/billing', '/system-settings/models', '/system-settings/security', '/system-settings/operations', '/system-settings/content'],
  user: ['/dashboard/overview', '/keys', '/usage-logs/common', '/profile', '/wallet', '/playground'],
}
const widths = process.env.QA_QUICK === '1' ? [1440, 390] : [1440, 1280, 1024, 768, 390]
const modes = ['dark', 'light']
let child
let browser
let serverLog

function assert(value, message) { if (!value) throw new Error(message) }
async function check(name, run) {
  try { await run(); report.checks.push({ name, pass: true }) }
  catch (error) { report.issues.push({ name, error: error.message }); console.log('FAIL', name, error.message.slice(0, 200)) }
}
async function responseJson(response) {
  const body = await response.json()
  assert(response.ok() && body.success, `API ${new URL(response.url()).pathname} failed (${response.status()}): ${String(body.message || 'business failure').replaceAll(password, '[redacted]').replaceAll(userPassword, '[redacted]')}`)
  return body
}
async function context(mode) {
  const ctx = await browser.newContext({ baseURL, viewport: { width: 1440, height: 1000 }, locale: 'en-US', colorScheme: mode, reducedMotion: 'reduce' })
  await ctx.addCookies([{ name: 'vite-ui-theme', value: mode, url: baseURL }])
  return ctx
}
async function login(ctx, name, secret) {
  const page = await ctx.newPage()
  await page.goto('/sign-in', { waitUntil: 'networkidle' })
  await page.locator('input[name="username"]').fill(name)
  await page.locator('input[name="password"]').fill(secret)
  const loginResponse = page.waitForResponse(response => response.url().includes('/api/user/login') && response.request().method() === 'POST')
  await page.locator('form button[type="submit"]').click()
  const body = await responseJson(await loginResponse)
  await page.waitForURL(/\/dashboard/)
  await page.close()
  return body.data
}
function diagnostics(page, label) {
  page.on('response', response => { if (response.status() >= 400) report.networkFailures.push({ label, path: new URL(response.url()).pathname, status: response.status() }) })
  page.on('pageerror', error => report.pageErrors.push({ label, message: error.message }))
  page.on('console', message => { if (message.type() === 'error') report.consoleErrors.push({ label, message: message.text().slice(0, 400) }) })
}
async function audit(ctx, role, mode, width, route) {
  const page = await ctx.newPage()
  diagnostics(page, `${role}:${mode}:${width}:${route}`)
  try {
    await page.setViewportSize({ width, height: width < 768 ? 900 : 1000 })
    await page.goto(route, { waitUntil: 'networkidle', timeout: 45000 })
    await page.waitForTimeout(220)
    const geometry = await page.evaluate(() => {
      const main = document.querySelector('main') || document.body
      return {
        path: location.pathname,
        textLength: main.innerText.trim().length,
        documentWidth: document.documentElement.scrollWidth,
        viewportWidth: innerWidth,
        mainCount: document.querySelectorAll('main').length,
        fatal: document.body.innerText.includes('Something went wrong') && !document.querySelector('h1'),
        theme: document.documentElement.className,
        panels: [...document.querySelectorAll('[data-ui="data-table-card"], [data-slot="card"], header')].filter(element => element.getBoundingClientRect().width > 0 && element.scrollWidth > element.clientWidth + 4).map(element => ({ slot: element.getAttribute('data-slot') || element.getAttribute('data-ui') || element.tagName, width: element.clientWidth, scroll: element.scrollWidth })).slice(0, 8),
      }
    })
    assert(geometry.textLength > 10, `Empty page: ${JSON.stringify(geometry)}`)
    assert(!geometry.fatal, 'Route crashed')
    assert(geometry.documentWidth <= width + 2, `Document overflow: ${JSON.stringify(geometry)}`)
    assert(geometry.mainCount <= 1, `Multiple main landmarks: ${geometry.mainCount}`)
    if (role !== 'public') assert(!geometry.path.includes('sign-in'), 'Authenticated route redirected to login')
    if (geometry.panels.length) report.issues.push({ name: `contained-overflow:${role}:${mode}:${width}:${route}`, details: geometry.panels })
    if (width === 1440 && process.env.QA_AXE !== '0') {
      const axe = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze()
      if (axe.violations.length) report.issues.push({ name: `axe:${role}:${mode}:${route}`, violations: axe.violations.map(item => ({ id: item.id, impact: item.impact, nodes: item.nodes.slice(0, 8).map(node => ({ target: node.target, html: node.html.slice(0, 1200), summary: node.failureSummary })) })) })
    }
  } finally { await page.close() }
}

;(async () => {
  assert(fs.existsSync(binary), `Build backend first, then set QA_BINARY. Missing: ${binary}`)
  fs.mkdirSync(output, { recursive: true })
  const env = { ...process.env, PORT: String(port), SQLITE_PATH: path.join(temporary, 'qa.db'), SESSION_SECRET: crypto.randomBytes(32).toString('hex'), COOKIE_SECURE: 'false', GIN_MODE: 'release', DEBUG: 'false', MEMORY_CACHE_ENABLED: 'false', OFFICIAL_PRICE_SYNC_ENABLED: 'false', UPDATE_TASK: 'false', BATCH_UPDATE_ENABLED: 'false', NODE_TYPE: 'slave', GLOBAL_API_RATE_LIMIT: '5000', GLOBAL_WEB_RATE_LIMIT: '5000', PANEL_READ_RATE_LIMIT: '5000', PANEL_WRITE_RATE_LIMIT: '500', SEARCH_RATE_LIMIT: '1000', TZ: 'Asia/Shanghai' }
  const migration = spawnSync(binary, ['migrate', '--up'], { cwd: temporary, env, encoding: 'utf8', timeout: 90000 })
  assert(migration.status === 0, `Migration failed: ${migration.stderr?.slice(-1000)}`)
  serverLog = fs.openSync(path.join(temporary, 'backend.log'), 'w')
  child = spawn(binary, [], { cwd: temporary, env, stdio: ['ignore', serverLog, serverLog] })
  let ready = false
  for (let i = 0; i < 120; i++) {
    if (child.exitCode !== null) throw new Error('Backend exited before ready')
    try { if ((await fetch(`${baseURL}/api/setup`)).ok) { ready = true; break } } catch {}
    await sleep(500)
  }
  assert(ready, 'Backend readiness timed out')
  browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true })
  const rootContext = await context('dark')
  await responseJson(await rootContext.request.post('/api/setup', { data: { username, password, confirmPassword: password, SelfUseModeEnabled: false, DemoSiteEnabled: false } }))
  const rootUser = await login(rootContext, username, password)
  const rootHeaders = { 'New-Api-User': String(rootUser.id) }
  await responseJson(await rootContext.request.post('/api/user/', { headers: rootHeaders, data: { username: userName, password: userPassword, display_name: 'Obsidian QA reader', role: 1 } }))
  await check('real API token create-list-revoke', async () => {
    await responseJson(await rootContext.request.post('/api/token/', { headers: rootHeaders, data: { name: 'obsidian-qa-key', remain_quota: 10000, expired_time: -1, unlimited_quota: false, model_limits_enabled: false } }))
    const tokens = await responseJson(await rootContext.request.get('/api/token/?p=1&size=10', { headers: rootHeaders }))
    const items = Array.isArray(tokens.data) ? tokens.data : tokens.data.items
    const token = items.find(item => item.name === 'obsidian-qa-key')
    assert(token, 'Created token is absent from backend list')
    await responseJson(await rootContext.request.delete(`/api/token/${token.id}`, { headers: rootHeaders }))
  })
  await check('custom Markdown home remains supported', async () => {
    await responseJson(await rootContext.request.put('/api/option/', { headers: rootHeaders, data: { key: 'HomePageContent', value: '# Obsidian configured homepage\n\nContent from the real backend.' } }))
    const customContext = await context('dark'); const page = await customContext.newPage()
    try { await page.goto('/', { waitUntil: 'networkidle' }); assert((await page.locator('body').innerText()).includes('Obsidian configured homepage'), 'Custom content was replaced by landing page') }
    finally { await customContext.close(); await responseJson(await rootContext.request.put('/api/option/', { headers: rootHeaders, data: { key: 'HomePageContent', value: '' } })) }
  })
  if (process.env.QA_PREFLIGHT !== '1') {
    await check('command shortcut opens one accessible dialog', async () => {
      const page = await rootContext.newPage()
      try {
        await page.goto('/dashboard/overview', { waitUntil: 'networkidle' })
        await page.keyboard.press('Control+k')
        const dialog = page.getByRole('dialog')
        await dialog.waitFor()
        assert(await dialog.count() === 1, 'Duplicate command dialogs')
        await dialog.locator('input').first().fill('API')
        assert(await dialog.locator('[cmdk-item]').count() > 0, 'Command navigation search is empty')
        await page.keyboard.press('Escape')
        await dialog.waitFor({ state: 'hidden' })
      } finally { await page.close() }
    })
    await check('fresh browser defaults to light even on a dark operating system', async () => {
      const ctx = await browser.newContext({ baseURL, colorScheme: 'dark', reducedMotion: 'reduce' })
      try {
        const page = await ctx.newPage(); await page.goto('/', { waitUntil: 'networkidle' })
        assert(await page.locator('html').evaluate(element => element.classList.contains('light')), 'Fresh browser did not use light default')
        const animated = await page.locator('.obsidian-home').evaluate(element => [...element.querySelectorAll('*')].filter(node => getComputedStyle(node).animationName !== 'none').length)
        assert(animated === 0, 'Reduced-motion still animates homepage')
      } finally { await ctx.close() }
    })
    await check('sidebar collapse keeps navigation usable and mobile links close the drawer', async () => {
      const page = await rootContext.newPage()
      try {
        await page.setViewportSize({ width: 1440, height: 1000 })
        await page.goto('/dashboard/overview', { waitUntil: 'networkidle' })
        const sidebar = page.locator('[data-slot="sidebar"][data-state]')
        assert(await sidebar.locator('[data-nav-group="overview"] [data-slot="sidebar-group-label"]').count() === 0, 'Overview label is duplicated')
        const activeStyle = await sidebar.locator('a[aria-current="page"]').first().evaluate(element => {
          const style = getComputedStyle(element)
          const reference = document.createElement('span')
          reference.style.color = 'var(--sidebar-accent)'
          element.append(reference)
          const expectedBackground = getComputedStyle(reference).color
          reference.remove()
          return { background: style.backgroundColor, expectedBackground, shadow: style.boxShadow }
        })
        assert(activeStyle.background === activeStyle.expectedBackground && activeStyle.shadow !== 'none', 'Selected item must use the neutral surface and narrow marker')
        await page.locator('[data-slot="sidebar-trigger"]').click()
        await page.waitForFunction(() => document.querySelector('[data-slot="sidebar"][data-state]')?.getAttribute('data-state') === 'collapsed')
        await sidebar.locator('a[href="/keys"]').click()
        await page.waitForURL('**/keys')
        assert(await sidebar.getAttribute('data-state') === 'collapsed', 'Navigation reset the collapsed sidebar')
        await page.locator('[data-slot="sidebar-trigger"]').click()
        await page.waitForFunction(() => document.querySelector('[data-slot="sidebar"][data-state]')?.getAttribute('data-state') === 'expanded')
        await page.setViewportSize({ width: 390, height: 900 })
        await sidebar.waitFor({ state: 'detached' })
        await page.locator('[data-slot="sidebar-trigger"]').click()
        const mobile = page.locator('[data-slot="sidebar"][data-mobile="true"]')
        await mobile.waitFor({ state: 'visible' })
        await mobile.locator('a[href="/profile"]').click()
        await page.waitForURL('**/profile')
        await mobile.waitFor({ state: 'hidden' })
        assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 2), 'Mobile sidebar navigation overflows')
      } finally { await page.close() }
    })
    await check('saved dark preference and browser theme color remain consistent', async () => {
      const ctx = await browser.newContext({ baseURL, colorScheme: 'light' })
      await ctx.addCookies([{ name: 'vite-ui-theme', value: 'dark', url: baseURL }])
      try {
        const page = await ctx.newPage(); await page.goto('/', { waitUntil: 'networkidle' })
        assert(await page.locator('html').evaluate(element => element.classList.contains('dark')), 'Explicit dark preference was lost')
        assert(await page.evaluate(() => document.querySelector('meta[name="theme-color"]')?.getAttribute('content') === getComputedStyle(document.documentElement).getPropertyValue('--background').trim()), 'Browser theme color differs from the page')
      } finally { await ctx.close() }
    })
    await check('saved system preference follows operating-system changes', async () => {
      const ctx = await browser.newContext({ baseURL, colorScheme: 'light' })
      await ctx.addCookies([{ name: 'vite-ui-theme', value: 'system', url: baseURL }])
      try {
        const page = await ctx.newPage(); await page.goto('/', { waitUntil: 'networkidle' })
        assert(await page.locator('html').evaluate(element => element.classList.contains('light')), 'System light preference lost')
        await page.emulateMedia({ colorScheme: 'dark' })
        await page.waitForFunction(() => document.documentElement.classList.contains('dark'))
      } finally { await ctx.close() }
    })
  }
  if (process.env.QA_MAPPING_FLOWS === '1') {
    await require('./qa/menu-mapping-flows.cjs')({ context: rootContext, rootHeaders, check, responseJson })
  }
  await rootContext.close()
  for (const mode of (process.env.QA_PREFLIGHT === '1' ? [] : modes)) {
    for (const role of ['public', 'admin', 'user']) {
      const ctx = await context(mode)
      if (role === 'admin') await login(ctx, username, password)
      if (role === 'user') await login(ctx, userName, userPassword)
      try {
        for (const width of widths) for (const route of routes[role]) {
          const name = `${role}:${mode}:${width}:${route}`
          console.log('CHECK', name)
          await check(name, () => audit(ctx, role, mode, width, route))
        }
        if (role === 'user') await check(`user:${mode}:admin-route-protection`, async () => {
          const page = await ctx.newPage()
          try { await page.goto('/system-settings', { waitUntil: 'networkidle' }); assert(!new URL(page.url()).pathname.startsWith('/system-settings'), 'Root settings route allowed ordinary user') }
          finally { await page.close() }
        })
      } finally { await ctx.close() }
    }
  }
  report.skipped.push('No real payment, paid relay, external email or third-party OAuth was executed. External integrations remain unverified.')
})().catch(error => { report.issues.push({ name: 'harness', error: error.stack }); process.exitCode = 1 }).finally(async () => {
  if (browser) await browser.close()
  if (child && child.exitCode === null) { child.kill(); await Promise.race([new Promise(resolve => child.once('exit', resolve)), sleep(4000)]) }
  if (serverLog !== undefined) fs.closeSync(serverLog)
  fs.mkdirSync(output, { recursive: true })
  const reportName = process.env.QA_PREFLIGHT === '1' ? 'preflight-report.json' : 'browser-report.json'
  fs.writeFileSync(path.join(output, reportName), JSON.stringify(report, null, 2))
  console.log(JSON.stringify({ checks: report.checks.length, issues: report.issues.length, pageErrors: report.pageErrors.length, consoleErrors: report.consoleErrors.length, output }, null, 2))
  if (report.issues.length || report.pageErrors.length) process.exitCode = 1
})
