import { spawn } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'
import { chromium } from 'playwright'

const binary = process.env.QA_BINARY
const dbPath = process.env.QA_DB
const outDir = path.resolve(process.env.QA_OUT || '../../qa-artifacts/settings-real-backend')
const port = Number(process.env.QA_PORT || 4173)
const baseURL = `http://127.0.0.1:${port}`
const username = 'qaadmin'
const password = 'QaRoot123!'
const sessionSecret = 'settings-real-backend-smoke-2026-08-23-9f47b2c1d6e8a305'

if (!binary || !dbPath) {
  throw new Error('QA_BINARY and QA_DB are required')
}

fs.mkdirSync(outDir, { recursive: true })

const observations = []
const consoleErrors = []
const pageErrors = []
let serverSequence = 0

function assert(condition, message, details) {
  if (condition) return
  const suffix = details === undefined ? '' : `\n${JSON.stringify(details, null, 2)}`
  throw new Error(`${message}${suffix}`)
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function readJson(response) {
  const text = await response.text()
  try {
    return JSON.parse(text)
  } catch {
    throw new Error(`Expected JSON from ${response.url()}, got: ${text.slice(0, 500)}`)
  }
}

async function waitForBackend(child) {
  let lastError = null
  for (let attempt = 0; attempt < 120; attempt += 1) {
    if (child.exitCode !== null) {
      throw new Error(`RenewAPI exited before becoming ready with code ${child.exitCode}`)
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
  child.stdout.pipe(log)
  child.stderr.pipe(log)
  await waitForBackend(child)
  observations.push({ type: 'backend-start', label, pid: child.pid, logPath })
  return { child, log }
}

async function stopBackend(server) {
  if (!server || server.child.exitCode !== null) return
  let exited = false
  const exitPromise = new Promise((resolve) => {
    server.child.once('exit', (code, signal) => {
      exited = true
      observations.push({ type: 'backend-exit', code, signal })
      resolve()
    })
  })
  server.child.kill('SIGTERM')
  await Promise.race([exitPromise, sleep(12000)])
  if (!exited) {
    server.child.kill('SIGKILL')
    await exitPromise
  }
  server.log.end()
}

async function setupAndLogin(context, { allowSetup }) {
  const setupResponse = await context.request.get(`${baseURL}/api/setup`)
  assert(setupResponse.ok(), 'GET /api/setup failed', { status: setupResponse.status() })
  const setupBody = await readJson(setupResponse)
  assert(setupBody.success === true, 'GET /api/setup business failure', setupBody)

  if (!setupBody.data?.status) {
    assert(allowSetup, 'Fresh setup unexpectedly required after restart', setupBody)
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
    assert(initializeBody.success === true, 'POST /api/setup failed', {
      status: initializeResponse.status(),
      body: initializeBody,
    })
    observations.push({ type: 'setup', status: initializeResponse.status() })
  }

  const loginResponse = await context.request.post(`${baseURL}/api/user/login`, {
    data: { username, password },
  })
  const loginBody = await readJson(loginResponse)
  assert(loginBody.success === true && loginBody.data?.id, 'Root login failed', {
    status: loginResponse.status(),
    body: loginBody,
  })
  const user = loginBody.data
  observations.push({ type: 'login', status: loginResponse.status(), userId: user.id, role: user.role })

  await context.addInitScript((seedUser) => {
    localStorage.setItem('user', JSON.stringify(seedUser))
    localStorage.setItem('uid', String(seedUser.id))
    localStorage.setItem('i18nextLng', 'en-US')
    localStorage.setItem('theme', 'light')
  }, user)

  return user
}

function authHeaders(user) {
  return { 'New-Api-User': String(user.id) }
}

async function getOptions(context, user) {
  const response = await context.request.get(`${baseURL}/api/option/`, {
    headers: authHeaders(user),
  })
  const body = await readJson(response)
  assert(response.ok() && body.success === true && Array.isArray(body.data), 'GET /api/option/ failed', {
    status: response.status(),
    body,
  })
  return Object.fromEntries(body.data.map((item) => [item.key, item.value]))
}

async function putBulk(context, user, options) {
  const response = await context.request.put(`${baseURL}/api/option/bulk`, {
    headers: authHeaders(user),
    data: { options },
  })
  return { response, body: await readJson(response) }
}

function attachPageDiagnostics(page, label) {
  page.on('console', (message) => {
    if (message.type() === 'error') {
      consoleErrors.push({ label, text: message.text() })
    }
  })
  page.on('pageerror', (error) => {
    pageErrors.push({ label, message: error.message, stack: error.stack })
  })
}

async function waitForInputValue(locator, expected, label) {
  await locator.waitFor({ state: 'visible', timeout: 20000 })
  for (let attempt = 0; attempt < 80; attempt += 1) {
    const value = await locator.inputValue()
    if (value === String(expected)) return
    await sleep(250)
  }
  throw new Error(`${label} did not reach expected value ${expected}; actual=${await locator.inputValue()}`)
}

async function expectOptionValues(context, user, expected, label) {
  const options = await getOptions(context, user)
  for (const [key, value] of Object.entries(expected)) {
    assert(options[key] === String(value), `${label}: option ${key} mismatch`, {
      expected: String(value),
      actual: options[key],
    })
  }
  observations.push({ type: 'option-check', label, expected })
  return options
}

async function main() {
  let server = null
  let browser = null
  try {
    server = await startBackend('initial')
    browser = await chromium.launch({ headless: true })
    const context = await browser.newContext({
      baseURL,
      viewport: { width: 1440, height: 1000 },
      deviceScaleFactor: 1,
      locale: 'en-US',
      timezoneId: 'Asia/Taipei',
      colorScheme: 'light',
    })
    const user = await setupAndLogin(context, { allowSetup: true })

    const baseline = {
      SystemName: 'RenewAPI QA Baseline',
      ServerAddress: baseURL,
      QuotaForNewUser: 111111,
      RetryTimes: 0,
    }
    const seed = await putBulk(context, user, baseline)
    assert(seed.response.ok() && seed.body.success === true, 'Failed to seed deterministic Settings baseline', {
      status: seed.response.status(),
      body: seed.body,
    })
    await expectOptionValues(context, user, baseline, 'seed baseline')

    const page = await context.newPage()
    attachPageDiagnostics(page, 'initial')
    const optionResponses = []
    page.on('response', (response) => {
      if (response.url().includes('/api/option/')) {
        optionResponses.push({ url: response.url(), method: response.request().method(), status: response.status() })
      }
    })

    await page.goto(`${baseURL}/system-settings`, { waitUntil: 'domcontentloaded' })
    assert(!page.url().includes('/sign-in'), 'Settings route redirected to sign-in despite real root session', { url: page.url() })

    const serverInput = page.getByLabel('Gateway base URL')
    const nameInput = page.getByLabel('System name')
    const quotaInput = page.getByLabel('New-user quota')
    const saveButton = page.getByRole('button', { name: 'Save all' })
    const retrySwitch = page.getByRole('switch', { name: 'Automatic retries' })

    await waitForInputValue(serverInput, baseline.ServerAddress, 'Gateway base URL')
    await waitForInputValue(nameInput, baseline.SystemName, 'System name')
    await waitForInputValue(quotaInput, baseline.QuotaForNewUser, 'New-user quota')

    await retrySwitch.waitFor({ state: 'visible', timeout: 20000 })
    const [singleEnableResponse] = await Promise.all([
      page.waitForResponse((response) =>
        response.url().endsWith('/api/option/') && response.request().method() === 'PUT'
      ),
      retrySwitch.click(),
    ])
    const singleEnableBody = await readJson(singleEnableResponse)
    assert(singleEnableBody.success === true, 'UI single-option enable mutation failed', singleEnableBody)
    await expectOptionValues(context, user, { RetryTimes: 2 }, 'single update enable')

    const [singleDisableResponse] = await Promise.all([
      page.waitForResponse((response) =>
        response.url().endsWith('/api/option/') && response.request().method() === 'PUT'
      ),
      retrySwitch.click(),
    ])
    const singleDisableBody = await readJson(singleDisableResponse)
    assert(singleDisableBody.success === true, 'UI single-option disable mutation failed', singleDisableBody)
    await expectOptionValues(context, user, { RetryTimes: 0 }, 'single update disable')

    await nameInput.fill('SHOULD_NOT_PERSIST')
    await serverInput.fill('not-a-valid-url')
    await quotaInput.fill('999999')
    const [failedBulkResponse] = await Promise.all([
      page.waitForResponse((response) =>
        response.url().endsWith('/api/option/bulk') && response.request().method() === 'PUT'
      ),
      saveButton.click(),
    ])
    const failedBulkBody = await readJson(failedBulkResponse)
    assert(failedBulkBody.success === false, 'Invalid UI bulk mutation unexpectedly succeeded', {
      status: failedBulkResponse.status(),
      body: failedBulkBody,
    })
    await page.getByText('服务器地址必须是完整的 http:// 或 https:// URL').waitFor({ state: 'visible', timeout: 10000 })
    await expectOptionValues(context, user, {
      SystemName: baseline.SystemName,
      ServerAddress: baseline.ServerAddress,
      QuotaForNewUser: baseline.QuotaForNewUser,
      RetryTimes: 0,
    }, 'failed bulk remains atomic')
    assert(await nameInput.inputValue() === 'SHOULD_NOT_PERSIST', 'Failed bulk unexpectedly discarded the local draft')
    assert(await serverInput.inputValue() === 'not-a-valid-url', 'Failed bulk unexpectedly replaced the invalid local draft')
    await page.screenshot({ path: path.join(outDir, 'settings-validation-error.png'), fullPage: true })

    const persisted = {
      SystemName: 'RenewAPI QA Browser',
      ServerAddress: `${baseURL}/qa`,
      QuotaForNewUser: 424242,
      RetryTimes: 0,
    }
    await nameInput.fill(persisted.SystemName)
    await serverInput.fill(persisted.ServerAddress)
    await quotaInput.fill(String(persisted.QuotaForNewUser))
    const [successfulBulkResponse] = await Promise.all([
      page.waitForResponse((response) =>
        response.url().endsWith('/api/option/bulk') && response.request().method() === 'PUT'
      ),
      saveButton.click(),
    ])
    const successfulBulkBody = await readJson(successfulBulkResponse)
    assert(successfulBulkBody.success === true, 'Valid UI bulk mutation failed', {
      status: successfulBulkResponse.status(),
      body: successfulBulkBody,
    })
    await page.getByText('Settings updated successfully').waitFor({ state: 'visible', timeout: 10000 })
    await expectOptionValues(context, user, persisted, 'successful UI bulk')
    await waitForInputValue(nameInput, persisted.SystemName, 'post-save System name')
    await waitForInputValue(serverInput, persisted.ServerAddress, 'post-save Gateway base URL')
    await waitForInputValue(quotaInput, persisted.QuotaForNewUser, 'post-save New-user quota')
    await page.screenshot({ path: path.join(outDir, 'settings-success.png'), fullPage: true })

    observations.push({ type: 'browser-option-responses', responses: optionResponses })
    await context.close()
    await stopBackend(server)
    server = null

    server = await startBackend('restart')
    const restartContext = await browser.newContext({
      baseURL,
      viewport: { width: 1440, height: 1000 },
      deviceScaleFactor: 1,
      locale: 'en-US',
      timezoneId: 'Asia/Taipei',
      colorScheme: 'light',
    })
    const restartedUser = await setupAndLogin(restartContext, { allowSetup: false })
    await expectOptionValues(restartContext, restartedUser, persisted, 'post-restart persistence')

    const restartPage = await restartContext.newPage()
    attachPageDiagnostics(restartPage, 'restart')
    await restartPage.goto(`${baseURL}/system-settings`, { waitUntil: 'domcontentloaded' })
    assert(!restartPage.url().includes('/sign-in'), 'Restarted Settings route redirected to sign-in', { url: restartPage.url() })
    await waitForInputValue(restartPage.getByLabel('System name'), persisted.SystemName, 'restart System name')
    await waitForInputValue(restartPage.getByLabel('Gateway base URL'), persisted.ServerAddress, 'restart Gateway base URL')
    await waitForInputValue(restartPage.getByLabel('New-user quota'), persisted.QuotaForNewUser, 'restart New-user quota')
    await restartPage.screenshot({ path: path.join(outDir, 'settings-after-restart.png'), fullPage: true })
    await restartContext.close()

    assert(consoleErrors.length === 0, 'Browser console errors detected', consoleErrors)
    assert(pageErrors.length === 0, 'Browser page errors detected', pageErrors)

    const summary = {
      result: 'passed',
      baseURL,
      dbPath,
      singleMutation: { RetryTimes: '0 -> 2 -> 0' },
      failedBulkAtomicity: true,
      successfulBulk: persisted,
      persistedAfterRestart: true,
      consoleErrors,
      pageErrors,
      observations,
    }
    fs.writeFileSync(path.join(outDir, 'summary.json'), `${JSON.stringify(summary, null, 2)}\n`)
    fs.writeFileSync(
      path.join(outDir, 'summary.txt'),
      [
        'result=passed',
        'backend=real RenewAPI binary + isolated SQLite',
        'auth=real root session + New-Api-User',
        'singleMutation=RetryTimes 0->2->0 via Settings UI',
        'invalidBulk=business failure; no partial persistence; draft preserved',
        `validBulk=${JSON.stringify(persisted)}`,
        'restartPersistence=passed',
        `consoleErrors=${consoleErrors.length}`,
        `pageErrors=${pageErrors.length}`,
      ].join('\n') + '\n'
    )
    console.log(fs.readFileSync(path.join(outDir, 'summary.txt'), 'utf8'))
  } catch (error) {
    const failure = {
      result: 'failed',
      message: error instanceof Error ? error.message : String(error),
      stack: error instanceof Error ? error.stack : null,
      consoleErrors,
      pageErrors,
      observations,
    }
    fs.writeFileSync(path.join(outDir, 'failure.json'), `${JSON.stringify(failure, null, 2)}\n`)
    throw error
  } finally {
    await browser?.close().catch(() => {})
    await stopBackend(server).catch(() => {})
  }
}

await main()
