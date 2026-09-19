import { spawn, spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import process from 'node:process'
import { chromium } from 'playwright'

const binary = process.env.QA_BINARY
const dbPath = process.env.QA_DB
const outDir = path.resolve(
  process.env.QA_OUT || '../../qa-artifacts/settings-advanced-phase-e-wave2'
)
const port = Number(process.env.QA_PORT || 4173)
const baseURL = `http://127.0.0.1:${port}`
const username = 'qaadmin'
const password = 'QaRoot123!'
const sessionSecret = 'settings-advanced-phase-e-wave2-2026-08-24'
const requestGuardSecret = 'wave2-request-guard-secret'
const auditSecretInitial = 'wave2-audit-secret-initial'
const auditSecretRotated = 'wave2-audit-secret-rotated'

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

function runSqlQuery(sql, params = []) {
  const script = String.raw`
import json
import sqlite3
import sys

path = sys.argv[1]
sql = sys.argv[2]
params = json.loads(sys.argv[3])
db = sqlite3.connect(path)
db.row_factory = sqlite3.Row
try:
    rows = db.execute(sql, params).fetchall()
    print(json.dumps([dict(row) for row in rows]))
finally:
    db.close()
`
  const result = spawnSync(
    'python3',
    ['-c', script, dbPath, sql, JSON.stringify(params)],
    { encoding: 'utf8' }
  )
  assert(result.status === 0, 'Direct SQLite query failed', {
    status: result.status,
    stderr: result.stderr,
  })
  return JSON.parse(result.stdout.trim() || '[]')
}

function runSqlExec(sql, params = []) {
  const script = String.raw`
import json
import sqlite3
import sys

path = sys.argv[1]
sql = sys.argv[2]
params = json.loads(sys.argv[3])
db = sqlite3.connect(path)
try:
    cursor = db.execute(sql, params)
    db.commit()
    print(cursor.rowcount)
finally:
    db.close()
`
  const result = spawnSync(
    'python3',
    ['-c', script, dbPath, sql, JSON.stringify(params)],
    { encoding: 'utf8' }
  )
  assert(result.status === 0, 'Direct SQLite mutation failed', {
    status: result.status,
    stderr: result.stderr,
  })
  return Number(result.stdout.trim() || '0')
}

function readDbOptions(keys) {
  if (keys.length === 0) return {}
  const placeholders = keys.map(() => '?').join(',')
  const rows = runSqlQuery(
    `SELECT "key", value FROM options WHERE "key" IN (${placeholders})`,
    keys
  )
  return Object.fromEntries(rows.map((row) => [row.key, row.value]))
}

function expectDbOptionValues(expected, label) {
  const rows = readDbOptions(Object.keys(expected))
  for (const [key, value] of Object.entries(expected)) {
    assert(rows[key] === String(value), `${label}: SQLite option ${key} mismatch`, {
      expected: String(value),
      actual: rows[key],
    })
  }
  observations.push({
    type: 'sqlite-option-check',
    label,
    keys: Object.keys(expected),
  })
  return rows
}

function readRequestGuardSecretRows() {
  return runSqlQuery(
    `SELECT "key", value FROM options WHERE "key" LIKE 'request_guard%' AND "key" <> 'request_guard_setting' ORDER BY "key"`
  )
}

function seedWave2Logs(nowSeconds) {
  runSqlExec('DELETE FROM logs WHERE content LIKE ?', ['wave2-log-%'])
  runSqlExec(
    'INSERT INTO logs (user_id, created_at, type, content, username) VALUES (?, ?, ?, ?, ?)',
    [1, nowSeconds - 48 * 60 * 60, 4, 'wave2-log-old', 'qaadmin']
  )
  runSqlExec(
    'INSERT INTO logs (user_id, created_at, type, content, username) VALUES (?, ?, ?, ?, ?)',
    [1, nowSeconds - 60 * 60, 4, 'wave2-log-new', 'qaadmin']
  )
}

function readWave2Logs() {
  return runSqlQuery(
    'SELECT content, created_at FROM logs WHERE content LIKE ? ORDER BY created_at ASC',
    ['wave2-log-%']
  )
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
  observations.push({ type: 'backend-start', label })
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
  assert(setupBody.success === true, 'GET /api/setup business failure', setupBody)

  if (!setupBody.data?.status) {
    assert(allowSetup, 'Fresh setup unexpectedly required after restart')
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
      message: initializeBody.message,
    })
    observations.push({ type: 'setup', status: initializeResponse.status() })
  }

  const authPage = await context.newPage()
  let loginResult
  try {
    await authPage.goto(`${baseURL}/sign-in`, { waitUntil: 'domcontentloaded' })
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
          body = { success: false, message: 'Non-JSON login response' }
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
    { status: loginResult.status, message: loginResult.body?.message }
  )
  const user = loginResult.body.data
  await context.addInitScript((seedUser) => {
    localStorage.setItem('user', JSON.stringify(seedUser))
    localStorage.setItem('uid', String(seedUser.id))
    localStorage.setItem('i18nextLng', 'en-US')
    localStorage.setItem('theme', 'light')
  }, user)
  observations.push({ type: 'login', userId: user.id, role: user.role })
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
    { status: response.status(), message: body.message }
  )
  return Object.fromEntries(body.data.map((item) => [item.key, item.value]))
}

