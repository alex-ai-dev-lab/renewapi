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
import type { PricingEndpointInfo, PricingModel } from '../pricing/types'
import type { CatalogModelHealthHour, ModelHealthModel } from './types'

const HEALTH_WINDOW_HOURS = 24

export type ModelHealthSnapshot = {
  successRate: number | null
  hourlyHealth: CatalogModelHealthHour[]
  icon?: string
}

export function normalizeModelName(modelName: string) {
  return modelName.trim().toLocaleLowerCase('en-US')
}

export type UsableGroupMap = Record<string, unknown>

/**
 * Resolve whether the current viewer can use a model.
 *
 * The backend `usable_group` map is authoritative: a model is usable when one
 * of its `enable_groups` is in the viewer's usable groups, or when the model is
 * universally enabled (`all`). `current_group` is only a fallback when the
 * usable-group set is not available. When neither source is known, access is
 * reported as `null` (unknown) instead of being denied by default.
 */
export function canUsePricingModel(
  model: { enable_groups?: string[] | null },
  currentGroup: string,
  usableGroups?: UsableGroupMap | null
): boolean | null {
  // An explicitly empty authoritative set means no access, even for `all`.
  const usableGroupNames =
    usableGroups == null ? null : Object.keys(usableGroups)
  if (usableGroupNames?.length === 0) return false
  const enabledGroups = model.enable_groups
  if (!Array.isArray(enabledGroups)) return null
  if (enabledGroups.length === 0) return false
  if (enabledGroups.includes('all')) {
    return true
  }

  if (usableGroupNames !== null) {
    return enabledGroups.some((group) => usableGroupNames.includes(group))
  }

  const normalizedGroup = currentGroup.trim()
  if (normalizedGroup.length > 0) {
    return enabledGroups.includes(normalizedGroup)
  }

  return null
}

/**
 * Provider label for a catalog model. Prefer the backend-provided vendor name;
 * icon strings are identifiers, not human-readable provider names.
 */
export function resolveProviderLabel(
  model: PricingModel,
  fallback: string
): string {
  const vendorName = model.vendor_name?.trim()
  return vendorName || fallback
}

export function buildCatalogEndpoints(
  model: PricingModel,
  endpointMap: Record<string, PricingEndpointInfo>
) {
  return [
    ...new Set(
      (model.supported_endpoint_types ?? [])
        .map((type) => {
          const endpoint = endpointMap[type]
          if (!endpoint) return null

          const path = endpoint.path
            ?.replaceAll('{model}', model.model_name)
            .trim()
          if (!path) return null

          const method = endpoint.method?.trim().toUpperCase() || 'POST'
          return `${method} ${path}`
        })
        .filter((endpoint): endpoint is string => endpoint !== null)
    ),
  ]
}

function mergeUniqueValues(
  current: string[] | undefined,
  next: string[] | undefined
) {
  return [...new Set([...(current ?? []), ...(next ?? [])])].sort((a, b) =>
    a.localeCompare(b)
  )
}

