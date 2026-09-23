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
import { useMemo, useState } from 'react'
import { Check, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { AnimateInView } from '@/components/animate-in-view'

type Lang = 'curl' | 'python' | 'node'

const baseUrlAtRender = () =>
  typeof window !== 'undefined'
    ? window.location.origin
    : 'https://your.gateway'

const LANGS = ['curl', 'python', 'node'] as const
const LANG_LABELS: Record<Lang, string> = {
  curl: 'cURL',
  python: 'Python',
  node: 'Node.js',
}

export function IzQuickstart() {
  const { t } = useTranslation()
  const [lang, setLang] = useState<Lang>('curl')
  const [copied, setCopied] = useState(false)
  const base = useMemo(() => baseUrlAtRender(), [])

  const snippets: Record<Lang, string> = useMemo(
    () => ({
      curl: `curl ${base}/v1/chat/completions \\
  -H "Authorization: Bearer $YOUR_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "YOUR_MODEL",
    "stream": true,
    "messages": [
      { "role": "user", "content": "Ping" }
    ]
  }'`,
      python: `from openai import OpenAI

client = OpenAI(
    base_url="${base}/v1",
    api_key="YOUR_KEY",
)

stream = client.chat.completions.create(
    model="YOUR_MODEL",
    stream=True,
    messages=[{"role": "user", "content": "Ping"}],
)

for chunk in stream:
    if chunk.choices:
        print(chunk.choices[0].delta.content or "", end="")`,
      node: `import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "${base}/v1",
  apiKey: process.env.YOUR_KEY,
});

const stream = await client.chat.completions.create({
  model: "YOUR_MODEL",
  stream: true,
  messages: [{ role: "user", content: "Ping" }],
});

for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0]?.delta?.content ?? "");
}`,
    }),
    [base]
  )

  const handleCopy = async () => {
    if (await copyToClipboard(snippets[lang])) {
      setCopied(true)
      setTimeout(() => setCopied(false), 1600)
    } else {
      toast.error(t('Copy failed. Please select and copy the text manually.'))
    }
  }

  const activePanelId = `iz-code-panel-${lang}`

  return (
    <section className='iz-block iz-block-alt' id='integrate'>
      <div className='iz-wrap'>
        <div className='iz-integrate-grid'>
          <AnimateInView animation='fade-up'>
            <header className='iz-integrate-head'>
              <span className='iz-index'>05 - {t('Integration')}</span>
              <h2>{t('Three lines apart, nothing more to learn.')}</h2>
              <p className='iz-section-desc'>
                {t(
                  'Use any SDK you already trust. Point the Base URL at the gateway, drop in your key, and ship. Streaming, tool calls and the Image API work exactly the same.'
                )}
              </p>
              <p className='iz-section-desc'>
                {t(
                  'Replace YOUR_KEY with your API key and YOUR_MODEL with a model available to your account.'
                )}
              </p>
            </header>
          </AnimateInView>

          <AnimateInView animation='fade-up' delay={120}>
            <div className='iz-code'>
              <div className='iz-code-bar'>
                <div
                  className='iz-code-tabs'
                  role='tablist'
                  aria-label={t('Code examples')}
                >
                  {LANGS.map((l) => (
                    <button
                      key={l}
                      id={`iz-code-tab-${l}`}
                      role='tab'
                      aria-controls={`iz-code-panel-${l}`}
                      aria-selected={lang === l}
                      tabIndex={lang === l ? 0 : -1}
                      onClick={() => setLang(l)}
                      onKeyDown={(event) => {
                        let next = LANGS.indexOf(l)
                        if (event.key === 'ArrowRight')
                          next = (next + 1) % LANGS.length
                        else if (event.key === 'ArrowLeft')
                          next = (next + LANGS.length - 1) % LANGS.length
                        else if (event.key === 'Home') next = 0
                        else if (event.key === 'End') next = LANGS.length - 1
                        else return
                        event.preventDefault()
                        const nextLang = LANGS[next]
                        setLang(nextLang)
                        document
                          .getElementById(`iz-code-tab-${nextLang}`)
                          ?.focus()
                      }}
                      className={`iz-code-tab ${lang === l ? 'is-active' : ''}`}
                      type='button'
                    >
                      {LANG_LABELS[l]}
                    </button>
                  ))}
                </div>
                <button
                  type='button'
                  onClick={handleCopy}
                  className='iz-code-copy'
                  aria-label={t('Copy snippet')}
                >
                  {copied ? (
                    <>
                      <Check className='size-3.5' /> {t('copied')}
                    </>
                  ) : (
                    <>
                      <Copy className='size-3.5' /> {t('copy')}
                    </>
                  )}
                </button>
              </div>
              <pre
                id={activePanelId}
                className='iz-code-body'
                role='tabpanel'
                aria-labelledby={`iz-code-tab-${lang}`}
                tabIndex={0}
              >
                <code>{snippets[lang]}</code>
              </pre>
              <div className='iz-code-foot'>
                <span className='iz-code-foot-dot' aria-hidden />
                <span className='iz-code-foot-txt'>
                  {t('Streaming, tool calls and image APIs work the same way.')}
                </span>
              </div>
            </div>
          </AnimateInView>
        </div>
      </div>
    </section>
  )
}