async function waitForOptionValues(context, user, expected, label) {
  let latest = null
  for (let attempt = 0; attempt < 80; attempt += 1) {
    latest = await getOptions(context, user)
    if (
      Object.entries(expected).every(
        ([key, value]) => latest[key] === String(value)
      )
    ) {
      observations.push({
        type: 'option-wait-passed',
        label,
        keys: Object.keys(expected),
      })
      return latest
    }
    await sleep(250)
  }
  const actual = Object.fromEntries(
    Object.keys(expected).map((key) => [key, latest?.[key]])
  )
  throw new Error(
    `${label}: options did not converge\n${JSON.stringify({ expected, actual }, null, 2)}`
  )
}

async function getRequestGuardConfig(context, user) {
  const response = await context.request.get(`${baseURL}/api/request-guard/config`, {
    headers: authHeaders(user),
  })
  const body = await readJson(response)
  assert(response.ok() && body.success === true, 'GET Request Guard config failed', {
    status: response.status(),
    message: body.message,
  })
  return body.data
}

async function waitForRequestGuard(context, user, predicate, label) {
  let latest = null
  for (let attempt = 0; attempt < 80; attempt += 1) {
    latest = await getRequestGuardConfig(context, user)
    if (predicate(latest)) {
      observations.push({ type: 'request-guard-wait-passed', label })
      return latest
    }
    await sleep(250)
  }
  throw new Error(`${label}: Request Guard config did not converge`)
}

function attachPageDiagnostics(page, label) {
  page.on('console', (message) => {
    if (message.type() === 'error') {
      consoleErrors.push({ label, text: message.text() })
    }
  })
  page.on('pageerror', (error) => {
    pageErrors.push({ label, message: error.message })
  })
}

async function openPage(context, route, label) {
  const page = await context.newPage()
  attachPageDiagnostics(page, label)
  await page.goto(`${baseURL}${route}`, { waitUntil: 'domcontentloaded' })
  assert(!page.url().includes('/sign-in'), `${label}: redirected to sign-in`, {
    url: page.url(),
  })
  await page.locator('main').first().waitFor({ state: 'visible', timeout: 20000 })
  await sleep(700)
  return page
}

async function setSwitch(locator, expected, label) {
  await locator.waitFor({ state: 'visible', timeout: 15000 })
  const current = await locator.isChecked()
  if (current !== expected) await locator.click()
  for (let attempt = 0; attempt < 40; attempt += 1) {
    if ((await locator.isChecked()) === expected) return
    await sleep(100)
  }
  throw new Error(`${label}: switch did not reach ${expected}`)
}

function captureRequests(page, matcher) {
  const requests = []
  page.on('request', (request) => {
    if (!matcher(request)) return
    let body = null
    const raw = request.postData()
    if (raw) {
      try {
        body = JSON.parse(raw)
      } catch {
        body = null
      }
    }
    requests.push({ method: request.method(), url: request.url(), body })
  })
  return requests
}

