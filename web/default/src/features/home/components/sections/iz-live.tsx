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
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import { AnimateInView } from '@/components/animate-in-view'
import { readGatewaySnapshot } from '../../lib/gateway-status'

export function IzLive() {
  const { t, i18n } = useTranslation()
  const status = useQuery({
    queryKey: ['gateway-status'],
    queryFn: async ({ signal }) => {
      const response = await api.get('/api/status', {
        signal,
        timeout: 10_000,
        skipErrorHandler: true,
      })
      return readGatewaySnapshot(response.data)
    },
    retry: false,
    refetchInterval: 15_000,
  })
  // 请求失败后不把上一次的成功结果继续显示成当前在线状态。
  const snapshot = status.isError ? undefined : status.data
  let state: 'online' | 'checking' | 'unavailable' = 'unavailable'
  let stateLabel = t('Unavailable')
  if (status.isPending) {
    state = 'checking'
    stateLabel = t('Checking')
  } else if (snapshot) {
    state = 'online'
    stateLabel = t('Online')
  }
  const elapsedMinutes = snapshot?.startedAt
    ? Math.floor((snapshot.checkedAt / 1000 - snapshot.startedAt) / 60)
    : null
  const uptime =
    elapsedMinutes === null
      ? t('Not available')
      : t('{{days}}d {{hours}}h {{minutes}}m', {
          days: Math.floor(elapsedMinutes / 1440),
          hours: Math.floor((elapsedMinutes % 1440) / 60),
          minutes: elapsedMinutes % 60,
        })
  const formatTime = (timestamp: number) =>
    new Date(timestamp).toLocaleString(i18n.resolvedLanguage || i18n.language)

  return (
    <section id='live' className='iz-block iz-block-alt iz-live'>
      <div className='iz-wrap'>
        <AnimateInView animation='fade-up'>
          <header className='iz-section-head'>
            <span className='iz-watermark' aria-hidden>
              03
            </span>
            <div className='iz-section-left'>
              <span className='iz-index'>03 - {t('Service status')}</span>
              <span className='iz-section-tag'>
                {t('The gateway, right now')}
              </span>
            </div>
            <div>
              <h2>{t('Service status you can verify.')}</h2>
              <p className='iz-section-desc'>
                {t(
                  'Gateway availability and runtime information, refreshed every 15 seconds. Model latency and channel health are available in the console.'
                )}
              </p>
            </div>
          </header>
        </AnimateInView>
        <div className='iz-live-grid'>
          <article className='iz-live-panel iz-live-chart-panel'>
            <div className='iz-live-panel-head'>
              <span>{t('Gateway availability')}</span>
              <strong role='status' data-gateway-state={state}>
                {stateLabel}
              </strong>
            </div>
            <p className='iz-service-description'>
              {snapshot
                ? t(
                    'The gateway responded successfully. Individual upstream services may have different availability.'
                  )
                : t(
                    'The current service status cannot be confirmed. Please retry in a moment.'
                  )}
            </p>
            <div className='iz-service-actions'>
              <button
                className='iz-button iz-button-dark iz-button-sm'
                type='button'
                onClick={() => void status.refetch()}
                disabled={status.isFetching}
              >
                {status.isFetching ? t('Checking') : t('Refresh status')}
              </button>
              <Link className='iz-text-link' to='/dashboard'>
                {t('View console metrics')}
              </Link>
            </div>
            <div className='iz-live-panel-foot'>
              <span>{t('Last successful check')}</span>
              <b>
                {status.data
                  ? formatTime(status.data.checkedAt)
                  : t('Not available')}
              </b>
            </div>
          </article>
          <article className='iz-live-panel'>
            <div className='iz-live-panel-head'>
              <span>{t('Runtime information')}</span>
            </div>
            <dl className='iz-service-details'>
              <div>
                <dt>{t('Version')}</dt>
                <dd>{snapshot?.version || t('Not available')}</dd>
              </div>
              <div>
                <dt>{t('Started at')}</dt>
                <dd>
                  {snapshot?.startedAt
                    ? formatTime(snapshot.startedAt * 1000)
                    : t('Not available')}
                </dd>
              </div>
              <div>
                <dt>{t('Running time')}</dt>
                <dd>{uptime}</dd>
              </div>
            </dl>
          </article>
        </div>
      </div>
    </section>
  )
}
