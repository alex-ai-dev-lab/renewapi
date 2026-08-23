import { chromium } from 'playwright'
import fs from 'node:fs/promises'
import path from 'node:path'

const APP = 'http://127.0.0.1:4173'
const outDir = process.env.QA_OUT || 'qa-artifacts/phase-c'
const fixedNow = Date.parse('2026-08-23T10:30:00Z')
const user = {
  id: 1,
  username: 'qaadmin',
  display_name: 'QA Admin',
  email: 'qa@example.com',
  role: 100,
  status: 1,
  group: 'default',
  permissions: { sidebar_settings: true },
}

const channels = Array.from({ length: 24 }, (_, index) => ({
  id: index + 1,
  name: `Channel ${String(index + 1).padStart(2, '0')}`,
  type: [1, 14, 3, 24, 18, 39][index % 6],
  status: index === 7 ? 2 : 1,
  response_time: 320 + index * 9,
  models: index % 2 ? 'claude-sonnet-4-5,gpt-5.6' : 'gpt-5.6,gpt-4.1',
  group: index % 3 === 0 ? 'premium' : 'default',
  priority: (index + 1) * 10,
  weight: 0,
  base_url: '',
  key: 'sk-qa',
  created_time: 1750000000 + index * 1000,
}))

const keys = Array.from({ length: 25 }, (_, index) => ({
  id: index + 1,
  name: `Key ${String(index + 1).padStart(2, '0')}`,
  key: `sk-qa-${index + 1}`,
  status: index === 8 ? 2 : 1,
  remain_quota: 9000000 - index * 120000,
  used_quota: 800000 + index * 90000,
  unlimited_quota: false,
  expired_time: -1,
  created_time: 1750000000 + index * 10000,
  accessed_time: 1750005000 + index * 10000,
  group: index % 3 === 0 ? 'premium' : 'default',
  cross_group_retry: false,
  model_limits_enabled: false,
  model_limits: '',
  allow_ips: '',
}))

const vendors = [
  { id: 1, name: 'OpenAI' },
  { id: 2, name: 'Anthropic' },
  { id: 3, name: 'Google' },
]

const modelNames = [
  'gpt-5.6',
  'claude-sonnet-4-5',
  'gemini-2.5-pro',
  'gpt-4.1-mini',
  'claude-opus-4-1',
  'gemini-2.5-flash',
]
const models = modelNames.map((model_name, index) => ({
  id: index + 1,
  model_name,
  vendor_id: (index % 3) + 1,
  description: 'Model metadata, pricing and routing configuration',
  endpoints: '/v1/chat/completions',
  status: index === 4 ? 0 : 1,
  sync_official: 1,
  created_time: 1750000000,
  updated_time: 1750000000,
  name_rule: 0,
  bound_channels: [{ name: channels[index].name, type: channels[index].type }],
  enable_groups: ['default', 'premium'],
  quota_types: [0],
  matched_models: [model_name],
  matched_count: 1,
  tags: [],
  icon: '',
}))

const users = Array.from({ length: 26 }, (_, index) => ({
  id: index + 10,
  username: `user${String(index + 1).padStart(2, '0')}`,
  display_name: `QA User ${index + 1}`,
  email: `user${index + 1}@example.com`,
  quota: 10000000 + index * 100000,
  used_quota: 1200000 + index * 40000,
  request_count: 1100 + index * 30,
  group: index % 4 === 0 ? 'premium' : 'default',
  status: index === 6 ? 2 : 1,
  role: index === 0 ? 10 : 1,
  created_at: 1740000000 + index * 120000,
  updated_at: 1750000000,
  last_login_at: 1750000000,
  remark: '',
}))

const logs = Array.from({ length: 120 }, (_, index) => ({
  id: 900 + index,
  user_id: users[index % users.length].id,
  created_at: 1787390000 - index * 86,
  type: index === 2 ? 5 : 2,
  content: 'relay completed',
  username: users[index % users.length].username,
  token_name: keys[index % keys.length].name,
  model_name: models[index % models.length].model_name,
  quota: 12000 + index * 30,
  prompt_tokens: 580 + index,
  completion_tokens: 220 + index,
  use_time: 0.82 + (index % 8) * 0.06,
  is_stream: true,
  channel: channels[index % channels.length].id,
  channel_name: channels[index % channels.length].name,
  token_id: keys[index % keys.length].id,
  group: index % 3 === 0 ? 'premium' : 'default',
  ip: `10.0.0.${20 + (index % 200)}`,
  other: JSON.stringify({ request_path: '/v1/chat/completions', frt: 0.42 }),
  request_id: `req_${index}`,
  upstream_request_id: `up_${index}`,
}))

