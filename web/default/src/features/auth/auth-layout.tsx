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
import '@/styles/obsidian-auth.css'
import { ArrowLeft, KeyRound, SlidersHorizontal, Activity } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Skeleton } from '@/components/ui/skeleton'
import { LanguageSwitcher } from '@/components/language-switcher'
import { ThemeSwitch } from '@/components/theme-switch'

type AuthLayoutProps = {
  children: React.ReactNode
}

export function AuthLayout(props: AuthLayoutProps) {
  const { t } = useTranslation()
  const { systemName, logo, loading } = useSystemConfig()

  return (
    <div className='obsidian-auth'>
      <a className='obsidian-auth-skip' href='#auth-main'>
        {t('Skip to content')}
      </a>
      <header className='obsidian-auth-header'>
        <Link to='/' className='obsidian-auth-brand'>
          {loading ? (
            <Skeleton className='size-7' />
          ) : (
            <img src={logo} alt='' className='size-7 object-contain' />
          )}
          {loading ? (
            <Skeleton className='h-4 w-24' />
          ) : (
            <span>{systemName}</span>
          )}
        </Link>
        <div className='obsidian-auth-tools'>
          <LanguageSwitcher />
          <ThemeSwitch />
        </div>
      </header>
      <main id='auth-main' tabIndex={-1} className='obsidian-auth-main'>
        <aside
          className='obsidian-auth-context'
          aria-label={t('Account access')}
        >
          <p className='obsidian-auth-eyebrow'>{t('Account access')}</p>
          <p className='obsidian-auth-context-title'>
            {t('Your gateway, within reach.')}
          </p>
          <p className='obsidian-auth-context-description'>
            {t('Manage your API keys and usage from one account.')}
          </p>
          <ul className='obsidian-auth-capabilities'>
            <li>
              <KeyRound size={16} aria-hidden='true' />
              <span>{t('API key management')}</span>
            </li>
            <li>
              <SlidersHorizontal size={16} aria-hidden='true' />
              <span>{t('Model access')}</span>
            </li>
            <li>
              <Activity size={16} aria-hidden='true' />
              <span>{t('Usage visibility')}</span>
            </li>
          </ul>
          <Link to='/' className='obsidian-auth-back'>
            <ArrowLeft size={14} aria-hidden='true' />
            {t('Back to home')}
          </Link>
        </aside>
        <div className='obsidian-auth-form'>{props.children}</div>
      </main>
      <footer className='obsidian-auth-footer'>
        <span>
          © {new Date().getFullYear()} {systemName}
        </span>
        <Link to='/'>{t('Back to home')}</Link>
      </footer>
    </div>
  )
}
