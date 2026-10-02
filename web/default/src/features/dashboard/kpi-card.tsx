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
import type { LucideIcon } from 'lucide-react'
import { cn } from '@/lib/utils'

interface KPICardProps {
  title: string
  value: string | number
  subtitle?: string
  icon?: LucideIcon
  trend?: {
    value: number
    isPositive: boolean
  }
  className?: string
}

export function KPICard(props: KPICardProps) {
  return (
    <div className={cn('obsidian-kpi', props.className)}>
      <p className='obsidian-kpi-label'>
        {props.icon && <props.icon className='size-3.5' aria-hidden='true' />}
        {props.title}
      </p>
      <p className='obsidian-kpi-value'>{props.value}</p>
      <div className='obsidian-kpi-detail'>
        {props.trend && (
          <span
            className={cn(
              'shrink-0',
              props.trend.isPositive ? 'text-success' : 'text-destructive'
            )}
          >
            {props.trend.isPositive ? '▲' : '▼'}{' '}
            {Math.abs(props.trend.value).toFixed(1)}%
          </span>
        )}
        {props.subtitle && <p>{props.subtitle}</p>}
      </div>
    </div>
  )
}
