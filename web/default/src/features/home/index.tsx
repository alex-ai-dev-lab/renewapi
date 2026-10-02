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
import { useEffect } from 'react'
import '@/styles/obsidian-home.css'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { parseHeaderNavModulesFromStatus } from '@/lib/nav-modules'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Markdown } from '@/components/ui/markdown'
import { PublicLayout } from '@/components/layout'
import { ObsidianHome } from './components/obsidian-home'
import { useHomePageContent } from './hooks'
import { resolveApiBase } from './lib/request-example'

const HOME_SEO = {
  description:
    'A unified, reliable, high-speed AI API gateway for OpenAI, Claude, Gemini, images, embeddings, routing, failover, and retries.',
  themeColor: '#101213',
}

function upsertMeta(
  attribute: 'name' | 'property',
  key: string,
  content: string
) {
  const selector = `meta[${attribute}="${key}"]`
  let element = document.querySelector(selector) as HTMLMetaElement | null
  const created = !element

  if (!element) {
    element = document.createElement('meta')
    element.setAttribute(attribute, key)
    document.head.appendChild(element)
  }

  const previous = element.getAttribute('content')
  element.setAttribute('content', content)

  return () => {
    if (created) {
      element.remove()
    } else if (previous === null) {
      element.removeAttribute('content')
    } else {
      element.setAttribute('content', previous)
    }
  }
}

function upsertCanonical(href: string) {
  let element = document.querySelector(
    'link[rel="canonical"]'
  ) as HTMLLinkElement | null
  const created = !element

  if (!element) {
    element = document.createElement('link')
    element.setAttribute('rel', 'canonical')
    document.head.appendChild(element)
  }

  const previous = element.getAttribute('href')
  element.setAttribute('href', href)

  return () => {
    if (created) {
      element.remove()
    } else if (previous === null) {
      element.removeAttribute('href')
    } else {
      element.setAttribute('href', previous)
    }
  }
}

function useHomeSeo(enabled: boolean, systemName: string) {
  const { t } = useTranslation()
  useEffect(() => {
    if (!enabled) return

    const title = `${systemName} - ${t('Unified AI Gateway')}`
    const description = t(HOME_SEO.description)
    const previousTitle = document.title
    const canonical = window.location.origin + window.location.pathname
    document.title = title

    const cleanup = [
      upsertMeta('name', 'title', title),
      upsertMeta('name', 'description', description),
      upsertMeta('name', 'theme-color', HOME_SEO.themeColor),
      upsertMeta('property', 'og:title', title),
      upsertMeta('property', 'og:description', description),
      upsertMeta('property', 'og:type', 'website'),
      upsertMeta('property', 'og:url', canonical),
      upsertMeta('name', 'twitter:card', 'summary_large_image'),
      upsertMeta('name', 'twitter:title', title),
      upsertMeta('name', 'twitter:description', description),
      upsertCanonical(canonical),
    ]

    return () => {
      document.title = previousTitle
      cleanup.forEach((restore) => restore())
    }
  }, [enabled, systemName, t])
}

function documentationLink(value: unknown): string {
  const fallback = 'https://github.com/alex-ai-dev-lab/renewapi#readme'
  if (typeof value !== 'string' || !value.trim()) return fallback
  try {
    const url = new URL(
      value,
      typeof window === 'undefined' ? fallback : window.location.origin
    )
    if (
      !['http:', 'https:'].includes(url.protocol) ||
      url.username ||
      url.password
    )
      return fallback
    return url.href
  } catch {
    return fallback
  }
}

export function Home() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const { status } = useStatus()
  const { systemName, footerHtml } = useSystemConfig()
  const registrationEnabled =
    status?.register_enabled === true && !status?.self_use_mode_enabled
  const docsLink = documentationLink(status?.docs_link)
  const navModules = parseHeaderNavModulesFromStatus(status)
  const apiBase = resolveApiBase(status?.server_address, window.location.origin)
  const isAuthenticated = !!user
  const { content, isLoaded, isUrl, hasError } = useHomePageContent({
    showErrorToast: false,
  })
  useHomeSeo(isLoaded && !content, systemName)
  const contentWarning = hasError && (
    <p role='status' className='obsidian-home-load-error'>
      {t(
        'Custom home content could not be loaded. Cached content is shown when available.'
      )}
    </p>
  )

  if (!isLoaded) {
    return (
      <PublicLayout showMainContainer={false} showNotifications={false}>
        <main className='obsidian-home-content-state flex items-center justify-center'>
          <div role='status' className='text-muted-foreground'>
            {t('Loading...')}
          </div>
        </main>
      </PublicLayout>
    )
  }

  if (content) {
    return (
      <PublicLayout showMainContainer={false} showNotifications={false}>
        <main className='obsidian-home-custom'>
          {contentWarning}
          {isUrl ? (
            <iframe src={content} title={t('Custom Home Page')} />
          ) : (
            <div className='bg-background text-foreground'>
              <div className='container mx-auto px-4 py-8'>
                <Markdown className='custom-home-content'>{content}</Markdown>
              </div>
            </div>
          )}
        </main>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout
      showMainContainer={false}
      showNotifications={false}
      skipLinkTarget='#home-main-content'
    >
      {contentWarning}
      <ObsidianHome
        systemName={systemName}
        apiBase={apiBase}
        docsLink={docsLink}
        docsEnabled={navModules.docs !== false}
        aboutEnabled={navModules.about !== false}
        isAuthenticated={isAuthenticated}
        registrationEnabled={registrationEnabled}
        termsEnabled={status?.user_agreement_enabled === true}
        privacyEnabled={status?.privacy_policy_enabled === true}
        footerHtml={footerHtml}
      />
    </PublicLayout>
  )
}