const pricing = models.map((model, index) => ({
  id: model.id,
  model_name: model.model_name,
  vendor_id: model.vendor_id,
  vendor_name: vendors.find((vendor) => vendor.id === model.vendor_id)?.name,
  quota_type: 0,
  model_ratio: [2.5, 3, 1.8, 0.6, 5, 1.2][index],
  completion_ratio: [4, 5, 3, 2, 5, 3][index],
  enable_groups: ['default', 'premium'],
  group_ratio: { default: 1, premium: 1.2 },
}))

const options = [
  ['RetryTimes', '2'],
  ['ServerAddress', 'https://api.renew.dev'],
  ['SystemName', 'RenewAPI'],
  ['QuotaForNewUser', '500000'],
  ['ModelPrice', '{}'],
  ['ModelRatio', '{}'],
  ['CacheRatio', '{}'],
  ['CompletionRatio', '{}'],
  ['ImageRatio', '{}'],
  ['AudioRatio', '{}'],
  ['AudioCompletionRatio', '{}'],
].map(([key, value]) => ({ key, value }))

const ok = (data, extra = {}) => ({ success: true, message: '', data, ...extra })
const observations = []
const consoleErrors = []
const pageErrors = []
const unhandled = []
let browser = null

function assert(condition, message, details) {
  if (condition) return
  throw new Error(`${message}${details === undefined ? '' : `\n${JSON.stringify(details, null, 2)}`}`)
}

function pageNumber(url) {
  return Number(url.searchParams.get('p') || url.searchParams.get('page') || 1)
}

function pageSize(url, fallback = 20) {
  return Number(url.searchParams.get('page_size') || url.searchParams.get('size') || fallback)
}

function paged(items, url, fallback = 20) {
  const p = pageNumber(url)
  const size = pageSize(url, fallback)
  const start = Math.max(0, (p - 1) * size)
  return { items: items.slice(start, start + size), total: items.length, page: p, page_size: size }
}

function targetForPath(pathname) {
  if (pathname === '/api/channel' || pathname === '/api/channel/' || pathname === '/api/channel/search') return 'channels'
  if (pathname === '/api/token/' || pathname === '/api/token/search') return 'keys'
  if (pathname === '/api/log/' || pathname === '/api/log' || pathname === '/api/log/self') return 'logs'
  if (pathname === '/api/models/' || pathname === '/api/models/search') return 'models'
  if (pathname === '/api/user/' || pathname === '/api/user' || pathname === '/api/user/search') return 'users'
  return null
}

function emptyPayload(target, url) {
  if (target === 'models') {
    return ok({ items: [], total: 0, page: 1, page_size: pageSize(url, 10), vendor_counts: { all: 0 } })
  }
  return ok({ items: [], total: 0, page: 1, page_size: pageSize(url, 20) })
}

