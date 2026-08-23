#!/usr/bin/env python3
from pathlib import Path

path = Path('/tmp/settings-real-backend-hardened.mjs')
source = path.read_text()


def replace_once(old: str, new: str) -> None:
    global source
    count = source.count(old)
    if count != 1:
        raise SystemExit(f'expected exactly one harness marker, found {count}: {old[:160]!r}')
    source = source.replace(old, new, 1)


helpers = r'''

const advancedBaseline = {
  WorkerUrl: '',
  WorkerValidKey: '',
  WorkerAllowHttpImageRequestEnabled: false,
  ModelRequestRateLimitEnabled: false,
  ModelRequestRateLimitDurationMinutes: 1,
  ModelRequestRateLimitCount: 0,
  ModelRequestRateLimitSuccessCount: 1000,
  ModelRequestRateLimitGroup: '',
  'checkin_setting.enabled': false,
  'checkin_setting.min_quota': 1000,
  'checkin_setting.max_quota': 10000,
  'fetch_setting.enable_ssrf_protection': true,
  'fetch_setting.allow_private_ip': false,
  'fetch_setting.domain_filter_mode': false,
  'fetch_setting.ip_filter_mode': false,
  'fetch_setting.domain_list': '[]',
  'fetch_setting.ip_list': '[]',
  'fetch_setting.allowed_ports': '[]',
  'fetch_setting.apply_ip_filter_for_domain': false,
}

const advancedPersisted = {
  WorkerUrl: 'https://worker.example.test/path',
  WorkerValidKey: 'phase-e-worker-secret',
  WorkerAllowHttpImageRequestEnabled: true,
  ModelRequestRateLimitEnabled: true,
  ModelRequestRateLimitDurationMinutes: 7,
  ModelRequestRateLimitCount: 123,
  ModelRequestRateLimitSuccessCount: 100,
  ModelRequestRateLimitGroup: '{"vip":[300,250]}',
  'checkin_setting.enabled': true,
  'checkin_setting.min_quota': 1000,
  'checkin_setting.max_quota': 5000,
  'fetch_setting.enable_ssrf_protection': true,
  'fetch_setting.allow_private_ip': false,
  'fetch_setting.domain_filter_mode': true,
  'fetch_setting.ip_filter_mode': false,
  'fetch_setting.domain_list': '["example.com","api.example.com"]',
  'fetch_setting.ip_list': '["192.0.2.1","198.51.100.0/24"]',
  'fetch_setting.allowed_ports': '["80","443"]',
  'fetch_setting.apply_ip_filter_for_domain': true,
}

const advancedPersistedApi = Object.fromEntries(
  Object.entries(advancedPersisted).filter(([key]) => key !== 'WorkerValidKey')
)

const advancedBaselineApi = Object.fromEntries(
  Object.entries(advancedBaseline).filter(([key]) => key !== 'WorkerValidKey')
)

async function waitForOptionValues(context, user, expected, label) {
  let latest = null
  for (let attempt = 0; attempt < 80; attempt += 1) {
    latest = await getOptions(context, user)
    const matches = Object.entries(expected).every(
      ([key, value]) => latest[key] === String(value)
    )
    if (matches) {
      observations.push({ type: 'option-wait-passed', label, expected })
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

async function openAdvancedPage(context, route, label) {
  const page = await context.newPage()
  attachPageDiagnostics(page, label)
  const optionMutations = []
  page.on('request', (request) => {
    if (
      request.method() === 'PUT' &&
      request.url().includes('/api/option')
    ) {
      optionMutations.push({
        url: request.url(),
        method: request.method(),
        postData: request.postData(),
      })
    }
  })
  await page.goto(`${baseURL}${route}`, { waitUntil: 'domcontentloaded' })
  assert(
    !page.url().includes('/sign-in'),
    `${label}: advanced Settings route redirected to sign-in`,
    { url: page.url() }
  )
  await page.locator('main').first().waitFor({ state: 'visible', timeout: 20000 })
  await sleep(700)
  return { page, optionMutations }
}

async function setSwitch(locator, expected, label) {
  await locator.waitFor({ state: 'visible', timeout: 15000 })
  const current = await locator.isChecked()
  if (current !== expected) await locator.click()
  await waitForSwitchState(locator, expected, label)
}

async function fillNumber(locator, value) {
  await locator.waitFor({ state: 'visible', timeout: 15000 })
  await locator.fill(String(value))
}

function assertNoNewMutations(optionMutations, before, label) {
  assert(
    optionMutations.length === before,
    `${label}: frontend validation unexpectedly sent an option mutation`,
    { before, after: optionMutations.length, optionMutations }
  )
  observations.push({ type: 'frontend-validation-no-mutation', label })
}

async function runWorkerWave(context, user) {
  const { page, optionMutations } = await openAdvancedPage(
    context,
    '/system-settings/operations/worker',
    'advanced-worker'
  )
  try {
    const workerUrl = page.getByLabel('Worker URL')
    const workerKey = page.getByLabel('Worker Access Key')
    const allowHttp = page.getByRole('switch', {
      name: 'Allow HTTP image requests',
    })
    const save = page.getByRole('button', { name: 'Save Worker settings' })

    await waitForInputValue(workerUrl, '', 'worker baseline URL')
    await setSwitch(allowHttp, false, 'worker baseline HTTP-image switch')

    await workerUrl.fill('worker.example.invalid')
    const invalidBefore = optionMutations.length
    await save.click()
    await page
      .getByText('Provide a valid URL starting with http:// or https://')
      .waitFor({ state: 'visible', timeout: 10000 })
    await sleep(300)
    assertNoNewMutations(optionMutations, invalidBefore, 'Worker invalid URL')
    await waitForOptionValues(
      context,
      user,
      {
        WorkerUrl: advancedBaseline.WorkerUrl,
        WorkerAllowHttpImageRequestEnabled:
          advancedBaseline.WorkerAllowHttpImageRequestEnabled,
      },
      'Worker invalid baseline API'
    )
    expectDbOptionValues(
      {
        WorkerUrl: advancedBaseline.WorkerUrl,
        WorkerValidKey: advancedBaseline.WorkerValidKey,
        WorkerAllowHttpImageRequestEnabled:
          advancedBaseline.WorkerAllowHttpImageRequestEnabled,
      },
      'Worker invalid baseline SQLite'
    )

    await workerUrl.fill('https://worker.example.test/path/')
    await workerKey.fill(advancedPersisted.WorkerValidKey)
    await setSwitch(allowHttp, true, 'worker valid HTTP-image switch')
    await save.click()
    await waitForOptionValues(
      context,
      user,
      {
        WorkerUrl: advancedPersisted.WorkerUrl,
        WorkerAllowHttpImageRequestEnabled:
          advancedPersisted.WorkerAllowHttpImageRequestEnabled,
      },
      'Worker valid API'
    )
    expectDbOptionValues(
      {
        WorkerUrl: advancedPersisted.WorkerUrl,
        WorkerValidKey: advancedPersisted.WorkerValidKey,
        WorkerAllowHttpImageRequestEnabled:
          advancedPersisted.WorkerAllowHttpImageRequestEnabled,
      },
      'Worker valid SQLite'
    )
    await page.screenshot({
      path: path.join(outDir, 'advanced-worker-valid.png'),
      fullPage: true,
    })
  } finally {
    await page.close()
  }
}

async function runRateLimitWave(context, user) {
  const { page, optionMutations } = await openAdvancedPage(
    context,
    '/system-settings/security/rate-limit',
    'advanced-rate-limit'
  )
  try {
    const enabled = page.getByRole('switch', { name: 'Enable rate limiting' })
    const duration = page.getByLabel('Limit period')
    const maxRequests = page.getByLabel('Max requests per period')
    const maxSuccess = page.getByLabel('Max successful requests')
    const save = page.getByRole('button', { name: 'Save rate limits' })

    await page.getByRole('button', { name: 'JSON Mode' }).click()
    const groupJson = page.locator('textarea').first()
    await groupJson.waitFor({ state: 'visible', timeout: 10000 })
    await groupJson.fill('{"vip":[-1,0]}')
    const invalidBefore = optionMutations.length
    await save.click()
    await page
      .getByText('Invalid JSON format or values out of allowed range')
      .waitFor({ state: 'visible', timeout: 10000 })
    await sleep(300)
    assertNoNewMutations(optionMutations, invalidBefore, 'Rate-limit invalid JSON')
    await waitForOptionValues(
      context,
      user,
      {
        ModelRequestRateLimitEnabled:
          advancedBaseline.ModelRequestRateLimitEnabled,
        ModelRequestRateLimitDurationMinutes:
          advancedBaseline.ModelRequestRateLimitDurationMinutes,
        ModelRequestRateLimitCount: advancedBaseline.ModelRequestRateLimitCount,
        ModelRequestRateLimitSuccessCount:
          advancedBaseline.ModelRequestRateLimitSuccessCount,
        ModelRequestRateLimitGroup: advancedBaseline.ModelRequestRateLimitGroup,
      },
      'Rate-limit invalid baseline API'
    )
    expectDbOptionValues(
      {
        ModelRequestRateLimitEnabled:
          advancedBaseline.ModelRequestRateLimitEnabled,
        ModelRequestRateLimitDurationMinutes:
          advancedBaseline.ModelRequestRateLimitDurationMinutes,
        ModelRequestRateLimitCount: advancedBaseline.ModelRequestRateLimitCount,
        ModelRequestRateLimitSuccessCount:
          advancedBaseline.ModelRequestRateLimitSuccessCount,
        ModelRequestRateLimitGroup: advancedBaseline.ModelRequestRateLimitGroup,
      },
      'Rate-limit invalid baseline SQLite'
    )

    await setSwitch(enabled, true, 'rate-limit enabled')
    await fillNumber(duration, 7)
    await fillNumber(maxRequests, 123)
    await fillNumber(maxSuccess, 100)
    await groupJson.fill(advancedPersisted.ModelRequestRateLimitGroup)
    await save.click()
    await waitForOptionValues(
      context,
      user,
      {
        ModelRequestRateLimitEnabled:
          advancedPersisted.ModelRequestRateLimitEnabled,
        ModelRequestRateLimitDurationMinutes:
          advancedPersisted.ModelRequestRateLimitDurationMinutes,
        ModelRequestRateLimitCount: advancedPersisted.ModelRequestRateLimitCount,
        ModelRequestRateLimitSuccessCount:
          advancedPersisted.ModelRequestRateLimitSuccessCount,
        ModelRequestRateLimitGroup: advancedPersisted.ModelRequestRateLimitGroup,
      },
      'Rate-limit valid API'
    )
    expectDbOptionValues(
      {
        ModelRequestRateLimitEnabled:
          advancedPersisted.ModelRequestRateLimitEnabled,
        ModelRequestRateLimitDurationMinutes:
          advancedPersisted.ModelRequestRateLimitDurationMinutes,
        ModelRequestRateLimitCount: advancedPersisted.ModelRequestRateLimitCount,
        ModelRequestRateLimitSuccessCount:
          advancedPersisted.ModelRequestRateLimitSuccessCount,
        ModelRequestRateLimitGroup: advancedPersisted.ModelRequestRateLimitGroup,
      },
      'Rate-limit valid SQLite'
    )
    await page.screenshot({
      path: path.join(outDir, 'advanced-rate-limit-valid.png'),
      fullPage: true,
    })
  } finally {
    await page.close()
  }
}

async function runCheckinWave(context, user) {
  const { page, optionMutations } = await openAdvancedPage(
    context,
    '/system-settings/billing/checkin',
    'advanced-checkin'
  )
  try {
    const enabled = page.getByRole('switch', { name: 'Enable check-in feature' })
    const save = page.getByRole('button', { name: 'Save check-in settings' })
    await setSwitch(enabled, true, 'check-in local enabled for validation')
    const minQuota = page.getByLabel('Minimum check-in quota')
    const maxQuota = page.getByLabel('Maximum check-in quota')
    await fillNumber(minQuota, 5000)
    await fillNumber(maxQuota, 1000)
    const invalidBefore = optionMutations.length
    await save.click()
    await page
      .getByText(
        'Maximum check-in quota must be greater than or equal to the minimum'
      )
      .waitFor({ state: 'visible', timeout: 10000 })
    await sleep(300)
    assertNoNewMutations(optionMutations, invalidBefore, 'Check-in inverted range')
    await waitForOptionValues(
      context,
      user,
      {
        'checkin_setting.enabled': advancedBaseline['checkin_setting.enabled'],
        'checkin_setting.min_quota':
          advancedBaseline['checkin_setting.min_quota'],
        'checkin_setting.max_quota':
          advancedBaseline['checkin_setting.max_quota'],
      },
      'Check-in invalid baseline API'
    )
    expectDbOptionValues(
      {
        'checkin_setting.enabled': advancedBaseline['checkin_setting.enabled'],
        'checkin_setting.min_quota':
          advancedBaseline['checkin_setting.min_quota'],
        'checkin_setting.max_quota':
          advancedBaseline['checkin_setting.max_quota'],
      },
      'Check-in invalid baseline SQLite'
    )

    await fillNumber(minQuota, advancedPersisted['checkin_setting.min_quota'])
    await fillNumber(maxQuota, advancedPersisted['checkin_setting.max_quota'])
    await save.click()
    await waitForOptionValues(
      context,
      user,
      {
        'checkin_setting.enabled': advancedPersisted['checkin_setting.enabled'],
        'checkin_setting.min_quota':
          advancedPersisted['checkin_setting.min_quota'],
        'checkin_setting.max_quota':
          advancedPersisted['checkin_setting.max_quota'],
      },
      'Check-in valid API'
    )
    expectDbOptionValues(
      {
        'checkin_setting.enabled': advancedPersisted['checkin_setting.enabled'],
        'checkin_setting.min_quota':
          advancedPersisted['checkin_setting.min_quota'],
        'checkin_setting.max_quota':
          advancedPersisted['checkin_setting.max_quota'],
      },
      'Check-in valid SQLite'
    )
    await page.screenshot({
      path: path.join(outDir, 'advanced-checkin-valid.png'),
      fullPage: true,
    })
  } finally {
    await page.close()
  }
}

async function runSsrfWave(context, user) {
  const { page } = await openAdvancedPage(
    context,
    '/system-settings/security/ssrf',
    'advanced-ssrf'
  )
  try {
    const domainMode = page.getByLabel('Domain Filter Mode')
    await domainMode.click()
    await page
      .getByRole('option', { name: 'Whitelist (Only allow listed domains)' })
      .click()

    await page.getByLabel('Domain Whitelist').fill('example.com\napi.example.com')
    await page
      .getByLabel('IP Blacklist')
      .fill('192.0.2.1\n198.51.100.0/24')
    await page.getByLabel('Allowed Ports').fill('80,443')
    await setSwitch(
      page.getByRole('switch', { name: 'Apply IP Filter to Resolved Domains' }),
      true,
      'SSRF apply-IP-filter'
    )

    const saveMutationStart = optionMutations.length
    await page.getByRole('button', { name: 'Save SSRF settings' }).click()
    await waitForOptionValues(
      context,
      user,
      {
        'fetch_setting.domain_filter_mode':
          advancedPersisted['fetch_setting.domain_filter_mode'],
        'fetch_setting.domain_list':
          advancedPersisted['fetch_setting.domain_list'],
        'fetch_setting.ip_list': advancedPersisted['fetch_setting.ip_list'],
        'fetch_setting.allowed_ports':
          advancedPersisted['fetch_setting.allowed_ports'],
        'fetch_setting.apply_ip_filter_for_domain':
          advancedPersisted['fetch_setting.apply_ip_filter_for_domain'],
      },
      'SSRF valid API'
    )
    const ssrfMutations = optionMutations.slice(saveMutationStart)
    assert(
      ssrfMutations.length === 1 &&
        ssrfMutations[0].url.includes('/api/option/bulk'),
      'SSRF save must use exactly one atomic bulk mutation',
      ssrfMutations
    )
    const ssrfRequest = JSON.parse(ssrfMutations[0].postData || '{}')
    assert(
      ssrfRequest.options?.['fetch_setting.allowed_ports'] ===
        '["80","443"]',
      'SSRF allowed_ports must preserve the backend []string contract',
      ssrfRequest
    )
    observations.push({
      type: 'ssrf-atomic-bulk-request',
      mutationCount: ssrfMutations.length,
      allowedPorts: ssrfRequest.options?.['fetch_setting.allowed_ports'],
    })
    expectDbOptionValues(
      {
        'fetch_setting.domain_filter_mode':
          advancedPersisted['fetch_setting.domain_filter_mode'],
        'fetch_setting.domain_list':
          advancedPersisted['fetch_setting.domain_list'],
        'fetch_setting.ip_list': advancedPersisted['fetch_setting.ip_list'],
        'fetch_setting.allowed_ports':
          advancedPersisted['fetch_setting.allowed_ports'],
        'fetch_setting.apply_ip_filter_for_domain':
          advancedPersisted['fetch_setting.apply_ip_filter_for_domain'],
      },
      'SSRF valid SQLite'
    )
    await page.screenshot({
      path: path.join(outDir, 'advanced-ssrf-valid.png'),
      fullPage: true,
    })
  } finally {
    await page.close()
  }
}

async function runAdvancedSettingsWave1(context, user) {
  await runWorkerWave(context, user)
  await runRateLimitWave(context, user)
  await runCheckinWave(context, user)
  await runSsrfWave(context, user)
  await waitForOptionValues(
    context,
    user,
    advancedPersistedApi,
    'Advanced Wave 1 aggregate API'
  )
  expectDbOptionValues(advancedPersisted, 'Advanced Wave 1 aggregate SQLite')
  return {
    worker: 'invalid-local-validation + valid mutation passed',
    rateLimit: 'invalid-local-validation + valid bulk mutation passed',
    checkin: 'cross-field validation + valid bulk mutation passed',
    ssrf: 'normalized atomic bulk mutation passed',
    apiSqliteSynchronized: true,
  }
}

async function verifyAdvancedSettingsRestart(context, user) {
  await waitForOptionValues(
    context,
    user,
    advancedPersistedApi,
    'Advanced Wave 1 restart API'
  )
  expectDbOptionValues(advancedPersisted, 'Advanced Wave 1 restart SQLite')

  {
    const { page } = await openAdvancedPage(
      context,
      '/system-settings/operations/worker',
      'advanced-worker-restart'
    )
    try {
      await waitForInputValue(
        page.getByLabel('Worker URL'),
        advancedPersisted.WorkerUrl,
        'Worker restart URL'
      )
      await waitForSwitchState(
        page.getByRole('switch', { name: 'Allow HTTP image requests' }),
        true,
        'Worker restart HTTP-image switch'
      )
      await page.screenshot({
        path: path.join(outDir, 'advanced-worker-restart.png'),
        fullPage: true,
      })
    } finally {
      await page.close()
    }
  }

  {
    const { page } = await openAdvancedPage(
      context,
      '/system-settings/security/rate-limit',
      'advanced-rate-limit-restart'
    )
    try {
      await waitForSwitchState(
        page.getByRole('switch', { name: 'Enable rate limiting' }),
        true,
        'Rate-limit restart enabled'
      )
      await waitForInputValue(
        page.getByLabel('Limit period'),
        advancedPersisted.ModelRequestRateLimitDurationMinutes,
        'Rate-limit restart duration'
      )
      await waitForInputValue(
        page.getByLabel('Max requests per period'),
        advancedPersisted.ModelRequestRateLimitCount,
        'Rate-limit restart max requests'
      )
      await waitForInputValue(
        page.getByLabel('Max successful requests'),
        advancedPersisted.ModelRequestRateLimitSuccessCount,
        'Rate-limit restart success count'
      )
      await page.getByRole('button', { name: 'JSON Mode' }).click()
      await waitForInputValue(
        page.locator('textarea').first(),
        advancedPersisted.ModelRequestRateLimitGroup,
        'Rate-limit restart group JSON'
      )
      await page.screenshot({
        path: path.join(outDir, 'advanced-rate-limit-restart.png'),
        fullPage: true,
      })
    } finally {
      await page.close()
    }
  }

  {
    const { page } = await openAdvancedPage(
      context,
      '/system-settings/billing/checkin',
      'advanced-checkin-restart'
    )
    try {
      await waitForSwitchState(
        page.getByRole('switch', { name: 'Enable check-in feature' }),
        true,
        'Check-in restart enabled'
      )
      await waitForInputValue(
        page.getByLabel('Minimum check-in quota'),
        advancedPersisted['checkin_setting.min_quota'],
        'Check-in restart min'
      )
      await waitForInputValue(
        page.getByLabel('Maximum check-in quota'),
        advancedPersisted['checkin_setting.max_quota'],
        'Check-in restart max'
      )
      await page.screenshot({
        path: path.join(outDir, 'advanced-checkin-restart.png'),
        fullPage: true,
      })
    } finally {
      await page.close()
    }
  }

  {
    const { page } = await openAdvancedPage(
      context,
      '/system-settings/security/ssrf',
      'advanced-ssrf-restart'
    )
    try {
      await page.getByText('Whitelist (Only allow listed domains)').first().waitFor({
        state: 'visible',
        timeout: 15000,
      })
      await waitForInputValue(
        page.getByLabel('Domain Whitelist'),
        'example.com\napi.example.com',
        'SSRF restart domains'
      )
      await waitForInputValue(
        page.getByLabel('IP Blacklist'),
        '192.0.2.1\n198.51.100.0/24',
        'SSRF restart IPs'
      )
      await waitForInputValue(
        page.getByLabel('Allowed Ports'),
        '80,443',
        'SSRF restart ports'
      )
      await waitForSwitchState(
        page.getByRole('switch', { name: 'Apply IP Filter to Resolved Domains' }),
        true,
        'SSRF restart apply-IP-filter'
      )
      await page.screenshot({
        path: path.join(outDir, 'advanced-ssrf-restart.png'),
        fullPage: true,
      })
    } finally {
      await page.close()
    }
  }

  observations.push({ type: 'advanced-wave1-restart-ui', passed: true })
}
'''

