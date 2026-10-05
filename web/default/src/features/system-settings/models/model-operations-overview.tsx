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
import {
  Activity,
  AlertTriangle,
  BadgeDollarSign,
  Brain,
  FileText,
  Fingerprint,
  GitBranch,
  Globe,
  RadioTower,
  Settings2,
  ShieldCheck,
  Sparkles,
  Timer,
  type LucideIcon,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { buttonVariants } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  type OverviewStats,
  type TimeRange,
  useOverviewStats,
} from '@/features/dashboard/stats-api'
import { buildUsageLogsTimeSearch } from '@/features/dashboard/time-range-utils'

type InternalLink =
  | {
      kind: 'model-settings'
      section:
        | 'global'
        | 'gemini'
        | 'claude'
        | 'grok'
        | 'user-agents'
        | 'client-identity'
        | 'model-pricing'
        | 'channel-affinity'
        | 'model-deployment'
    }
  | {
      kind: 'dashboard'
      section: 'overview' | 'models' | 'channels' | 'users'
    }
  | {
      kind: 'security-settings'
      section: 'upstream-error-rules'
    }
  | {
      kind: 'plain'
      to: '/channels' | '/models'
    }

type OperationsCard = {
  title: string
  description: string
  icon: LucideIcon
  status: string
  cta: string
  link: InternalLink
}

const PRIMARY_ACTIONS: OperationsCard[] = [
  {
    title: 'Official pricing',
    description:
      'Sync official-only model prices from models.dev and keep reseller aliases out of the pricing table.',
    icon: BadgeDollarSign,
    status: 'Official-only',
    cta: 'Open pricing',
    link: { kind: 'model-settings', section: 'model-pricing' },
  },
  {
    title: 'User-Agent profiles',
    description:
      'Manage default, model-category, and channel-level User-Agent behavior for OpenAI, Claude, Grok, Gemini, and other models.',
    icon: Globe,
    status: 'Hot path',
    cta: 'Manage UA',
    link: { kind: 'model-settings', section: 'user-agents' },
  },
  {
    title: 'Client identity',
    description:
      'Force Codex, Claude, and future provider identifier fields, with manual and scheduled rotation controls.',
    icon: Fingerprint,
    status: 'Forced',
    cta: 'Manage IDs',
    link: { kind: 'model-settings', section: 'client-identity' },
  },
  {
    title: 'Model analytics',
    description:
      'Inspect model request volume, success rate, first-token latency, cost, token volume, and trend windows.',
    icon: Activity,
    status: 'Admin stats',
    cta: 'View models',
    link: { kind: 'dashboard', section: 'models' },
  },
  {
    title: 'Channel analytics',
    description:
      'Drill into channel reliability, first-token latency, user spend, and channel-level trend data.',
    icon: RadioTower,
    status: 'Admin only',
    cta: 'View channels',
    link: { kind: 'dashboard', section: 'channels' },
  },
  {
    title: 'Claude thinking',
    description:
      'Review Claude defaults here, then mark thinking-compatible channels from channel edit settings.',
    icon: Brain,
    status: 'Sanitizer',
    cta: 'Claude settings',
    link: { kind: 'model-settings', section: 'claude' },
  },
]

