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
import { Card, CardContent } from '@/components/ui/card'

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
    <Card
      className={cn(
        'border-border/60 bg-card/70 min-h-[120px] overflow-hidden',
        props.className
      )}
    >
      <CardContent className='h-full p-5'>
        <p className='text-muted-foreground text-[11px] font-bold tracking-[1.2px] uppercase'>
          {props.title}
        </p>
        <p className='mt-2 text-[34px] leading-none font-extrabold tracking-[-0.03em] tabular-nums'>
          {props.value}
        </p>
        <div className='mt-2 flex min-w-0 items-center gap-2'>
          {props.trend && (
            <span
              className={cn(
                'shrink-0 text-[11px] font-semibold',
                props.trend.isPositive ? 'text-success' : 'text-destructive'
              )}
            >
              {props.trend.isPositive ? '▲' : '▼'}{' '}
              {Math.abs(props.trend.value).toFixed(1)}%
            </span>
          )}
          {props.subtitle && (
            <p className='truncate text-[11px] font-semibold text-[#3E8E5A]'>
              {props.subtitle}
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
