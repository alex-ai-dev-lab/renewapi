from pathlib import Path

path = Path('/tmp/aurora-deep-states.mjs')
source = path.read_text()

old_open = r'''async function openRowDelete(page, feature) {
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
}'''

new_open = r'''async function openRowDelete(page, feature) {
  if (feature === 'models') {
    await openManagement(page)
  }
  const table = page.locator('tbody').first()
  await table.waitFor({ state: 'visible', timeout: 10000 })
  const namedTrigger = table.getByRole('button', { name: /Open menu|打开菜单/i }).first()
  const slotTrigger = table.locator('[data-slot="dropdown-menu-trigger"]:visible').first()
  const trigger = (await namedTrigger.count()) > 0 ? namedTrigger : slotTrigger
  await trigger.waitFor({ state: 'visible', timeout: 10000 })
  await trigger.click()
  await page.waitForTimeout(350)

  const menu = page.locator('[data-slot="dropdown-menu-content"]:visible').last()
  await menu.waitFor({ state: 'visible', timeout: 7000 })
  let deleteItem = menu.getByRole('menuitem', { name: /Delete|删除/i }).last()
  if ((await deleteItem.count()) === 0) {
    deleteItem = menu.locator('[data-slot="dropdown-menu-item"].text-destructive').last()
  }
  if ((await deleteItem.count()) === 0) {
    throw new Error(`${feature}: visible row-action menu has no delete action`)
  }
  await deleteItem.click()
  await page.waitForTimeout(300)
  const dialog = page.locator('[role="alertdialog"]:visible, [role="dialog"]:visible').last()
  await dialog.waitFor({ state: 'visible', timeout: 7000 })
  return dialog
}'''

if old_open not in source:
    raise SystemExit('openRowDelete block not found')
source = source.replace(old_open, new_open, 1)

old_run = r'''async function runDeleteState(browser, feature, language = 'en-US', theme = 'light') {
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
}'''

new_run = r'''async function runDeleteState(browser, feature, language = 'en-US', theme = 'light') {
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
    try {
      const dialog = await openRowDelete(page, feature)
      const dialogText = await dialog.innerText()
      if (language.startsWith('zh') && dialogText.includes('Are you sure you want to delete')) {
        addIssue('P2', scenario.name, 'delete confirmation description bypasses i18n', {
          dialogText,
        })
      }
      await snapshot(page, scenario)
      await auditViewport(page, scenario)
    } catch (error) {
      addIssue('P0', scenario.name, 'destructive-state traversal failed', {
        message: error instanceof Error ? error.message : String(error),
      })
      await snapshot(page, scenario, 'traversal-failure').catch(() => {})
    }
  } finally {
    await context.close()
  }
}'''

if old_run not in source:
    raise SystemExit('runDeleteState block not found')
source = source.replace(old_run, new_run, 1)

path.write_text(source)
