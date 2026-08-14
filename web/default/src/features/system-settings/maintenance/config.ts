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
export type HeaderNavAccessConfig = {
  enabled: boolean
  requireAuth: boolean
}

export type HeaderNavModulesConfig = {
  order?: string[]
  home: boolean
  console: boolean
  pricing: HeaderNavAccessConfig
  rankings: HeaderNavAccessConfig
  docs: boolean
  about: boolean
  [key: string]: boolean | HeaderNavAccessConfig | string[] | undefined
}

export type SidebarSectionConfig = {
  enabled: boolean
  order?: string[]
  [key: string]: boolean | string[] | undefined
}

export type SidebarModulesAdminConfig = Record<string, SidebarSectionConfig>

export type SystemSettingsAreaConfig = {
  enabled: boolean
  order?: string[]
  sections?: Record<string, boolean>
}

export type SystemSettingsNavigationConfig = {
  order: string[]
  areas: Record<string, SystemSettingsAreaConfig>
}

export const SIDEBAR_SECTION_ORDER_DEFAULT = [
  'chat',
  'console',
  'personal',
  'admin',
]

export const SYSTEM_SETTINGS_AREA_ORDER_DEFAULT = [
  'site',
  'auth',
  'models',
  'security',
  'billing',
  'operations',
  'content',
]

export const SYSTEM_SETTINGS_SECTION_ORDER_DEFAULT: Record<string, string[]> = {
  site: ['system-info', 'notice'],
  auth: ['basic-auth', 'oauth', 'passkey', 'bot-protection', 'custom-oauth'],
  models: [
    'overview',
    'global',
    'gemini',
    'claude',
    'grok',
    'channel-affinity',
    'model-deployment',
    'header-rules',
    'upstream-error-rules',
  ],
  security: [
    'rate-limit',
    'sensitive-words',
    'ssrf',
    'anti-poison-guard',
    'request-guard',
    'client-identity',
    'user-agents',
  ],
  billing: [
    'quota',
    'currency',
    'model-pricing',
    'group-pricing',
    'payment',
    'checkin',
  ],
  operations: [
    'email',
    'worker',
    'monitoring',
    'channel-test',
    'uptime-kuma',
    'behavior',
    'logs',
    'performance',
    'update-checker',
  ],
  content: [
    'appearance',
    'header-navigation',
    'sidebar-modules',
    'settings-navigation',
    'dashboard',
    'announcements',
    'api-info',
    'faq',
    'chat',
    'drawing',
  ],
}

export const HEADER_NAV_ORDER_DEFAULT = [
  'home',
  'console',
  'pricing',
  'rankings',
  'docs',
  'about',
]

export const HEADER_NAV_DEFAULT: HeaderNavModulesConfig = {
  order: [...HEADER_NAV_ORDER_DEFAULT],
  home: true,
  console: true,
  pricing: {
    enabled: true,
    requireAuth: false,
  },
  rankings: {
    enabled: true,
    requireAuth: false,
  },
  docs: true,
  about: true,
}

export const SIDEBAR_MODULES_DEFAULT: SidebarModulesAdminConfig = {
  chat: {
    enabled: true,
    order: ['playground', 'chat'],
    playground: true,
    chat: true,
  },
  console: {
    enabled: true,
    order: ['detail', 'token', 'log', 'midjourney', 'task'],
    detail: true,
    token: true,
    log: true,
    midjourney: true,
    task: true,
  },
  personal: {
    enabled: true,
    order: ['topup', 'personal'],
    topup: true,
    personal: true,
  },
  admin: {
    enabled: true,
    order: [
      'channel',
      'models',
      'redemption',
      'user',
      'setting',
      'subscription',
    ],
    channel: true,
    models: true,
    redemption: true,
    user: true,
    setting: true,
    subscription: true,
  },
}

const toBoolean = (value: unknown, fallback: boolean): boolean => {
  if (typeof value === 'boolean') return value
  if (typeof value === 'number') return value === 1
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (normalized === 'true' || normalized === '1') return true
    if (normalized === 'false' || normalized === '0') return false
  }
  return fallback
}