async function runRequestGuardCreateEdit(context, user) {
  const page = await openPage(
    context,
    '/system-settings/security/request-guard',
    'wave2-request-guard-create'
  )
  const mutations = captureRequests(
    page,
    (request) =>
      request.method() === 'PUT' &&
      request.url().includes('/api/request-guard/config')
  )
  try {
    const baseline = await getRequestGuardConfig(context, user)
    assert(
      Array.isArray(baseline.endpoints) && baseline.endpoints.length === 0,
      'Request Guard fresh baseline should be empty'
    )

    await page.getByRole('button', { name: 'Add Endpoint' }).click()
    await page.getByLabel('Endpoint ID').fill('wave2-primary')
    await page.getByLabel('Base URL').fill('https://guard.example.test/v1')
    await page.getByLabel('Model').fill('guard-v1')
    await page.getByLabel('API Key').fill(requestGuardSecret)
    await page.getByRole('button', { name: 'Save Changes' }).click()

    const created = await waitForRequestGuard(
      context,
      user,
      (config) =>
        config.endpoints?.length === 1 &&
        config.endpoints[0].id === 'wave2-primary' &&
        config.endpoints[0].model === 'guard-v1' &&
        config.endpoints[0].has_secret === true,
      'Request Guard create'
    )
    assert(
      !('secret' in created.endpoints[0]),
      'Request Guard API must not return endpoint secret'
    )
    assert(mutations.length === 1, 'Request Guard create should issue one config PUT', {
      count: mutations.length,
    })

    const configRows = readDbOptions(['request_guard_setting'])
    assert(
      configRows.request_guard_setting?.includes('wave2-primary'),
      'Request Guard config missing from SQLite'
    )
    const secretRows = readRequestGuardSecretRows()
    assert(
      secretRows.some((row) => row.value === requestGuardSecret),
      'Request Guard secret not persisted in SQLite'
    )
    observations.push({ type: 'request-guard-secret-check', configured: true })

    await page.getByLabel('Model').fill('guard-v2')
    await page.getByRole('button', { name: 'Save Changes' }).click()
    await waitForRequestGuard(
      context,
      user,
      (config) => config.endpoints?.[0]?.model === 'guard-v2',
      'Request Guard edit'
    )
    assert(
      mutations.length === 2,
      'Request Guard edit should issue one additional config PUT',
      { count: mutations.length }
    )
    await page.screenshot({
      path: path.join(outDir, 'request-guard-created-edited.png'),
      fullPage: true,
    })
  } finally {
    await page.close()
  }
}

async function verifyRequestGuardRestartAndDelete(context, user) {
  const page = await openPage(
    context,
    '/system-settings/security/request-guard',
    'wave2-request-guard-delete'
  )
  const mutations = captureRequests(
    page,
    (request) =>
      request.method() === 'PUT' &&
      request.url().includes('/api/request-guard/config')
  )
  try {
    await page.getByLabel('Endpoint ID').waitFor({
      state: 'visible',
      timeout: 15000,
    })
    assert(
      (await page.getByLabel('Endpoint ID').inputValue()) === 'wave2-primary',
      'Request Guard endpoint did not survive restart'
    )
    assert(
      (await page.getByLabel('Model').inputValue()) === 'guard-v2',
      'Request Guard edit did not survive restart'
    )
    await page.getByText('Secret configured').waitFor({
      state: 'visible',
      timeout: 10000,
    })

    const beforeCancel = await getRequestGuardConfig(context, user)
    await page
      .getByRole('button', { name: 'Remove endpoint wave2-primary' })
      .click()
    await page
      .getByRole('heading', { name: 'Remove Request Guard endpoint?' })
      .waitFor({ state: 'visible' })
    await page.getByRole('button', { name: 'Cancel' }).click()
    await page.getByLabel('Endpoint ID').waitFor({ state: 'visible' })
    const afterCancel = await getRequestGuardConfig(context, user)
    assert(
      afterCancel.endpoints?.[0]?.id === beforeCancel.endpoints?.[0]?.id,
      'Request Guard cancel changed backend config'
    )
    assert(
      mutations.length === 0,
      'Request Guard cancel unexpectedly issued config PUT'
    )

    await page
      .getByRole('button', { name: 'Remove endpoint wave2-primary' })
      .click()
    await page
      .getByRole('button', { name: 'Remove endpoint', exact: true })
      .click()
    await page
      .getByText('No guard endpoints configured')
      .waitFor({ state: 'visible' })
    const beforeSave = await getRequestGuardConfig(context, user)
    assert(
      beforeSave.endpoints?.length === 1,
      'Draft removal changed backend before Save'
    )
    assert(
      mutations.length === 0,
      'Draft removal unexpectedly issued config PUT'
    )

    await page.getByRole('button', { name: 'Save Changes' }).click()
    await waitForRequestGuard(
      context,
      user,
      (config) => Array.isArray(config.endpoints) && config.endpoints.length === 0,
      'Request Guard delete save'
    )
    assert(
      mutations.length === 1,
      'Request Guard delete Save should issue one config PUT',
      { count: mutations.length }
    )
    const secretRows = readRequestGuardSecretRows()
    assert(
      secretRows.length >= 1,
      'Expected persisted Request Guard secret option row'
    )
    assert(
      secretRows.every((row) => row.value === ''),
      'Removed Request Guard endpoint secret was not cleared'
    )
    observations.push({
      type: 'request-guard-delete',
      cancelPreserved: true,
      secretCleared: true,
    })
  } finally {
    await page.close()
  }
}

