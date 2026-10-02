/* Copyright (C) 2026 RenewAPI contributors. SPDX-License-Identifier: AGPL-3.0-or-later */
import { describe, expect, test } from 'bun:test'
import { createInstance } from 'i18next'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'
import type { LogOtherData } from '../../types'
import { RelayAttemptDetails } from './relay-attempt-details'

async function renderDetails(info: NonNullable<LogOtherData['admin_info']>) {
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: {} } },
  })
  return renderToStaticMarkup(
    <I18nextProvider i18n={i18n}>
      <RelayAttemptDetails info={info} />
    </I18nextProvider>
  )
}

describe('model mapping attempt diagnostics', () => {
  test('shows the actual target and rule for same-channel attempts', async () => {
    const html = await renderDetails({
      attempts: [
        {
          channel_id: 1,
          channel_name: 'qa',
          priority: 10,
          attempt: 1,
          switch_count: 0,
          status_code: 404,
          elapsed_ms: 3,
          upstream_model: 'upstream-A',
          mapping_rule_id: 'rule-A',
          real_error: '<script>not a model</script>',
        },
        {
          channel_id: 1,
          channel_name: 'qa',
          priority: 10,
          attempt: 2,
          switch_count: 0,
          status_code: 200,
          elapsed_ms: 4,
          upstream_model: 'upstream-B',
          mapping_rule_id: 'rule-B',
        },
      ],
    })
    expect(html).toContain('upstream-A')
    expect(html).toContain('upstream-B')
    expect(html).toContain('rule-A')
    expect(html).toContain('rule-B')
    expect(html).toContain('Channel priority')
    expect(html).not.toContain('<script>')
    expect(html).toContain('&lt;script&gt;')
  })

  test('older attempt records remain readable without new fields', async () => {
    const html = await renderDetails({
      attempts: [
        {
          channel_id: 1,
          channel_name: 'legacy',
          priority: 0,
          attempt: 1,
          switch_count: 0,
          status_code: 200,
          elapsed_ms: 1,
        },
      ],
    })
    expect(html).toContain('legacy')
    expect(html).not.toContain('Mapping rule')
    expect(html).not.toContain('undefined')
  })
})