replace_once('async function main() {', helpers + '\nasync function main() {')
replace_once(
    '  let activeContext = null\n',
    '  let activeContext = null\n  let advancedResult = null\n',
)
replace_once(
    "    expectDbOptionValues(baseline, 'seed baseline SQLite')\n",
    "    expectDbOptionValues(baseline, 'seed baseline SQLite')\n\n"
    "    const advancedSeed = await putBulk(activeContext, user, advancedBaseline)\n"
    "    assert(\n"
    "      advancedSeed.response.ok() && advancedSeed.body.success === true,\n"
    "      'Failed to seed deterministic Advanced Settings baseline',\n"
    "      { status: advancedSeed.response.status(), body: advancedSeed.body }\n"
    "    )\n"
    "    await waitForOptionValues(\n"
    "      activeContext,\n"
    "      user,\n"
    "      advancedBaselineApi,\n"
    "      'Advanced Wave 1 seed API'\n"
    "    )\n"
    "    expectDbOptionValues(advancedBaseline, 'Advanced Wave 1 seed SQLite')\n",
)
replace_once(
    "    await activeContext.close()\n    activeContext = null\n\n    assert(consoleErrors.length === 0, 'Browser console errors detected', consoleErrors)",
    "    advancedResult = await runAdvancedSettingsWave1(\n"
    "      activeContext,\n"
    "      restartedUser\n"
    "    )\n"
    "\n"
    "    await activeContext.close()\n"
    "    activeContext = null\n"
    "    await stopBackend(server, 'after-advanced-wave1')\n"
    "    server = null\n"
    "\n"
    "    server = await startBackend('after-advanced-wave1-restart')\n"
    "    activeContext = await createBrowserContext(browser)\n"
    "    const advancedRestartUser = await setupAndLogin(activeContext, {\n"
    "      allowSetup: false,\n"
    "    })\n"
    "    await verifyAdvancedSettingsRestart(activeContext, advancedRestartUser)\n"
    "\n"
    "    await activeContext.close()\n"
    "    activeContext = null\n"
    "\n"
    "    assert(consoleErrors.length === 0, 'Browser console errors detected', consoleErrors)",
)
replace_once(
    '      persistedAfterFinalRestart: true,\n',
    '      persistedAfterFinalRestart: true,\n      advancedSettingsPhaseEWave1: advancedResult,\n',
)
replace_once(
    "        'validBulkRestart=passed',\n",
    "        'validBulkRestart=passed',\n"
    "        `advancedWave1=${JSON.stringify(advancedResult)}`,\n"
    "        'advancedWave1Restart=UI+API+SQLite passed',\n",
)

path.write_text(source)
