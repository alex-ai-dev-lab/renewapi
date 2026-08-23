import { chromium } from 'playwright'
import fs from 'node:fs/promises'
import path from 'node:path'

const APP = 'http://127.0.0.1:4173'
const outDir = process.env.QA_OUT || 'qa-artifacts/phase-c-deep-states'
const fixedNow = Date.parse('2026-08-23T08:45:00Z')
const user = {
  id: 1,
  username: 'qa-root',
  display_name: 'QA Root',
  email: 'qa@example.com',
  role: 100,
  status: 1,
  group: 'default',
  permissions: { sidebar_settings: true },
}

const channelSeeds = [
  [1, 'OpenAI-Main', 1, 412, 'gpt-5.6,gpt-4.1', 'default'],
  [2, 'Claude-Prod', 14, 528, 'claude-sonnet-4-5,claude-opus-4-1', 'premium'],
  [3, 'Azure-East', 3, 355, 'gpt-5.6,gpt-4.1-mini', 'premium'],
  [4, 'Gemini-Flash', 24, 890, 'gemini-2.5-pro,gemini-2.5-flash', 'default'],
  [5, 'DeepSeek-V3', 18, 305, 'deepseek-v3,deepseek-r1', 'budget'],
  [6, 'Moonshot-K2', 39, 460, 'kimi-k2,kimi-k2-thinking', 'default'],
]
const channels = channelSeeds.map(([id, name, type, response_time, models, group], i) => ({
  id,
  name,
  type,
  status: i === 4 ? 2 : 1,
  response_time,
  models,
  group,
  priority: (i + 1) * 10,
  weight: 0,
  base_url: '',
  key: 'sk-qa',
  created_time: 1750000000,
  test_time: 1750005000,
  balance: 100 - i * 8,
  used_quota: 1000000 + i * 200000,
  tag: i < 2 ? 'prod' : '',
  settings: '',
  other_info: '',
}))
const keys = Array.from({ length: 9 }, (_, i) => ({
  id: i + 1,
  name: ['Production', 'Mobile', 'Analytics', 'Staging', 'Partners', 'CLI', 'Batch', 'Search', 'Support'][i],
  key: `sk-qa-${i + 1}`,
  status: i === 7 ? 2 : 1,
  remain_quota: 9000000 - i * 530000,
  used_quota: 800000 + i * 620000,
  unlimited_quota: false,
  expired_time: -1,
  created_time: 1750000000 + i * 10000,
  accessed_time: 1750005000 + i * 10000,
  group: i % 3 === 0 ? 'premium' : 'default',
  cross_group_retry: false,
  model_limits_enabled: i % 2 === 0,
  model_limits: i % 2 === 0 ? 'gpt-5.6,claude-sonnet-4-5' : '',
  allow_ips: '',
}))
const vendors = [
  { id: 1, name: 'OpenAI' },
  { id: 2, name: 'Anthropic' },
  { id: 3, name: 'Google' },
  { id: 4, name: 'DeepSeek' },
]
const modelSeed = [
  ['gpt-5.6', 1],
  ['claude-sonnet-4-5', 2],
  ['gemini-2.5-pro', 3],
  ['deepseek-v3', 4],
  ['gpt-4.1-mini', 1],
  ['claude-opus-4-1', 2],
]
const models = modelSeed.map(([model_name, vendor_id], i) => ({
  id: i + 1,
  model_name,
  vendor_id,
  description: `QA metadata for ${model_name}`,
  endpoints: '/v1/chat/completions',
  status: i === 4 ? 0 : 1,
  sync_official: 1,
  created_time: 1750000000,
  updated_time: 1750000000,
  name_rule: 0,
  bound_channels: [{ name: channels[i % channels.length].name, type: channels[i % channels.length].type }],
  enable_groups: ['default', 'premium'],
  quota_types: [0],
  matched_models: [model_name],
  matched_count: 1,
  tags: [],
}))
const pricing = models.map((m, i) => ({
  id: m.id,
  model_name: m.model_name,
  vendor_id: m.vendor_id,
  vendor_name: vendors.find((v) => v.id === m.vendor_id)?.name,
  quota_type: 0,
  model_ratio: [2.5, 3, 1.8, 0.8, 0.6, 5][i],
  completion_ratio: [4, 5, 3, 2, 2, 5][i],
  enable_groups: ['default', 'premium'],
  group_ratio: { default: 1, premium: 1.2 },
}))
const users = Array.from({ length: 8 }, (_, i) => ({
  id: i + 10,
  username: ['alice', 'bob', 'charlie', 'diana', 'eve', 'frank', 'grace', 'henry'][i],
  display_name: ['Alice Chen', 'Bob Li', 'Charlie Wang', 'Diana Xu', 'Eve Zhou', 'Frank Lin', 'Grace Wu', 'Henry Luo'][i],
  email: `user${i + 1}@example.com`,
  quota: 10000000 + i * 1000000,
  used_quota: 1200000 + i * 540000,
  request_count: 1100 + i * 370,
  group: i < 2 ? 'premium' : i < 6 ? 'default' : 'trial',
  status: i === 6 ? 2 : 1,
  role: i === 0 ? 10 : 1,
  created_at: 1740000000 + i * 120000,
  updated_at: 1750000000,
  last_login_at: 1750000000,
  remark: '',
}))
const logs = Array.from({ length: 10 }, (_, i) => ({
  id: 900 + i,
  user_id: users[i % users.length].id,
  created_at: 1787390000 - i * 86,
  type: i === 3 ? 5 : 2,
  content: i === 3 ? 'upstream error' : 'relay completed',
  username: users[i % users.length].username,
  token_name: keys[i % keys.length].name,
  model_name: models[i % models.length].model_name,
  quota: 12000 + i * 900,
  prompt_tokens: 580 + i * 30,
  completion_tokens: 220 + i * 20,
  use_time: 0.82 + i * 0.06,
  is_stream: true,
  channel: channels[i % channels.length].id,
  channel_name: channels[i % channels.length].name,
  token_id: keys[i % keys.length].id,
  group: i % 3 === 0 ? 'premium' : 'default',
  ip: `10.0.0.${20 + i}`,
  other: JSON.stringify({ request_path: '/v1/chat/completions', frt: 0.42 + i * 0.01 }),
  request_id: `req_${i}`,
  upstream_request_id: `up_${i}`,
}))
const channelStats = channels.map((c, i) => ({
  channel_id: c.id,
  channel_name: c.name,
  total_requests: 3800 - i * 420,
  success_requests: 3700 - i * 410,
  failed_requests: 20 + i * 7,
  success_rate: [99.8, 99.5, 99.9, 96.2, 99.7, 98.9][i],
  error_rate: [0.2, 0.5, 0.1, 3.8, 0.3, 1.1][i],
  avg_first_token: c.response_time,
  avg_use_time: c.response_time + 680,
  total_cost: [96.4, 142.18, 58.02, 12.44, 31.2, 18.72][i],
  total_prompt_tokens: 1800000,
  total_output_tokens: 820000,
}))
const trend = Array.from({ length: 24 }, (_, i) => {
  const requests = Math.round(1200 + 3600 * Math.exp(-Math.pow((i - 18) / 6, 2)))
  const success = Math.round(requests * 0.991)
  return {
    timestamp: 1787310000 + i * 3600,
    requests,
    success,
    failure: requests - success,
    success_rate: (success / requests) * 100,
    error_rate: ((requests - success) / requests) * 100,
    avg_first_token: 430 + (i % 4) * 18,
    avg_use_time: 1200 + (i % 5) * 90,
    total_cost: requests * 0.0032,
    total_prompt_tokens: requests * 500,
    total_output_tokens: requests * 225,
  }
})
const overview = {
  total_requests: 128402,
  success_requests: 127388,
  failed_requests: 1014,
  success_rate: 99.21,
  error_rate: 0.79,
  requests_per_minute: 392,
  avg_first_token_time: 486,
  avg_use_time: 1288,
  total_cost: 412.36,
  total_prompt_tokens: 59300000,
  total_output_tokens: 27100000,
  active_channels: 26,
  active_users: 1847,
  trend,
  top_channels: channelStats.slice(0, 5),
  top_failing_channels: [channelStats[3]],
  slowest_channels: [channelStats[3], channelStats[1]],
  top_models: [],
  top_cost_users: [],
}
const options = [
  ['RetryTimes', '2'],
  ['AutomaticDisableChannelEnabled', 'true'],
  ['ServerAddress', 'https://api.renew.dev'],
  ['SystemName', 'RenewAPI'],
  ['QuotaForNewUser', '500000'],
].map(([key, value]) => ({ key, value }))

