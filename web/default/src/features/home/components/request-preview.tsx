/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import { useState } from 'react'
import { Copy, ArrowRight, Terminal } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { Button } from '@/components/ui/button'
import { buildRequestExample } from '../lib/request-example'

type RequestPreviewProps = {
  systemName: string
  apiBase: string
}

export function RequestPreview(props: RequestPreviewProps) {
  const { t } = useTranslation()
  const [copyMessage, setCopyMessage] = useState('')
  const example = buildRequestExample(props.apiBase)

  async function copyExample() {
    const copied = await copyToClipboard(example)
    const message = copied ? t('Copied') : t('Failed to copy')
    setCopyMessage(message)
    if (!copied) toast.error(message)
  }

  return (
    <section
      className='obsidian-home-preview'
      aria-label={t('API request example')}
    >
      <figure className='obsidian-home-flow'>
        <figcaption className='obsidian-home-section-label'>
          <span>
            <span aria-hidden='true'>01 / </span>
            {t('Request path')}
          </span>
          <span>{t('Illustration')}</span>
        </figcaption>
        <div className='obsidian-home-flow-path'>
          <div className='obsidian-home-flow-node'>
            <span className='obsidian-home-node-index' aria-hidden='true'>
              01
            </span>
            <strong>{t('Your app')}</strong>
            <span>{t('API request')}</span>
          </div>
          <svg
            viewBox='0 0 40 24'
            className='obsidian-home-flow-line'
            aria-hidden='true'
          >
            <path d='M0 12H40' className='obsidian-home-flow-track' />
            <path d='M0 12H40' className='obsidian-home-flow-pulse' />
            <path d='m33 7 5 5-5 5' />
          </svg>
          <div className='obsidian-home-flow-node obsidian-home-flow-gateway'>
            <span className='obsidian-home-node-index' aria-hidden='true'>
              02
            </span>
            <strong>{props.systemName}</strong>
            <span>{t('Route & authenticate')}</span>
          </div>
          <svg
            viewBox='0 0 40 24'
            className='obsidian-home-flow-line'
            aria-hidden='true'
          >
            <path d='M0 12H40' className='obsidian-home-flow-track' />
            <path d='M0 12H40' className='obsidian-home-flow-pulse' />
            <path d='m33 7 5 5-5 5' />
          </svg>
          <div className='obsidian-home-flow-node'>
            <span className='obsidian-home-node-index' aria-hidden='true'>
              03
            </span>
            <strong>{t('Model API')}</strong>
            <span>{t('Selected upstream')}</span>
          </div>
        </div>
        <div className='obsidian-home-flow-return'>
          <ArrowRight aria-hidden='true' size={14} />
          <span>{t('Response returns through the same gateway')}</span>
        </div>
      </figure>
      <div className='obsidian-home-terminal'>
        <div className='obsidian-home-section-label'>
          <span className='obsidian-home-terminal-label'>
            <Terminal size={14} aria-hidden='true' />
            {t('Example request')}
            <span className='obsidian-home-terminal-language'>cURL</span>
          </span>
          <Button size='sm' variant='ghost' type='button' onClick={copyExample}>
            <Copy size={14} aria-hidden='true' />
            {copyMessage || t('Copy')}
          </Button>
        </div>
        <pre tabIndex={0} aria-label={t('Example cURL command')}>
          <code>{example}</code>
        </pre>
        <div className='obsidian-home-example-note'>
          <p>
            {t(
              'Example only. Set API_KEY and replace YOUR_MODEL before running. Requests may incur charges.'
            )}
          </p>
          <span className='sr-only' role='status'>
            {copyMessage}
          </span>
        </div>
      </div>
    </section>
  )
}
