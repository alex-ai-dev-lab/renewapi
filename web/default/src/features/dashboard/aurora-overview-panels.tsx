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
  ArrowUpRight,
  CheckCircle2,
  CircleAlert,
  CircleMinus,
  KeyRound,
  ListFilter,
  Wallet,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { useSidebarView } from '@/hooks/use-sidebar-view'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/status-badge'
import type { OverviewStats, SelfOverviewStats, TrendPoint } from './stats-api'
import { useDashboardHealthThresholds } from './use-dashboard-controls'

export function OverviewHealthPanel(props: { stats: OverviewStats }) {
  const { t, i18n } = useTranslation()
  const thresholds = useDashboardHealthThresholds()
  const stats = props.stats
  let health = {
    label: t('No requests'),
    variant: 'neutral' as 'neutral' | 'success' | 'warning' | 'danger',
    icon: CircleMinus,
  }
  if (stats.total_requests > 0) {
    if (stats.success_rate >= thresholds.successRateGoodThreshold) {
      health = { label: t('Healthy'), variant: 'success', icon: CheckCircle2 }
    } else if (stats.success_rate >= thresholds.successRateDegradedThreshold) {
      health = { label: t('Degraded'), variant: 'warning', icon: CircleAlert }
    } else {
      health = { label: t('Critical'), variant: 'danger', icon: CircleAlert }
    }
  }
  const topCostModel = [...stats.top_models].sort(
    (a, b) => b.total_cost - a.total_cost
  )[0]
  const topCostShare =
    stats.total_cost > 0 && topCostModel
      ? Math.min(100, (topCostModel.total_cost / stats.total_cost) * 100)
      : 0
  const formatNumber = (value: number) =>
    value.toLocaleString(i18n.resolvedLanguage)

  return (
    <div className='obsidian-checks'>
      <section className='obsidian-panel'>
        <header className='obsidian-panel-heading'>
          <h2>{t('Status and latency')}</h2>
          <StatusBadge
            label={health.label}
            variant={health.variant}
            icon={health.icon}
            copyable={false}
            showDot={false}
          />
        </header>
        <dl className='px-4 py-2'>
          <div className='obsidian-detail-row'>
            <dt>{t('Failed requests')}</dt>
            <dd>{formatNumber(stats.failed_requests)}</dd>
          </div>
          <div className='obsidian-detail-row'>
            <dt>{t('Error rate')}</dt>
            <dd>
              {stats.total_requests > 0
                ? `${stats.error_rate.toFixed(2)}%`
                : t('N/A')}
            </dd>
          </div>
          <div className='obsidian-detail-row'>
            <dt>{t('First token')}</dt>
            <dd>
              {stats.avg_first_token_time > 0
                ? `${stats.avg_first_token_time.toFixed(0)} ms`
                : t('N/A')}
            </dd>
          </div>
          <div className='obsidian-detail-row'>
            <dt>{t('Average duration')}</dt>
            <dd>
              {stats.avg_use_time > 0
                ? `${stats.avg_use_time.toFixed(2)} s`
                : t('N/A')}
            </dd>
          </div>
        </dl>
      </section>
      <section className='obsidian-panel'>
        <header className='obsidian-panel-heading'>
          <h2>{t('Usage summary')}</h2>
        </header>
        <dl className='px-4 py-2'>
          <div className='obsidian-detail-row'>
            <dt>{t('Requests per minute')}</dt>
            <dd>{formatNumber(stats.requests_per_minute)}</dd>
          </div>
          <div className='obsidian-detail-row'>
            <dt>{t('Active channels')}</dt>
            <dd>{formatNumber(stats.active_channels)}</dd>
          </div>
          <div className='obsidian-detail-row'>
            <dt>{t('Active users')}</dt>
            <dd>{formatNumber(stats.active_users)}</dd>
          </div>
        </dl>
        {topCostModel && (
          <div className='border-t px-4 py-3 text-xs'>
            <p className='text-muted-foreground'>{t('Highest-cost model')}</p>
            <p className='mt-1 font-medium break-all'>
              {topCostModel.model_name}
            </p>
            <p className='text-muted-foreground mt-1 tabular-nums'>
              {t('{{cost}} USD · {{share}}% of total cost', {
                cost: topCostModel.total_cost.toFixed(4),
                share: topCostShare.toFixed(1),
              })}
            </p>
          </div>
        )}
      </section>
    </div>
  )
}