const ok = (data, extra = {}) => ({ success: true, message: '', data, ...extra })
const failed = (message = 'QA forced failure') => ({ success: false, message, data: null })
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

function paged(baseItems, url, defaultSize = 20, totalOverride) {
  const p = Math.max(1, Number(url.searchParams.get('p') || url.searchParams.get('page') || 1))
  const size = Math.max(1, Number(url.searchParams.get('page_size') || url.searchParams.get('size') || defaultSize))
  const total = totalOverride ?? baseItems.length
  const start = ((p - 1) * size) % Math.max(baseItems.length, 1)
  const items = baseItems.length ? Array.from({ length: Math.min(size, baseItems.length) }, (_, i) => ({ ...baseItems[(start + i) % baseItems.length] })) : []
  if (p > 1 && items[0]?.name) items[0].name = `${items[0].name}-Page-${p}`
  if (p > 1 && items[0]?.username) items[0].username = `${items[0].username}-page-${p}`
  if (p > 1 && items[0]?.model_name) items[0].model_name = `${items[0].model_name}-page-${p}`
  return { items, total, page: p, page_size: size }
}

function primaryKind(pathname) {
  if (pathname === '/api/channel' || pathname === '/api/channel/' || pathname === '/api/channel/search') return 'channels'
  if (pathname === '/api/token/' || pathname === '/api/token/search') return 'keys'
  if (pathname === '/api/log/' || pathname === '/api/log' || pathname === '/api/log/self') return 'logs'
  if (pathname === '/api/models/' || pathname === '/api/models/search') return 'models'
  if (pathname === '/api/user/' || pathname === '/api/user' || pathname === '/api/user/search') return 'users'
  return null
}

