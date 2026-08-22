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

export function KPICard({
  title,
  value,
  subtitle,
  icon: Icon,
  trend,
  className,
}: KPICardProps) {
  return (
    <Card className={cn('transition-all', className)}>
      <CardContent className='p-6'>
        <div className='flex items-start justify-between'>
          <div className='flex-1 space-y-2'>
            <p className='text-muted-foreground text-[11px] font-bold tracking-[1.4px] uppercase'>
              {title}
            </p>
            <p className='text-3xl font-extrabold tracking-[-0.03em] tabular-nums'>
              {value}
            </p>
            {subtitle && (
              <p className='text-muted-foreground text-xs'>{subtitle}</p>
            )}
          </div>
          {Icon && (
            <div className='flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-(--aurora-from) to-(--aurora-to) shadow-[0_6px_18px_var(--aurora-glow)]'>
              <Icon className='h-6 w-6 text-white' />
            </div>
          )}
        </div>
        {trend && (
          <div className='mt-4 flex items-center gap-1 text-sm'>
            <span
              className={cn(
                'font-medium',
                trend.isPositive ? 'text-success' : 'text-destructive'
              )}
            >
              {trend.isPositive ? '↑' : '↓'} {Math.abs(trend.value).toFixed(1)}%
            </span>
            <span className='text-muted-foreground'>vs last period</span>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
