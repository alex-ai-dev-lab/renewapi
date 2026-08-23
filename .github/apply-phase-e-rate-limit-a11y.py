#!/usr/bin/env python3
from pathlib import Path

path = Path('web/default/src/features/system-settings/request-limits/rate-limit-section.tsx')
source = path.read_text()


def replace_once(old: str, new: str) -> None:
    global source
    count = source.count(old)
    if count != 1:
        raise SystemExit(
            f'expected exactly one Rate Limit accessibility target, found {count}: {old[:160]!r}'
        )
    source = source.replace(old, new, 1)


replace_once(
    """<FormLabel>{t('Limit period')}</FormLabel>\n                  <FormControl>\n                    <div className='flex items-center gap-2'>\n                      <Input\n                        type='number'\n                        min={1}\n                        step={1}\n                        {...field}""",
    """<FormLabel>{t('Limit period')}</FormLabel>\n                  <FormControl>\n                    <div className='flex items-center gap-2'>\n                      <Input\n                        type='number'\n                        min={1}\n                        step={1}\n                        aria-label={t('Limit period')}\n                        {...field}""",
)

replace_once(
    """<FormLabel>{t('Max requests per period')}</FormLabel>\n                  <FormControl>\n                    <div className='flex items-center gap-2'>\n                      <Input\n                        type='number'\n                        min={0}\n                        max={100000000}\n                        step={1}\n                        {...field}""",
    """<FormLabel>{t('Max requests per period')}</FormLabel>\n                  <FormControl>\n                    <div className='flex items-center gap-2'>\n                      <Input\n                        type='number'\n                        min={0}\n                        max={100000000}\n                        step={1}\n                        aria-label={t('Max requests per period')}\n                        {...field}""",
)

replace_once(
    """<FormLabel>{t('Max successful requests')}</FormLabel>\n                  <FormControl>\n                    <div className='flex items-center gap-2'>\n                      <Input\n                        type='number'\n                        min={0}\n                        max={100000000}\n                        step={1}\n                        {...field}""",
    """<FormLabel>{t('Max successful requests')}</FormLabel>\n                  <FormControl>\n                    <div className='flex items-center gap-2'>\n                      <Input\n                        type='number'\n                        min={0}\n                        max={100000000}\n                        step={1}\n                        aria-label={t('Max successful requests')}\n                        {...field}""",
)

path.write_text(source)
