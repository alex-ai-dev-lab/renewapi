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

const PROVIDERS = [
  'OpenAI',
  'Anthropic',
  'Google Gemini',
  'DeepSeek',
  'Qwen',
  'Mistral',
  'Meta Llama',
  'xAI Grok',
  'Doubao',
  'Kimi',
]

export function IzModels() {
  const { t } = useTranslation()
  const items = [...PROVIDERS, ...PROVIDERS]

  return (
    <section
      className='iz-model-strip'
      id='models'
      aria-label={t('Supported providers')}
    >
      <div className='iz-wrap iz-model-strip-inner'>
        <span className='iz-label'>{t('Works with')}</span>
        <div className='iz-marquee'>
          <div className='iz-marquee-track'>
            {items.map((provider, index) => (
              <span
                key={`${provider}-${index}`}
                aria-hidden={index >= PROVIDERS.length}
              >
                {provider}
              </span>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