function responseFor(url, method = 'GET') {
  const pathname = url.pathname
  if (pathname === '/api/setup') return ok({ status: true, root_init: true, database_type: 'postgres' })
  if (pathname === '/api/user/self') return ok(user)
  if (pathname === '/api/status') {
    return ok({
      system_name: 'RenewAPI',
      version: 'v1.0.0-rc.3',
      demo_site_enabled: false,
      display_token_stat_enabled: true,
      display_in_currency: true,
      quota_display_type: 'USD',
      quota_per_unit: 500000,
      usd_exchange_rate: 1,
      dashboard_default_time_range: '1d',
      dashboard_auto_refresh: false,
      dashboard_refresh_interval: 30,
      theme_customization: {
        preset: 'default',
        font: 'sans',
        radius: 'lg',
        scale: 'normal',
        content_layout: 'centered',
      },
    })
  }
  if (pathname === '/api/notice') return ok('')
  if (pathname === '/api/channel' || pathname === '/api/channel/' || pathname === '/api/channel/search') {
    return ok({ ...paged(channels, url, 10), type_counts: { 1: 4, 14: 4, 3: 4, 24: 4, 18: 4, 39: 4 } })
  }
  if (pathname === '/api/group/') return ok(['default', 'premium', 'trial'])
  if (pathname === '/api/token/' || pathname === '/api/token/search') return ok(paged(keys, url, 20))
  if (pathname === '/api/user/self/groups') return ok({ default: { desc: 'Default', ratio: 1 }, premium: { desc: 'Premium', ratio: 1.2 } })
  if (pathname === '/api/user/models') return ok(models.map((model) => model.model_name))
  if (pathname === '/api/log/stat' || pathname === '/api/log/self/stat') return ok({ quota: 86400000, rpm: 392, tpm: 128400 })
  if (pathname === '/api/log/' || pathname === '/api/log' || pathname === '/api/log/self') return ok(paged(logs, url, 100))
  if (pathname === '/api/models/' || pathname === '/api/models/search') {
    const page = paged(models, url, 10)
    return ok({ ...page, vendor_counts: { all: models.length, '1': 2, '2': 2, '3': 2 } })
  }
  if (/^\/api\/models\/\d+$/.test(pathname)) {
    const id = Number(pathname.split('/').pop())
    return ok(models.find((model) => model.id === id) || models[0])
  }
  if (pathname === '/api/vendors/' || pathname === '/api/vendors/search') {
    return ok({ items: vendors, total: vendors.length, page: 1, page_size: 1000 })
  }
  if (pathname === '/api/pricing') {
    return {
      success: true,
      data: pricing,
      vendors,
      group_ratio: { default: 1, premium: 1.2 },
      usable_group: { default: { desc: 'Default', ratio: 1 }, premium: { desc: 'Premium', ratio: 1.2 } },
      supported_endpoint: {},
      auto_groups: [],
    }
  }
  if (pathname === '/api/option/' || pathname === '/api/option') return ok(options)
  if (pathname === '/api/deployments/settings') return ok({ enabled: false })
  if (pathname === '/api/user/' || pathname === '/api/user' || pathname === '/api/user/search') return ok(paged(users, url, 20))

  if (pathname.startsWith('/api/')) {
    unhandled.push(`${method} ${pathname}${url.search}`)
    if (method === 'DELETE' || method === 'PUT' || method === 'POST') return ok(true)
    return ok([])
  }
  return null
}

async function createContext({
  theme = 'light',
  viewport = { width: 1440, height: 1000 },
  failTarget = null,
  emptyTarget = null,
  delayTarget = null,
} = {}) {
  let failureArmed = Boolean(failTarget)
  const context = await browser.newContext({
    viewport,
    deviceScaleFactor: 1,
    locale: 'en-US',
    timezoneId: 'Asia/Taipei',
    colorScheme: theme,
  })
  await context.addInitScript(({ user, fixedNow, theme }) => {
    localStorage.setItem('user', JSON.stringify(user))
    localStorage.setItem('uid', String(user.id))
    localStorage.setItem('i18nextLng', 'en-US')
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
  }, { user, fixedNow, theme })

  await context.route('**/api/**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const target = targetForPath(url.pathname)

    if (delayTarget && target === delayTarget) {
      await new Promise((resolve) => setTimeout(resolve, 2200))
    }
    if (failureArmed && failTarget && target === failTarget) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ success: false, message: `QA forced ${failTarget} failure` }),
      })
    }
    if (emptyTarget && target === emptyTarget) {
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(emptyPayload(emptyTarget, url)),
      })
    }

    const payload = responseFor(url, request.method())
    if (payload == null) {
      unhandled.push(`${request.method()} ${url.pathname}${url.search}`)
      return route.fulfill({
        status: 404,
        contentType: 'application/json',
        body: JSON.stringify({ success: false, message: 'QA mock missing' }),
      })
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(payload) })
  })

  return {
    context,
    disarmFailure() {
      failureArmed = false
    },
  }
}

async function preparePage(context, label, route) {
  const page = await context.newPage()
  page.on('console', (message) => {
    if (message.type() === 'error') consoleErrors.push(`[${label}] ${message.text()}`)
  })
  page.on('pageerror', (error) => pageErrors.push(`[${label}] ${error.message}`))
  await page.goto(`${APP}${route}`, { waitUntil: 'domcontentloaded' })
  await page.addStyleTag({ content: '*{animation:none!important;transition:none!important;caret-color:transparent!important}' })
  return page
}

