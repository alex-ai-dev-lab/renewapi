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

type IzFooterProps = {
  systemName: string
  docsLink: string
  termsEnabled: boolean
  privacyEnabled: boolean
}

export function IzFooter(props: IzFooterProps) {
  const { t } = useTranslation()
  const siteLinks: [string, string][] = [['About', '/about']]
  if (props.termsEnabled) siteLinks.push(['Terms', '/user-agreement'])
  if (props.privacyEnabled) siteLinks.push(['Privacy', '/privacy-policy'])
  const columns: { title: string; links: [string, string][] }[] = [
    {
      title: 'Product',
      links: [
        ['Models', '/pricing'],
        ['Routing', '#routing'],
        ['Live status', '#live'],
        ['Protocols', '#protocols'],
      ],
    },
    {
      title: 'Developers',
      links: [
        ['Docs', props.docsLink],
        ['API reference', '#protocols'],
        ['Status page', '#live'],
        ['Changelog', 'https://github.com/alex-ai-dev-lab/renewapi/releases'],
      ],
    },
    { title: 'Site', links: siteLinks },
  ]
  return (
    <footer className='iz-footer'>
      <div className='iz-wrap'>
        <div className='iz-footer-grid'>
          <div>
            <div className='iz-site-brand iz-footer-brand'>
              {props.systemName}
            </div>
            <p>
              {t(
                'A unified, reliable, high-speed AI API gateway. One endpoint, every model.'
              )}
            </p>
          </div>
          {columns.map((column) => (
            <div className='iz-footer-column' key={column.title}>
              <h4>{t(column.title)}</h4>
              {column.links.map(([label, href]) => (
                <a
                  key={href}
                  href={href}
                  target={href.startsWith('http') ? '_blank' : undefined}
                  rel={href.startsWith('http') ? 'noreferrer' : undefined}
                >
                  {t(label)}
                </a>
              ))}
            </div>
          ))}
        </div>
        <p className='iz-footer-attribution'>
          Frontend design and development by{' '}
          <a
            href='https://github.com/QuantumNous/new-api'
            target='_blank'
            rel='noreferrer'
          >
            New API contributors.
          </a>
        </p>
        <div className='iz-footer-bottom'>
          <span>
            © {new Date().getFullYear()} {props.systemName}
          </span>
          <a href='#live'>{t('View service status')}</a>
        </div>
      </div>
    </footer>
  )
}
