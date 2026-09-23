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
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { AnimateInView } from '@/components/animate-in-view'

interface IzClosingProps {
  isAuthenticated?: boolean
  registrationEnabled: boolean
}

export function IzClosing(props: IzClosingProps) {
  const { t } = useTranslation()

  return (
    <section className='iz-final'>
      <div className='iz-blueprint' aria-hidden />
      <div className='iz-wrap'>
        <AnimateInView animation='fade-up'>
          <h2>{t('Swap the Base URL, keep everything else.')}</h2>
          <p>
            {t(
              'Sign in to manage your API keys and use the models available to your account.'
            )}
          </p>
          <div className='iz-final-actions'>
            {props.isAuthenticated ? (
              <Button
                className='iz-button iz-button-light iz-button-lg group'
                render={<Link to='/dashboard' />}
              >
                {t('Open Dashboard')}
                <ArrowRight className='ml-1 size-3.5 transition-transform duration-200 group-hover:translate-x-0.5' />
              </Button>
            ) : (
              <>
                <Button
                  className='iz-button iz-button-light iz-button-lg group'
                  render={
                    <Link
                      to={props.registrationEnabled ? '/sign-up' : '/sign-in'}
                    />
                  }
                >
                  {props.registrationEnabled
                    ? t('Create your key')
                    : t('Sign in')}
                  <ArrowRight className='ml-1 size-3.5 transition-transform duration-200 group-hover:translate-x-0.5' />
                </Button>
                <Button
                  variant='link'
                  className='iz-text-link iz-text-link-light'
                  render={<Link to='/pricing' />}
                >
                  {t('Browse models')}
                </Button>
              </>
            )}
          </div>
        </AnimateInView>
      </div>
    </section>
  )
}
