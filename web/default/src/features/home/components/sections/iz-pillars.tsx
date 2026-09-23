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

interface Pillar {
  number: string
  title: string
  desc: string
  meta: string
}

export function IzPillars() {
  const { t } = useTranslation()

  const pillars: Pillar[] = [
    {
      number: '01',
      title: t('Low latency, by design'),
      desc: t(
        'Upstream connection reuse and streamed responses reduce unnecessary waiting during model requests.'
      ),
      meta: t('Streaming responses'),
    },
    {
      number: '02',
      title: t('Composed under pressure'),
      desc: t(
        'Channel-level health checks, automatic failover and quota-aware retries — your client always faces one stable endpoint.'
      ),
      meta: t('Channel failover'),
    },
    {
      number: '03',
      title: t('Native compatibility'),
      desc: t(
        'First-class support for OpenAI Chat & Responses, Claude Messages, Gemini and the Image API — keep the SDK, change the Base URL.'
      ),
      meta: t('Multiple protocols'),
    },
    {
      number: '04',
      title: t('Observable end to end'),
      desc: t(
        'Per-request tracing, token accounting, model-level usage and cost — every byte through the gateway is auditable.'
      ),
      meta: t('Request logs'),
    },
  ]

  return (
    <section className='iz-block' id='principles'>
      <div className='iz-wrap'>
        <AnimateInView animation='fade-up'>
          <header className='iz-section-head'>
            <span className='iz-watermark'>01</span>
            <div className='iz-section-left'>
              <span className='iz-index'>01 - {t('Principles')}</span>
              <span className='iz-section-tag'>
                {t('Built for serious workloads')}
              </span>
            </div>
            <div>
              <h2>{t('Tuned for AI workloads that run for hours.')}</h2>
              <p className='iz-section-desc'>
                {t(
                  'A small set of deliberate, explicit promises: low and predictable latency, resilience under failure, and one contract across every upstream.'
                )}
              </p>
            </div>
          </header>
        </AnimateInView>

        <div className='iz-principles'>
          {pillars.map((p, i) => (
            <AnimateInView key={i} animation='fade-up' delay={i * 80}>
              <article className='iz-principle'>
                <span className='iz-principle-number'>{p.number}</span>
                <div className='iz-principle-body'>
                  <h3>{p.title}</h3>
                  <p>{p.desc}</p>
                </div>
                <div className='iz-principle-meta'>{p.meta}</div>
              </article>
            </AnimateInView>
          ))}
        </div>
      </div>
    </section>
  )
}
