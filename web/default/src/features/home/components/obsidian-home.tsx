/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import { useMemo } from 'react'
import { Link } from '@tanstack/react-router'
import { ArrowRight, ArrowUpRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { sanitizeHtml } from '@/lib/sanitize-html'
import { Button } from '@/components/ui/button'
import { RequestPreview } from './request-preview'

type ObsidianHomeProps = {
  systemName: string
  apiBase: string
  docsLink: string
  docsEnabled: boolean
  aboutEnabled: boolean
  isAuthenticated: boolean
  registrationEnabled: boolean
  termsEnabled: boolean
  privacyEnabled: boolean
  footerHtml?: string
}

export function ObsidianHome(props: ObsidianHomeProps) {
  const { t } = useTranslation()
  const footerHtml = useMemo(
    () => sanitizeHtml(props.footerHtml || ''),
    [props.footerHtml]
  )
  let actionHref: '/dashboard' | '/sign-up' | '/sign-in' = '/sign-in'
  let actionLabel = t('Sign in')
  if (props.isAuthenticated) {
    actionHref = '/dashboard'
    actionLabel = t('Go to Dashboard')
  } else if (props.registrationEnabled) {
    actionHref = '/sign-up'
    actionLabel = t('Get started')
  }

  return (
    <div className='obsidian-home'>
      <main id='home-main-content' tabIndex={-1} className='obsidian-home-main'>
        <div className='obsidian-home-intro'>
          <p className='obsidian-home-eyebrow'>
            <span aria-hidden='true' className='obsidian-home-mark' />
            {t('Unified AI Gateway')}
          </p>
          <h1 className='obsidian-home-title'>
            {t('One endpoint.')}
            <br />
            <span>{t('A clear path to AI.')}</span>
          </h1>
          <p className='obsidian-home-description'>
            {t(
              'Connect your applications to model APIs. Manage keys, routing, and usage in one place.'
            )}
          </p>
          <div className='obsidian-home-actions'>
            <Button render={<Link to={actionHref} />}>
              {actionLabel}
              <ArrowRight size={16} aria-hidden='true' />
            </Button>
            {props.docsEnabled && (
              <a
                className='obsidian-home-text-link'
                href={props.docsLink}
                target='_blank'
                rel='noopener noreferrer'
              >
                {t('Read the docs')}
                <ArrowUpRight size={14} aria-hidden='true' />
              </a>
            )}
          </div>
          <dl className='obsidian-home-endpoint'>
            <dt>{t('API base URL')}</dt>
            <dd>
              <code>{props.apiBase}</code>
            </dd>
          </dl>
        </div>
        <RequestPreview apiBase={props.apiBase} systemName={props.systemName} />
      </main>
      <div
        className='obsidian-home-capabilities'
        aria-label={t('Gateway capabilities')}
      >
        <span>{t('Model routing')}</span>
        <span>{t('API key management')}</span>
        <span>{t('Usage visibility')}</span>
        <span>{t('Streaming responses')}</span>
      </div>
      <footer className='obsidian-home-footer'>
        {footerHtml && (
          <div
            className='obsidian-home-custom-footer'
            dangerouslySetInnerHTML={{ __html: footerHtml }}
          />
        )}
        <div className='obsidian-home-footer-row'>
          <div className='obsidian-home-attribution'>
            <span>
              © {new Date().getFullYear()} {props.systemName}
            </span>
            <a
              href='https://github.com/QuantumNous/new-api'
              target='_blank'
              rel='noopener noreferrer'
            >
              {t('Frontend by New API contributors')}
            </a>
          </div>
          <nav aria-label={t('Footer')}>
            {props.aboutEnabled && <Link to='/about'>{t('About')}</Link>}
            {props.termsEnabled && (
              <Link to='/user-agreement'>{t('Terms')}</Link>
            )}
            {props.privacyEnabled && (
              <Link to='/privacy-policy'>{t('Privacy')}</Link>
            )}
          </nav>
        </div>
      </footer>
    </div>
  )
}
