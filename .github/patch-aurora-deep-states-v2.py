from pathlib import Path

path = Path('/tmp/aurora-deep-states.mjs')
source = path.read_text()

source = source.replace(
    "  models: ['No Models Found', '未找到模型'],",
    "  models: ['No Models Found', '未找到模型', 'No models registered yet', '还没有登记模型'],",
    1,
)

old_data_state = r'''async function runDataState(browser, feature, mode) {
  const scenario = {
    name: `${feature}-${mode}`,
    feature,
    mode,
    language: 'en-US',
    theme: 'light',
  }
  const context = await createContext(browser, scenario)
  try {
    const page = await preparePage(context, scenario)
    if (mode === 'loading') {
      await sleep(450)
      const skeletons = await page.locator('[data-slot="skeleton"]:visible').count()
      if (skeletons === 0) {
        addIssue('P2', scenario.name, 'loading state has no visible skeleton/progress surface')
      }
    } else {
      await sleep(900)
      const bodyText = await page.locator('body').innerText()
      const titleMatched = emptyTitles[feature].some((title) => bodyText.includes(title))
      if (mode === 'empty' && !titleMatched) {
        addIssue('P2', scenario.name, 'empty result has no explicit empty-state message', {
          expected: emptyTitles[feature],
        })
      }
      if (mode === 'error') {
        const marker = `QA_FORCED_${feature.toUpperCase()}_ERROR`
        if (!bodyText.includes(marker)) {
          addIssue('P2', scenario.name, 'forced API error is indistinguishable from an empty result', {
            marker,
          })
        }
        if (titleMatched) {
          addIssue('P2', scenario.name, 'error state simultaneously renders the normal empty-state message', {
            emptyTitles: emptyTitles[feature],
          })
        }
      }
    }
    await snapshot(page, scenario)
    await auditViewport(page, scenario)
  } finally {
    await context.close()
  }
}'''

new_data_state = r'''async function runDataState(browser, feature, mode) {
  const scenario = {
    name: `${feature}-${mode}`,
    feature,
    mode,
    language: 'en-US',
    theme: 'light',
  }
  const context = await createContext(browser, scenario)
  try {
    const page = await preparePage(context, scenario)
    if (mode === 'loading') {
      await sleep(450)
      const skeletons = await page.locator('[data-slot="skeleton"]:visible').count()
      if (skeletons === 0) {
        addIssue('P2', scenario.name, 'loading state has no visible skeleton/progress surface')
      }
    } else {
      if (mode === 'error') {
        const marker = `QA_FORCED_${feature.toUpperCase()}_ERROR`
        const persistentError = page.locator('main [role="alert"]', { hasText: marker }).first()
        try {
          await persistentError.waitFor({ state: 'visible', timeout: 15000 })
        } catch {
          addIssue('P2', scenario.name, 'forced API error never reaches a persistent in-page error surface', {
            marker,
          })
        }
      } else {
        await sleep(900)
      }
      const bodyText = await page.locator('body').innerText()
      const titleMatched = emptyTitles[feature].some((title) => bodyText.includes(title))
      if (mode === 'empty' && !titleMatched) {
        addIssue('P2', scenario.name, 'empty result has no explicit empty-state message', {
          expected: emptyTitles[feature],
        })
      }
      if (mode === 'error') {
        const marker = `QA_FORCED_${feature.toUpperCase()}_ERROR`
        if (!bodyText.includes(marker)) {
          addIssue('P2', scenario.name, 'forced API error is indistinguishable from an empty result', {
            marker,
          })
        }
        if (titleMatched) {
          addIssue('P2', scenario.name, 'error state simultaneously renders the normal empty-state message', {
            emptyTitles: emptyTitles[feature],
          })
        }
      }
    }
    await snapshot(page, scenario)
    await auditViewport(page, scenario)
  } finally {
    await context.close()
  }
}'''

if old_data_state not in source:
    raise SystemExit('runDataState block not found')
source = source.replace(old_data_state, new_data_state, 1)

old_management = r'''async function openManagement(page) {
  const button = page.getByRole('button', {
    name: /Manage models|Close management/i,
  })
  await button.waitFor({ state: 'visible', timeout: 15000 })
  if ((await button.getAttribute('aria-expanded')) !== 'true') {
    await button.click()
  }
  await page.locator('tbody').first().waitFor({ state: 'visible', timeout: 10000 })
}'''

new_management = r'''async function openManagement(page) {
  const button = page.getByRole('button', {
    name: /Manage models|Close management|管理模型|收起管理/i,
  })
  await button.waitFor({ state: 'visible', timeout: 15000 })
  if ((await button.getAttribute('aria-expanded')) !== 'true') {
    await button.click()
  }
  await page.locator('tbody').first().waitFor({ state: 'visible', timeout: 10000 })
}'''

if old_management not in source:
    raise SystemExit('openManagement block not found')