function responseFor(url, scenario) {
  const p = url.pathname
  const kind = primaryKind(p)
  if (scenario.mode === 'error' && kind === scenario.kind) return failed()
  if (scenario.mode === 'empty' && kind === scenario.kind) {
    if (kind === 'logs') return ok({ items: [], total: 0, page: 1, page_size: 100 })
    return ok({ items: [], total: 0, page: 1, page_size: 20, vendor_counts: { all: 0 } })
  }

  if (p === '/api/setup') return ok({ status: true, root_init: true, database_type: 'postgres' })
  if (p === '/api/user/self') return ok(user)
  if (p === '/api/status') return ok({ system_name: 'RenewAPI', version: 'v1.0.0-rc.3', demo_site_enabled: false, display_token_stat_enabled: true, display_in_currency: true, quota_display_type: 'USD', quota_per_unit: 500000, usd_exchange_rate: 1, dashboard_default_time_range: '1d', dashboard_auto_refresh: false, dashboard_refresh_interval: 30, dashboard_success_good: 99, dashboard_success_degraded: 97, theme_customization: { preset: 'default', font: 'sans', radius: 'lg', scale: 'normal', content_layout: 'centered' } })
  if (p === '/api/notice') return ok('')
  if (p === '/api/stats/overview') return ok(overview)
  if (p === '/api/stats/channels') return ok(channelStats)
  if (p === '/api/channel' || p === '/api/channel/' || p === '/api/channel/search') {
    const data = paged(channels, url, 20, scenario.mode === 'pagination' && scenario.kind === 'channels' ? 46 : 26)
    return ok({ ...data, type_counts: { '1': 8, '3': 4, '14': 4, '18': 3, '24': 4, '39': 3 } })
  }
  if (p === '/api/group/') return ok(['default', 'premium', 'trial', 'internal'])
  if (p === '/api/token/' || p === '/api/token/search') return ok(paged(keys, url, 20, 49))
  if (p === '/api/user/self/groups') return ok({ default: { desc: 'Default', ratio: 1 }, premium: { desc: 'Premium', ratio: 1.2 } })
  if (p === '/api/user/models') return ok(models.map((m) => m.model_name))
  if (p === '/api/log/stat' || p === '/api/log/self/stat') return ok({ quota: 86400000, rpm: 392, tpm: 128400 })
  if (p === '/api/log/' || p === '/api/log' || p === '/api/log/self') return ok(paged(logs, url, 100, 245))
  if (p === '/api/models/' || p === '/api/models/search') {
    const data = paged(models, url, 10, 42)
    return ok({ ...data, vendor_counts: { all: 42, '1': 16, '2': 10, '3': 9, '4': 7 } })
  }
  if (p === '/api/vendors/' || p === '/api/vendors/search') return ok({ items: vendors, total: vendors.length, page: 1, page_size: 1000 })
  if (p === '/api/pricing') return { success: true, data: pricing, vendors, group_ratio: { default: 1, premium: 1.2 }, usable_group: { default: { desc: 'Default', ratio: 1 }, premium: { desc: 'Premium', ratio: 1.2 } }, supported_endpoint: {}, auto_groups: [] }
  if (p === '/api/option/' || p === '/api/option') return ok(options)
  if (p === '/api/deployments/settings') return ok({ enabled: false })
  if (p === '/api/user/' || p === '/api/user' || p === '/api/user/search') return ok(paged(users, url, 20, 1847))
  if (p.startsWith('/api/')) return ok([])
  return null
}

