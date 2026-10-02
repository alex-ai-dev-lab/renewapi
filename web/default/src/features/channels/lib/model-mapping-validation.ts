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
// ============================================================================
// Model Mapping Validation Utilities
// ============================================================================

/**
 * Parse models string to array
 */
export function parseModelsString(modelsStr: string): string[] {
  return modelsStr
    ? modelsStr
        .split(',')
        .map((m) => m.trim())
        .filter(Boolean)
    : []
}

/**
 * Format models array to string
 */
export function formatModelsArray(models: string[]): string {
  return Array.from(new Set(models)).join(',')
}

/**
 * Normalize model name
 */
export function normalizeModelName(model: string): string {
  return typeof model === 'string' ? model.trim() : ''
}

export type ModelMappingRule = {
  id: string
  from: string
  to: string
  priority: number
  enabled: boolean
}
export type ModelMappingConfig =
  | { version: 1; legacy: Record<string, string[]> }
  | { version: 2; rules: ModelMappingRule[] }
export const EMPTY_RULE_MAPPING = '{"version":2,"rules":[]}'
const fail = (message: string): never => {
  throw new Error(message)
}
const isObject = (value: unknown): value is Record<string, unknown> =>
  !!value && typeof value === 'object' && !Array.isArray(value)
function name(value: unknown, max = 255): string {
  if (typeof value !== 'string' || !value.trim())
    return fail('Mapping fields must be non-empty strings')
  if (Array.from(value.trim()).length > max)
    return fail('Mapping field exceeds character limit')
  return value.trim()
}

export function parseModelMappingConfig(raw: string): ModelMappingConfig {
  if (new TextEncoder().encode(raw).length > 1048576)
    return fail('Model mapping exceeds 1 MiB')
  if (!raw.trim()) return { version: 1, legacy: Object.create(null) }
  let value: unknown
  try {
    value = JSON.parse(raw)
  } catch {
    return fail('Model mapping must be valid JSON format')
  }
  if (!isObject(value)) return fail('Model mapping must be a valid JSON object')
  if (typeof value.version === 'number') {
    if (value.version !== 2) return fail('Unsupported model mapping version')
    if (Object.keys(value).length !== 2 || !Array.isArray(value.rules))
      return fail('Model mapping v2 requires only version and rules')
    if (value.rules.length > 1024)
      return fail('Model mapping exceeds 1024 rules or sources')
    const ids = new Set<string>()
    const rules = value.rules.map((item): ModelMappingRule => {
      if (
        !isObject(item) ||
        Object.keys(item).some(
          (k) => !['id', 'from', 'to', 'priority', 'enabled'].includes(k)
        )
      )
        return fail('Invalid mapping rule fields')
      const id = name(item.id, 128)
      if (ids.has(id)) return fail('Mapping rule IDs must be unique')
      ids.add(id)
      const priority = item.priority === undefined ? 0 : item.priority
      const enabled = item.enabled === undefined ? true : item.enabled
      if (
        typeof priority !== 'number' ||
        !Number.isInteger(priority) ||
        priority < -2147483648 ||
        priority > 2147483647
      )
        return fail('Mapping priority must be a signed 32-bit integer')
      if (typeof enabled !== 'boolean')
        return fail('Mapping enabled must be a boolean')
      return { id, from: name(item.from), to: name(item.to), priority, enabled }
    })
    return { version: 2, rules }
  }
  const legacy: Record<string, string[]> = Object.create(null)
  if (Object.keys(value).length > 1024)
    return fail('Model mapping exceeds 1024 rules or sources')
  let edges = 0
  for (const [source, target] of Object.entries(value)) {
    const from = name(source)
    if (Object.hasOwn(legacy, from))
      return fail('Duplicate source model mappings are not allowed')
    const targets = Array.isArray(target) ? target : [target]
    if (!targets.length) return fail('Mapping targets must not be empty')
    edges += targets.length
    if (edges > 4096) return fail('Model mapping exceeds 4096 edges')
    legacy[from] = [...new Set(targets.map((v) => name(v)))]
  }
  return { version: 1, legacy }
}

export function mappingSources(config: ModelMappingConfig): string[] {
  return [
    ...new Set(
      config.version === 2
        ? config.rules.filter((r) => r.enabled).map((r) => r.from)
        : Object.keys(config.legacy)
    ),
  ].sort()
}
export function mappingCandidates(
  config: ModelMappingConfig,
  source: string,
  budget = { remaining: 8192 }
): string[] {
  const result = new Set<string>()
  const add = (target: string) => {
    result.add(target)
    if (result.size > 128) fail('Model mapping exceeds 128 resolved candidates')
  }
  if (config.version === 2) {
    config.rules
      .filter((r) => r.enabled && r.from === source)
      .sort((a, b) => b.priority - a.priority)
      .forEach((r) => add(r.to))
  } else {
    if (!Object.hasOwn(config.legacy, source)) return []
    const path = new Set<string>()
    const step = () => {
      if (--budget.remaining < 0)
        fail('Model mapping exceeds expansion work limit')
    }
    const expand = (current: string, depth: number) => {
      step()
      if (depth > 32) fail('Model mapping exceeds maximum depth')
      if (path.has(current)) fail('Legacy model mapping contains a cycle')
      if (!Object.hasOwn(config.legacy, current)) {
        add(current)
        return
      }
      path.add(current)
      for (const target of config.legacy[current]) {
        if (target === current) {
          step()
          add(target)
        } else expand(target, depth + 1)
      }
      path.delete(current)
    }
    expand(source, 0)
  }
  return [...result]
}
export function upgradeLegacyMapping(
  config: ModelMappingConfig
): ModelMappingConfig & { version: 2 } {
  if (config.version === 2) return config
  const rules: ModelMappingRule[] = []
  const budget = { remaining: 65536 }
  for (const from of mappingSources(config)) {
    for (const to of mappingCandidates(config, from, budget))
      rules.push({
        id: `upgraded-${rules.length + 1}`,
        from,
        to,
        priority: 0,
        enabled: true,
      })
  }
  if (rules.length > 1024) fail('Model mapping exceeds 1024 rules or sources')
  return { version: 2, rules }
}

