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
import { useId } from 'react'
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  DEFAULT_REFRESH_INTERVAL_MS,
  type RefreshIntervalMs,
} from './stats-api'

const REFRESH_INTERVAL_OPTIONS: Array<{
  label: string
  value: RefreshIntervalMs
}> = [
  { label: '5s', value: 5000 },
  { label: '15s', value: 15000 },
  { label: '30s', value: 30000 },
  { label: '60s', value: 60000 },
]

interface AutoRefreshToggleProps {
  value: boolean
  onChange: (value: boolean) => void
  intervalMs?: RefreshIntervalMs
  onIntervalChange?: (value: RefreshIntervalMs) => void
  onRefresh?: () => void
  isRefreshing?: boolean
  lastUpdatedAt?: number
  className?: string
}

function formatLastUpdated(timestamp?: number): string | null {
  if (!timestamp) return null
  const updatedAt = new Date(timestamp)
  if (Number.isNaN(updatedAt.getTime())) return null
  return `已更新 ${updatedAt.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })}`
}

export function AutoRefreshToggle({
  value,
  onChange,
  intervalMs = DEFAULT_REFRESH_INTERVAL_MS,
  onIntervalChange,
  onRefresh,
  isRefreshing = false,
  lastUpdatedAt,
  className,
}: AutoRefreshToggleProps) {
  const id = useId()
  const { t } = useTranslation()
  const lastUpdatedLabel = formatLastUpdated(lastUpdatedAt)

  return (
    <div className={cn('flex flex-wrap items-center gap-2', className)}>
      {onRefresh ? (
        <Button
          type='button'
          variant='outline'
          size='icon-sm'
          onClick={() => onRefresh()}
          disabled={isRefreshing}
          aria-label='刷新统计'
          title='刷新统计'
        >
          <RefreshCw
            className={cn('size-4', isRefreshing && 'animate-spin')}
            aria-hidden='true'
          />
        </Button>
      ) : (
        <RefreshCw
          className={cn(
            'text-muted-foreground h-4 w-4',
            isRefreshing && 'animate-spin'
          )}
          aria-hidden='true'
        />
      )}
      <Switch id={id} checked={value} onCheckedChange={onChange} />
      <Label
        htmlFor={id}
        className='text-muted-foreground cursor-pointer text-sm'
      >
        自动刷新
      </Label>
      {onIntervalChange ? (
        <Select
          value={String(intervalMs)}
          onValueChange={(nextValue) =>
            onIntervalChange(Number(nextValue) as RefreshIntervalMs)
          }
          disabled={!value}
        >
          <SelectTrigger
            className='h-8 w-20'
            aria-label={t('Refresh interval')}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {REFRESH_INTERVAL_OPTIONS.map((option) => (
              <SelectItem key={option.value} value={String(option.value)}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : (
        <span className='text-muted-foreground text-sm'>
          {REFRESH_INTERVAL_OPTIONS.find(
            (option) => option.value === intervalMs
          )?.label ?? '5s'}
        </span>
      )}
      {lastUpdatedLabel ? (
        <span className='text-muted-foreground text-xs tabular-nums'>
          {lastUpdatedLabel}
        </span>
      ) : null}
    </div>
  )
}
