/* Focused, local-only checks run inside the existing disposable backend harness. */
module.exports = async function runMenuMappingFlows({ context, rootHeaders, check, responseJson }) {
  const assert = (value, message) => { if (!value) throw new Error(message) }
  const putOption = (key, value) => context.request.put('/api/option/', { headers: rootHeaders, data: { key, value } })
  const status = async () => (await responseJson(await context.request.get('/api/status'))).data
  const originalStatus = await status()

  await check('task menu order has independent persisted status field', async () => {
    const order = 'models,operations,system,access,usage,account'
    try {
      await responseJson(await putOption('SidebarTaskSectionOrder', order))
      const changed = await status()
      assert(changed.SidebarTaskSectionOrder === order, 'New task order was not exposed by status')
      assert(changed.SidebarSectionOrder === originalStatus.SidebarSectionOrder, 'Classic order was overwritten')
      const invalid = await putOption('SidebarTaskSectionOrder', 'chat,unknown')
      assert((await invalid.json()).success === false, 'Unknown task category was accepted')
    } finally {
      await responseJson(await putOption('SidebarTaskSectionOrder', originalStatus.SidebarTaskSectionOrder || ''))
    }
  })

  await check('system menu deep link obeys the legacy setting permission', async () => {
    let modules
    try { modules = JSON.parse(originalStatus.SidebarModulesAdmin || '{}') } catch { modules = {} }
    modules.admin = { ...modules.admin, enabled: true, setting: false }
    const page = await context.newPage()
    try {
      await responseJson(await putOption('SidebarModulesAdmin', JSON.stringify(modules)))
      await page.goto('/dashboard/overview', { waitUntil: 'networkidle' })
      assert(await page.locator('[data-slot="sidebar"] a[href^="/system-settings"]').count() === 0, 'Disabled settings deep link remained visible')
    } finally {
      await page.close()
      await responseJson(await putOption('SidebarModulesAdmin', originalStatus.SidebarModulesAdmin || '{}'))
    }
  })

  async function createChannel(name, mapping) {
    await responseJson(await context.request.post('/api/channel', {
      headers: rootHeaders,
      data: { mode: 'single', channel: { name, type: 1, status: 1, key: 'sk-local-qa-placeholder', base_url: 'http://127.0.0.1:9', models: 'gpt-4o-mini,gpt-4o', group: 'default', model_mapping: mapping, priority: 0, weight: 1, auto_ban: 0 } },
    }))
    const found = await responseJson(await context.request.get('/api/channel/search', { headers: rootHeaders, params: { keyword: name } }))
    const rows = Array.isArray(found.data) ? found.data : found.data?.items
    const channel = rows?.find(row => row.name === name)
    assert(channel?.id, 'Created QA channel was not found')
    return channel.id
  }
  async function getChannel(id) {
    return (await responseJson(await context.request.get(`/api/channel/${id}`, { headers: rootHeaders }))).data
  }
  async function saveEditor(page, id) {
    const saved = page.waitForResponse(response => response.request().method() === 'PUT' && response.url().includes(`/api/channel/${id}/config`))
    await page.locator('button[type="submit"]').first().click()
    await responseJson(await saved)
  }
  const editor = page => page.locator('[data-slot="model-mapping-editor"]')
  const rows = page => editor(page).locator('[data-mapping-rule-id]')
  const field = (row, name) => row.getByLabel(name, { exact: true })
  let mixedChannel

  await check('mapping editor first click, shared sources and targets survive save', async () => {
    mixedChannel = await createChannel('qa-mixed-mapping', '')
    const page = await context.newPage()
    try {
      await page.goto(`/channels/${mixedChannel}/edit`, { waitUntil: 'networkidle' })
      await editor(page).scrollIntoViewIfNeeded()
      assert(await rows(page).count() === 0, 'Empty channel already has unexpected rule rows')
      await editor(page).getByRole('button', { name: 'Add Mapping', exact: true }).click()
      await page.waitForTimeout(100)
      assert(await rows(page).count() === 1, 'First add was lost after the parent echoed its value')
      const first = rows(page).first()
      assert(await field(first, 'Original Model').evaluate(element => element === document.activeElement), 'First new row did not focus its source')
      const firstID = await first.getAttribute('data-mapping-rule-id')
      await field(first, 'Replacement Model').fill('qa-upstream-A')
      await editor(page).getByRole('button', { name: 'Add Mapping', exact: true }).click()
      const second = rows(page).nth(1)
      await field(second, 'Original Model').fill('gpt-4o-mini')
      await field(second, 'Replacement Model').fill('qa-upstream-B')
      await field(second, 'Priority').fill('80')
      assert(await rows(page).first().getAttribute('data-mapping-rule-id') === firstID, 'Editing another row changed draft identity')
      assert(await field(rows(page).first(), 'Replacement Model').inputValue() === 'qa-upstream-A', 'Target-first draft was discarded')
      await field(rows(page).first(), 'Original Model').fill('gpt-4o-mini')
      await field(rows(page).first(), 'Priority').fill('100')
      for (const [target, priority] of [['qa-upstream-A', '100'], ['qa-upstream-B', '60']]) {
        await editor(page).getByRole('button', { name: 'Add Mapping', exact: true }).click()
        const row = rows(page).last()
        await field(row, 'Original Model').fill('gpt-4o')
        await field(row, 'Replacement Model').fill(target)
        await field(row, 'Priority').fill(priority)
      }
      await saveEditor(page, mixedChannel)
      const stored = await getChannel(mixedChannel)
      const config = JSON.parse(stored.model_mapping)
      assert(config.version === 2 && config.rules.length === 4, 'Rules were collapsed into a dictionary')
      assert(config.rules.filter(rule => rule.from === 'gpt-4o-mini').length === 2, 'Repeated source was lost')
      assert(config.rules.filter(rule => rule.to === 'qa-upstream-A').length === 2, 'Shared upstream target was lost')
      assert(config.rules.every(rule => rule.id && rule.enabled === true), 'Rule identity or enabled state was lost')
      await page.goto(`/channels/${mixedChannel}/edit`, { waitUntil: 'networkidle' })
      await editor(page).scrollIntoViewIfNeeded()
      assert(await rows(page).count() === 4, 'Saved rules did not reload')
      await page.setViewportSize({ width: 390, height: 900 })
      await editor(page).scrollIntoViewIfNeeded()
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 2), 'Mapping editor overflows mobile viewport')
    } finally { await page.close() }
  })

  if (mixedChannel) await check('legacy update cannot silently downgrade saved rules', async () => {
    const before = await getChannel(mixedChannel)
    const downgrade = await context.request.put('/api/channel/', { headers: rootHeaders, data: { id: mixedChannel, model_mapping: '{"gpt-4o-mini":"old-target"}' } })
    assert((await downgrade.json()).success === false, 'Legacy client silently downgraded rules')
    assert((await getChannel(mixedChannel)).model_mapping === before.model_mapping, 'Rejected update changed stored mapping')
    await responseJson(await context.request.put('/api/channel/', { headers: rootHeaders, data: { id: mixedChannel, name: 'qa-mixed-mapping-renamed' } }))
    assert((await getChannel(mixedChannel)).model_mapping === before.model_mapping, 'Unrelated legacy update replaced rules')
  })

  if (mixedChannel) await check('explicit editor clear stores an empty v2 rules array', async () => {
    const page = await context.newPage()
    try {
      await page.goto(`/channels/${mixedChannel}/edit`, { waitUntil: 'networkidle' })
      await editor(page).scrollIntoViewIfNeeded()
      await editor(page).getByRole('button', { name: 'Clear mapping', exact: true }).click()
      await saveEditor(page, mixedChannel)
      const config = JSON.parse((await getChannel(mixedChannel)).model_mapping)
      assert(config.version === 2 && config.rules.length === 0, 'Clear used an unsafe legacy empty-string downgrade')
    } finally { await page.close() }
  })

  await check('legacy upgrade previews resolved targets rather than copying chain edges', async () => {
    const id = await createChannel('qa-legacy-mapping', JSON.stringify({ 'gpt-4o-mini': ['gpt-4o', 'qa-final'], 'gpt-4o': 'qa-upstream' }))
    const page = await context.newPage()
    try {
      await page.goto(`/channels/${id}/edit`, { waitUntil: 'networkidle' })
      await editor(page).scrollIntoViewIfNeeded()
      await editor(page).getByRole('button', { name: 'Preview upgrade to direct rules', exact: true }).click()
      await page.getByRole('button', { name: 'Confirm upgrade', exact: true }).click()
      await saveEditor(page, id)
      const config = JSON.parse((await getChannel(id)).model_mapping)
      assert(config.version === 2, 'Explicit upgrade did not change format')
      const targets = config.rules.filter(rule => rule.from === 'gpt-4o-mini').sort((a, b) => b.priority - a.priority).map(rule => rule.to)
      assert(JSON.stringify(targets) === JSON.stringify(['qa-upstream', 'qa-final']), 'Upgrade changed legacy final candidate order')
    } finally { await page.close() }
  })
}