function assertSingleBulkMutation(requests, expectedKeys, label) {
  assert(requests.length === 1, `${label}: expected exactly one bulk mutation`, {
    count: requests.length,
  })
  const options = requests[0].body?.options
  assert(
    options && typeof options === 'object',
    `${label}: bulk mutation body missing options`
  )
  const actualKeys = Object.keys(options).sort()
  const wanted = [...expectedKeys].sort()
  assert(
    JSON.stringify(actualKeys) === JSON.stringify(wanted),
    `${label}: unexpected bulk keys`,
    { expected: wanted, actual: actualKeys }
  )
}

async function runModelPricing(context, user) {
  const page = await openPage(
    context,
    '/system-settings/billing/model-pricing',
    'wave2-model-pricing'
  )
  const bulkRequests = captureRequests(
    page,
    (request) =>
      request.method() === 'PUT' && request.url().includes('/api/option/bulk')
  )
  try {
    await page.getByRole('button', { name: 'Switch to JSON' }).click()
    const modelRatio = page.getByLabel('Model ratio')
    const cacheRatio = page.getByLabel('Prompt cache ratio')

    await modelRatio.fill('{"broken":}')
    const invalidBefore = bulkRequests.length
    await page.getByRole('button', { name: 'Save model prices' }).click()
    await sleep(500)
    assert(
      bulkRequests.length === invalidBefore,
      'Invalid model pricing JSON unexpectedly mutated backend'
    )

    const modelRatioValue = '{"wave2-model":1.25}'
    const cacheRatioValue = '{"wave2-model":0.5}'
    await modelRatio.fill(modelRatioValue)
    await cacheRatio.fill(cacheRatioValue)
    await page.getByRole('button', { name: 'Save model prices' }).click()
    await waitForOptionValues(
      context,
      user,
      { ModelRatio: modelRatioValue, CacheRatio: cacheRatioValue },
      'Model pricing valid save'
    )
    assertSingleBulkMutation(
      bulkRequests.slice(invalidBefore),
      ['ModelRatio', 'CacheRatio'],
      'Model pricing'
    )
    expectDbOptionValues(
      { ModelRatio: modelRatioValue, CacheRatio: cacheRatioValue },
      'Model pricing'
    )
    await page.screenshot({
      path: path.join(outDir, 'model-pricing-valid.png'),
      fullPage: true,
    })
  } finally {
    await page.close()
  }
}

