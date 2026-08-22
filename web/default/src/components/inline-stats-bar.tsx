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
import { cn } from '@/lib/utils'

type StatItemTone = 'default' | 'success' | 'destructive' | 'accent'

interface StatItem {
  label: string
  value: string | number
  tone?: StatItemTone
}

const toneClasses: Record<StatItemTone, string> = {
  default: 'text-foreground',
  success: 'text-success',
  destructive: 'text-destructive',
  accent: 'text-primary',
}

export function InlineStatsBar({
  items,
  className,
}: {
  items: StatItem[]
  className?: string
}) {
  return (
    <div
      className={cn(
        'glass-tile flex flex-wrap items-center gap-x-6 gap-y-2 px-5 py-3 text-xs',
        className
      )}
    >
      {items.map((item, index) => (
        <div key={index} className='flex items-center gap-1.5'>
          <span className='text-muted-foreground text-[10px] font-bold uppercase tracking-[1.2px]'>
            {item.label}
          </span>
          <span
            className={cn(
              'text-base font-extrabold tracking-[-0.02em] tabular-nums',
              toneClasses[item.tone ?? 'default']
            )}
          >
            {item.value}
          </span>
          {index < items.length - 1 && (
            <span className='text-muted-foreground/40'>·</span>
          )}
        </div>
      ))}
    </div>
  )
}
