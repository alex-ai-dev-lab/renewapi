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
import { useTranslation } from 'react-i18next'
import { AnimateInView } from '@/components/animate-in-view'

const ROUTING_LABELS = ['health', 'failover', 'retry']
const PROVIDERS = [
  { name: 'OpenAI', y: 76, detail: 'chat · responses · images' },
  { name: 'Claude', y: 142, detail: 'messages · tools · stream' },
  { name: 'Gemini', y: 208, detail: 'generateContent · models' },
  { name: 'More', y: 274, detail: 'Configured channels' },
]

export function IzRouting(props: { systemName: string }) {
  const { t } = useTranslation()

  return (
    <section id='routing' className='iz-block iz-block-dark iz-routing'>
      <div className='iz-wrap iz-routing-inner'>
        <AnimateInView animation='fade-up'>
          <header className='iz-section-head'>
            <span className='iz-watermark'>02</span>
            <div className='iz-section-left'>
              <span className='iz-index'>02 - {t('Routing')}</span>
              <span className='iz-section-tag'>
                {t('One contract, many upstreams')}
              </span>
            </div>
            <div>
              <h2>{t('Your request, intelligently routed.')}</h2>
              <p className='iz-section-desc'>
                {t(
                  'A single call hits the gateway and is dispatched to a healthy upstream. Health checks, failover and quota-aware retries happen behind one stable endpoint — your client never sees the churn.'
                )}
              </p>
            </div>
          </header>
        </AnimateInView>

        <AnimateInView animation='fade-up' delay={120}>
          <div
            className='iz-routing-board'
            role='region'
            tabIndex={0}
            aria-label={t('Routing diagram')}
          >
            <div className='iz-routing-scroll'>
              <svg
                className='iz-routing-svg'
                viewBox='0 0 980 360'
                role='img'
                aria-labelledby='iz-routing-title iz-routing-desc'
              >
                <title id='iz-routing-title'>{t('Gateway routing flow')}</title>
                <desc id='iz-routing-desc'>
                  {t(
                    'Your application sends a request to the gateway, which selects a configured provider using routing and retry rules.'
                  )}
                </desc>
                <defs>
                  <marker
                    id='iz-arrow'
                    viewBox='0 0 10 10'
                    refX='8'
                    refY='5'
                    markerWidth='6'
                    markerHeight='6'
                    orient='auto-start-reverse'
                  >
                    <path
                      d='M 0 0 L 10 5 L 0 10 z'
                      className='iz-routing-arrow'
                    />
                  </marker>
                </defs>

                <rect
                  x='34'
                  y='126'
                  width='176'
                  height='108'
                  rx='8'
                  className='iz-routing-node'
                />
                <text x='64' y='174' className='iz-routing-node-title'>
                  {t('Your App')}
                </text>
                <text x='64' y='202' className='iz-routing-node-sub'>
                  {t('SDK unchanged')}
                </text>

                <rect
                  x='420'
                  y='96'
                  width='210'
                  height='168'
                  rx='8'
                  className='iz-routing-gateway'
                />
                <text x='454' y='158' className='iz-routing-gateway-title'>
                  {props.systemName}
                </text>
                <text x='454' y='190' className='iz-routing-gateway-sub'>
                  {t('one base URL')}
                </text>
                <text x='454' y='224' className='iz-routing-gateway-meta'>
                  {t('health · failover · retry')}
                </text>

                <path
                  d='M 210 180 C 290 180, 350 180, 420 180'
                  className='iz-routing-line iz-routing-line-active'
                  markerEnd='url(#iz-arrow)'
                />
                {ROUTING_LABELS.map((label, index) => (
                  <g key={t(label)}>
                    <rect
                      x={216 + index * 70}
                      y='142'
                      width='58'
                      height='22'
                      rx='11'
                      className='iz-routing-chip'
                    />
                    <text
                      x={245 + index * 70}
                      y='157'
                      className='iz-routing-chip-text'
                    >
                      {t(label)}
                    </text>
                  </g>
                ))}

                {PROVIDERS.map((provider) => (
                  <g key={t(provider.name)}>
                    <path
                      d={`M 630 180 C 700 180, 700 ${provider.y}, 752 ${provider.y}`}
                      className='iz-routing-line iz-routing-line-active'
                      markerEnd='url(#iz-arrow)'
                    />
                    <rect
                      x='752'
                      y={provider.y - 24}
                      width='188'
                      height='48'
                      rx='8'
                      className='iz-routing-provider'
                    />
                    <text
                      x='774'
                      y={provider.y - 4}
                      className='iz-routing-provider-title'
                    >
                      {t(provider.name)}
                    </text>
                    <text
                      x='774'
                      y={provider.y + 16}
                      className='iz-routing-provider-sub'
                    >
                      {t(provider.detail)}
                    </text>
                  </g>
                ))}
              </svg>
            </div>
          </div>
        </AnimateInView>
      </div>
    </section>
  )
}
