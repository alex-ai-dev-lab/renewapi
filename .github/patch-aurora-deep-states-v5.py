from pathlib import Path

path = Path('/tmp/aurora-deep-states.mjs')
source = path.read_text()

old = r'''    await page.getByText('128,402').first().waitFor({ state: 'visible', timeout: 15000 })
    const trendItems = page.locator('ol.sr-only[aria-label] li')
    const count = await trendItems.count()'''
new = r'''    await page.getByText('128,402').first().waitFor({ state: 'visible', timeout: 15000 })
    const trendItems = page.locator('ol.sr-only[aria-label] li')
    await trendItems.nth(23).waitFor({ state: 'attached', timeout: 15000 }).catch(() => {})
    const count = await trendItems.count()'''

if old not in source:
    raise SystemExit('dashboard trend accessibility block not found')
source = source.replace(old, new, 1)

path.write_text(source)
