/* Local browser acceptance; disposable SQLite and a non-routable synthetic channel. */
const { spawn, spawnSync } = require('node:child_process')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const crypto = require('node:crypto')
const { chromium } = require('playwright')
const assert = require('node:assert/strict')
const { createDisposableEnvironment } = require('./disposable-environment.cjs')
const binary = process.env.QA_BINARY
const root = path.resolve(__dirname, '../..')
const output = path.resolve(process.env.QA_OUT || path.join(root, 'qa-artifacts/codex-channels'))
const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'renewapi-codex-channels-'))
const baseURL = 'http://127.0.0.1:39117'
const report = { checks: [], errors: [] }
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms))
let child, browser, log, page
;(async () => {
  assert(binary && fs.existsSync(binary), 'Set QA_BINARY to the local build')
  const env = { ...createDisposableEnvironment(process.env, temporary, crypto.randomBytes(32).toString('hex')), PORT: '39117' }
  const migration = spawnSync(binary, ['migrate', '--up'], { cwd: temporary, env, encoding: 'utf8', windowsHide: true })
  assert.equal(migration.status, 0, migration.stderr)
  log = fs.openSync(path.join(temporary, 'backend.log'), 'w')
  child = spawn(binary, [], { cwd: temporary, env, stdio: ['ignore', log, log], windowsHide: true })
  for (let n = 0; n < 100; n++) {
    try { if ((await fetch(baseURL + '/api/setup')).ok) break } catch {}
    assert.equal(child.exitCode, null, 'Local backend exited')
    await sleep(300)
  }
  browser = await chromium.launch({ executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true })
  const context = await browser.newContext({ baseURL, locale: 'en-US', viewport: { width: 1440, height: 1000 }, reducedMotion: 'reduce' })
  await context.addInitScript(() => localStorage.setItem('i18nextLng', 'en'))
  page = await context.newPage()
  page.on('pageerror', error => report.errors.push(error.message))
  const json = async response => { const body = await response.json(); assert(response.ok() && body.success, body.message); return body }
  const password = 'Qa!' + crypto.randomBytes(12).toString('hex')
  await json(await context.request.post('/api/setup', { data: { username: 'codex_qa', password, confirmPassword: password, SelfUseModeEnabled: false, DemoSiteEnabled: false } }))
  await page.goto('/sign-in', { waitUntil: 'networkidle' })
	await page.getByRole('button', { name: 'Continue with password', exact: true }).click()
  await page.locator('input[name="username"]').fill('codex_qa')
  await page.locator('input[name="password"]').fill(password)
  const loggedIn = page.waitForResponse(r => r.url().includes('/api/user/login') && r.request().method() === 'POST')
  await page.locator('form button[type="submit"]').click()
  const user = (await json(await loggedIn)).data
  await page.waitForURL(/\/dashboard/)
  const headers = { 'New-Api-User': String(user.id) }
  await json(await context.request.post('/api/channel', { headers, data: { mode: 'single', channel: { name: 'codex-icons-qa', type: 1, status: 1, key: 'local-placeholder', base_url: 'http://127.0.0.1:9', models: 'gemini-test,deepseek-test,claude-test,gpt-test,qwen-test,unrecognized-test', group: 'default', priority: 0, weight: 1, auto_ban: 0 } } }))
  const list = (await json(await context.request.get('/api/channel/search?keyword=codex-icons-qa', { headers }))).data
  const channel = (Array.isArray(list) ? list : list.items).find(item => item.name === 'codex-icons-qa')
  assert(channel)
  fs.mkdirSync(output, { recursive: true })
  for (const width of (process.env.QA_DESKTOP_ONLY === '1' ? [1440] : [1440, 390])) {
    await page.setViewportSize({ width, height: 1000 })
    await page.goto('/channels', { waitUntil: 'networkidle' })
    if (width === 1440) {
      await page.getByRole('button', { name: 'View', exact: true }).click()
      await page.getByRole('menuitemcheckbox', { name: 'Models', exact: true }).click()
      await page.keyboard.press('Escape')
    }
    const openai = page.getByRole('button', { name: /^OpenAI: gpt-test/ }).first()
    await openai.waitFor()
    await openai.locator('svg').waitFor()
    const labels = await openai.locator('..').locator('button').evaluateAll(nodes => nodes.map(n => n.getAttribute('aria-label')))
    assert(labels[0].startsWith('OpenAI:'), labels.join('|'))
    assert(labels[1].startsWith('Claude:'), labels.join('|'))
    assert(labels.length <= 5)
    await openai.focus()
    await page.keyboard.press('Enter')
    await page.getByRole('dialog').waitFor()
    assert((await page.getByRole('dialog').innerText()).includes('gpt-test'))
    await page.keyboard.press('Escape')
    await page.getByRole('dialog').waitFor({ state: 'hidden' })
    await page.screenshot({ path: path.join(output, `icons-${width}.png`) })
    report.checks.push(`provider order, five-slot limit, keyboard popup at ${width}px`)
  }
  await page.setViewportSize({ width: 1440, height: 1000 })
  await page.goto(`/channels/${channel.id}/edit`, { waitUntil: 'networkidle' })
  const profile = page.getByRole('combobox', { name: 'Responses request profile' })
  if (!(await profile.isVisible())) {
    const advanced = page.getByRole('button', { name: /Advanced Settings|高级设置/ }).first()
    if (await advanced.count()) await advanced.click()
  }
  await profile.scrollIntoViewIfNeeded()
  await profile.click()
  await page.getByRole('option', { name: 'Codex compatible', exact: true }).click()
  const limit = page.getByRole('combobox', { name: 'Upstream output token limit' })
  await limit.click()
  await page.getByRole('option', { name: 'Unsupported: reject explicit limit', exact: true }).click()
  await page.getByLabel('Parameter overrides after protocol conversion', { exact: true }).fill('{"include":["reasoning.encrypted_content"]}')
  const saved = page.waitForResponse(r => r.request().method() === 'PUT' && r.url().includes(`/api/channel/${channel.id}/config`))
  await page.locator('button[type="submit"]').first().click()
  await json(await saved)
  const updated = (await json(await context.request.get(`/api/channel/${channel.id}`, { headers }))).data
  const setting = JSON.parse(updated.setting)
  assert.equal(setting.responses_request_profile, 'codex')
  assert.equal(setting.responses_supports_max_output_tokens, false)
  assert.deepEqual(setting.upstream_param_override, { include: ['reasoning.encrypted_content'] })
  report.checks.push('channel editor saves Codex profile, explicit false capability and upstream overrides')
  assert.deepEqual(report.errors, [])
})().catch(async error => { report.errors.push(error.stack); if (page) { fs.mkdirSync(output,{recursive:true}); await page.screenshot({path:path.join(output,'failure.png')}); report.url=page.url(); report.body=(await page.locator('body').innerText()).slice(0,2000) }; process.exitCode = 1 }).finally(async () => {
  if (browser) await browser.close()
  if (child && child.exitCode === null) { child.kill(); await Promise.race([new Promise(resolve => child.once('exit', resolve)), sleep(3000)]) }
  if (log !== undefined) fs.closeSync(log)
  fs.mkdirSync(output, { recursive: true })
  fs.writeFileSync(path.join(output, 'report.json'), JSON.stringify(report, null, 2))
  console.log(JSON.stringify(report, null, 2))
})
