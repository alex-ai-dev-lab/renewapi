/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useSearch } from '@/context/search-provider'
import { Button } from '@/components/ui/button'

export function CommandPaletteTrigger() {
  const { t } = useTranslation()
  const { setOpen } = useSearch()
  const isMac =
    typeof navigator !== 'undefined' &&
    /Mac|iPhone|iPad/.test(navigator.userAgent)

  return (
    <Button
      variant='outline'
      aria-label={t('Open command menu')}
      aria-haspopup='dialog'
      className='obsidian-command-trigger text-muted-foreground h-8 w-8 shrink-0 gap-2 p-0 sm:w-36 sm:justify-start sm:px-2 lg:w-52'
      onClick={() => setOpen(true)}
    >
      <Search className='size-4 shrink-0' aria-hidden='true' />
      <span className='hidden flex-1 text-left text-xs sm:block'>
        {t('Search...')}
      </span>
      <kbd
        aria-hidden='true'
        className='bg-muted hidden shrink-0 rounded-sm border px-1 font-mono text-[10px] sm:block'
      >
        {isMac ? '⌘ K' : 'Ctrl K'}
      </kbd>
    </Button>
  )
}
