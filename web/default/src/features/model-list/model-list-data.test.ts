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
import assert from 'node:assert/strict'
import test from 'node:test'
import type { PricingModel } from '../pricing/types'
import {
  aggregateModelHealth,
  buildCatalogEndpoints,
  canUsePricingModel,
  mergePricingModels,
  resolveProviderLabel,
} from './model-list-data.ts'
import type { ModelHealthModel } from './types'

function healthModel(
  modelName: string,
  channelId: number,
  hour: number,
  totalCount: number,
  successCount: number
): ModelHealthModel {
  return {
    model_name: modelName,
    channel_id: channelId,
    buckets: [
      {
        hour,
        total_count: totalCount,
        success_count: successCount,
        probe_count: 0,
        success_rate: totalCount > 0 ? (successCount / totalCount) * 100 : 0,
      },
    ],
  }
}

function pricingModel(
  id: number,
  modelName: string,
  endpoints: string[]
): PricingModel {
  return {
    id,
    model_name: modelName,
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    supported_endpoint_types: endpoints,
  }
}

test('aggregates every visible channel by normalized model and hour', () => {
  const snapshots = aggregateModelHealth([
    healthModel(' GPT-4o ', 1, 100, 3, 2),
    healthModel('gpt-4O', 2, 100, 7, 5),
    healthModel('gpt-4o', 1, 200, 2, 1),
  ])

  assert.deepEqual(snapshots.get('gpt-4o'), {
    successRate: 60,
    hourlyHealth: [
      { hour: 100, totalCount: 10, successCount: 7, successRate: 70 },
      { hour: 200, totalCount: 2, successCount: 1, successRate: 50 },
    ],
  })
})

test('keeps unknown buckets null and ignores overall rows when channels exist', () => {
  const snapshots = aggregateModelHealth([
    healthModel('model-a', 0, 100, 100, 100),
    healthModel('MODEL-A', 4, 100, 0, 0),
  ])

  assert.deepEqual(snapshots.get('model-a'), {
    successRate: null,
    hourlyHealth: [
      { hour: 100, totalCount: 0, successCount: 0, successRate: null },
    ],
  })
})

test('hourly average is not traffic weighted and only includes the last 24 slots', () => {
  const rows = [
    healthModel('model-a', 0, 0, 100, 100),
    ...Array.from({ length: 22 }, (_, index) =>
      healthModel('model-a', 0, index + 1, 0, 0)
    ),
    healthModel('model-a', 0, 23, 1, 1),
    healthModel('model-a', 0, 24, 100, 0),
  ]
  const health = aggregateModelHealth(rows).get('model-a')
  assert.equal(health?.successRate, 50)
  assert.equal(health?.hourlyHealth.length, 24)
  assert.equal(health?.hourlyHealth[0]?.hour, 1)
})

test('merges duplicate catalog rows deterministically without changing display spelling', () => {
  const laterRow = {
    ...pricingModel(9, 'MODEL-A', ['responses']),
    description: 'From later row',
  }
  const preferredRow = pricingModel(2, 'Model-A', ['chat'])

  const firstOrder = mergePricingModels([laterRow, preferredRow])
  const reversedOrder = mergePricingModels([preferredRow, laterRow])

  assert.deepEqual(firstOrder, reversedOrder)
  assert.equal(firstOrder[0]?.model_name, 'Model-A')
  assert.equal(firstOrder[0]?.description, 'From later row')
  assert.deepEqual(firstOrder[0]?.supported_endpoint_types, [
    'chat',
    'responses',
  ])
})

test('formats configured endpoint metadata and omits unresolved internal endpoint types', () => {
  const model = pricingModel(1, 'model-a', ['chat', 'responses', 'unknown'])

  assert.deepEqual(
    buildCatalogEndpoints(model, {
      chat: { method: 'post', path: '/v1/chat/completions' },
      responses: { method: 'POST', path: '/v1/models/{model}/responses' },
    }),
    ['POST /v1/chat/completions', 'POST /v1/models/model-a/responses']
  )
})

test('reports access from authoritative usable groups and keeps unknown distinct from denied', () => {
  const currentGroupModel = {
    ...pricingModel(1, 'model-a', ['chat']),
    enable_groups: ['Moderate'],
  }
  const universalModel = {
    ...pricingModel(2, 'model-b', ['chat']),
    enable_groups: ['all'],
  }

  // The authoritative usable-group set wins over the current group.
  assert.equal(
    canUsePricingModel(currentGroupModel, 'Light', { Moderate: 'Moderate' }),
    true
  )
  assert.equal(
    canUsePricingModel(currentGroupModel, 'Moderate', { Light: 'Light' }),
    false
  )

  // Without a usable-group set, the current group is the fallback.
  assert.equal(canUsePricingModel(currentGroupModel, 'Moderate'), true)
  assert.equal(canUsePricingModel(currentGroupModel, 'Light'), false)

  // Empty group with no usable-group data is unknown, not denied.
  assert.equal(canUsePricingModel(currentGroupModel, ''), null)

  // Universally enabled models are always usable.
  assert.equal(canUsePricingModel(universalModel, 'Free'), true)
  assert.equal(canUsePricingModel(universalModel, '', null), true)
})

test('provider label prefers vendor_name over icon identifiers', () => {
  const model = {
    ...pricingModel(1, 'model-a', ['chat']),
    icon: 'OpenAI.Color',
    vendor_name: 'Acme AI',
  }

  assert.equal(resolveProviderLabel(model, 'Unknown'), 'Acme AI')
  assert.equal(
    resolveProviderLabel({ ...model, vendor_name: '' }, 'Unknown'),
    'Unknown'
  )
  assert.equal(
    resolveProviderLabel(pricingModel(2, 'model-b', []), 'Unknown'),
    'Unknown'
  )
})

test('missing model groups stay unknown while explicit empty groups deny access', () => {
  const model = pricingModel(1, 'model-a', ['chat'])
  assert.equal(
    canUsePricingModel({ ...model, enable_groups: undefined }, 'default', {
      default: 'Default',
    }),
    null
  )
  assert.equal(
    canUsePricingModel({ ...model, enable_groups: null }, 'default'),
    null
  )
  assert.equal(
    canUsePricingModel({ ...model, enable_groups: [] }, 'default', {
      default: 'Default',
    }),
    false
  )
})

test('an explicit empty usable-group set denies even current-group and universal models', () => {
  for (const enable_groups of [['default'], ['all']]) {
    const model = { ...pricingModel(1, 'model-a', ['chat']), enable_groups }
    assert.equal(canUsePricingModel(model, 'default', {}), false)
    assert.equal(canUsePricingModel(model, 'default', undefined), true)
  }
})

test('catalog merging preserves incomplete group data before computing access', () => {
  const model = {
    ...pricingModel(1, 'model-a', ['chat']),
    enable_groups: undefined,
  }
  for (const rows of [
    [model],
    [model, { ...model, id: 2, enable_groups: [] }],
  ]) {
    const merged = mergePricingModels(rows)
    assert.equal(
      canUsePricingModel(merged[0], 'default', { default: 'Default' }),
      null
    )
  }
})