const cloneHeaderNavDefault = (): HeaderNavModulesConfig => ({
  ...HEADER_NAV_DEFAULT,
  order: [...HEADER_NAV_ORDER_DEFAULT],
  pricing: { ...HEADER_NAV_DEFAULT.pricing },
  rankings: { ...HEADER_NAV_DEFAULT.rankings },
})

const parseAccessModule = (
  raw: unknown,
  fallback: HeaderNavAccessConfig
): HeaderNavAccessConfig => {
  if (
    typeof raw === 'boolean' ||
    typeof raw === 'string' ||
    typeof raw === 'number'
  ) {
    return {
      enabled: toBoolean(raw, fallback.enabled),
      requireAuth: fallback.requireAuth,
    }
  }
  if (raw && typeof raw === 'object') {
    const record = raw as Record<string, unknown>
    return {
      enabled: toBoolean(record.enabled, fallback.enabled),
      requireAuth: toBoolean(record.requireAuth, fallback.requireAuth),
    }
  }
  return { ...fallback }
}

const cloneSidebarDefault = (): SidebarModulesAdminConfig =>
  Object.entries(SIDEBAR_MODULES_DEFAULT).reduce<SidebarModulesAdminConfig>(
    (acc, [section, config]) => {
      acc[section] = {
        ...config,
        order: Array.isArray(config.order) ? [...config.order] : undefined,
      }
      return acc
    },
    {}
  )

const cloneSystemSettingsNavigationDefault =
  (): SystemSettingsNavigationConfig => ({
    order: [...SYSTEM_SETTINGS_AREA_ORDER_DEFAULT],
    areas: Object.fromEntries(
      Object.entries(SYSTEM_SETTINGS_SECTION_ORDER_DEFAULT).map(
        ([area, order]) => [
          area,
          {
            enabled: true,
            order: [...order],
            sections: Object.fromEntries(
              order.map((section) => [section, true])
            ),
          },
        ]
      )
    ),
  })

export function getSidebarModuleKeys(
  sectionConfig: SidebarSectionConfig
): string[] {
  return Object.entries(sectionConfig)
    .filter(
      ([moduleKey, moduleValue]) =>
        moduleKey !== 'enabled' &&
        moduleKey !== 'order' &&
        typeof moduleValue === 'boolean'
    )
    .map(([moduleKey]) => moduleKey)
}

export function getSidebarSectionOrder(
  sectionConfig: SidebarSectionConfig
): string[] {
  const moduleKeys = getSidebarModuleKeys(sectionConfig)
  const moduleKeySet = new Set(moduleKeys)
  const seen = new Set<string>()
  const configuredOrder = Array.isArray(sectionConfig.order)
    ? sectionConfig.order
    : []

  const ordered = configuredOrder.filter((moduleKey) => {
    if (!moduleKeySet.has(moduleKey) || seen.has(moduleKey)) return false
    seen.add(moduleKey)
    return true
  })

  moduleKeys.forEach((moduleKey) => {
    if (!seen.has(moduleKey)) {
      ordered.push(moduleKey)
      seen.add(moduleKey)
    }
  })

  return ordered
}

function normalizeSidebarSectionConfig(
  sectionConfig: SidebarSectionConfig
): SidebarSectionConfig {
  return {
    ...sectionConfig,
    order: getSidebarSectionOrder(sectionConfig),
  }
}

function normalizeSidebarModulesAdmin(
  config: SidebarModulesAdminConfig
): SidebarModulesAdminConfig {
  return Object.entries(config).reduce<SidebarModulesAdminConfig>(
    (acc, [sectionKey, sectionConfig]) => {
      acc[sectionKey] = normalizeSidebarSectionConfig(sectionConfig)
      return acc
    },
    {}
  )
}

