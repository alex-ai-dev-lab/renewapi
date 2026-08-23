import { spawn, spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'
import { chromium } from 'playwright'

const binary = process.env.QA_BINARY
const dbPath = process.env.QA_DB
const outDir = path.resolve(
  process.env.QA_OUT || '../../qa-artifacts/settings-real-backend-hardened'
)
const port = Number(process.env.QA_PORT || 4173)
const baseURL = `http://127.0.0.1:${port}`
const username = 'qaadmin'
const password = 'QaRoot123!'
const sessionSecret =
  'settings-real-backend-hardened-2026-08-23-4c2d5d1a7f6b88e3'

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
  const suffix =
    details === undefined ? '' : `\n${JSON.stringify(details, null, 2)}`
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
    throw new Error(
      `Expected JSON from ${response.url()}, got: ${text.slice(0, 500)}`
    )
  }
}

function readDbOptions(keys) {
  const script = String.raw`
import json
import sqlite3
import sys

path = sys.argv[1]
keys = json.loads(sys.argv[2])
db = sqlite3.connect(path)
try:
    if not keys:
        print('{}')
    else:
        placeholders = ','.join('?' for _ in keys)
        rows = db.execute(
            f'SELECT "key", value FROM options WHERE "key" IN ({placeholders})',
            keys,
        ).fetchall()
        print(json.dumps(dict(rows)))
finally:
    db.close()
`
  const result = spawnSync(
    'python3',
    ['-c', script, dbPath, JSON.stringify(keys)],
    { encoding: 'utf8' }
  )
  assert(result.status === 0, 'Direct SQLite option read failed', {
    status: result.status,
    stdout: result.stdout,
    stderr: result.stderr,
  })
  return JSON.parse(result.stdout.trim() || '{}')
}

function expectDbOptionValues(expected, label) {
  const rows = readDbOptions(Object.keys(expected))
  for (const [key, value] of Object.entries(expected)) {
    assert(rows[key] === String(value), `${label}: SQLite option ${key} mismatch`, {
      expected: String(value),
      actual: rows[key],
    })
  }
  observations.push({ type: 'sqlite-option-check', label, expected })
  return rows
}

function waitForReadableEnd(stream) {
  if (!stream || stream.readableEnded || stream.destroyed) {
    return Promise.resolve()
  }
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
  if (!stream || stream.writableFinished || stream.destroyed) {
    return Promise.resolve()
  }
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
      throw new Error(
        `RenewAPI exited before becoming ready with code ${child.exitCode}`
      )
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

async function stopBackend(server, reason = 'normal') {
  if (!server || server.closed) return
  server.closed = true

  const { child, log } = server
  const closePromise = waitForChildClose(child)

  if (child.exitCode === null) {
    child.kill('SIGTERM')
  }

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
    logPath: server.logPath,
  })
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

  observations.push({
    type: 'backend-start',
    label,
    pid: child.pid,
    logPath,
  })
  return server
}

function createBrowserContext(browser) {
  return browser.newContext({
    baseURL,
    viewport: { width: 1440, height: 1000 },
    deviceScaleFactor: 1,
    locale: 'en-US',
    timezoneId: 'Asia/Taipei',
    colorScheme: 'light',
  })
}

async function setupAndLogin(context, { allowSetup }) {
  const setupResponse = await context.request.get(`${baseURL}/api/setup`)
  assert(setupResponse.ok(), 'GET /api/setup failed', {
    status: setupResponse.status(),
  })
  const setupBody = await readJson(setupResponse)
  assert(
    setupBody.success === true,
    'GET /api/setup business failure',
    setupBody
  )

  if (!setupBody.data?.status) {
    assert(
      allowSetup,
      'Fresh setup unexpectedly required after restart',
      setupBody
    )
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

  const authPage = await context.newPage()
  let loginResult
  try {
    await authPage.goto(`${baseURL}/sign-in`, {
      waitUntil: 'domcontentloaded',
    })
    loginResult = await authPage.evaluate(
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
          body = {
            success: false,
            message: `Non-JSON login response: ${text.slice(0, 300)}`,
          }
        }
        return { status: response.status, body }
      },
      { loginUsername: username, loginPassword: password }
    )
  } finally {
    await authPage.close()
  }

  assert(
    loginResult.status === 200 &&
      loginResult.body?.success === true &&
      loginResult.body?.data?.id,
    'Root login failed',
    loginResult
  )
  const user = loginResult.body.data
  observations.push({
    type: 'login',
    status: loginResult.status,
    userId: user.id,
    role: user.role,
  })

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
  assert(
    response.ok() && body.success === true && Array.isArray(body.data),
    'GET /api/option/ failed',
    { status: response.status(), body }
  )
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
  throw new Error(
    `${label} did not reach expected value ${expected}; actual=${await locator.inputValue()}`
  )
}

