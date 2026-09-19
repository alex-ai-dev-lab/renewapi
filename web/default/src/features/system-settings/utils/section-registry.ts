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
import type { ReactNode } from 'react'
import type { TFunction } from 'i18next'

/**
 * Section definition for settings pages
 */
export type SectionDefinition<TSettings, TExtraArgs extends unknown[] = []> = {
  id: string
  titleKey: string
  /**
   * Optional description key, surfaced by the Settings Catalog / Search.
   */
  descriptionKey?: string
  /**
   * Optional search aliases that cannot be derived from the title, id or
   * category, e.g. `['smtp', 'mail', '邮件']`. Keep these minimal — the
   * localized title, id and category title are already searchable.
   */
  keywords?: readonly string[]
  build: (settings: TSettings, ...extraArgs: TExtraArgs) => ReactNode
}

/**
 * Flat, resolved section metadata used by the Settings Catalog, Settings
 * Search and the QA route inventory. Derived from the section registry so
 * there is never a second source of truth for section id/title/url.
 */
export type SectionCatalogItem = {
  id: string
  title: string
  titleKey: string
  description?: string
  keywords: readonly string[]
  url: string
}

/**
 * Section registry configuration
 */
export type SectionRegistryConfig<
  TSectionId extends string,
  TSettings,
  TExtraArgs extends unknown[] = [],
> = {
  sections: readonly SectionDefinition<TSettings, TExtraArgs>[]
  defaultSection: TSectionId
  basePath: string
  /** 'query' = `${basePath}?section=${id}`, 'path' = `${basePath}/${id}` */
  urlStyle?: 'query' | 'path'
}

/**
 * Create a section registry with helper functions
 */
export function createSectionRegistry<
  TSectionId extends string,
  TSettings,
  TExtraArgs extends unknown[] = [],
>(config: SectionRegistryConfig<TSectionId, TSettings, TExtraArgs>) {
  const { sections, defaultSection, basePath, urlStyle = 'query' } = config

  type SectionId = TSectionId

  const sectionIds = sections.map((section) => section.id) as [
    SectionId,
    ...SectionId[],
  ]

  const buildSectionUrl = (id: string) =>
    urlStyle === 'path' ? `${basePath}/${id}` : `${basePath}?section=${id}`

  /**
   * Get navigation items for sidebar
   */
  function getSectionNavItems(t: TFunction) {
    return sections.map((section) => ({
      title: t(section.titleKey),
      url: buildSectionUrl(section.id),
    }))
  }

  /**
   * Get catalog items for the Settings Catalog / Settings Search / QA
   * inventory. Same `sections` source as the nav items and detail content.
   */
  function getSectionCatalogItems(t: TFunction): SectionCatalogItem[] {
    return sections.map((section) => ({
      id: section.id,
      title: t(section.titleKey),
      titleKey: section.titleKey,
      description: section.descriptionKey
        ? t(section.descriptionKey)
        : undefined,
      keywords: section.keywords ?? [],
      url: buildSectionUrl(section.id),
    }))
  }

  /**
   * Get section content by section ID
   */
  function getSectionContent(
    sectionId: SectionId,
    settings: TSettings,
    ...extraArgs: TExtraArgs
  ) {
    return getSectionMeta(sectionId).build(settings, ...extraArgs)
  }

  function getSectionMeta(sectionId: SectionId) {
    const section =
      sections.find((item) => item.id === sectionId) ?? sections[0]
    return section
  }

  return {
    sectionIds,
    defaultSection,
    basePath,
    getSectionNavItems,
    getSectionCatalogItems,
    getSectionContent,
    getSectionMeta,
  }
}