async function screenshot(page, name) {
  await page.evaluate(() => document.fonts?.ready)
  await page.screenshot({ path: path.join(outDir, `${name}.png`), fullPage: false })
}

async function auditErrorScenario(scenario) {
  const handle = await createContext({ failTarget: scenario.target })
  const page = await preparePage(handle.context, `${scenario.target}-error`, scenario.route)
  try {
    const surface = page.getByRole('alert').filter({ hasText: scenario.title })
    await surface.waitFor({ state: 'visible', timeout: 25000 })
    await surface.scrollIntoViewIfNeeded()
    await screenshot(page, `${scenario.target}-error-light`)
    handle.disarmFailure()
    await surface.getByRole('button', { name: 'Retry' }).click()
    await page.getByText(scenario.recovery, { exact: false }).first().waitFor({ state: 'visible', timeout: 15000 })
    observations.push({ state: `${scenario.target}-error-retry`, passed: true })
  } finally {
    await page.close()
    await handle.context.close()
  }
}

async function auditEmptyScenario(scenario) {
  const handle = await createContext({ emptyTarget: scenario.target })
  const page = await preparePage(handle.context, `${scenario.target}-empty`, scenario.route)
  try {
    await page.getByText(scenario.text).first().waitFor({ state: 'visible', timeout: 15000 })
    await screenshot(page, `${scenario.target}-empty-light`)
    observations.push({ state: `${scenario.target}-empty`, passed: true })
  } finally {
    await page.close()
    await handle.context.close()
  }
}