source = source.replace(old_management, new_management, 1)

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
  await trigger.focus()
  await trigger.press('Enter')
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
      if (language.startsWith('zh') && /Are you sure|This action cannot be undone|Delete\s*$/im.test(dialogText)) {
        addIssue('P2', scenario.name, 'delete confirmation contains untranslated English copy', {
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

old_bulk = r'''async function runBulkState(browser, feature, language = 'en-US') {
  const scenario = {
    name: `${feature}-bulk-${language}`,
    feature,
    mode: 'normal',
    language,
    theme: 'light',
  }
  const context = await createContext(browser, scenario)
  try {
    const page = await preparePage(context, scenario)
    await waitForBaseline(page, feature)
    const toolbar = await selectTwoRows(page, feature)
    const ariaLabel = (await toolbar.getAttribute('aria-label')) || ''
    const liveText = await page.locator('[role="status"]').last().innerText().catch(() => '')
    if (language.startsWith('zh')) {
      if (/Bulk actions for/i.test(ariaLabel)) {
        addIssue('P2', scenario.name, 'bulk toolbar accessible name is hard-coded English', {
          ariaLabel,
        })
      }
      if (/selected\. Bulk actions toolbar is available\./i.test(liveText)) {
        addIssue('P2', scenario.name, 'bulk selection live announcement is hard-coded English', {
          liveText,
        })
      }
    }
    await snapshot(page, scenario, 'toolbar')

    const buttons = toolbar.locator('button')
    if ((await buttons.count()) > 1) {
      await buttons.last().click()
      const dialog = page.locator('[role="dialog"]:visible, [role="alertdialog"]:visible').last()
      await dialog.waitFor({ state: 'visible', timeout: 5000 })
      await snapshot(page, scenario, 'confirm')
    }
    await auditViewport(page, scenario)
  } finally {
    await context.close()
  }
}'''

new_bulk = r'''async function runBulkState(browser, feature, language = 'en-US') {
  const scenario = {
    name: `${feature}-bulk-${language}`,
    feature,
    mode: 'normal',
    language,
    theme: 'light',
  }
  const context = await createContext(browser, scenario)
  try {
    const page = await preparePage(context, scenario)
    await waitForBaseline(page, feature)
    try {
      const toolbar = await selectTwoRows(page, feature)
      const ariaLabel = (await toolbar.getAttribute('aria-label')) || ''
      const liveText = await page.locator('[role="status"]').last().innerText().catch(() => '')
      if (language.startsWith('zh')) {
        if (/Bulk actions|channels|keys|models|selected/i.test(ariaLabel)) {
          addIssue('P2', scenario.name, 'bulk toolbar accessible name contains untranslated English copy', {
            ariaLabel,
          })
        }
        if (/Bulk actions|channels|keys|models|selected/i.test(liveText)) {
          addIssue('P2', scenario.name, 'bulk selection live announcement contains untranslated English copy', {
            liveText,
          })
        }
      }
      await snapshot(page, scenario, 'toolbar')

      const buttons = toolbar.locator('button')
      if ((await buttons.count()) > 1) {
        await buttons.last().click()
        const dialog = page.locator('[role="dialog"]:visible, [role="alertdialog"]:visible').last()
        await dialog.waitFor({ state: 'visible', timeout: 7000 })
        await snapshot(page, scenario, 'confirm')
      }
      await auditViewport(page, scenario)
    } catch (error) {
      addIssue('P0', scenario.name, 'bulk-state traversal failed', {
        message: error instanceof Error ? error.message : String(error),
      })
      await snapshot(page, scenario, 'traversal-failure').catch(() => {})
    }
  } finally {
    await context.close()
  }
}'''

if old_bulk not in source:
    raise SystemExit('runBulkState block not found')
source = source.replace(old_bulk, new_bulk, 1)

old_model_edit = r'''    const trigger = page.locator('tbody [data-slot="dropdown-menu-trigger"]').first()
    await trigger.click()
    const menu = page.locator('[data-slot="dropdown-menu-content"]:visible')
    await menu.waitFor({ state: 'visible', timeout: 5000 })'''
new_model_edit = r'''    const trigger = page.locator('tbody [data-slot="dropdown-menu-trigger"]:visible').first()
    await trigger.focus()
    await trigger.press('Enter')
    const menu = page.locator('[data-slot="dropdown-menu-content"]:visible').last()
    await menu.waitFor({ state: 'visible', timeout: 7000 })'''
if old_model_edit not in source:
    raise SystemExit('model edit menu block not found')
source = source.replace(old_model_edit, new_model_edit, 1)

old_key_assertion = r'''    if (!label || label === 'Show full API key') {
      addIssue('P2', keyScenario.name, 'API-key reveal trigger uses an untranslated or empty accessible label', {
        label,
      })
    }'''
new_key_assertion = r'''    if (label !== '完整 API 密钥') {
      addIssue('P2', keyScenario.name, 'API-key reveal trigger is not using the localized Chinese accessible label', {
        expected: '完整 API 密钥',
        label,
      })
    }'''
if old_key_assertion not in source:
    raise SystemExit('API-key localization assertion not found')
source = source.replace(old_key_assertion, new_key_assertion, 1)

path.write_text(source)