const scenarios = [
  { name: 'channels-loading', route: '/channels', kind: 'channels', mode: 'loading' },
  { name: 'channels-empty', route: '/channels', kind: 'channels', mode: 'empty' },
  { name: 'channels-error', route: '/channels', kind: 'channels', mode: 'error', expectErrorSignal: true },
  { name: 'channels-pagination', route: '/channels', kind: 'channels', mode: 'pagination', action: 'paginate' },
  { name: 'channels-bulk-selected', route: '/channels', kind: 'channels', mode: 'normal', action: 'channels-bulk' },
  { name: 'channels-bulk-delete-confirm', route: '/channels', kind: 'channels', mode: 'normal', action: 'channels-bulk-delete' },
  { name: 'keys-loading', route: '/keys', kind: 'keys', mode: 'loading' },
  { name: 'keys-empty', route: '/keys', kind: 'keys', mode: 'empty' },
  { name: 'keys-error', route: '/keys', kind: 'keys', mode: 'error', expectErrorSignal: true },
  { name: 'keys-delete-confirm', route: '/keys', kind: 'keys', mode: 'normal', action: 'row-delete' },
  { name: 'logs-loading', route: '/usage-logs/common', kind: 'logs', mode: 'loading' },
  { name: 'logs-empty', route: '/usage-logs/common', kind: 'logs', mode: 'empty' },
  { name: 'logs-error', route: '/usage-logs/common', kind: 'logs', mode: 'error', expectErrorSignal: true },
  { name: 'logs-filtered', route: '/usage-logs/common?model=gpt-5.6&type=2', kind: 'logs', mode: 'normal' },
  { name: 'models-loading', route: '/models/metadata', kind: 'models', mode: 'loading' },
  { name: 'models-empty', route: '/models/metadata', kind: 'models', mode: 'empty' },
  { name: 'models-error', route: '/models/metadata', kind: 'models', mode: 'error', expectErrorSignal: true },
  { name: 'models-management', route: '/models/metadata', kind: 'models', mode: 'normal', action: 'models-management' },
  { name: 'models-edit-drawer', route: '/models/metadata', kind: 'models', mode: 'normal', action: 'models-edit' },
  { name: 'models-delete-confirm', route: '/models/metadata', kind: 'models', mode: 'normal', action: 'models-delete' },
  { name: 'users-loading', route: '/users', kind: 'users', mode: 'loading' },
  { name: 'users-empty', route: '/users', kind: 'users', mode: 'empty' },
  { name: 'users-error', route: '/users', kind: 'users', mode: 'error', expectErrorSignal: true },
  { name: 'users-delete-confirm', route: '/users', kind: 'users', mode: 'normal', action: 'row-delete' },
]

await fs.rm(outDir, { recursive: true, force: true })
await fs.mkdir(path.join(outDir, 'screenshots'), { recursive: true })
const browser = await chromium.launch({ headless: true })
const issues = []
const summaries = []
const allConsoleErrors = []
const allUnhandled = []

function issue(scenario, severity, code, message, evidence = {}) {
  issues.push({ scenario: scenario.name, severity, code, message, evidence })
}

async function assertViewport(page, scenario, label, locator) {
  const count = await locator.count()
  if (!count) {
    issue(scenario, 'P1', `${label}-missing`, `${label} did not open or render`)
    return
  }
  const box = await locator.first().boundingBox()
  if (!box) return
  const viewport = page.viewportSize()
  if (!viewport) return
  if (box.x < -1 || box.y < -1 || box.x + box.width > viewport.width + 1 || box.y + box.height > viewport.height + 1) issue(scenario, 'P1', `${label}-overflow`, `${label} exceeds the viewport`, { box, viewport })
}

async function clickRowMenuDelete(page, scenario) {
  const menuTriggers = page.getByRole('button', { name: /Open menu|打开菜单|菜单/i })
  if (!(await menuTriggers.count())) {
    issue(scenario, 'P1', 'row-menu-missing', 'No accessible row action menu trigger found')
    return
  }
  await menuTriggers.first().click()
  const deleteItem = page.getByRole('menuitem', { name: /^Delete$|^删除$/i })
  if (!(await deleteItem.count())) {
    issue(scenario, 'P1', 'delete-action-missing', 'Delete row action not found')
    return
  }
  await deleteItem.click()
  await page.waitForTimeout(150)
  await assertViewport(page, scenario, 'delete-dialog', page.getByRole('dialog'))
}

