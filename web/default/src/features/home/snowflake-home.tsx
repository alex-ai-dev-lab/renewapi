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
import { ArrowUpRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useSystemConfig } from '@/hooks/use-system-config'
import { HomeHeader } from './components/home-header'
import { SnowflakeCanvas } from './components/snowflake-canvas'
import './home.css'

export default function SnowflakeHome() {
  const { t } = useTranslation()
  const { systemName } = useSystemConfig()

  return (
    <main className='snowapi-deeix-home'>
      <HomeHeader />

      <section id='home' className='snowapi-deeix-hero'>
        <div className='snowapi-deeix-ambient-particles' aria-hidden='true'>
          <span />
          <span />
        </div>
        <div className='snowapi-deeix-container snowapi-deeix-hero-layout'>
          <div className='snowapi-deeix-hero-copy'>
            <h1>
              <span className='snowapi-deeix-title'>{systemName}</span>
              <span className='snowapi-deeix-subtitle'>
                {t('Intelligence, delivered.')}
              </span>
            </h1>
            <p>
              {t(
                'Reliable model access, clear pricing, usage visibility, and simple API key management in one gateway.'
              )}
            </p>
            <Link to='/sign-in' className='snowapi-deeix-primary-link'>
              {t('Sign in')}
              <ArrowUpRight aria-hidden='true' />
            </Link>
          </div>

          <SnowflakeCanvas />
        </div>
      </section>

      <footer className='snowapi-deeix-copyright'>
        <span>
          © {new Date().getFullYear()} {systemName}
        </span>
        <span>
          UI:{' '}
          <a
            href='https://github.com/Ooxygen7/SnowAPI'
            target='_blank'
            rel='noreferrer'
          >
            SnowAPI
          </a>
        </span>
        <span>
          Frontend design and development by{' '}
          <a
            href='https://github.com/QuantumNous/new-api'
            target='_blank'
            rel='noreferrer'
          >
            New API contributors
          </a>
          .
        </span>
      </footer>
    </main>
  )
}