export function parseSidebarSectionOrder(
  value: string | null | undefined,
  availableSections = SIDEBAR_SECTION_ORDER_DEFAULT
): string[] {
  const sectionSet = new Set(availableSections)
  const seen = new Set<string>()
  const configuredOrder = value
    ? value
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean)
    : []

  const ordered = configuredOrder.filter((sectionKey) => {
    if (!sectionSet.has(sectionKey) || seen.has(sectionKey)) return false
    seen.add(sectionKey)
    return true
  })

  availableSections.forEach((sectionKey) => {
    if (!seen.has(sectionKey)) {
      ordered.push(sectionKey)
      seen.add(sectionKey)
    }
  })

  return ordered
}

export function serializeSidebarSectionOrder(
  order: string[],
  availableSections = SIDEBAR_SECTION_ORDER_DEFAULT
): string {
  return parseSidebarSectionOrder(order.join(','), availableSections).join(',')
}

export function normalizeHeaderNavOrder(
  value: unknown,
  availableModules = HEADER_NAV_ORDER_DEFAULT
): string[] {
  const moduleSet = new Set(availableModules)
  const seen = new Set<string>()
  const configuredOrder = Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string')
    : []

  const ordered = configuredOrder.filter((moduleKey) => {
    if (!moduleSet.has(moduleKey) || seen.has(moduleKey)) return false
    seen.add(moduleKey)
    return true
  })

  availableModules.forEach((moduleKey) => {
    if (!seen.has(moduleKey)) {
      ordered.push(moduleKey)
      seen.add(moduleKey)
    }
  })

  return ordered
}

export function parseHeaderNavModules(
  value: string | null | undefined
): HeaderNavModulesConfig {
  const base = cloneHeaderNavDefault()
  if (!value) {
    return base
  }
  try {
    const parsed = JSON.parse(value) as Record<string, unknown>
    const result: HeaderNavModulesConfig = {
      ...base,
      order: normalizeHeaderNavOrder(parsed.order),
      pricing: { ...base.pricing },
      rankings: { ...base.rankings },
    }

    Object.entries(parsed).forEach(([key, raw]) => {
      if (key === 'order') return
      if (key === 'pricing') {
        result.pricing = parseAccessModule(raw, base.pricing)
        return
      }
      if (key === 'rankings') {
        result.rankings = parseAccessModule(raw, base.rankings)
        return
      }

      if (typeof raw === 'boolean') {
        result[key] = raw
        return
      }
      if (typeof raw === 'string' || typeof raw === 'number') {
        result[key] = toBoolean(raw, Boolean(base[key]))
        return
      }
    })

    return result
  } catch {
    return base
  }
}

export function serializeHeaderNavModules(
  config: HeaderNavModulesConfig
): string {
  return JSON.stringify({
    ...config,
    order: normalizeHeaderNavOrder(config.order),
  })
}

export function parseSidebarModulesAdmin(
  value: string | null | undefined
): SidebarModulesAdminConfig {
  const defaults = cloneSidebarDefault()
  // If empty string, null, or undefined, use default config
  if (!value || value.trim() === '') return defaults

  try {
    const parsed = JSON.parse(value) as Record<string, unknown>
    const result: SidebarModulesAdminConfig = {}

    Object.entries(parsed).forEach(([sectionKey, raw]) => {
      if (!raw || typeof raw !== 'object') return

      const defaultSection = defaults[sectionKey] ?? { enabled: true }
      const sectionConfig: SidebarSectionConfig = {
        enabled: toBoolean(
          (raw as Record<string, unknown>).enabled,
          defaultSection.enabled ?? true
        ),
      }
      const rawOrder = (raw as Record<string, unknown>).order
      if (Array.isArray(rawOrder)) {
        sectionConfig.order = rawOrder.filter(
          (item): item is string => typeof item === 'string'
        )
      }

      Object.entries(raw as Record<string, unknown>).forEach(
        ([moduleKey, moduleValue]) => {
          if (moduleKey === 'enabled' || moduleKey === 'order') return
          sectionConfig[moduleKey] = toBoolean(
            moduleValue,
            typeof defaultSection[moduleKey] === 'boolean'
              ? defaultSection[moduleKey]
              : true
          )
        }
      )

      result[sectionKey] = sectionConfig
    })

    // Merge defaults to ensure expected sections exist
    Object.entries(defaults).forEach(([sectionKey, config]) => {
      if (!result[sectionKey]) {
        result[sectionKey] = { ...config }
        return
      }

      Object.entries(config).forEach(([moduleKey, moduleValue]) => {
        if (!(moduleKey in result[sectionKey])) {
          result[sectionKey][moduleKey] = moduleValue
        }
      })
    })

    return normalizeSidebarModulesAdmin(result)
  } catch {
    return defaults
  }
}

