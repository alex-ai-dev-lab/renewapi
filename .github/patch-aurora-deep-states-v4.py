from pathlib import Path

path = Path('/tmp/aurora-deep-states.mjs')
source = path.read_text()

old = r'''    let payload = null
    if (request.method() === 'GET' && matchesFeature(scenario.feature, url.pathname)) {'''
new = r'''    let payload = null
    if (
      request.method() === 'POST' &&
      /^\/api\/token\/\d+\/key$/.test(url.pathname)
    ) {
      payload = {
        success: true,
        message: '',
        data: { key: 'qa-resolved-real-key' },
      }
    }
    if (
      payload == null &&
      request.method() === 'GET' &&
      matchesFeature(scenario.feature, url.pathname)
    ) {'''

if old not in source:
    raise SystemExit('createContext payload marker not found')
source = source.replace(old, new, 1)

path.write_text(source)
