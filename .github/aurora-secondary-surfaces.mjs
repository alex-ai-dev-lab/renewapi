import { spawn } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'
import { createRequire } from 'node:module'
import { chromium } from 'playwright'

const require = createRequire(import.meta.url)
const axeSource = fs.readFileSync(require.resolve('axe-core/axe.min.js'), 'utf8')

const binary = process.env.QA_BINARY
const dbPath = process.env.QA_DB
const outDir = path.resolve(
  process.env.QA_OUT || '../../qa-artifacts/aurora-secondary-surfaces'
)
const port = Number(process.env.QA_PORT || 4173)
const baseURL = `http://127.0.0.1:${port}`
const username = 'qaadmin'
const password = 'QaRoot123!'
const sessionSecret = 'aurora-secondary-surfaces-2026-08-24'

if (!binary || !dbPath) {
  throw new Error('QA_BINARY and QA_DB are required')
}

const publicCases = [
  ['sign-in', '/sign-in'],
  ['sign-up', '/sign-up'],
  ['forgot-password', '/forgot-password'],
  ['about', '/about'],
  ['pricing', '/pricing'],
  ['privacy-policy', '/privacy-policy'],
  ['terms-of-service', '/terms-of-service'],
  ['rankings', '/rankings'],
  ['error-401', '/401'],
  ['error-403', '/403'],
  ['error-404', '/404'],
  ['error-500', '/500'],
  ['error-503', '/503'],
]

const authenticatedCases = [
  ['profile', '/profile'],
  ['wallet', '/wallet'],
  ['subscriptions', '/subscriptions'],
  ['playground', '/playground'],
  ['redemption-codes', '/redemption-codes'],
]

const themes = ['light', 'dark']
const viewports = [
  ['desktop', { width: 1440, height: 1000 }],
  ['mobile', { width: 375, height: 812 }],
]

fs.mkdirSync(outDir, { recursive: true })
fs.mkdirSync(path.join(outDir, 'screenshots'), { recursive: true })
fs.mkdirSync(path.join(outDir, 'aria'), { recursive: true })

const observations = []
const failures = []
const consoleErrors = []
const pageErrors = []
let serverSequence = 0

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function sanitize(value) {
  return String(value).replace(/[^A-Za-z0-9_.-]+/g, '-')
}

function normalizePathname(value) {
  if (value === '/') return '/'
  return value.replace(/\/+$/, '') || '/'
}

async function readJson(response) {
  const text = await response.text()
  try {
    return JSON.parse(text)
  } catch {
    throw new Error(
      `Expected JSON from ${response.url()}, got: ${text.slice(0, 500)}`
    )
  }
}

function waitForReadableEnd(stream) {
  if (!stream || stream.readableEnded || stream.destroyed) return Promise.resolve()
  return new Promise((resolve) => {
    let settled = false
    const done = () => {
      if (settled) return
      settled = true
      resolve()
    }
    stream.once('end', done)
    stream.once('close', done)
  })
}

function endWritable(stream) {
  if (!stream || stream.writableFinished || stream.destroyed) return Promise.resolve()
  return new Promise((resolve) => {
    stream.once('finish', resolve)
    stream.end()
  })
}

function waitForChildClose(child) {
  if (
    child.exitCode !== null &&
    child.stdout?.readableEnded &&
    child.stderr?.readableEnded
  ) {
    return Promise.resolve({ code: child.exitCode, signal: child.signalCode })
  }
  return new Promise((resolve) => {
    child.once('close', (code, signal) => resolve({ code, signal }))
  })
}

async function waitForBackend(child) {
  let lastError = null
  for (let attempt = 0; attempt < 120; attempt += 1) {
    if (child.exitCode !== null) {
      throw new Error(`RenewAPI exited before ready with code ${child.exitCode}`)
    }
    try {
      const response = await fetch(`${baseURL}/api/setup`)
      if (response.ok) return
      lastError = new Error(`setup returned ${response.status}`)
    } catch (error) {
      lastError = error
    }
    await sleep(500)
  }
  throw new Error(`RenewAPI did not become ready: ${lastError}`)
}

