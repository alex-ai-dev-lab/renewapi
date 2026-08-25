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
import type { TFunction } from 'i18next'
import {
  Activity,
  CreditCard,
  Cpu,
  Globe,
  KeyRound,
  LayoutDashboard,
  ShieldCheck,
  type LucideIcon,
} from 'lucide-react'
import { getAuthSectionCatalogItems } from './auth/section-registry'
import { getBillingSectionCatalogItems } from './billing/section-registry'
import { getContentSectionNavItems } from './content/section-registry'
import { getModelsSectionCatalogItems } from './models/section-registry'
import { getOperationsSectionCatalogItems } from './operations/section-registry'
import { getSecuritySectionCatalogItems } from './security/section-registry'
import { getSiteSectionCatalogItems } from './site/section-registry'
import type { SectionCatalogItem } from './utils/section-registry'


function getContentSectionCatalogItems(t: TFunction): SectionCatalogItem[] {
  return getContentSectionNavItems(t).map((item) => {
    const id = item.url.split('/').filter(Boolean).at(-1) ?? item.url
    return {
      id,
      title: item.title,
      titleKey: item.title,
      description: undefined,
      keywords: [],
      url: item.url,
    }
  })
}

export type SettingsCategoryId =
  | 'site'
  | 'auth'
  | 'billing'
  | 'models'
  | 'security'
  | 'operations'
  | 'content'

export type SettingsNavItem = {
  title: string
  url: string
}

export type SettingsCategoryDefinition = {
  id: SettingsCategoryId
  /** i18n key for the category title; falls back to titleZh / titleEn. */
  titleKey: string
  titleZh: string
  titleEn: string
  descriptionZh: string
  descriptionEn: string
  /** Base route path of the category, e.g. '/system-settings/operations'. */
  basePath: string
  icon: LucideIcon
  /**
   * Section items come straight from the category's section registry — the
   * single source of truth shared by detail pages, category nav, catalog,
   * search and the QA inventory. Never duplicate section id/title/url here.
   */
  getItems: (t: TFunction) => SectionCatalogItem[]
}

/**
 * The 7 settings categories. This file only maintains category-level
 * metadata (order, title, description, icon); every section entry is
 * derived from the section registries.
 */
export const SETTINGS_CATEGORIES: readonly SettingsCategoryDefinition[] = [
  {
    id: 'site',
    titleKey: 'settingsCatalog.category.site',
    titleZh: '网关与站点',
    titleEn: 'Gateway & Site',
    descriptionZh: '网关地址、站点信息与公告',
    descriptionEn: 'Gateway address, site info and notices',
    basePath: '/system-settings/site',
    icon: Globe,
    getItems: getSiteSectionCatalogItems,
  },
  {
    id: 'auth',
    titleKey: 'settingsCatalog.category.auth',
    titleZh: '身份与认证',
    titleEn: 'Identity & Authentication',
    descriptionZh: '登录、OAuth、Passkey 与机器人防护',
    descriptionEn: 'Login, OAuth, passkeys and bot protection',
    basePath: '/system-settings/auth',
    icon: KeyRound,
    getItems: getAuthSectionCatalogItems,
  },
  {
    id: 'billing',
    titleKey: 'settingsCatalog.category.billing',
    titleZh: '计费与额度',
    titleEn: 'Billing & Quota',
    descriptionZh: '额度、定价、支付与签到奖励',
    descriptionEn: 'Quota, pricing, payments and check-in rewards',
    basePath: '/system-settings/billing',
    icon: CreditCard,
    getItems: getBillingSectionCatalogItems,
  },
  {
    id: 'models',
    titleKey: 'settingsCatalog.category.models',
    titleZh: '模型与路由',
    titleEn: 'Models & Routing',
    descriptionZh: '全局模型配置、厂商适配与路由规则',
    descriptionEn: 'Global model config, providers and routing rules',
    basePath: '/system-settings/models',
    icon: Cpu,
    getItems: getModelsSectionCatalogItems,
  },
  {
    id: 'security',
    titleKey: 'settingsCatalog.category.security',
    titleZh: '安全与请求控制',
    titleEn: 'Security & Request Control',
    descriptionZh: '限流、SSRF、防投毒与请求守卫',
    descriptionEn: 'Rate limiting, SSRF, anti-poison and request guards',
    basePath: '/system-settings/security',
    icon: ShieldCheck,
    getItems: getSecuritySectionCatalogItems,
  },
  {
    id: 'operations',
    titleKey: 'settingsCatalog.category.operations',
    titleZh: '运维与监控',
    titleEn: 'Operations & Monitoring',
    descriptionZh: '邮件、监控、日志与系统维护',
    descriptionEn: 'Email, monitoring, logs and maintenance',
    basePath: '/system-settings/operations',
    icon: Activity,
    getItems: getOperationsSectionCatalogItems,
  },
  {
    id: 'content',
    titleKey: 'settingsCatalog.category.content',
    titleZh: '控制台与内容',
    titleEn: 'Console & Content',
    descriptionZh: '仪表盘、外观、公告与导航自定义',
    descriptionEn: 'Dashboard, appearance, announcements and navigation',
    basePath: '/system-settings/content',
    icon: LayoutDashboard,
    getItems: getContentSectionCatalogItems,
  },
]

