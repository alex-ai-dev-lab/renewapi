#!/usr/bin/env python3
from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    file = Path(path)
    source = file.read_text()
    if old not in source:
        raise SystemExit(f'expected patch target not found: {path}: {old[:120]!r}')
    source = source.replace(old, new, 1)
    file.write_text(source)


replace_once(
    'web/default/src/components/data-table/pagination.tsx',
    "<SelectTrigger className='h-7 w-[60px] sm:w-[66px]'>",
    "<SelectTrigger\n              className='h-7 w-[60px] sm:w-[66px]'\n              aria-label={t('每页行数')}\n            >",
)

replace_once(
    'web/default/src/features/dashboard/auto-refresh-toggle.tsx',
    "import { Switch } from '@/components/ui/switch'",
    "import { useTranslation } from 'react-i18next'\nimport { Switch } from '@/components/ui/switch'",
)
replace_once(
    'web/default/src/features/dashboard/auto-refresh-toggle.tsx',
    "  const id = useId()\n  const lastUpdatedLabel = formatLastUpdated(lastUpdatedAt)",
    "  const id = useId()\n  const { t } = useTranslation()\n  const lastUpdatedLabel = formatLastUpdated(lastUpdatedAt)",
)
replace_once(
    'web/default/src/features/dashboard/auto-refresh-toggle.tsx',
    "<SelectTrigger className='h-8 w-20'>",
    "<SelectTrigger className='h-8 w-20' aria-label={t('Refresh interval')}>",
)

replace_once(
    'web/default/src/features/usage-logs/components/common-logs-filter-bar.tsx',
    '        <SelectTrigger>\n          <SelectValue>{logTypeLabel}</SelectValue>',
    "        <SelectTrigger aria-label={t('Log Type')}>\n          <SelectValue>{logTypeLabel}</SelectValue>",
)

replace_once(
    'web/default/src/features/models/components/drawers/model-mutate-drawer.tsx',
    '                        <SelectTrigger>\n                          <SelectValue placeholder={t(\'Select vendor\')} />',
    "                        <SelectTrigger aria-label={t('Vendor')}>\n                          <SelectValue placeholder={t('Select vendor')} />",
)
replace_once(
    'web/default/src/features/models/components/drawers/model-mutate-drawer.tsx',
    "                  <SelectTrigger size='sm' className='w-[200px]'>",
    "                  <SelectTrigger\n                    size='sm'\n                    className='w-[200px]'\n                    aria-label={t('Load template...')}\n                  >",
)

replace_once(
    'web/default/src/features/models/components/data-table-row-actions.tsx',
    "            className='data-popup-open:bg-muted flex h-8 w-8 p-0'\n          />",
    "            className='data-popup-open:bg-muted flex h-8 w-8 p-0'\n            aria-label={t('Open menu')}\n          />",
)

for path in [
    'web/default/src/features/keys/components/api-keys-columns.tsx',
    'web/default/src/features/users/components/users-columns.tsx',
]:
    replace_once(
        path,
        "              <Progress\n                value={percentage}\n                className={cn('h-1.5', getQuotaProgressColor(percentage))}\n              />",
        "              <Progress\n                value={percentage}\n                aria-label={t('Remaining quota')}\n                className={cn('h-1.5', getQuotaProgressColor(percentage))}\n              />",
    )

replace_once(
    'web/default/src/features/models/components/models-stats.tsx',
    "          aria-label={registryHeadline}\n          aria-busy='true'\n        >\n          {Array.from({ length: 3 }).map((_, index) => (",
    "          aria-busy='true'\n        >\n          <span className='sr-only'>{registryHeadline}</span>\n          {Array.from({ length: 3 }).map((_, index) => (",
)

replace_once(
    'web/default/src/styles/theme.css',
    '  --destructive: oklch(0.577 0.245 27.325);',
    '  --destructive: oklch(0.48 0.245 27.325);',
)
replace_once(
    'web/default/src/styles/theme.css',
    '  --success: oklch(0.628 0.148 165.5);',
    '  --success: oklch(0.46 0.148 165.5);',
)
replace_once(
    'web/default/src/styles/theme.css',
    '  --warning: oklch(0.681 0.162 75.834);',
    '  --warning: oklch(0.48 0.162 75.834);',
)
replace_once(
    'web/default/src/styles/theme.css',
    '  --info: oklch(0.62 0.21 254);',
    '  --info: oklch(0.48 0.21 254);',
)

replace_once(
    'web/default/src/features/dashboard/aurora-overview-panels.tsx',
    'text-[#3E8E5A]',
    'text-[#2F7748]',
)
