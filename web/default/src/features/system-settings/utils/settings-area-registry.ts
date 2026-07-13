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
import type { ElementType } from 'react'
import type { TFunction } from 'i18next'
import {
  Box,
  CreditCard,
  Layout,
  Settings,
  Shield,
  ShieldAlert,
  Wrench,
} from 'lucide-react'
import { getAuthSectionNavItems } from '../auth/section-registry'
import { getBillingSectionNavItems } from '../billing/section-registry'
import { getContentSectionNavItems } from '../content/section-registry'
import { getModelsSectionNavItems } from '../models/section-registry'
import { getOperationsSectionNavItems } from '../operations/section-registry'
import { getSecuritySectionNavItems } from '../security/section-registry'
import { getSiteSectionNavItems } from '../site/section-registry'
import type { SettingsNavItem } from './section-registry'

export type SettingsAreaId =
  | 'site'
  | 'auth'
  | 'billing'
  | 'models'
  | 'security'
  | 'content'
  | 'operations'

export type SettingsAreaDefinition = {
  id: SettingsAreaId
  titleKey: string
  descriptionKey: string
  icon: ElementType
  getItems: (t: TFunction) => SettingsNavItem[]
}

export const SETTINGS_AREA_REGISTRY = [
  {
    id: 'site',
    titleKey: 'Site & Branding',
    descriptionKey: 'Site identity, branding, notices, and public links',
    icon: Settings,
    getItems: getSiteSectionNavItems,
  },
  {
    id: 'auth',
    titleKey: 'Access & Identity',
    descriptionKey: 'Login methods, OAuth, Passkeys, and access policy',
    icon: Shield,
    getItems: getAuthSectionNavItems,
  },
  {
    id: 'billing',
    titleKey: 'Billing & Payment',
    descriptionKey: 'Quota, pricing, payment, subscriptions, and check-in',
    icon: CreditCard,
    getItems: getBillingSectionNavItems,
  },
  {
    id: 'models',
    titleKey: 'Models & Routing',
    descriptionKey: 'Model behavior, error rules, affinity, and deployment',
    icon: Box,
    getItems: getModelsSectionNavItems,
  },
  {
    id: 'security',
    titleKey: 'Security & Risk',
    descriptionKey: 'Rate limits, SSRF, sensitive words, and anti-poison',
    icon: ShieldAlert,
    getItems: getSecuritySectionNavItems,
  },
  {
    id: 'content',
    titleKey: 'Console & Content',
    descriptionKey: 'Console appearance, announcements, chat, and API info',
    icon: Layout,
    getItems: getContentSectionNavItems,
  },
  {
    id: 'operations',
    titleKey: 'Operations',
    descriptionKey: 'Monitoring, testing, email, logs, and performance',
    icon: Wrench,
    getItems: getOperationsSectionNavItems,
  },
] as const satisfies readonly SettingsAreaDefinition[]

export function getSettingsAreas(t: TFunction) {
  return SETTINGS_AREA_REGISTRY.map((area) => ({
    id: area.id,
    title: t(area.titleKey),
    description: t(area.descriptionKey),
    icon: area.icon,
    items: area.getItems(t),
  }))
}