export function getSettingsCategory(id: SettingsCategoryId) {
  return SETTINGS_CATEGORIES.find((category) => category.id === id)
}

export function getSettingsSectionCount(t: TFunction) {
  return SETTINGS_CATEGORIES.reduce(
    (total, category) => total + category.getItems(t).length,
    0
  )
}

/* -------------------------------------------------------------------------- */
/* Settings Search                                                             */
/* -------------------------------------------------------------------------- */

export type SettingsSearchEntry = {
  categoryId: SettingsCategoryId
  categoryTitle: string
  id: string
  title: string
  titleKey: string
  description?: string
  keywords: readonly string[]
  url: string
}

/**
 * Build the search corpus from the catalog (which in turn comes from the
 * section registries). Never hand-maintain a separate list of sections for
 * search — if it is not in a registry, it is not searchable.
 */
export function buildSettingsSearchEntries(
  t: TFunction,
  options?: { isChinese?: boolean }
): SettingsSearchEntry[] {
  return SETTINGS_CATEGORIES.flatMap((category) => {
    const categoryTitle = t(category.titleKey, {
      defaultValue: options?.isChinese ? category.titleZh : category.titleEn,
    })
    return category.getItems(t).map((item) => ({
      categoryId: category.id,
      categoryTitle,
      id: item.id,
      title: item.title,
      titleKey: item.titleKey,
      description: item.description,
      keywords: item.keywords,
      url: item.url,
    }))
  })
}

/**
 * Normalize free-text search input: trim, case-insensitive, and treat
 * spaces / hyphens / underscores as equivalent for latin text. Chinese
 * terms keep working via plain substring matching.
 */
export function normalizeSettingsSearchText(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[\s\-_]+/g, ' ')
}

/**
 * Filter search entries. Every whitespace-separated query term must appear
 * in the entry corpus (title + id + category title + keywords +
 * description, with the route url as a low-weight supplement).
 */
export function filterSettingsEntries(
  entries: readonly SettingsSearchEntry[],
  query: string
): SettingsSearchEntry[] {
  const normalizedQuery = normalizeSettingsSearchText(query)
  if (!normalizedQuery) return []
  const terms = normalizedQuery.split(' ')
  return entries.filter((entry) => {
    const corpus = normalizeSettingsSearchText(
      [
        entry.title,
        entry.id,
        entry.categoryTitle,
        entry.description ?? '',
        ...entry.keywords,
        entry.url,
      ].join('\n')
    )
    return terms.every((term) => corpus.includes(term))
  })
}