async function runGroupPricing(context, user) {
  const page = await openPage(
    context,
    '/system-settings/billing/group-pricing',
    'wave2-group-pricing'
  )
  const bulkRequests = captureRequests(
    page,
    (request) =>
      request.method() === 'PUT' && request.url().includes('/api/option/bulk')
  )
  try {
    await page.getByRole('button', { name: 'Switch to JSON' }).click()
    const groupRatio = page.getByLabel('Group ratios')
    const autoGroups = page.getByLabel('Auto assignment order')

    await groupRatio.fill('{"broken":}')
    const invalidBefore = bulkRequests.length
    await page.getByRole('button', { name: 'Save group ratios' }).click()
    await sleep(500)
    assert(
      bulkRequests.length === invalidBefore,
      'Invalid group pricing JSON unexpectedly mutated backend'
    )

    const groupRatioValue = '{"default":1,"wave2":1.1}'
    const autoGroupsValue = '["wave2","default"]'
    await groupRatio.fill(groupRatioValue)
    await autoGroups.fill(autoGroupsValue)
    await page.getByRole('button', { name: 'Save group ratios' }).click()
    await waitForOptionValues(
      context,
      user,
      { GroupRatio: groupRatioValue, AutoGroups: autoGroupsValue },
      'Group pricing valid save'
    )
    assertSingleBulkMutation(
      bulkRequests.slice(invalidBefore),
      ['GroupRatio', 'AutoGroups'],
      'Group pricing'
    )
    expectDbOptionValues(
      { GroupRatio: groupRatioValue, AutoGroups: autoGroupsValue },
      'Group pricing'
    )
  } finally {
    await page.close()
  }
}

async function runLogDeletion(context) {
  const nowSeconds = Math.floor(Date.now() / 1000)
  seedWave2Logs(nowSeconds)
  assert(readWave2Logs().length === 2, 'Failed to seed Wave 2 logs')

  const page = await openPage(
    context,
    '/system-settings/operations/logs',
    'wave2-log-deletion'
  )
  try {
    await page.getByRole('button', { name: '24 hours ago' }).click()
    await page.getByRole('button', { name: 'Clean logs' }).click()
    await page
      .getByRole('heading', { name: 'Confirm log cleanup' })
      .waitFor({ state: 'visible' })
    await page.getByRole('button', { name: 'Cancel' }).click()
    await sleep(300)
    assert(readWave2Logs().length === 2, 'Cancel log cleanup deleted rows')

    await page.getByRole('button', { name: 'Clean logs' }).click()
    await page.getByRole('button', { name: 'Delete logs' }).click()
    let remaining = readWave2Logs()
    for (
      let attempt = 0;
      attempt < 40 && remaining.length !== 1;
      attempt += 1
    ) {
      await sleep(250)
      remaining = readWave2Logs()
    }
    assert(
      remaining.length === 1 && remaining[0].content === 'wave2-log-new',
      'Log cleanup did not preserve only the newer row',
      { remaining: remaining.map((row) => row.content) }
    )
    observations.push({
      type: 'log-deletion',
      cancelPreserved: true,
      deletedOldOnly: true,
    })
  } finally {
    await page.close()
  }
}