const SECONDARY_ACTIONS: OperationsCard[] = [
  {
    title: 'Global model behavior',
    description:
      'Configure pass-through behavior, ping intervals, thinking blacklists, and chat-to-responses routing policy.',
    icon: Settings2,
    status: 'Core',
    cta: 'Open global',
    link: { kind: 'model-settings', section: 'global' },
  },
  {
    title: 'Channel affinity',
    description:
      'Tune sticky channel selection and fallback behavior for model calls that need stable upstream routing.',
    icon: GitBranch,
    status: 'Routing',
    cta: 'Open affinity',
    link: { kind: 'model-settings', section: 'channel-affinity' },
  },
  {
    title: 'Model catalog',
    description:
      'Maintain the visible model list, model metadata, vendor grouping, and model marketplace entries.',
    icon: Sparkles,
    status: 'Catalog',
    cta: 'Open models',
    link: { kind: 'plain', to: '/models' },
  },
  {
    title: 'Channel settings',
    description:
      'Edit upstream channels, scheduled tests, retry windows, per-channel UA, and Claude thinking support.',
    icon: ShieldCheck,
    status: 'Runtime',
    cta: 'Open channels',
    link: { kind: 'plain', to: '/channels' },
  },
  {
    title: 'Channel test scheduler',
    description:
      'Find channels that need per-channel test intervals, retry attempts, disable thresholds, and cross-day test windows.',
    icon: Timer,
    status: 'Recovery',
    cta: 'Open channels',
    link: { kind: 'plain', to: '/channels' },
  },
  {
    title: 'Upstream error rules',
    description:
      'Normalize unsafe upstream error bodies for clients while keeping masked previews in backend logs.',
    icon: AlertTriangle,
    status: 'Anti-poison',
    cta: 'Open rules',
    link: { kind: 'security-settings', section: 'upstream-error-rules' },
  },
]

const SNAPSHOT_RANGE: TimeRange = '7d'

export function ModelOperationsOverview() {
  const {
    data: overview,
    isLoading: overviewLoading,
    error: overviewError,
  } = useOverviewStats(SNAPSHOT_RANGE, false)

  return (
    <section className='space-y-4'>
      <div className='bg-card rounded-lg border px-4 py-4'>
        <div className='flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between'>
          <div className='max-w-3xl space-y-1'>
            <p className='text-sm font-medium'>Model control center</p>
            <p className='text-muted-foreground text-sm'>
              A compact map of model pricing, routing, identity, User-Agent,
              channel health, and analytics workspaces.
            </p>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Badge variant='secondary'>Pricing</Badge>
            <Badge variant='secondary'>Routing</Badge>
            <Badge variant='secondary'>Identity</Badge>
            <Badge variant='secondary'>Analytics</Badge>
          </div>
        </div>
      </div>

      <ModelHealthSnapshot
        overview={overview}
        loading={overviewLoading}
        error={overviewError}
      />

      <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-3'>
        {PRIMARY_ACTIONS.map((action) => (
          <OperationsActionCard key={action.title} action={action} />
        ))}
      </div>

      <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-3'>
        {SECONDARY_ACTIONS.map((action) => (
          <OperationsActionCard key={action.title} action={action} compact />
        ))}
      </div>
    </section>
  )
}

