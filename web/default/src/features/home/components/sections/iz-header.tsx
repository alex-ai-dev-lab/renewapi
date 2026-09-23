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
import { useEffect, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

type IzHeaderProps = {
  isAuthenticated?: boolean
  registrationEnabled: boolean
  systemName: string
  docsLink: string
}

const NAV_LINKS = [
  ['Models', '/pricing'],
  ['Routing', '#routing'],
  ['Live', '#live'],
  ['Protocols', '#protocols'],
] as const

export function IzHeader(props: IzHeaderProps) {
  const { t } = useTranslation()
  const [mobileOpen, setMobileOpen] = useState(false)
  const [scrolled, setScrolled] = useState(false)

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 12)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  useEffect(() => {
    document.body.classList.toggle('menu-open', mobileOpen)
    return () => document.body.classList.remove('menu-open')
  }, [mobileOpen])

  useEffect(() => {
    if (!mobileOpen) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setMobileOpen(false)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [mobileOpen])

  let actionHref: '/dashboard' | '/sign-up' | '/sign-in' =
    props.registrationEnabled ? '/sign-up' : '/sign-in'
  if (props.isAuthenticated) actionHref = '/dashboard'
  let actionLabel = t('Sign in')
  if (props.isAuthenticated) actionLabel = t('Console')
  else if (props.registrationEnabled) actionLabel = t('Get a key')

  return (
    <>
      <header className={`iz-site-header ${scrolled ? 'is-scrolled' : ''}`}>
        <div className='iz-wrap'>
          <nav className='iz-site-nav' aria-label={t('Primary')}>
            <a className='iz-site-brand' href='#top'>
              {props.systemName}
            </a>
            <div className='iz-site-links'>
              {NAV_LINKS.map(([label, href]) => (
                <a key={href} href={href}>
                  {t(label)}
                </a>
              ))}
              <a href={props.docsLink} target='_blank' rel='noreferrer'>
                {t('Docs')}
              </a>
            </div>
            <div className='iz-site-actions'>
              <Link
                className='iz-text-link'
                to={props.isAuthenticated ? '/dashboard' : '/sign-in'}
              >
                {t('Console')}
              </Link>
              <Link
                className='iz-button iz-button-dark iz-button-sm'
                to={actionHref}
              >
                {actionLabel}
              </Link>
              <button
                type='button'
                className={`iz-burger ${mobileOpen ? 'is-active' : ''}`}
                aria-label={mobileOpen ? t('Close menu') : t('Open menu')}
                aria-expanded={mobileOpen}
                aria-controls='iz-mobile-menu'
                onClick={() => setMobileOpen((open) => !open)}
              >
                <span />
                <span />
                <span />
              </button>
            </div>
          </nav>
        </div>
      </header>
      <div
        id='iz-mobile-menu'
        className={`iz-mobile-menu ${mobileOpen ? 'is-open' : ''}`}
        aria-hidden={!mobileOpen}
        inert={!mobileOpen}
      >
        <nav className='iz-mobile-menu-inner' aria-label={t('Mobile')}>
          {NAV_LINKS.map(([label, href]) => (
            <a key={href} href={href} onClick={() => setMobileOpen(false)}>
              {t(label)}
            </a>
          ))}
          <a
            href={props.docsLink}
            target='_blank'
            rel='noreferrer'
            onClick={() => setMobileOpen(false)}
          >
            {t('Docs')}
          </a>
          <div className='iz-mobile-actions'>
            <Link
              className='iz-text-link'
              to={props.isAuthenticated ? '/dashboard' : '/sign-in'}
              onClick={() => setMobileOpen(false)}
            >
              {t('Console')}
            </Link>
            <Link
              className='iz-button iz-button-dark'
              to={actionHref}
              onClick={() => setMobileOpen(false)}
            >
              {actionLabel}
            </Link>
          </div>
        </nav>
      </div>
    </>
  )
}