export function OverviewUsagePanel(props: { stats: SelfOverviewStats }) {
  const { t, i18n } = useTranslation()
  return (
    <section className='obsidian-panel'>
      <header className='obsidian-panel-heading'>
        <h2>{t('Account usage')}</h2>
      </header>
      <dl className='px-4 py-2'>
        <div className='obsidian-detail-row'>
          <dt>{t('Credit remaining')}</dt>
          <dd>{formatQuota(props.stats.remaining_quota)}</dd>
        </div>
        <div className='obsidian-detail-row'>
          <dt>{t('Usage in the selected period')}</dt>
          <dd>{formatQuota(props.stats.range_usage)}</dd>
        </div>
        <div className='obsidian-detail-row'>
          <dt>{t('Lifetime usage')}</dt>
          <dd>{formatQuota(props.stats.used_quota)}</dd>
        </div>
        <div className='obsidian-detail-row'>
          <dt>{t('Models used')}</dt>
          <dd>
            {props.stats.top_models.length.toLocaleString(
              i18n.resolvedLanguage
            )}
          </dd>
        </div>
      </dl>
      <p className='text-muted-foreground border-t px-4 py-3 text-xs leading-relaxed'>
        {t(
          'Usage records include request and token totals. Reliability and latency are not available for this account view.'
        )}
      </p>
    </section>
  )
}

export function OverviewActivityPanel(props: {
  data: TrendPoint[]
  usageOnly?: boolean
}) {
  const { t, i18n } = useTranslation()
  const recent = [...props.data]
    .sort((a, b) => b.timestamp - a.timestamp)
    .slice(0, 5)
  const locale = i18n.resolvedLanguage
  return (
    <section className='obsidian-panel'>
      <header className='obsidian-panel-heading'>
        <div>
          <h2>{t('Recent activity')}</h2>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t('Aggregated usage, not individual request logs.')}
          </p>
        </div>
      </header>
      {recent.length === 0 ? (
        <p className='text-muted-foreground px-4 py-8 text-xs'>
          {t('No activity in this period')}
        </p>
      ) : (
        <ol className='obsidian-activity-list'>
          {recent.map((point) => (
            <li key={point.timestamp}>
              <time
                className='text-muted-foreground text-[11px] tabular-nums'
                dateTime={new Date(point.timestamp * 1000).toISOString()}
              >
                {new Date(point.timestamp * 1000).toLocaleString(locale, {
                  month: 'short',
                  day: 'numeric',
                  hour: '2-digit',
                  minute: '2-digit',
                })}
              </time>
              <p className='mt-1 font-medium tabular-nums'>
                {t('{{requests}} requests · {{tokens}} tokens', {
                  requests: point.requests.toLocaleString(locale),
                  tokens: (
                    point.total_prompt_tokens + point.total_output_tokens
                  ).toLocaleString(locale),
                })}
              </p>
              {!props.usageOnly && (
                <p className='text-muted-foreground mt-1'>
                  {t('{{failures}} failed requests', {
                    failures: point.failure.toLocaleString(locale),
                  })}
                </p>
              )}
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}

export function OverviewQuickActions() {
  const { t } = useTranslation()
  const { navGroups } = useSidebarView()
  const allowedUrls = new Set(
    navGroups.flatMap((group) =>
      group.items.flatMap((item) =>
        item.items ? item.items.map((child) => child.url) : [item.url]
      )
    )
  )
  const showLogs = allowedUrls.has('/usage-logs/common')
  return (
    <div className='obsidian-tools'>
      <section className='obsidian-panel'>
        <header className='obsidian-panel-heading'>
          <h2>{t('Quick actions')}</h2>
        </header>
        <nav
          className='obsidian-quick-links p-4'
          aria-label={t('Quick actions')}
        >
          {allowedUrls.has('/keys') && (
            <Button variant='outline' size='sm' render={<Link to='/keys' />}>
              <KeyRound aria-hidden='true' />
              {t('API Keys')}
            </Button>
          )}
          {showLogs && (
            <Button
              variant='outline'
              size='sm'
              render={
                <Link
                  to='/usage-logs/$section'
                  params={{ section: 'common' }}
                />
              }
            >
              <ListFilter aria-hidden='true' />
              {t('Usage Logs')}
            </Button>
          )}
          {allowedUrls.has('/wallet') && (
            <Button variant='outline' size='sm' render={<Link to='/wallet' />}>
              <Wallet aria-hidden='true' />
              {t('Wallet')}
            </Button>
          )}
          {allowedUrls.has('/channels') && (
            <Button
              variant='outline'
              size='sm'
              render={<Link to='/channels' />}
            >
              <ArrowUpRight aria-hidden='true' />
              {t('Channels')}
            </Button>
          )}
        </nav>
      </section>
      {showLogs && (
        <aside className='obsidian-runbook'>
          <h2 className='font-semibold'>{t('Inspect before retrying')}</h2>
          <p className='text-muted-foreground mt-2 leading-relaxed'>
            {t(
              'Use request logs to inspect failures and upstream responses before changing your configuration.'
            )}
          </p>
          <Link
            className='mt-3 inline-flex items-center gap-1 font-medium hover:underline'
            to='/usage-logs/$section'
            params={{ section: 'common' }}
          >
            {t('Open request logs')}
            <ArrowUpRight className='size-3.5' aria-hidden='true' />
          </Link>
        </aside>
      )}
    </div>
  )
}