function ModelHealthSnapshot(props: {
  overview?: OverviewStats
  loading: boolean
  error: unknown
}) {
  const topModel = props.overview?.top_models?.[0]
  const topChannel = props.overview?.top_channels?.[0]
  const slowChannel = props.overview?.slowest_channels?.[0]
  const logsTimeSearch = buildUsageLogsTimeSearch(SNAPSHOT_RANGE)

  return (
    <div className='grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(18rem,0.45fr)]'>
      <Card className='rounded-lg shadow-none'>
        <CardHeader className='flex flex-col gap-2 border-b pb-3 md:flex-row md:items-center md:justify-between'>
          <div>
            <CardTitle className='text-sm'>7-day model health</CardTitle>
            <CardDescription>
              Request volume, reliability, first-token latency, and spend from
              the existing operations stats API.
            </CardDescription>
          </div>
          <Link
            to='/dashboard/$section'
            params={{ section: 'overview' }}
            search={{ time_range: SNAPSHOT_RANGE }}
            className={buttonVariants({ variant: 'outline', size: 'sm' })}
          >
            Open analytics
          </Link>
        </CardHeader>
        <CardContent className='grid gap-3 pt-4 sm:grid-cols-2 xl:grid-cols-4'>
          <SnapshotMetric
            label='Requests'
            value={formatCount(props.overview?.total_requests)}
            loading={props.loading}
          />
          <SnapshotMetric
            label='Success rate'
            value={formatPercent(props.overview?.success_rate)}
            loading={props.loading}
            valueClassName={
              props.overview && props.overview.success_rate < 95
                ? 'text-warning-text'
                : undefined
            }
          />
          <SnapshotMetric
            label='First token'
            value={formatMs(props.overview?.avg_first_token_time)}
            loading={props.loading}
          />
          <SnapshotMetric
            label='Spend'
            value={formatUsd(props.overview?.total_cost)}
            loading={props.loading}
          />
        </CardContent>
      </Card>

      <Card className='rounded-lg shadow-none'>
        <CardHeader className='border-b pb-3'>
          <CardTitle className='text-sm'>Attention queue</CardTitle>
          <CardDescription>
            Fast links into the busiest and slowest model traffic.
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-3 pt-4'>
          {props.error ? (
            <div className='border-destructive/30 bg-destructive/5 text-destructive-text flex items-start gap-2 rounded-md border p-3 text-sm'>
              <AlertTriangle className='mt-0.5 size-4 shrink-0' />
              <span>Unable to load the operations snapshot.</span>
            </div>
          ) : props.loading ? (
            Array.from({ length: 3 }).map((_, index) => (
              <Skeleton key={index} className='h-12 w-full rounded-md' />
            ))
          ) : (
            <>
              <SnapshotLink
                icon={Activity}
                label='Top model'
                value={topModel?.model_name}
                detail={
                  formatCount(topModel?.total_requests, '0') + ' requests'
                }
                toSection='models'
                search={
                  topModel
                    ? {
                        model_name: topModel.model_name,
                        time_range: SNAPSHOT_RANGE,
                      }
                    : { time_range: SNAPSHOT_RANGE }
                }
                logsSearch={
                  topModel
                    ? { page: 1, ...logsTimeSearch, model: topModel.model_name }
                    : undefined
                }
              />
              <SnapshotLink
                icon={RadioTower}
                label='Top channel'
                value={topChannel?.channel_name}
                detail={
                  formatCount(topChannel?.total_requests, '0') + ' requests'
                }
                toSection='channels'
                search={
                  topChannel
                    ? {
                        channel_id: topChannel.channel_id,
                        time_range: SNAPSHOT_RANGE,
                      }
                    : { time_range: SNAPSHOT_RANGE }
                }
                logsSearch={
                  topChannel
                    ? {
                        page: 1,
                        ...logsTimeSearch,
                        channel: String(topChannel.channel_id),
                      }
                    : undefined
                }
              />
              <SnapshotLink
                icon={Timer}
                label='Slowest first token'
                value={slowChannel?.channel_name}
                detail={formatMs(slowChannel?.avg_first_token)}
                toSection='channels'
                search={
                  slowChannel
                    ? {
                        channel_id: slowChannel.channel_id,
                        time_range: SNAPSHOT_RANGE,
                      }
                    : { time_range: SNAPSHOT_RANGE }
                }
                logsSearch={
                  slowChannel
                    ? {
                        page: 1,
                        ...logsTimeSearch,
                        channel: String(slowChannel.channel_id),
                      }
                    : undefined
                }
              />
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function SnapshotMetric(props: {
  label: string
  value: string
  loading: boolean
  valueClassName?: string
}) {
  return (
    <div className='bg-muted/20 rounded-md border p-3'>
      <p className='text-muted-foreground text-xs'>{props.label}</p>
      {props.loading ? (
        <Skeleton className='mt-2 h-6 w-24' />
      ) : (
        <p
          className={cn(
            'mt-1 truncate text-lg font-semibold tabular-nums',
            props.valueClassName
          )}
        >
          {props.value}
        </p>
      )}
    </div>
  )
}

function SnapshotLink(props: {
  icon: LucideIcon
  label: string
  value?: string
  detail: string
  toSection: 'models' | 'channels'
  search:
    | { time_range: TimeRange; model_name?: string }
    | { time_range: TimeRange; channel_id?: number }
  logsSearch?: {
    page: number
    startTime: number
    endTime: number
    model?: string
    channel?: string
  }
}) {
  const Icon = props.icon
  return (
    <div className='bg-muted/20 flex items-center gap-2 rounded-md border p-2'>
      <Link
        to='/dashboard/$section'
        params={{ section: props.toSection }}
        search={props.search}
        className='hover:bg-muted/50 focus-visible:ring-ring flex min-w-0 flex-1 items-center justify-between gap-3 rounded-md px-1 py-1 text-sm transition-colors outline-none focus-visible:ring-2'
      >
        <span className='flex min-w-0 items-center gap-3'>
          <span className='bg-background flex size-8 shrink-0 items-center justify-center rounded-md'>
            <Icon className='text-muted-foreground size-4' />
          </span>
          <span className='min-w-0'>
            <span className='text-muted-foreground block text-xs'>
              {props.label}
            </span>
            <span className='block truncate font-medium'>
              {props.value ?? 'No data'}
            </span>
          </span>
        </span>
        <span className='text-muted-foreground shrink-0 text-xs'>
          {props.detail}
        </span>
      </Link>
      {props.logsSearch ? (
        <Link
          to='/usage-logs/$section'
          params={{ section: 'common' }}
          search={props.logsSearch}
          className='bg-background text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:ring-ring inline-flex h-9 shrink-0 items-center gap-1 rounded-md border px-2 text-xs font-medium transition-colors outline-none focus-visible:ring-2'
        >
          <FileText className='size-3.5' />
          Logs
        </Link>
      ) : null}
    </div>
  )
}

function OperationsActionCard(props: {
  action: OperationsCard
  compact?: boolean
}) {
  const Icon = props.action.icon
  return (
    <Card className='rounded-lg shadow-none'>
      <CardHeader className='gap-3'>
        <div className='flex items-start justify-between gap-3'>
          <div className='flex min-w-0 items-center gap-3'>
            <div className='bg-muted flex size-9 shrink-0 items-center justify-center rounded-md'>
              <Icon className='text-muted-foreground size-4' />
            </div>
            <div className='min-w-0'>
              <CardTitle className='truncate text-sm'>
                {props.action.title}
              </CardTitle>
              <Badge variant='outline' className='mt-1'>
                {props.action.status}
              </Badge>
            </div>
          </div>
        </div>
        <CardDescription
          className={cn(
            'leading-relaxed',
            props.compact ? 'min-h-16' : 'min-h-20'
          )}
        >
          {props.action.description}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <ActionLink action={props.action} />
      </CardContent>
    </Card>
  )
}

function formatCount(value: number | undefined, fallback = 'N/A') {
  if (value === undefined || !Number.isFinite(value)) return fallback
  return Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(value)
}

function formatPercent(value: number | undefined) {
  if (value === undefined || !Number.isFinite(value)) return 'N/A'
  return `${value.toFixed(2)}%`
}

function formatMs(value: number | undefined) {
  if (value === undefined || !Number.isFinite(value) || value <= 0) return 'N/A'
  return `${Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(value)}ms`
}

function formatUsd(value: number | undefined) {
  if (value === undefined || !Number.isFinite(value)) return 'N/A'
  return `$${Intl.NumberFormat('en-US', {
    maximumFractionDigits: value >= 1 ? 2 : 4,
  }).format(value)}`
}

function ActionLink(props: { action: OperationsCard }) {
  const className = cn(
    buttonVariants({ variant: 'outline', size: 'sm' }),
    'w-full'
  )
  const { link } = props.action
  if (link.kind === 'model-settings') {
    return (
      <Link
        to='/system-settings/models/$section'
        params={{ section: link.section }}
        className={className}
      >
        {props.action.cta}
      </Link>
    )
  }
  if (link.kind === 'dashboard') {
    return (
      <Link
        to='/dashboard/$section'
        params={{ section: link.section }}
        className={className}
      >
        {props.action.cta}
      </Link>
    )
  }
  if (link.kind === 'security-settings') {
    return (
      <Link
        to='/system-settings/security/$section'
        params={{ section: link.section }}
        className={className}
      >
        {props.action.cta}
      </Link>
    )
  }
  return (
    <Link to={link.to} className={className}>
      {props.action.cta}
    </Link>
  )
}