export function serializeSidebarModulesAdmin(
  config: SidebarModulesAdminConfig
): string {
  return JSON.stringify(normalizeSidebarModulesAdmin(config))
}

function normalizeStringOrder(
  value: unknown,
  availableItems: readonly string[]
): string[] {
  const itemSet = new Set(availableItems)
  const seen = new Set<string>()
  const configuredOrder = Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string')
    : []

  const ordered = configuredOrder.filter((item) => {
    if (!itemSet.has(item) || seen.has(item)) return false
    seen.add(item)
    return true
  })

  availableItems.forEach((item) => {
    if (!seen.has(item)) {
      ordered.push(item)
      seen.add(item)
    }
  })

  return ordered
}

export function getSystemSettingsAreaOrder(
  config: SystemSettingsNavigationConfig
): string[] {
  return normalizeStringOrder(config.order, Object.keys(config.areas))
}

export function getSystemSettingsSectionOrder(
  config: SystemSettingsNavigationConfig,
  area: string
): string[] {
  return normalizeStringOrder(
    config.areas[area]?.order,
    SYSTEM_SETTINGS_SECTION_ORDER_DEFAULT[area] ?? []
  )
}

export function parseSystemSettingsNavigation(
  value: string | null | undefined
): SystemSettingsNavigationConfig {
  const defaults = cloneSystemSettingsNavigationDefault()
  if (!value || value.trim() === '') return defaults

  try {
    const parsed = JSON.parse(value) as Record<string, unknown>
    const rawAreas =
      parsed.areas && typeof parsed.areas === 'object'
        ? (parsed.areas as Record<string, unknown>)
        : {}
    const areas = Object.entries(defaults.areas).reduce<
      SystemSettingsNavigationConfig['areas']
    >((acc, [area, fallback]) => {
      const rawArea = rawAreas[area]
      if (!rawArea || typeof rawArea !== 'object') {
        acc[area] = {
          enabled: fallback.enabled,
          order: [...(fallback.order ?? [])],
          sections: { ...(fallback.sections ?? {}) },
        }
        return acc
      }
      const record = rawArea as Record<string, unknown>
      const rawSections =
        record.sections && typeof record.sections === 'object'
          ? (record.sections as Record<string, unknown>)
          : {}
      acc[area] = {
        enabled: toBoolean(record.enabled, fallback.enabled),
        order: normalizeStringOrder(record.order, fallback.order ?? []),
        sections: Object.fromEntries(
          (fallback.order ?? []).map((section) => [
            section,
            toBoolean(rawSections[section], true),
          ])
        ),
      }
      return acc
    }, {})

    return {
      order: normalizeStringOrder(parsed.order, defaults.order),
      areas,
    }
  } catch {
    return defaults
  }
}

export function serializeSystemSettingsNavigation(
  config: SystemSettingsNavigationConfig
): string {
  const defaults = cloneSystemSettingsNavigationDefault()
  const areas = Object.entries(defaults.areas).reduce<
    SystemSettingsNavigationConfig['areas']
  >((acc, [area, fallback]) => {
    const current = config.areas[area] ?? fallback
    acc[area] = {
      enabled: toBoolean(current.enabled, fallback.enabled),
      order: normalizeStringOrder(current.order, fallback.order ?? []),
      sections: Object.fromEntries(
        (fallback.order ?? []).map((section) => [
          section,
          toBoolean(current.sections?.[section], true),
        ])
      ),
    }
    return acc
  }, {})
  return JSON.stringify({
    order: normalizeStringOrder(config.order, defaults.order),
    areas,
  })
}