async function runAction(page, scenario) {
  if (!scenario.action) return
  if (scenario.action === 'paginate') {
    const page2 = page.getByRole('button', { name: /前往第 2 页|Go to page 2/i })
    if (!(await page2.count())) {
      issue(scenario, 'P1', 'pagination-page-2-missing', 'Page 2 pagination control was not available')
      return
    }
    await page2.click()
    await page.waitForTimeout(650)
    if (!page.url().match(/[?&](page|p)=2/)) issue(scenario, 'P2', 'pagination-url-state', 'Pagination did not persist page 2 in URL state', { url: page.url() })
    return
  }
  if (scenario.action === 'channels-bulk' || scenario.action === 'channels-bulk-delete') {
    const boxes = page.getByRole('checkbox')
    if ((await boxes.count()) < 3) {
      issue(scenario, 'P1', 'bulk-checkboxes-missing', 'Expected header + at least two row selection checkboxes')
      return
    }
    await boxes.nth(1).click()
    await boxes.nth(2).click()
    await page.waitForTimeout(150)
    const deleteSelected = page.getByRole('button', { name: /Delete selected channels|删除.*渠道/i })
    if (!(await deleteSelected.count())) {
      issue(scenario, 'P1', 'bulk-toolbar-missing', 'Selected rows did not reveal the bulk action toolbar')
      return
    }
    if (scenario.action === 'channels-bulk-delete') {
      await deleteSelected.click()
      await page.waitForTimeout(150)
      await assertViewport(page, scenario, 'bulk-delete-dialog', page.getByRole('dialog'))
    }
    return
  }
  if (scenario.action === 'row-delete') {
    await clickRowMenuDelete(page, scenario)
    return
  }
  if (scenario.action.startsWith('models-')) {
    const manage = page.getByRole('button', { name: /Manage models|管理模型/i })
    if (!(await manage.count())) {
      issue(scenario, 'P1', 'model-management-toggle-missing', 'Manage models toggle not found')
      return
    }
    await manage.click()
    await page.waitForTimeout(250)
    if (scenario.action === 'models-management') return
    const menu = page.getByRole('button', { name: /Open menu|打开菜单|菜单/i })
    if (!(await menu.count())) {
      issue(scenario, 'P1', 'model-row-menu-missing', 'Model row menu not found after opening management')
      return
    }
    await menu.first().click()
    if (scenario.action === 'models-edit') {
      const edit = page.getByRole('menuitem', { name: /^Edit$|^编辑$/i })
      if (!(await edit.count())) {
        issue(scenario, 'P1', 'model-edit-missing', 'Model edit action not found')
        return
      }
      await edit.click()
      await page.waitForTimeout(200)
      await assertViewport(page, scenario, 'model-edit-drawer', page.getByRole('dialog'))
      return
    }
    if (scenario.action === 'models-delete') {
      const del = page.getByRole('menuitem', { name: /^Delete$|^删除$/i })
      if (!(await del.count())) {
        issue(scenario, 'P1', 'model-delete-missing', 'Model delete action not found')
        return
      }
      await del.click()
      await page.waitForTimeout(150)
      await assertViewport(page, scenario, 'model-delete-dialog', page.getByRole('dialog'))
    }
  }
}

