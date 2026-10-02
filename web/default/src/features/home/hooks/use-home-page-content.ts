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
import { useEffect, useState } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import { getHomePageContent } from '../api'
import type { HomePageContentResult } from '../types'

const STORAGE_KEY = 'home_page_content'

/**
 * Hook to load and manage custom home page content
 * Supports both Markdown/HTML content and iframe URLs
 */
export function useHomePageContent(options?: {
  showErrorToast?: boolean
}): HomePageContentResult {
  const [content, setContent] = useState<string>('')
  const [isLoaded, setIsLoaded] = useState(false)
  const [hasError, setHasError] = useState(false)

  useEffect(() => {
    let mounted = true

    const loadContent = async () => {
      // A blocked browser cache must not prevent the network request.
      try {
        const cached = localStorage.getItem(STORAGE_KEY)
        if (cached && mounted) setContent(cached)
      } catch {
        // Storage is optional.
      }

      try {
        const response = await getHomePageContent()
        const { success, data } = response

        if (!mounted) return
        if (!success)
          throw new Error(
            response.message || 'Failed to load home page content'
          )

        setContent(data || '')
        setHasError(false)
        try {
          if (data) localStorage.setItem(STORAGE_KEY, data)
          else localStorage.removeItem(STORAGE_KEY)
        } catch {
          // Keep successfully loaded content even when storage is unavailable.
        }
      } catch (error) {
        if (!mounted) return
        setHasError(true)
        // eslint-disable-next-line no-console
        console.error('Failed to load home page content:', error)
        if (options?.showErrorToast !== false) {
          toast.error(i18next.t('Failed to load home page content'))
        }
      } finally {
        if (mounted) {
          setIsLoaded(true)
        }
      }
    }

    loadContent()

    return () => {
      mounted = false
    }
  }, [options?.showErrorToast])

  let isUrl = false
  try {
    const url = new URL(content)
    isUrl = url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    // not a URL
  }

  return { content, isLoaded, isUrl, hasError }
}