export function mergePricingModels(models: PricingModel[]) {
  const sortedModels = [...models].sort((a, b) => {
    const nameComparison = normalizeModelName(a.model_name).localeCompare(
      normalizeModelName(b.model_name)
    )
    if (nameComparison !== 0) return nameComparison
    if (a.id !== b.id) return a.id - b.id
    return a.model_name.localeCompare(b.model_name)
  })
  const mergedModels = new Map<string, PricingModel>()

  for (const model of sortedModels) {
    const identity = normalizeModelName(model.model_name)
    if (!identity) continue

    const current = mergedModels.get(identity)
    if (!current) {
      mergedModels.set(identity, {
        ...model,
        enable_groups: Array.isArray(model.enable_groups)
          ? mergeUniqueValues([], model.enable_groups)
          : undefined,
        supported_endpoint_types: mergeUniqueValues(
          [],
          model.supported_endpoint_types
        ),
      })
      continue
    }

    mergedModels.set(identity, {
      ...current,
      description: current.description ?? model.description,
      icon: current.icon ?? model.icon,
      vendor_id: current.vendor_id ?? model.vendor_id,
      vendor_name: current.vendor_name ?? model.vendor_name,
      vendor_icon: current.vendor_icon ?? model.vendor_icon,
      vendor_description:
        current.vendor_description ?? model.vendor_description,
      tags: current.tags ?? model.tags,
      key: current.key ?? model.key,
      group_ratio: current.group_ratio ?? model.group_ratio,
      billing_mode: current.billing_mode ?? model.billing_mode,
      billing_expr: current.billing_expr ?? model.billing_expr,
      pricing_version: current.pricing_version ?? model.pricing_version,
      context_length: current.context_length ?? model.context_length,
      max_output_tokens: current.max_output_tokens ?? model.max_output_tokens,
      knowledge_cutoff: current.knowledge_cutoff ?? model.knowledge_cutoff,
      release_date: current.release_date ?? model.release_date,
      parameter_count: current.parameter_count ?? model.parameter_count,
      input_modalities: current.input_modalities ?? model.input_modalities,
      output_modalities: current.output_modalities ?? model.output_modalities,
      capabilities: current.capabilities ?? model.capabilities,
      enable_groups:
        Array.isArray(current.enable_groups) &&
        Array.isArray(model.enable_groups)
          ? mergeUniqueValues(current.enable_groups, model.enable_groups)
          : undefined,
      supported_endpoint_types: mergeUniqueValues(
        current.supported_endpoint_types,
        model.supported_endpoint_types
      ),
    })
  }

  return [...mergedModels.values()]
}

export function aggregateModelHealth(models: ModelHealthModel[]) {
  const modelsByIdentity = new Map<string, ModelHealthModel[]>()

  for (const model of models) {
    const identity = normalizeModelName(model.model_name)
    if (!identity) continue
    const groupedModels = modelsByIdentity.get(identity) ?? []
    groupedModels.push(model)
    modelsByIdentity.set(identity, groupedModels)
  }

  const snapshots = new Map<string, ModelHealthSnapshot>()

  for (const [identity, groupedModels] of modelsByIdentity) {
    const channelModels = groupedModels.filter(
      (model) => Number(model.channel_id) > 0
    )
    const visibleModels =
      channelModels.length > 0 ? channelModels : groupedModels
    const bucketsByHour = new Map<
      number,
      { totalCount: number; successCount: number }
    >()

    for (const model of visibleModels) {
      for (const bucket of model.buckets) {
        if (!Number.isFinite(bucket.hour)) continue
        const totalCount = Number.isFinite(bucket.total_count)
          ? Math.max(0, bucket.total_count)
          : 0
        const successCount = Number.isFinite(bucket.success_count)
          ? Math.max(0, bucket.success_count)
          : 0
        const current = bucketsByHour.get(bucket.hour) ?? {
          totalCount: 0,
          successCount: 0,
        }
        current.totalCount += totalCount
        current.successCount += successCount
        bucketsByHour.set(bucket.hour, current)
      }
    }

    const hourlyHealth = [...bucketsByHour]
      .sort(([hourA], [hourB]) => hourA - hourB)
      .slice(-HEALTH_WINDOW_HOURS)
      .map(([hour, counts]) => ({
        hour,
        totalCount: counts.totalCount,
        successCount: counts.successCount,
        successRate:
          counts.totalCount > 0
            ? Math.min(
                100,
                Math.max(
                  0,
                  Number(
                    ((counts.successCount / counts.totalCount) * 100).toFixed(2)
                  )
                )
              )
            : null,
      }))
    const observedHours = hourlyHealth.filter((health) => health.totalCount > 0)
    const meanSuccessRate = observedHours.length
      ? Number(
          (
            observedHours.reduce(
              (sum, health) =>
                sum + (health.successCount / health.totalCount) * 100,
              0
            ) / observedHours.length
          ).toFixed(2)
        )
      : null
    const icon = [...visibleModels]
      .sort((a, b) => {
        const channelComparison =
          Number(a.channel_id ?? 0) - Number(b.channel_id ?? 0)
        if (channelComparison !== 0) return channelComparison
        return (a.icon ?? '').localeCompare(b.icon ?? '')
      })
      .find((model) => Boolean(model.icon))?.icon

    snapshots.set(identity, {
      successRate: meanSuccessRate,
      hourlyHealth,
      ...(icon ? { icon } : {}),
    })
  }

  return snapshots
}