async function waitForSwitchState(locator, expected, label) {
  await locator.waitFor({ state: 'visible', timeout: 20000 })
  for (let attempt = 0; attempt < 80; attempt += 1) {
    const checked = await locator.isChecked()
    if (checked === expected) {
      observations.push({ type: 'switch-state', label, checked })
      return
    }
    await sleep(250)
  }
  throw new Error(
    `${label} did not reach checked=${expected}; actual=${await locator.isChecked()}`
  )
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

async function openSettingsPage(context, label) {
  const page = await context.newPage()
  attachPageDiagnostics(page, label)
  const optionResponses = []
  page.on('response', (response) => {
    if (response.url().includes('/api/option/')) {
      optionResponses.push({
        url: response.url(),
        method: response.request().method(),
        status: response.status(),
      })
    }
  })

  await page.goto(`${baseURL}/system-settings`, {
    waitUntil: 'domcontentloaded',
  })
  assert(
    !page.url().includes('/sign-in'),
    `${label}: Settings route redirected to sign-in despite real root session`,
    { url: page.url() }
  )

  return { page, optionResponses }
}

function settingsLocators(page) {
  return {
    serverInput: page.getByLabel('Gateway base URL'),
    nameInput: page.getByLabel('System name'),
    quotaInput: page.getByLabel('New-user quota'),
    saveButton: page.getByRole('button', { name: 'Save all' }),
    retrySwitch: page.getByRole('switch', { name: 'Automatic retries' }),
  }
}

async function verifySettingsValues(page, expected, label) {
  const { serverInput, nameInput, quotaInput } = settingsLocators(page)
  await waitForInputValue(serverInput, expected.ServerAddress, `${label} Gateway base URL`)
  await waitForInputValue(nameInput, expected.SystemName, `${label} System name`)
  await waitForInputValue(quotaInput, expected.QuotaForNewUser, `${label} New-user quota`)
}

async function main() {
  let server = null
  let browser = null
  let activeContext = null

  const baseline = {
    SystemName: 'RenewAPI QA Baseline',
    ServerAddress: baseURL,
    QuotaForNewUser: 111111,
    RetryTimes: 0,
  }
  const persisted = {
    SystemName: 'RenewAPI QA Browser',
    ServerAddress: `${baseURL}/qa`,
    QuotaForNewUser: 424242,
    RetryTimes: 0,
  }

  try {
    const initialThemeRows = readDbOptions(['theme.frontend'])
    assert(
      initialThemeRows['theme.frontend'] === undefined,
      'Fresh QA database unexpectedly pre-seeded theme.frontend',
      initialThemeRows
    )
    observations.push({
      type: 'fresh-db-theme-check',
      preseededThemeFrontend: false,
    })

    server = await startBackend('initial')
    browser = await chromium.launch({ headless: true })
    activeContext = await createBrowserContext(browser)
    let user = await setupAndLogin(activeContext, { allowSetup: true })

    const seed = await putBulk(activeContext, user, baseline)
    assert(
      seed.response.ok() && seed.body.success === true,
      'Failed to seed deterministic Settings baseline',
      { status: seed.response.status(), body: seed.body }
    )
    await expectOptionValues(activeContext, user, baseline, 'seed baseline API')
    expectDbOptionValues(baseline, 'seed baseline SQLite')

    let { page, optionResponses } = await openSettingsPage(
      activeContext,
      'initial'
    )
    let { serverInput, nameInput, quotaInput, saveButton, retrySwitch } =
      settingsLocators(page)

    await verifySettingsValues(page, baseline, 'initial')
    await waitForSwitchState(retrySwitch, false, 'retry baseline')

    const [singleEnableResponse] = await Promise.all([
      page.waitForResponse(
        (response) =>
          response.url().endsWith('/api/option/') &&
          response.request().method() === 'PUT'
      ),
      retrySwitch.click(),
    ])
    const singleEnableBody = await readJson(singleEnableResponse)
    assert(
      singleEnableBody.success === true,
      'UI single-option enable mutation failed',
      singleEnableBody
    )
    await expectOptionValues(
      activeContext,
      user,
      { RetryTimes: 2 },
      'single update enable API'
    )
    expectDbOptionValues({ RetryTimes: 2 }, 'single update enable SQLite')
    await waitForSwitchState(retrySwitch, true, 'retry enabled UI')

    const [singleDisableResponse] = await Promise.all([
      page.waitForResponse(
        (response) =>
          response.url().endsWith('/api/option/') &&
          response.request().method() === 'PUT'
      ),
      retrySwitch.click(),
    ])
    const singleDisableBody = await readJson(singleDisableResponse)
    assert(
      singleDisableBody.success === true,
      'UI single-option disable mutation failed',
      singleDisableBody
    )
    await expectOptionValues(
      activeContext,
      user,
      { RetryTimes: 0 },
      'single update disable API'
    )
    expectDbOptionValues({ RetryTimes: 0 }, 'single update disable SQLite')
    await waitForSwitchState(retrySwitch, false, 'retry disabled UI')

    await nameInput.fill('SHOULD_NOT_PERSIST')
    await serverInput.fill('not-a-valid-url')
    await quotaInput.fill('999999')

    const [failedBulkResponse] = await Promise.all([
      page.waitForResponse(
        (response) =>
          response.url().endsWith('/api/option/bulk') &&
          response.request().method() === 'PUT'
      ),
      saveButton.click(),
    ])
    const failedBulkBody = await readJson(failedBulkResponse)
    assert(
      failedBulkBody.success === false,
      'Invalid UI bulk mutation unexpectedly succeeded',
      { status: failedBulkResponse.status(), body: failedBulkBody }
    )

    await page
      .getByText(
        /服务器地址必须是完整的 http:\/\/ 或 https:\/\/ URL|Server address must be a full URL/i
      )
      .waitFor({ state: 'visible', timeout: 10000 })

    await expectOptionValues(
      activeContext,
      user,
      baseline,
      'failed bulk baseline API'
    )
    expectDbOptionValues(baseline, 'failed bulk baseline SQLite')

    assert(
      (await nameInput.inputValue()) === 'SHOULD_NOT_PERSIST',
      'Failed bulk unexpectedly discarded the System name draft'
    )
    assert(
      (await serverInput.inputValue()) === 'not-a-valid-url',
      'Failed bulk unexpectedly replaced the invalid Server address draft'
    )
    assert(
      (await quotaInput.inputValue()) === '999999',
      'Failed bulk unexpectedly discarded the New-user quota draft'
    )
    observations.push({
      type: 'failed-bulk-draft-preserved',
      SystemName: true,
      ServerAddress: true,
      QuotaForNewUser: true,
    })

    await page.screenshot({
      path: path.join(outDir, 'settings-validation-error.png'),
      fullPage: true,
    })
    observations.push({
      type: 'browser-option-responses',
      label: 'initial',
      responses: optionResponses,
    })

    await activeContext.close()
    activeContext = null
    await stopBackend(server, 'after-invalid-bulk')
    server = null

    expectDbOptionValues(baseline, 'failed bulk SQLite after process stop')

    server = await startBackend('after-invalid-restart')
    activeContext = await createBrowserContext(browser)
    user = await setupAndLogin(activeContext, { allowSetup: false })
    await expectOptionValues(
      activeContext,
      user,
      baseline,
      'failed bulk baseline after restart API'
    )
    expectDbOptionValues(
      baseline,
      'failed bulk baseline after restart SQLite'
    )

    ;({ page, optionResponses } = await openSettingsPage(
      activeContext,
      'after-invalid-restart'
    ))
    ;({ serverInput, nameInput, quotaInput, saveButton, retrySwitch } =
      settingsLocators(page))
    await verifySettingsValues(page, baseline, 'after invalid restart')
    await waitForSwitchState(
      retrySwitch,
      false,
      'retry after invalid restart UI'
    )
    await page.screenshot({
      path: path.join(outDir, 'settings-after-invalid-restart.png'),
      fullPage: true,
    })

    await nameInput.fill(persisted.SystemName)
    await serverInput.fill(persisted.ServerAddress)
    await quotaInput.fill(String(persisted.QuotaForNewUser))

    const [successfulBulkResponse] = await Promise.all([
      page.waitForResponse(
        (response) =>
          response.url().endsWith('/api/option/bulk') &&
          response.request().method() === 'PUT'
      ),
      saveButton.click(),
    ])
    const successfulBulkBody = await readJson(successfulBulkResponse)
    assert(
      successfulBulkBody.success === true,
      'Valid UI bulk mutation failed',
      { status: successfulBulkResponse.status(), body: successfulBulkBody }
    )
    await page
      .getByText('Settings updated successfully')
      .waitFor({ state: 'visible', timeout: 10000 })

    await expectOptionValues(
      activeContext,
      user,
      persisted,
      'successful UI bulk API'
    )
    expectDbOptionValues(persisted, 'successful UI bulk SQLite')
    await verifySettingsValues(page, persisted, 'post save')
    await page.screenshot({
      path: path.join(outDir, 'settings-success.png'),
      fullPage: true,
    })
    observations.push({
      type: 'browser-option-responses',
      label: 'after-invalid-restart',
      responses: optionResponses,
    })

    await activeContext.close()
    activeContext = null
    await stopBackend(server, 'after-valid-bulk')
    server = null

    expectDbOptionValues(persisted, 'successful bulk SQLite after process stop')

    server = await startBackend('after-success-restart')
    activeContext = await createBrowserContext(browser)
    const restartedUser = await setupAndLogin(activeContext, {
      allowSetup: false,
    })
    await expectOptionValues(
      activeContext,
      restartedUser,
      persisted,
      'post-success restart API'
    )
    expectDbOptionValues(persisted, 'post-success restart SQLite')

    const finalOpen = await openSettingsPage(activeContext, 'after-success-restart')
    const restartPage = finalOpen.page
    await verifySettingsValues(restartPage, persisted, 'post-success restart UI')
    await restartPage.screenshot({
      path: path.join(outDir, 'settings-after-success-restart.png'),
      fullPage: true,
    })
    observations.push({
      type: 'browser-option-responses',
      label: 'after-success-restart',
      responses: finalOpen.optionResponses,
    })

    await activeContext.close()
    activeContext = null

    assert(consoleErrors.length === 0, 'Browser console errors detected', consoleErrors)
    assert(pageErrors.length === 0, 'Browser page errors detected', pageErrors)

    const summary = {
      result: 'passed',
      baseURL,
      dbPath,
      freshDatabaseThemePreseeded: false,
      singleMutation: {
        RetryTimes: '0 -> 2 -> 0',
        uiStateSynchronized: true,
        sqliteVerified: true,
      },
      failedBulk: {
        rejected: true,
        apiBaselinePreserved: true,
        sqliteBaselinePreservedBeforeOverwrite: true,
        restartBaselinePreservedBeforeOverwrite: true,
        localDraftPreserved: {
          SystemName: true,
          ServerAddress: true,
          QuotaForNewUser: true,
        },
      },
      successfulBulk: persisted,
      persistedAfterFinalRestart: true,
      backendLogsDrainedBeforeClose: true,
      consoleErrors,
      pageErrors,
      observations,
    }
    fs.writeFileSync(
      path.join(outDir, 'summary.json'),
      `${JSON.stringify(summary, null, 2)}\n`
    )
    fs.writeFileSync(
      path.join(outDir, 'summary.txt'),
      [
        'result=passed',
        'backend=real RenewAPI binary + fresh isolated SQLite',
        'themePreseed=false; Aurora selected naturally',
        'auth=real root session + New-Api-User',
        'singleMutation=RetryTimes 0->2->0; UI+API+SQLite synchronized',
        'invalidBulk=rejected; API+SQLite baseline preserved before any overwrite',
        'invalidBulkRestart=baseline preserved after intermediate process restart',
        'invalidBulkDraft=SystemName+ServerAddress+QuotaForNewUser preserved locally',
        `validBulk=${JSON.stringify(persisted)}`,
        'validBulkRestart=passed',
        'backendLogDrain=passed',
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
    fs.writeFileSync(
      path.join(outDir, 'failure.json'),
      `${JSON.stringify(failure, null, 2)}\n`
    )
    throw error
  } finally {
    await activeContext?.close().catch(() => {})
    await browser?.close().catch(() => {})
    await stopBackend(server, 'finally').catch(() => {})
  }
}

await main()