async function runAntiPoisonSecret(context, user) {
  const page = await openPage(
    context,
    '/system-settings/security/anti-poison-guard',
    'wave2-anti-poison-secret'
  )
  try {
    const auditEnabled = page.getByRole('switch', {
      name: 'Signed Header Audit',
    })
    const auditSecret = page.getByLabel('Audit Secret')
    await setSwitch(auditEnabled, true, 'Signed Header Audit')
    await auditSecret.fill(auditSecretInitial)
    await page.getByRole('button', { name: 'Save Changes' }).click()
    let options = await waitForOptionValues(
      context,
      user,
      {
        'anti_poison_setting.signed_header_audit_enabled': true,
        'anti_poison_setting.signed_header_audit_secret_configured': true,
      },
      'Anti-Poison initial secret save'
    )
    assert(
      options['anti_poison_setting.signed_header_audit_secret'] === undefined,
      'Anti-Poison secret leaked through option API'
    )
    let secretRow = readDbOptions([
      'anti_poison_setting.signed_header_audit_secret',
    ])
    assert(
      secretRow['anti_poison_setting.signed_header_audit_secret'] ===
        auditSecretInitial,
      'Anti-Poison initial secret missing from SQLite'
    )

    await page.reload({ waitUntil: 'domcontentloaded' })
    await page.locator('main').first().waitFor({ state: 'visible' })
    await page.getByLabel('Audit Secret').waitFor({ state: 'visible' })
    assert(
      (await page.getByLabel('Audit Secret').inputValue()) === '',
      'Configured Anti-Poison secret should not be rehydrated into the input'
    )
    await page.getByLabel('Guard Scan Limit').fill('65537')
    await page.getByRole('button', { name: 'Save Changes' }).click()
    await waitForOptionValues(
      context,
      user,
      { 'anti_poison_setting.max_guard_scan_bytes': 65537 },
      'Anti-Poison unrelated save'
    )
    secretRow = readDbOptions([
      'anti_poison_setting.signed_header_audit_secret',
    ])
    assert(
      secretRow['anti_poison_setting.signed_header_audit_secret'] ===
        auditSecretInitial,
      'Unrelated Anti-Poison save overwrote configured secret'
    )

    await page.reload({ waitUntil: 'domcontentloaded' })
    await page.locator('main').first().waitFor({ state: 'visible' })
    await page.getByLabel('Audit Secret').fill(auditSecretRotated)
    await page.getByRole('button', { name: 'Save Changes' }).click()
    for (let attempt = 0; attempt < 40; attempt += 1) {
      secretRow = readDbOptions([
        'anti_poison_setting.signed_header_audit_secret',
      ])
      if (
        secretRow['anti_poison_setting.signed_header_audit_secret'] ===
        auditSecretRotated
      ) {
        break
      }
      await sleep(250)
    }
    assert(
      secretRow['anti_poison_setting.signed_header_audit_secret'] ===
        auditSecretRotated,
      'Explicit Anti-Poison secret rotation did not persist'
    )
    options = await getOptions(context, user)
    assert(
      options['anti_poison_setting.signed_header_audit_secret'] === undefined,
      'Rotated Anti-Poison secret leaked through option API'
    )
    assert(
      options['anti_poison_setting.signed_header_audit_secret_configured'] ===
        'true',
      'Anti-Poison configured state not reported after rotation'
    )
    observations.push({
      type: 'anti-poison-secret',
      masked: true,
      unrelatedSavePreserved: true,
      rotationPersisted: true,
    })
  } finally {
    await page.close()
  }
}

async function verifyFinalRestart(context, user) {
  const requestGuard = await getRequestGuardConfig(context, user)
  assert(
    Array.isArray(requestGuard.endpoints) && requestGuard.endpoints.length === 0,
    'Deleted Request Guard endpoint reappeared after restart'
  )

  const pricing = await getOptions(context, user)
  assert(
    pricing.ModelRatio === '{"wave2-model":1.25}',
    'ModelRatio did not survive restart'
  )
  assert(
    pricing.CacheRatio === '{"wave2-model":0.5}',
    'CacheRatio did not survive restart'
  )
  assert(
    pricing.GroupRatio === '{"default":1,"wave2":1.1}',
    'GroupRatio did not survive restart'
  )
  assert(
    pricing.AutoGroups === '["wave2","default"]',
    'AutoGroups did not survive restart'
  )
  assert(
    pricing['anti_poison_setting.signed_header_audit_secret'] === undefined,
    'Anti-Poison secret leaked after restart'
  )
  assert(
    pricing['anti_poison_setting.signed_header_audit_secret_configured'] ===
      'true',
    'Anti-Poison configured state lost after restart'
  )
  expectDbOptionValues(
    {
      ModelRatio: '{"wave2-model":1.25}',
      CacheRatio: '{"wave2-model":0.5}',
      GroupRatio: '{"default":1,"wave2":1.1}',
      AutoGroups: '["wave2","default"]',
      'anti_poison_setting.signed_header_audit_secret': auditSecretRotated,
    },
    'Final restart options'
  )
  const logs = readWave2Logs()
  assert(
    logs.length === 1 && logs[0].content === 'wave2-log-new',
    'Log deletion state did not survive restart'
  )

  const requestGuardPage = await openPage(
    context,
    '/system-settings/security/request-guard',
    'wave2-final-request-guard'
  )
  try {
    await requestGuardPage
      .getByText('No guard endpoints configured')
      .waitFor({ state: 'visible', timeout: 15000 })
  } finally {
    await requestGuardPage.close()
  }

  const modelPage = await openPage(
    context,
    '/system-settings/billing/model-pricing',
    'wave2-final-model-pricing'
  )
  try {
    await modelPage.getByRole('button', { name: 'Switch to JSON' }).click()
    const modelRatio = await modelPage.getByLabel('Model ratio').inputValue()
    assert(
      JSON.stringify(JSON.parse(modelRatio)) === '{"wave2-model":1.25}',
      'Restart UI ModelRatio mismatch'
    )
  } finally {
    await modelPage.close()
  }

  const antiPoisonPage = await openPage(
    context,
    '/system-settings/security/anti-poison-guard',
    'wave2-final-anti-poison'
  )
  try {
    assert(
      await antiPoisonPage
        .getByRole('switch', { name: 'Signed Header Audit' })
        .isChecked(),
      'Signed Header Audit lost after restart'
    )
    assert(
      (await antiPoisonPage.getByLabel('Audit Secret').inputValue()) === '',
      'Anti-Poison secret should remain masked after restart'
    )
  } finally {
    await antiPoisonPage.close()
  }

  observations.push({
    type: 'final-restart',
    requestGuardAbsent: true,
    pricingPersisted: true,
    logsPersisted: true,
    secretMasked: true,
  })
}