export function extractMappingSourceModels(raw: string): string[] {
  try {
    return mappingSources(parseModelMappingConfig(raw))
  } catch {
    return []
  }
}
export function extractRedirectModels(raw: string): string[] {
  try {
    const config = parseModelMappingConfig(raw)
    return [
      ...new Set(
        config.version === 2
          ? config.rules.filter((r) => r.enabled).map((r) => r.to)
          : Object.values(config.legacy).flat()
      ),
    ]
  } catch {
    return []
  }
}

/**
 * Check if model configuration has changed
 */
export function hasModelConfigChanged(
  currentModels: string[],
  currentModelMapping: string,
  initialModels: string[],
  initialModelMapping: string
): boolean {
  // Always return true if not editing (new channel)
  if (initialModels.length === 0 && !initialModelMapping) {
    return true
  }

  // Check if models array changed
  if (currentModels.length !== initialModels.length) {
    return true
  }
  for (let i = 0; i < currentModels.length; i++) {
    if (currentModels[i] !== initialModels[i]) {
      return true
    }
  }

  // Check if model_mapping changed
  const normalizedCurrent = (currentModelMapping || '').trim()
  const normalizedInitial = (initialModelMapping || '').trim()

  return normalizedCurrent !== normalizedInitial
}

/**
 * Find models in model_mapping that are missing from the models list
 */
export function findMissingModelsInMapping(
  modelMapping: string,
  currentModels: string[]
): string[] {
  const modelSet = new Set(currentModels.map(normalizeModelName))
  return extractMappingSourceModels(modelMapping).filter(
    (source) => !modelSet.has(source)
  )
}

/**
 * Validate model mapping JSON format
 */
function hasDuplicateJsonMembers(raw: string): boolean {
  // JSON.parse has already checked grammar. Scan string/punctuation tokens so
  // escaped keys and nested objects are checked before any normalization.
  const stack: { keys: Set<string> | null; expectKey: boolean }[] = []
  for (const token of raw.match(/"(?:\\.|[^"\\])*"|[{},:]|\[|\]/g) || []) {
    if (token === '{' || token === '[')
      stack.push({
        keys: token === '{' ? new Set() : null,
        expectKey: token === '{',
      })
    else if (token === '}' || token === ']') stack.pop()
    else {
      const frame = stack[stack.length - 1]
      if (!frame?.keys) continue
      if (token === ',') frame.expectKey = true
      else if (token.startsWith('"') && frame.expectKey) {
        const key = JSON.parse(token) as string
        if (frame.keys.has(key)) return true
        frame.keys.add(key)
        frame.expectKey = false
      }
    }
  }
  return false
}

export function validateModelMappingJson(modelMapping: string): {
  valid: boolean
  error?: string
} {
  try {
    const config = parseModelMappingConfig(modelMapping)
    if (hasDuplicateJsonMembers(modelMapping))
      return {
        valid: false,
        error: 'Duplicate JSON object members are not allowed',
      }
    const budget = { remaining: 65536 }
    for (const source of mappingSources(config)) {
      mappingCandidates(config, source)
      mappingCandidates(config, source, budget)
    }
    return { valid: true }
  } catch (error) {
    return {
      valid: false,
      error:
        error instanceof Error
          ? error.message
          : 'Model mapping must be valid JSON format',
    }
  }
}

/**
 * Get redirect models that are also in the models list
 * (These should be removed from models list to keep /v1/models clean)
 */
export function findExposedTargetModels(
  modelMapping: string,
  currentModels: string[]
): string[] {
  const redirectModels = extractRedirectModels(modelMapping)
  if (redirectModels.length === 0) return []

  const normalizedModels = currentModels.map((m) => normalizeModelName(m))
  const modelSet = new Set(normalizedModels)

  return redirectModels.filter((model) =>
    modelSet.has(normalizeModelName(model))
  )
}

/**
 * Categorize models into different sets for UI display
 */
export function categorizeModelsWithRedirect(
  currentModels: string[],
  redirectModels: string[]
): {
  normalizedCurrentModels: Set<string>
  normalizedRedirectModels: Set<string>
  classificationSet: Set<string>
  redirectOnlySet: Set<string>
} {
  const normalizedCurrentModels = new Set(
    currentModels.map((m) => normalizeModelName(m)).filter(Boolean)
  )

  const normalizedRedirectModels = new Set(
    redirectModels.map((m) => normalizeModelName(m)).filter(Boolean)
  )

  const classificationSet = new Set([
    ...normalizedCurrentModels,
    ...normalizedRedirectModels,
  ])

  const redirectOnlySet = new Set(
    Array.from(normalizedRedirectModels).filter(
      (m) => !normalizedCurrentModels.has(m)
    )
  )

  return {
    normalizedCurrentModels,
    normalizedRedirectModels,
    classificationSet,
    redirectOnlySet,
  }
}