for (const scenario of scenarios) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 1, locale: 'zh-CN', timezoneId: 'Asia/Taipei', colorScheme: 'light' })
  await context.addInitScript(({ user, fixedNow }) => {
    localStorage.setItem('user', JSON.stringify(user))
    localStorage.setItem('uid', String(user.id))
    localStorage.setItem('i18nextLng', 'zh-CN')
    localStorage.setItem('theme', 'light')
    const NativeDate = Date
    class FixedDate extends NativeDate {
      constructor(...args) { super(...(args.length ? args : [fixedNow])) }
      static now() { return fixedNow }
    }
    Date = FixedDate
  }, { user, fixedNow })

  const requests = []
  const scenarioConsoleErrors = []
  await context.route('**/api/**', async (route) => {
    const url = new URL(route.request().url())
    const kind = primaryKind(url.pathname)
    requests.push(`${route.request().method()} ${url.pathname}${url.search}`)
    if (scenario.mode === 'loading' && kind === scenario.kind) await sleep(5000)
    const payload = responseFor(url, scenario)
    if (payload == null) {
      allUnhandled.push(`[${scenario.name}] ${route.request().method()} ${url.pathname}${url.search}`)
      return route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ success: false, message: 'QA mock missing' }) })
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(payload) })
  })

  const page = await context.newPage()
  page.on('console', (m) => { if (m.type() === 'error') scenarioConsoleErrors.push(m.text()) })
  page.on('pageerror', (e) => scenarioConsoleErrors.push(`PAGEERROR ${e.message}`))
  await page.goto(`${APP}${scenario.route}`, { waitUntil: 'domcontentloaded' })
  await page.addStyleTag({ content: '*{animation:none!important;transition:none!important;caret-color:transparent!important}' })

  if (scenario.mode === 'loading') await page.waitForTimeout(700)
  else {
    await page.waitForTimeout(1700)
    await runAction(page, scenario)
    await page.waitForTimeout(250)
  }

  const bodyText = await page.locator('body').innerText()
  const hasForcedFailure = bodyText.includes('QA forced failure')
  if (scenario.expectErrorSignal && !hasForcedFailure) {
    const emptyTokens = ['未找到渠道', 'No API Keys Found', 'No Logs Found', 'No Models Found', 'No Users Found', '未找到']
    const emptyEvidence = emptyTokens.find((token) => bodyText.includes(token))
    issue(scenario, scenario.kind === 'channels' || scenario.kind === 'models' ? 'P1' : 'P2', 'error-state-not-persistent', emptyEvidence ? `Primary API failure is represented as an empty state (${emptyEvidence}) without persistent error recovery UI` : 'Primary API failure has no persistent in-page error recovery UI', { emptyEvidence: emptyEvidence || null })
  }

  if (scenario.mode === 'empty') {
    const rowCount = await page.locator('tbody tr').count()
    if (!rowCount) issue(scenario, 'P2', 'empty-state-missing', 'Empty response rendered no table empty-state row')
  }
  if (scenario.mode === 'loading') {
    const skeletonCount = await page.locator('[class*="skeleton"], [class*="animate-pulse"]').count()
    if (!skeletonCount) issue(scenario, 'P2', 'loading-state-missing', 'Delayed primary request did not render a visible skeleton/loading state')
  }

  const metrics = await page.evaluate(() => ({ scrollWidth: document.documentElement.scrollWidth, clientWidth: document.documentElement.clientWidth, scrollHeight: document.documentElement.scrollHeight, clientHeight: document.documentElement.clientHeight }))
  if (metrics.scrollWidth > metrics.clientWidth + 1) issue(scenario, 'P1', 'horizontal-overflow', 'Scenario causes page-level horizontal overflow', metrics)
  if (scenarioConsoleErrors.length) {
    for (const error of scenarioConsoleErrors) allConsoleErrors.push(`[${scenario.name}] ${error}`)
    issue(scenario, 'P1', 'console-error', 'Browser console/page error occurred', { errors: scenarioConsoleErrors })
  }

  await page.screenshot({ path: path.join(outDir, 'screenshots', `${scenario.name}.png`), fullPage: false })
  summaries.push({ scenario: scenario.name, url: page.url(), requests, metrics, consoleErrors: scenarioConsoleErrors })
  await context.close()
}

await browser.close()
await fs.writeFile(path.join(outDir, 'issues.json'), JSON.stringify(issues, null, 2))
await fs.writeFile(path.join(outDir, 'summary.json'), JSON.stringify(summaries, null, 2))
await fs.writeFile(path.join(outDir, 'console-errors.txt'), allConsoleErrors.join('\n'))
await fs.writeFile(path.join(outDir, 'unhandled.txt'), allUnhandled.join('\n'))
await fs.writeFile(path.join(outDir, 'summary.txt'), [`scenarios=${scenarios.length}`, `issues=${issues.length}`, `P0=${issues.filter((x) => x.severity === 'P0').length}`, `P1=${issues.filter((x) => x.severity === 'P1').length}`, `P2=${issues.filter((x) => x.severity === 'P2').length}`, `consoleErrors=${allConsoleErrors.length}`, `unhandled=${allUnhandled.length}`].join('\n'))
if (issues.length || allUnhandled.length || allConsoleErrors.length) {
  console.error(JSON.stringify({ issues, unhandled: allUnhandled, consoleErrors: allConsoleErrors }, null, 2))
  process.exit(1)
}
