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
import type { ComponentProps, ReactNode } from 'react'
import { ArrowDownRight, ArrowUpRight, Database, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

type PageContainerProps = ComponentProps<'div'> & {
  width?: 'reading' | 'fluid'
}

export function PageContainer({
  className,
  width = 'reading',
  ...props
}: PageContainerProps) {
  return (
    <div
      data-slot='page-container'
      className={cn(
        'flex w-full flex-col gap-5 px-4 py-5 sm:gap-6 sm:px-6 sm:py-7 lg:px-8',
        width === 'reading' ? 'mx-auto max-w-[1180px]' : 'max-w-none xl:px-8',
        className
      )}
      {...props}
    />
  )
}

type PageHeaderProps = Omit<ComponentProps<'header'>, 'title'> & {
  title?: ReactNode
  description?: ReactNode
  actions?: ReactNode
  size?: 'default' | 'compact'
}

export function PageHeader({
  title,
  description,
  actions,
  children,
  className,
  size = 'default',
  ...props
}: PageHeaderProps) {
  return (
    <header
      data-slot='page-header'
      className={cn(
        'flex flex-col gap-1.5 pb-2 sm:flex-row sm:items-end sm:justify-between',
        className
      )}
      {...props}
    >
      <div className='min-w-0 flex-1 space-y-0.5'>
        {title != null && (
          <h1
            className={cn(
              'leading-[1.06] font-extrabold tracking-[-0.02em]',
              size === 'compact'
                ? 'text-[15px] sm:text-base'
                : 'text-2xl sm:text-3xl'
            )}
          >
            {title}
          </h1>
        )}
        {description != null && (
          <p
            className={cn(
              'text-muted-foreground max-w-2xl',
              size === 'compact' ? 'text-xs leading-4' : 'text-sm leading-5'
            )}
          >
            {description}
          </p>
        )}
        {children}
      </div>
      {actions != null && (
        <div className='flex shrink-0 flex-wrap items-center gap-2'>
          {actions}
        </div>
      )}
    </header>
  )
}

type SectionCardProps = Omit<ComponentProps<typeof Card>, 'title'> & {
  title?: ReactNode
  description?: ReactNode
  action?: ReactNode
  contentClassName?: string
}

export function SectionCard({
  title,
  description,
  action,
  children,
  className,
  contentClassName,
  ...props
}: SectionCardProps) {
  const hasHeader = title != null || description != null || action != null

  return (
    <Card
      data-slot='section-card'
      className={cn(
        'border-border/60 min-w-0 gap-0 rounded-[calc(var(--radius)*1.375)] py-0 shadow-none',
        className
      )}
      {...props}
    >
      {hasHeader && (
        <CardHeader className='border-border border-b p-4 sm:p-5'>
          <div className='flex flex-col gap-2.5 sm:flex-row sm:items-start sm:justify-between'>
            <div className='min-w-0 space-y-0.5'>
              {title != null && (
                <CardTitle className='text-sm font-semibold tracking-tight'>
                  {title}
                </CardTitle>
              )}
              {description != null && (
                <CardDescription className='truncate text-xs leading-4'>
                  {description}
                </CardDescription>
              )}
            </div>
            {action != null && (
              <div className='flex shrink-0 items-center gap-2'>{action}</div>
            )}
          </div>
        </CardHeader>
      )}
      <CardContent className={cn('p-5 sm:p-6', contentClassName)}>
        {children}
      </CardContent>
    </Card>
  )
}

type PrimitiveStatCardProps = ComponentProps<'div'> & {
  label: ReactNode
  value: ReactNode
  description?: ReactNode
  trend?: number | string | null
  trendLabel?: ReactNode
  tone?: 'default' | 'accent' | 'success' | 'warning' | 'destructive'
}

const statDotClass: Record<
  NonNullable<PrimitiveStatCardProps['tone']>,
  string
> = {
  default: 'bg-muted-foreground',
  accent: 'bg-chart-1',
  success: 'bg-success',
  warning: 'bg-warning',
  destructive: 'bg-destructive',
}

export function StatCard({
  label,
  value,
  description,
  trend,
  trendLabel,
  tone = 'default',
  className,
  ...props
}: PrimitiveStatCardProps) {
  const numericTrend =
    typeof trend === 'number'
      ? trend
      : typeof trend === 'string'
        ? Number.parseFloat(trend)
        : null
  const hasTrend = trend != null && `${trend}` !== ''
  const isDown = numericTrend != null && numericTrend < 0
  const TrendIcon = isDown ? ArrowDownRight : ArrowUpRight

  return (
    <SectionCard className={cn('p-3.5', className)} {...props}>
      <div className='flex h-full flex-col justify-between gap-2.5'>
        <div className='flex items-center justify-between gap-2'>
          <div className='flex min-w-0 items-center gap-2'>
            <span
              className={cn('size-2 rounded-full', statDotClass[tone])}
              aria-hidden='true'
            />
            <div className='text-muted-foreground truncate text-xs font-medium'>
              {label}
            </div>
          </div>
          {hasTrend && (
            <Badge
              variant={isDown ? 'destructive' : 'outline'}
              className={cn(
                'rounded-lg text-xs',
                !isDown && 'border-success/25 text-success'
              )}
            >
              <TrendIcon className='size-3' />
              {trendLabel ?? trend}
            </Badge>
          )}
        </div>
        <div>
          <div className='text-[26px] leading-none font-extrabold tracking-[-0.03em] tabular-nums'>
            {value}
          </div>
          {description != null && (
            <p className='text-muted-foreground mt-0.5 truncate text-xs'>
              {description}
            </p>
          )}
        </div>
      </div>
    </SectionCard>
  )
}

export function DataTable({
  className,
  ...props
}: ComponentProps<typeof Table>) {
  return (
    <div
      data-slot='primitive-data-table'
      className='border-border/60 bg-card overflow-x-auto rounded-[calc(var(--radius)*1.375)] border'
    >
      <Table
        className={cn('[&_th]:text-muted-foreground', className)}
        {...props}
      />
    </div>
  )
}

export {
  TableBody as DataTableBody,
  TableCell as DataTableCell,
  TableHead as DataTableHead,
  TableHeader as DataTableHeader,
  TableRow as DataTableRow,
}

type PillOption = {
  value: string
  label: ReactNode
  disabled?: boolean
}

type FilterPillsProps = ComponentProps<'div'> & {
  value?: string
  options: PillOption[]
  onValueChange?: (value: string) => void
}

export function FilterPills({
  value,
  options,
  onValueChange,
  className,
  ...props
}: FilterPillsProps) {
  return (
    <div
      data-slot='filter-pills'
      className={cn('flex flex-wrap items-center gap-2', className)}
      {...props}
    >
      {options.map((option) => {
        const selected = option.value === value
        return (
          <Button
            key={option.value}
            type='button'
            size='sm'
            variant={selected ? 'default' : 'outline'}
            disabled={option.disabled}
            onClick={() => onValueChange?.(option.value)}
            className='rounded-full'
          >
            {option.label}
          </Button>
        )
      })}
    </div>
  )
}

type SegmentedTabsProps = ComponentProps<typeof Tabs> & {
  options: PillOption[]
}

export function SegmentedTabs({
  options,
  className,
  ...props
}: SegmentedTabsProps) {
  return (
    <Tabs className={cn('w-full sm:w-auto', className)} {...props}>
      <TabsList className='border-border/60 bg-card h-auto max-w-full flex-wrap justify-start rounded-full border p-1'>
        {options.map((option) => (
          <TabsTrigger
            key={option.value}
            value={option.value}
            disabled={option.disabled}
            className='data-active:bg-primary data-active:text-primary-foreground data-active:shadow-none'
          >
            {option.label}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  )
}

type EmptyStateProps = Omit<ComponentProps<'div'>, 'title'> & {
  icon?: ReactNode
  title?: ReactNode
  description?: ReactNode
  action?: ReactNode
}

export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
  ...props
}: EmptyStateProps) {
  const { t } = useTranslation()

  return (
    <Empty
      className={cn(
        'border-border/60 bg-card min-h-64 rounded-[calc(var(--radius)*1.375)] border border-dashed p-8',
        className
      )}
      {...props}
    >
      <EmptyHeader>
        <EmptyMedia variant='icon'>{icon ?? <Database />}</EmptyMedia>
        <EmptyTitle>{title ?? t('No Data')}</EmptyTitle>
        {description != null && (
          <EmptyDescription>{description}</EmptyDescription>
        )}
      </EmptyHeader>
      {action != null && <EmptyContent>{action}</EmptyContent>}
    </Empty>
  )
}

type ToolbarProps = ComponentProps<'div'> & {
  searchValue?: string
  searchPlaceholder?: string
  onSearchChange?: (value: string) => void
  filters?: ReactNode
  actions?: ReactNode
}

export function Toolbar({
  searchValue,
  searchPlaceholder,
  onSearchChange,
  filters,
  actions,
  children,
  className,
  ...props
}: ToolbarProps) {
  return (
    <div
      data-slot='toolbar'
      className={cn(
        'border-border/60 bg-card flex flex-col gap-3 rounded-[calc(var(--radius)*1.125)] border p-3 sm:flex-row sm:items-center sm:justify-between',
        className
      )}
      {...props}
    >
      <div className='flex min-w-0 flex-1 flex-wrap items-center gap-2'>
        {onSearchChange != null && (
          <div className='relative w-full sm:max-w-xs'>
            <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2' />
            <Input
              value={searchValue ?? ''}
              onChange={(event) => onSearchChange(event.target.value)}
              placeholder={searchPlaceholder}
              className='pl-8'
            />
          </div>
        )}
        {filters}
        {children}
      </div>
      {actions != null && (
        <div className='flex shrink-0 flex-wrap items-center gap-2'>
          {actions}
        </div>
      )}
    </div>
  )
}
