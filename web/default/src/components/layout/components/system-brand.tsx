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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'

type SystemBrandProps = {
  defaultName?: string
  defaultVersion?: string
  variant?: 'sidebar' | 'inline'
}

/** Brand and version are supplied by the existing system configuration. */
export function SystemBrand(props: SystemBrandProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const { logo, systemName } = useSystemConfig()
  const inline = props.variant === 'inline'
  const name = systemName || props.defaultName
  const version = status?.version || props.defaultVersion

  return (
    <Link
      to='/'
      aria-label={t('Go to home')}
      className={cn(
        'text-foreground flex min-w-0 items-center gap-2 rounded-sm outline-none',
        'focus-visible:outline-ring focus-visible:outline-2 focus-visible:outline-offset-2',
        inline ? 'h-8 px-1' : 'h-12'
      )}
    >
      <img
        src={logo}
        alt=''
        className='size-6 shrink-0 rounded-sm object-contain'
      />
      <span className='grid min-w-0 leading-tight group-data-[collapsible=icon]:hidden'>
        <span className='truncate text-[13px] font-semibold'>{name}</span>
        {!inline && version && (
          <span className='text-muted-foreground truncate font-mono text-[11px]'>
            {version}
          </span>
        )}
      </span>
    </Link>
  )
}