async function startBackend(label) {
  serverSequence += 1
  const logPath = path.join(outDir, `backend-${serverSequence}-${label}.log`)
  const log = fs.createWriteStream(logPath, { flags: 'w' })
  const child = spawn(binary, [], {
    cwd: path.dirname(binary),
    env: {
      ...process.env,
      PORT: String(port),
      SQLITE_PATH: dbPath,
      SESSION_SECRET: sessionSecret,
      COOKIE_SECURE: 'false',
      GIN_MODE: 'release',
      DEBUG: 'false',
      MEMORY_CACHE_ENABLED: 'false',
      OFFICIAL_PRICE_SYNC_ENABLED: 'false',
      UPDATE_TASK: 'false',
      BATCH_UPDATE_ENABLED: 'false',
      NODE_TYPE: 'slave',
      TZ: 'Asia/Taipei',
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  child.stdout.pipe(log, { end: false })
  child.stderr.pipe(log, { end: false })
  const server = { child, log, logPath, closed: false }
  try {
    await waitForBackend(child)
  } catch (error) {
    await stopBackend(server, 'readiness-failure').catch(() => {})
    throw error
  }
  observations.push({ type: 'backend-start', label, logPath })
  return server
}

async function stopBackend(server, reason = 'normal') {
  if (!server || server.closed) return
  server.closed = true
  const { child, log } = server
  const closePromise = waitForChildClose(child)
  if (child.exitCode === null) child.kill('SIGTERM')
  let closeResult = await Promise.race([
    closePromise.then((value) => ({ closed: true, value })),
    sleep(12000).then(() => ({ closed: false, value: null })),
  ])
  if (!closeResult.closed) {
    child.kill('SIGKILL')
    closeResult = { closed: true, value: await closePromise }
  }
  await Promise.all([
    waitForReadableEnd(child.stdout),
    waitForReadableEnd(child.stderr),
  ])
  await endWritable(log)
  observations.push({
    type: 'backend-close',
    reason,
    code: closeResult.value?.code ?? child.exitCode,
    signal: closeResult.value?.signal ?? child.signalCode,
  })
}

async function setupRoot(browser) {
  const context = await browser.newContext({ baseURL })
  try {
    const setupResponse = await context.request.get(`${baseURL}/api/setup`)
    const setupBody = await readJson(setupResponse)
    if (!setupBody.success) throw new Error('GET /api/setup failed')
    if (!setupBody.data?.status) {
      const initializeResponse = await context.request.post(`${baseURL}/api/setup`, {
        data: {
          username,
          password,
          confirmPassword: password,
          SelfUseModeEnabled: false,
          DemoSiteEnabled: false,
        },
      })
      const initializeBody = await readJson(initializeResponse)
      if (!initializeBody.success) {
        throw new Error(`POST /api/setup failed: ${JSON.stringify(initializeBody)}`)
      }
    }
  } finally {
    await context.close()
  }
}

function createContext(browser, theme, viewport) {
  return browser.newContext({
    baseURL,
    viewport,
    deviceScaleFactor: 1,
    locale: 'en-US',
    timezoneId: 'Asia/Taipei',
    colorScheme: theme,
  })
}

async function seedClientState(context, theme, user = null) {
  await context.addInitScript(
    ({ selectedTheme, seedUser }) => {
      localStorage.setItem('i18nextLng', 'en-US')
      localStorage.setItem('theme', selectedTheme)
      if (seedUser) {
        localStorage.setItem('user', JSON.stringify(seedUser))
        localStorage.setItem('uid', String(seedUser.id))
      }
    },
    { selectedTheme: theme, seedUser: user }
  )
}

async function loginContext(context) {
  const page = await context.newPage()
  try {
    await page.goto(`${baseURL}/sign-in`, { waitUntil: 'domcontentloaded' })
    const result = await page.evaluate(
      async ({ loginUsername, loginPassword }) => {
        const response = await fetch('/api/user/login', {
          method: 'POST',
          credentials: 'include',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            username: loginUsername,
            password: loginPassword,
          }),
        })
        const text = await response.text()
        let body = null
        try {
          body = JSON.parse(text)
        } catch {
          body = { success: false, message: text.slice(0, 300) }
        }
        return { status: response.status, body }
      },
      { loginUsername: username, loginPassword: password }
    )
    if (
      result.status !== 200 ||
      result.body?.success !== true ||
      !result.body?.data?.id
    ) {
      throw new Error(`Root login failed: ${JSON.stringify(result)}`)
    }
    return result.body.data
  } finally {
    await page.close()
  }
}

function attachDiagnostics(page, label) {
  page.on('console', (message) => {
    if (message.type() === 'error') {
      consoleErrors.push({ label, text: message.text() })
    }
  })
  page.on('pageerror', (error) => {
    pageErrors.push({ label, message: error.message, stack: error.stack })
  })
}

async function waitForSurface(page) {
  await page.locator('body').waitFor({ state: 'visible', timeout: 20000 })
  await sleep(900)
  await page.evaluate(() => document.fonts?.ready)
}

async function runAxe(page) {
  await page.addScriptTag({ content: axeSource })
  return page.evaluate(async () => {
    return window.axe.run(document, {
      runOnly: {
        type: 'tag',
        values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'],
      },
      resultTypes: ['violations'],
    })
  })
}

function compactViolation(violation) {
  return {
    id: violation.id,
    impact: violation.impact,
    help: violation.help,
    helpUrl: violation.helpUrl,
    nodes: violation.nodes.slice(0, 5).map((node) => ({
      target: node.target,
      html: node.html,
      failureSummary: node.failureSummary,
    })),
  }
}

async function auditPage(context, testCase, theme, viewportName, authRequired) {
  const [id, route] = testCase
  const label = `${authRequired ? 'auth' : 'public'}-${id}-${theme}-${viewportName}`
  const page = await context.newPage()
  attachDiagnostics(page, label)
  const beforeConsole = consoleErrors.length
  const beforePageErrors = pageErrors.length
  try {
    const response = await page.goto(`${baseURL}${route}`, {
      waitUntil: 'domcontentloaded',
    })
    await waitForSurface(page)

    const currentURL = page.url()
    const currentPathname = normalizePathname(new URL(currentURL).pathname)
    const expectedPathname = normalizePathname(route)
    if (currentPathname !== expectedPathname) {
      failures.push({
        label,
        type: 'unexpected-route',
        expectedPathname,
        currentPathname,
        currentURL,
      })
    }
    if (authRequired && currentURL.includes('/sign-in')) {
      failures.push({ label, type: 'auth-redirect', currentURL })
    }
    if (!response || response.status() >= 500) {
      failures.push({
        label,
        type: 'navigation-status',
        status: response?.status() ?? null,
        currentURL,
      })
    }

    const overflow = await page.evaluate(() => {
      const root = document.documentElement
      const body = document.body
      return {
        rootScrollWidth: root.scrollWidth,
        rootClientWidth: root.clientWidth,
        bodyScrollWidth: body?.scrollWidth ?? 0,
        bodyClientWidth: body?.clientWidth ?? 0,
      }
    })
    const overflowed =
      overflow.rootScrollWidth > overflow.rootClientWidth + 1 ||
      overflow.bodyScrollWidth > overflow.bodyClientWidth + 1
    if (overflowed) failures.push({ label, type: 'horizontal-overflow', overflow })

    const axe = await runAxe(page)
    const violations = axe.violations.map(compactViolation)
    if (violations.length > 0) {
      failures.push({ label, type: 'axe', violations })
    }

    const ariaPath = path.join(outDir, 'aria', `${sanitize(label)}.txt`)
    try {
      const snapshot = await page.locator('body').ariaSnapshot({ timeout: 10000 })
      fs.writeFileSync(ariaPath, snapshot)
    } catch (error) {
      observations.push({
        type: 'aria-snapshot-unavailable',
        label,
        message: error instanceof Error ? error.message : String(error),
      })
    }

    await page.screenshot({
      path: path.join(outDir, 'screenshots', `${sanitize(label)}.png`),
      fullPage: true,
    })

    const caseConsoleErrors = consoleErrors.slice(beforeConsole)
    const casePageErrors = pageErrors.slice(beforePageErrors)
    if (caseConsoleErrors.length > 0) {
      failures.push({ label, type: 'console-errors', errors: caseConsoleErrors })
    }
    if (casePageErrors.length > 0) {
      failures.push({ label, type: 'page-errors', errors: casePageErrors })
    }

    observations.push({
      type: 'surface-audit',
      label,
      route,
      currentURL,
      theme,
      viewport: viewportName,
      authRequired,
      overflow,
      axeViolations: violations.length,
      consoleErrors: caseConsoleErrors.length,
      pageErrors: casePageErrors.length,
    })
  } catch (error) {
    failures.push({
      label,
      type: 'audit-exception',
      message: error instanceof Error ? error.message : String(error),
      stack: error instanceof Error ? error.stack : undefined,
    })
  } finally {
    await page.close()
  }
}

function artifactText() {
  const files = fs.readdirSync(outDir, { withFileTypes: true })
  const chunks = []
  for (const entry of files) {
    if (!entry.isFile()) continue
    const filePath = path.join(outDir, entry.name)
    if (!/\.(json|txt|log)$/i.test(entry.name)) continue
    chunks.push(fs.readFileSync(filePath, 'utf8'))
  }
  return chunks.join('\n')
}

async function main() {
  let server = null
  let browser = null
  try {
    server = await startBackend('secondary-surfaces')
    browser = await chromium.launch({ headless: true })
    await setupRoot(browser)

    for (const theme of themes) {
      for (const [viewportName, viewport] of viewports) {
        const publicContext = await createContext(browser, theme, viewport)
        await seedClientState(publicContext, theme)
        for (const testCase of publicCases) {
          await auditPage(publicContext, testCase, theme, viewportName, false)
        }
        await publicContext.close()

        const authContext = await createContext(browser, theme, viewport)
        const user = await loginContext(authContext)
        await seedClientState(authContext, theme, user)
        for (const testCase of authenticatedCases) {
          await auditPage(authContext, testCase, theme, viewportName, true)
        }
        await authContext.close()
      }
    }

    const summary = {
      passed: failures.length === 0,
      publicCases: publicCases.length,
      authenticatedCases: authenticatedCases.length,
      themes: themes.length,
      viewports: viewports.length,
      totalAudits:
        (publicCases.length + authenticatedCases.length) *
        themes.length *
        viewports.length,
      failures: failures.length,
      consoleErrors: consoleErrors.length,
      pageErrors: pageErrors.length,
    }
    fs.writeFileSync(
      path.join(outDir, 'observations.json'),
      JSON.stringify(observations, null, 2)
    )
    fs.writeFileSync(
      path.join(outDir, 'failures.json'),
      JSON.stringify(failures, null, 2)
    )
    fs.writeFileSync(
      path.join(outDir, 'summary.json'),
      JSON.stringify(summary, null, 2)
    )
    fs.writeFileSync(
      path.join(outDir, 'summary.txt'),
      [
        `passed=${summary.passed}`,
        `publicCases=${summary.publicCases}`,
        `authenticatedCases=${summary.authenticatedCases}`,
        `themes=${summary.themes}`,
        `viewports=${summary.viewports}`,
        `totalAudits=${summary.totalAudits}`,
        `failures=${summary.failures}`,
        `consoleErrors=${summary.consoleErrors}`,
        `pageErrors=${summary.pageErrors}`,
      ].join('\n') + '\n'
    )

    const text = artifactText()
    for (const forbidden of [password, sessionSecret]) {
      if (text.includes(forbidden)) {
        throw new Error('QA artifact contains a forbidden test credential')
      }
    }

    if (failures.length > 0) {
      throw new Error(`Secondary Surfaces gate found ${failures.length} failure(s)`)
    }
  } catch (error) {
    const failure = {
      message: error instanceof Error ? error.message : String(error),
      stack: error instanceof Error ? error.stack : undefined,
    }
    fs.writeFileSync(
      path.join(outDir, 'failure.json'),
      JSON.stringify(failure, null, 2)
    )
    throw error
  } finally {
    if (browser) await browser.close().catch(() => {})
    if (server) await stopBackend(server).catch(() => {})
  }
}

await main()