async function restartSession(browser, server, context, label) {
  if (context) await context.close()
  await stopBackend(server, `restart-${label}`)
  const nextServer = await startBackend(label)
  const nextContext = await createBrowserContext(browser)
  const nextUser = await setupAndLogin(nextContext, { allowSetup: false })
  return { server: nextServer, context: nextContext, user: nextUser }
}

function writeSummary(status, error = null) {
  const summary = {
    status,
    observations,
    consoleErrors,
    pageErrors,
    error: error ? { message: error.message, stack: error.stack } : null,
  }
  fs.writeFileSync(
    path.join(outDir, status === 'passed' ? 'summary.json' : 'failure.json'),
    JSON.stringify(summary, null, 2)
  )
  if (status === 'passed') {
    fs.writeFileSync(
      path.join(outDir, 'summary.txt'),
      [
        'Settings Advanced Phase E Wave 2: PASS',
        'Request Guard CRUD + destructive confirmation + secret cleanup: PASS',
        'Model/group pricing validation + single bulk persistence: PASS',
        'Log deletion cancel/confirm + cutoff persistence: PASS',
        'Anti-Poison secret masking + no-op preservation + rotation: PASS',
        'Restart persistence: PASS',
        `consoleErrors=${consoleErrors.length}`,
        `pageErrors=${pageErrors.length}`,
      ].join('\n') + '\n'
    )
  }
}

async function main() {
  let server = null
  let browser = null
  let context = null
  let user = null
  try {
    server = await startBackend('initial')
    browser = await chromium.launch({ headless: true })
    context = await createBrowserContext(browser)
    user = await setupAndLogin(context, { allowSetup: true })

    await runRequestGuardCreateEdit(context, user)

    let restarted = await restartSession(
      browser,
      server,
      context,
      'request-guard-persistence'
    )
    server = restarted.server
    context = restarted.context
    user = restarted.user

    await verifyRequestGuardRestartAndDelete(context, user)
    await runModelPricing(context, user)
    await runGroupPricing(context, user)
    await runLogDeletion(context)
    await runAntiPoisonSecret(context, user)

    restarted = await restartSession(browser, server, context, 'final')
    server = restarted.server
    context = restarted.context
    user = restarted.user

    await verifyFinalRestart(context, user)

    assert(consoleErrors.length === 0, 'Browser console errors detected', consoleErrors)
    assert(pageErrors.length === 0, 'Browser page errors detected', pageErrors)
    writeSummary('passed')
  } catch (error) {
    writeSummary('failed', error)
    throw error
  } finally {
    if (context) await context.close().catch(() => {})
    if (browser) await browser.close().catch(() => {})
    if (server) await stopBackend(server, 'final-cleanup').catch(() => {})
  }
}

await main()
