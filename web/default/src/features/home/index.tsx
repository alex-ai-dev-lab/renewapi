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
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { Markdown } from '@/components/ui/markdown'
import { PublicLayout } from '@/components/layout'
import { IzClosing } from './components/sections/iz-closing'
import { IzFaq } from './components/sections/iz-faq'
import { IzFooter } from './components/sections/iz-footer'
import { IzHeader } from './components/sections/iz-header'
import { IzHero } from './components/sections/iz-hero'
import { IzLive } from './components/sections/iz-live'
import { IzModels } from './components/sections/iz-models'
import { IzPillars } from './components/sections/iz-pillars'
import { IzProtocols } from './components/sections/iz-protocols'
import { IzQuickstart } from './components/sections/iz-quickstart'
import { IzRouting } from './components/sections/iz-routing'
import { useHomePageContent } from './hooks'

const HOME_SEO = {
  description:
    'A unified, reliable, high-speed AI API gateway for OpenAI, Claude, Gemini, images, embeddings, routing, failover, and retries.',
  themeColor: '#0D0D10',
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

function useInterfaceZeroSeo(enabled: boolean, systemName: string) {
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
  const { auth } = useAuthStore()
  const { status } = useStatus()
  const { systemName } = useSystemConfig()
  const registrationEnabled = status?.register_enabled === true
  const docsLink = documentationLink(status?.docs_link)
  const isAuthenticated = !!auth.user
  const { content, isLoaded, isUrl } = useHomePageContent({
    showErrorToast: false,
  })
  useInterfaceZeroSeo(isLoaded && !content, systemName)

  if (!isLoaded) {
    return (
      <PublicLayout showMainContainer={false}>
        <main className='flex min-h-screen items-center justify-center'>
          <div className='text-muted-foreground'>{t('Loading...')}</div>
        </main>
      </PublicLayout>
    )
  }

  if (content) {
    return (
      <PublicLayout showMainContainer={false}>
        <main className='overflow-x-hidden'>
          {isUrl ? (
            <iframe
              src={content}
              className='h-screen w-full border-none'
              title={t('Custom Home Page')}
            />
          ) : (
            <div className='bg-background text-foreground min-h-screen'>
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
      showHeader={false}
      skipLinkTarget='#home-main-content'
    >
      <IzHeader
        isAuthenticated={isAuthenticated}
        registrationEnabled={registrationEnabled}
        systemName={systemName}
        docsLink={docsLink}
      />
      <main
        id='home-main-content'
        tabIndex={-1}
        className='iz-root outline-none'
      >
        <IzHero
          isAuthenticated={isAuthenticated}
          registrationEnabled={registrationEnabled}
        />
        <IzModels />
        <IzPillars />
        <IzRouting systemName={systemName} />
        <IzLive />
        <IzProtocols />
        <IzQuickstart />
        <IzFaq />
        <IzClosing
          isAuthenticated={isAuthenticated}
          registrationEnabled={registrationEnabled}
        />
      </main>
      <IzFooter
        systemName={systemName}
        docsLink={docsLink}
        termsEnabled={status?.user_agreement_enabled === true}
        privacyEnabled={status?.privacy_policy_enabled === true}
      />
    </PublicLayout>
  )
}