async function main() {
  await fs.rm(outDir, { recursive: true, force: true })
  await fs.mkdir(outDir, { recursive: true })
  browser = await chromium.launch({ headless: true })

  const errorScenarios = [
    { target: 'channels', route: '/channels', title: 'Failed to load channels', recovery: 'Channel 01' },
    { target: 'keys', route: '/keys', title: 'Failed to load API keys', recovery: 'Key 01' },
    { target: 'logs', route: '/usage-logs/common', title: 'Failed to load logs', recovery: 'gpt-5.6' },
    { target: 'models', route: '/models/metadata', title: 'Failed to load models', recovery: 'gpt-5.6' },
    { target: 'users', route: '/users', title: 'Failed to load users', recovery: 'user01' },
  ]
  for (const scenario of errorScenarios) await auditErrorScenario(scenario)

  const darkMobile = await createContext({
    theme: 'dark',
    viewport: { width: 375, height: 812 },
    failTarget: 'keys',
  })
  const darkMobilePage = await preparePage(darkMobile.context, 'keys-error-dark-mobile', '/keys')
  try {
    const surface = darkMobilePage.getByRole('alert').filter({ hasText: 'Failed to load API keys' })
    await surface.waitFor({ state: 'visible', timeout: 25000 })
    await surface.scrollIntoViewIfNeeded()
    await screenshot(darkMobilePage, 'keys-error-dark-mobile')
    observations.push({ state: 'keys-error-dark-mobile', passed: true })
  } finally {
    await darkMobilePage.close()
    await darkMobile.context.close()
  }

  const emptyScenarios = [
    { target: 'channels', route: '/channels', text: /未找到渠道|No Channels Found|No channels/i },
    { target: 'keys', route: '/keys', text: /No API Keys Found/i },
    { target: 'logs', route: '/usage-logs/common', text: /No Logs Found/i },
    { target: 'models', route: '/models/metadata', text: /No Models Found/i },
    { target: 'users', route: '/users', text: /No Users Found/i },
  ]
  for (const scenario of emptyScenarios) await auditEmptyScenario(scenario)

  const loading = await createContext({ delayTarget: 'channels' })
  const loadingPage = await preparePage(loading.context, 'channels-loading', '/channels')
  try {
    await loadingPage.locator('[data-slot="skeleton"]').first().waitFor({ state: 'visible', timeout: 1500 })
    await screenshot(loadingPage, 'channels-loading-light')
    observations.push({ state: 'channels-loading', passed: true })
  } finally {
    await loadingPage.close()
    await loading.context.close()
  }

  const keysDeep = await createContext()
  const keysPage = await preparePage(keysDeep.context, 'keys-deep-states', '/keys')
  try {
    await keysPage.getByText('Key 01', { exact: true }).waitFor({ state: 'visible', timeout: 15000 })
    const firstRowCheckbox = keysPage.getByRole('checkbox', { name: 'Select row' }).first()
    await firstRowCheckbox.click()
    const bulkDelete = keysPage.getByRole('button', { name: 'Delete selected API keys' })
    await bulkDelete.waitFor({ state: 'visible' })
    await screenshot(keysPage, 'keys-bulk-selection-light')
    await bulkDelete.click()
    const confirm = keysPage.getByRole('alertdialog').filter({ hasText: /Delete 1 API key/i })
    await confirm.waitFor({ state: 'visible' })
    await screenshot(keysPage, 'keys-bulk-delete-confirm-light')
    await confirm.getByRole('button', { name: 'Cancel' }).click()
    await firstRowCheckbox.click()
    await keysPage.getByRole('button', { name: '2', exact: true }).click()
    await keysPage.getByText('Key 21', { exact: true }).waitFor({ state: 'visible', timeout: 10000 })
    await screenshot(keysPage, 'keys-page-2-light')
    observations.push({ state: 'keys-bulk-confirm-pagination', passed: true })
  } finally {
    await keysPage.close()
    await keysDeep.context.close()
  }

  const modelsDeep = await createContext()
  const modelsPage = await preparePage(modelsDeep.context, 'models-management', '/models/metadata')
  try {
    await modelsPage.getByText('gpt-5.6', { exact: true }).first().waitFor({ state: 'visible', timeout: 15000 })
    await modelsPage.getByRole('button', { name: 'Manage models' }).click()
    const firstMenu = modelsPage.getByRole('button', { name: 'Open menu' }).first()
    await firstMenu.waitFor({ state: 'visible' })
    await screenshot(modelsPage, 'models-management-open-light')

    await firstMenu.click()
    await modelsPage.getByRole('menuitem', { name: 'Edit' }).click()
    const editDrawer = modelsPage.getByRole('dialog').filter({ hasText: 'Edit Model' })
    await editDrawer.waitFor({ state: 'visible', timeout: 10000 })
    await editDrawer.getByDisplayValue('gpt-5.6').waitFor({ state: 'visible', timeout: 10000 })
    await screenshot(modelsPage, 'models-edit-drawer-light')
    await modelsPage.keyboard.press('Escape')
    await editDrawer.waitFor({ state: 'hidden', timeout: 10000 })

    await firstMenu.click()
    await modelsPage.getByRole('menuitem', { name: 'Delete' }).click()
    const deleteDialog = modelsPage.getByRole('alertdialog').filter({ hasText: 'Delete Model' })
    await deleteDialog.waitFor({ state: 'visible', timeout: 10000 })
    await screenshot(modelsPage, 'models-delete-confirm-light')
    observations.push({ state: 'models-management-edit-delete', passed: true })
  } finally {
    await modelsPage.close()
    await modelsDeep.context.close()
  }

  assert(unhandled.length === 0, 'Unhandled fixture API requests detected', unhandled)
  assert(consoleErrors.length === 0, 'Browser console errors detected', consoleErrors)
  assert(pageErrors.length === 0, 'Browser page errors detected', pageErrors)
}

let failure = null
try {
  await main()
} catch (error) {
  failure = error instanceof Error ? { message: error.message, stack: error.stack } : { message: String(error) }
} finally {
  await browser?.close().catch(() => {})
  await fs.mkdir(outDir, { recursive: true })
  const summary = {
    result: failure ? 'failed' : 'passed',
    observations,
    consoleErrors,
    pageErrors,
    unhandled,
    failure,
  }
  await fs.writeFile(path.join(outDir, 'summary.json'), `${JSON.stringify(summary, null, 2)}\n`)
  await fs.writeFile(
    path.join(outDir, 'summary.txt'),
    [
      `result=${summary.result}`,
      `states=${observations.length}`,
      `consoleErrors=${consoleErrors.length}`,
      `pageErrors=${pageErrors.length}`,
      `unhandled=${unhandled.length}`,
      failure ? `failure=${failure.message}` : 'failure=',
    ].join('\n') + '\n'
  )
}

if (failure) throw new Error(failure.message)
