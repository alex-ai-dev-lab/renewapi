#!/usr/bin/env python3
from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    file = Path(path)
    source = file.read_text()
    count = source.count(old)
    if count != 1:
        raise SystemExit(
            f'expected exactly one patch target in {path}, found {count}: {old[:180]!r}'
        )
    file.write_text(source.replace(old, new, 1))


def replace_count(path: str, old: str, new: str, expected: int) -> None:
    file = Path(path)
    source = file.read_text()
    count = source.count(old)
    if count != expected:
        raise SystemExit(
            f'expected {expected} patch targets in {path}, found {count}: {old[:180]!r}'
        )
    file.write_text(source.replace(old, new))


ssrf = 'web/default/src/features/system-settings/request-limits/ssrf-section.tsx'
types = 'web/default/src/features/system-settings/types.ts'
harness = '.github/patch-settings-advanced-phase-e.py'

replace_once(
    ssrf,
    "import { useUpdateOption } from '../hooks/use-update-option'",
    "import { useUpdateOptionsBulk } from '../hooks/use-update-option'",
)
replace_count(
    ssrf,
    "  'fetch_setting.allowed_ports': number[]",
    "  'fetch_setting.allowed_ports': string[]",
    2,
)
replace_once(
    ssrf,
    """const parsePorts = (value: string) =>
  value
    .split(',')
    .map((item) => Number.parseInt(item.trim(), 10))
    .filter((port) => Number.isFinite(port))""",
    """const parsePorts = (value: string) =>
  value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)""",
)
replace_once(
    ssrf,
    "  const updateOption = useUpdateOption()",
    "  const updateOptionsBulk = useUpdateOptionsBulk()",
)
replace_once(
    ssrf,
    """    for (const key of updates) {
      const value = normalized[key]
      await updateOption.mutateAsync({
        key,
        value: Array.isArray(value) ? JSON.stringify(value) : value,
      })
    }

    baselineRef.current = normalized""",
    """    const options: Record<string, string | boolean> = {}
    for (const key of updates) {
      const value = normalized[key]
      options[key] = Array.isArray(value) ? JSON.stringify(value) : value
    }

    await updateOptionsBulk.mutateAsync({ options })
    baselineRef.current = normalized""",
)
replace_once(
    ssrf,
    "            isSaving={updateOption.isPending}",
    "            isSaving={updateOptionsBulk.isPending}",
)
replace_once(
    types,
    "  'fetch_setting.allowed_ports': number[]",
    "  'fetch_setting.allowed_ports': string[]",
)
replace_once(
    harness,
    "  'fetch_setting.allowed_ports': '[80,443]',",
    "  'fetch_setting.allowed_ports': '[\"80\",\"443\"]',",
)
replace_once(
    harness,
    """    await page.getByRole('button', { name: 'Save SSRF settings' }).click()
    await waitForOptionValues(""",
    """    const saveMutationStart = optionMutations.length
    await page.getByRole('button', { name: 'Save SSRF settings' }).click()
    await waitForOptionValues(""",
)
replace_once(
    harness,
    """      'SSRF valid API'
    )
    expectDbOptionValues(""",
    """      'SSRF valid API'
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
        '[\"80\",\"443\"]',
      'SSRF allowed_ports must preserve the backend []string contract',
      ssrfRequest
    )
    observations.push({
      type: 'ssrf-atomic-bulk-request',
      mutationCount: ssrfMutations.length,
      allowedPorts: ssrfRequest.options?.['fetch_setting.allowed_ports'],
    })
    expectDbOptionValues(""",
)
replace_once(
    harness,
    "    ssrf: 'normalized sequential mutations passed',",
    "    ssrf: 'normalized atomic bulk mutation passed',",
)
