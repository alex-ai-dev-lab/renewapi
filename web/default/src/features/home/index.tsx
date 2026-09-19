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
  title: 'Interface Zero - Unified AI Gateway',
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

function useInterfaceZeroSeo(enabled: boolean) {
  useEffect(() => {
    if (!enabled) return

    const previousTitle = document.title
    const canonical = window.location.origin + window.location.pathname
    document.title = HOME_SEO.title

    const cleanup = [
      upsertMeta('name', 'title', HOME_SEO.title),
      upsertMeta('name', 'description', HOME_SEO.description),
      upsertMeta('name', 'theme-color', HOME_SEO.themeColor),
      upsertMeta('property', 'og:title', HOME_SEO.title),
      upsertMeta('property', 'og:description', HOME_SEO.description),
      upsertMeta('property', 'og:type', 'website'),
      upsertMeta('property', 'og:url', canonical),
      upsertMeta('name', 'twitter:card', 'summary_large_image'),
      upsertMeta('name', 'twitter:title', HOME_SEO.title),
      upsertMeta('name', 'twitter:description', HOME_SEO.description),
      upsertCanonical(canonical),
    ]

    return () => {
      document.title = previousTitle
      cleanup.forEach((restore) => restore())
    }
  }, [enabled])
}

export function Home() {
  const { t } = useTranslation()
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user
  const { content, isLoaded, isUrl } = useHomePageContent({
    showErrorToast: false,
  })
  useInterfaceZeroSeo(isLoaded && !content)

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
    <PublicLayout showMainContainer={false} showHeader={false}>
      <IzHeader isAuthenticated={isAuthenticated} />
      <main id='main-content' className='iz-root'>
        <IzHero isAuthenticated={isAuthenticated} />
        <IzModels />
        <IzPillars />
        <IzRouting />
        <IzLive />
        <IzProtocols />
        <IzQuickstart />
        <IzFaq />
        <IzClosing isAuthenticated={isAuthenticated} />
      </main>
      <IzFooter />
    </PublicLayout>
  )
}
