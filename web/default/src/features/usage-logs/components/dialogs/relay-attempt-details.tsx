/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import { useTranslation } from 'react-i18next'
import type { LogOtherData } from '../../types'

export function RelayAttemptDetails(props: {
  info: NonNullable<LogOtherData['admin_info']>
}) {
  const { t } = useTranslation()
  if (!props.info.real_error && !props.info.attempts?.length) return null
  return (
    <section className='space-y-2 border-t pt-3'>
      <h3 className='text-sm font-semibold'>{t('Channel attempt chain')}</h3>
      {props.info.real_error && (
        <pre className='max-h-40 overflow-auto text-xs break-words whitespace-pre-wrap'>
          {props.info.real_error}
        </pre>
      )}
      <ol className='space-y-2'>
        {props.info.attempts?.map((attempt) => (
          <li
            key={attempt.attempt}
            className='space-y-1 rounded-md border p-2 text-xs'
          >
            <p className='font-medium break-all'>
              {attempt.attempt}. {attempt.channel_name} (#{attempt.channel_id})
            </p>
            <p>
              {t('Priority')}: {attempt.priority} · {t('Switches')}:{' '}
              {attempt.switch_count} · HTTP {attempt.status_code} ·{' '}
              {attempt.elapsed_ms} ms
            </p>
            {attempt.timeout_stage && (
              <p>
                {t('Timeout stage')}: {attempt.timeout_stage}
              </p>
            )}
            {attempt.upstream_request_id && (
              <p className='break-all'>
                {t('Upstream Request ID')}: {attempt.upstream_request_id}
              </p>
            )}
            <pre className='max-h-40 overflow-auto break-words whitespace-pre-wrap'>
              {attempt.real_error || t('Success')}
            </pre>
          </li>
        ))}
      </ol>
    </section>
  )
}
