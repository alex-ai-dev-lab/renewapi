from pathlib import Path

path = Path('/tmp/aurora-deep-states.mjs')
source = path.read_text()

old_row_open = r'''  await trigger.waitFor({ state: 'visible', timeout: 10000 })
  await trigger.focus()
  await trigger.press('Enter')
  await page.waitForTimeout(350)

  const menu = page.locator('[data-slot="dropdown-menu-content"]:visible').last()'''
new_row_open = r'''  await trigger.waitFor({ state: 'visible', timeout: 10000 })
  await trigger.scrollIntoViewIfNeeded()
  await trigger.click()
  await page.waitForTimeout(350)

  const menu = page.locator('[data-slot="dropdown-menu-content"]:visible').last()'''
if old_row_open not in source:
    raise SystemExit('hardened row-menu trigger block not found')
source = source.replace(old_row_open, new_row_open, 1)

old_logs = r'''async function runLogsPagination(browser) {
  const scenario = {
    name: 'logs-pagination-page-2',
    feature: 'logs',
    mode: 'normal',
    language: 'en-US',
    theme: 'light',
  }
  const context = await createContext(browser, scenario)
  try {
    const page = await preparePage(context, scenario)
    await waitForBaseline(page, 'logs')
    const next = page.getByRole('button', { name: /next page/i })
    await next.click()
    await sleep(650)
    const relevant = requestLog.filter((line) => line.startsWith(`${scenario.name} `))
    if (!relevant.some((line) => /\/api\/log.*[?&]p=2/.test(line))) {
      addIssue('P2', scenario.name, 'pagination did not request page 2', { relevant })
    }
    await snapshot(page, scenario)
    await auditViewport(page, scenario)
  } finally {
    await context.close()
  }
}'''
new_logs = r'''async function runLogsPagination(browser) {
  const scenario = {
    name: 'logs-pagination-page-2',
    feature: 'logs',
    mode: 'normal',
    language: 'en-US',
    theme: 'light',
  }
  const context = await createContext(browser, scenario)
  try {
    const page = await preparePage(context, scenario)
    await waitForBaseline(page, 'logs')
    try {
      const named = page.getByRole('button', {
        name: /Go to next page|next page|前往下一页/i,
      }).first()
      const icon = page.locator('button:has(svg.lucide-chevron-right):visible').last()
      const next = (await named.count()) > 0 ? named : icon
      await next.waitFor({ state: 'visible', timeout: 7000 })
      const labels = await page.locator('button[aria-label]').evaluateAll((nodes) =>
        nodes.map((node) => node.getAttribute('aria-label')).filter(Boolean)
      )
      if (await next.isDisabled()) {
        addIssue('P0', scenario.name, 'page-2 fixture exposes a disabled next-page control', {
          labels,
        })
      } else {
        await next.click()
        await sleep(850)
        const relevant = requestLog.filter((line) => line.startsWith(`${scenario.name} `))
        if (!relevant.some((line) => /\/api\/log.*[?&]p=2/.test(line))) {
          addIssue('P2', scenario.name, 'pagination did not request page 2', {
            relevant,
            labels,
          })
        }
      }
      await snapshot(page, scenario)
      await auditViewport(page, scenario)
    } catch (error) {
      const labels = await page.locator('button[aria-label]').evaluateAll((nodes) =>
        nodes.map((node) => node.getAttribute('aria-label')).filter(Boolean)
      ).catch(() => [])
      addIssue('P0', scenario.name, 'pagination traversal failed', {
        message: error instanceof Error ? error.message : String(error),
        labels,
      })
      await snapshot(page, scenario, 'traversal-failure').catch(() => {})
    }
  } finally {
    await context.close()
  }
}'''
if old_logs not in source:
    raise SystemExit('logs pagination block not found')
source = source.replace(old_logs, new_logs, 1)

path.write_text(source)
