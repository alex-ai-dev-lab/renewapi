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
import { expect, test } from 'bun:test'
import {
  channelFormSchema,
  CHANNEL_FORM_DEFAULT_VALUES,
  validateModelMapping,
} from './channel-form'
import {
  parseModelMappingConfig,
  mappingCandidates,
  upgradeLegacyMapping,
  validateModelMappingJson,
  extractMappingSourceModels,
} from './model-mapping-validation'

test('v2 direct candidates use descending priority, stable ties and deduplicate targets', () => {
  const config = parseModelMappingConfig(
    JSON.stringify({
      version: 2,
      rules: [
        { id: '1', from: 'A', to: 'B' },
        { id: '2', from: 'A', to: 'C', priority: 9 },
        { id: '3', from: 'A', to: 'B', priority: 9 },
        { id: '4', from: 'B', to: 'A' },
        { id: '5', from: 'A', to: 'D', enabled: false },
      ],
    })
  )
  expect(mappingCandidates(config, 'A')).toEqual(['C', 'B'])
  expect(mappingCandidates(config, 'B')).toEqual(['A'])
})
test('legacy upgrade expands real terminal candidates and rejects cycles', () => {
  const config = parseModelMappingConfig('{"A":["B","C"],"B":"D"}')
  const next = upgradeLegacyMapping(config)
  expect(mappingCandidates(next, 'A')).toEqual(['D', 'C'])
  expect(validateModelMappingJson('{"A":"B","B":"A"}').valid).toBe(false)
})
test('strict JSON validation rejects duplicate members before JSON.parse can erase them', () => {
  for (const raw of [
    '{"a":"b","a":"c"}',
    '{"a":"b","\\u0061":"c"}',
    '{"version":2,"rules":[{"id":"x","from":"a","from":"b","to":"c"}]}',
  ])
    expect(validateModelMappingJson(raw).valid).toBe(false)
  expect(validateModelMappingJson('{"a":"a","b":"a"}').valid).toBe(true)
})
test('source, edge, candidate, name, id and integer limits match server bounds', () => {
  const rules = (count: number) =>
    JSON.stringify({
      version: 2,
      rules: Array.from({ length: count }, (_, i) => ({
        id: `r${i}`,
        from: 'alias',
        to: `target${i}`,
      })),
    })
  expect(validateModelMappingJson(rules(128)).valid).toBe(true)
  expect(validateModelMappingJson(rules(129)).valid).toBe(false)
  expect(validateModelMappingJson(rules(1025)).valid).toBe(false)
  expect(
    validateModelMappingJson(JSON.stringify({ ['a'.repeat(256)]: 'b' })).valid
  ).toBe(false)
  expect(
    validateModelMappingJson(JSON.stringify({ ['😀'.repeat(255)]: 'b' })).valid
  ).toBe(true)
  for (const patch of [
    { id: 'x'.repeat(129) },
    { priority: 0.1 },
    { priority: null },
    { enabled: null },
    { enabled: 0 },
    { extra: true },
  ]) {
    expect(
      validateModelMappingJson(
        JSON.stringify({
          version: 2,
          rules: [{ id: 'x', from: 'a', to: 'b', ...patch }],
        })
      ).valid
    ).toBe(false)
  }
  expect(
    validateModelMappingJson(JSON.stringify({ a: Array(4097).fill('b') })).valid
  ).toBe(false)
  expect(validateModelMappingJson(' '.repeat(1048577)).valid).toBe(false)
  const chain = Object.fromEntries(
    Array.from({ length: 34 }, (_, i) => [`a${i}`, `a${i + 1}`])
  )
  expect(validateModelMappingJson(JSON.stringify(chain)).valid).toBe(false)
})
test('form and helper share mapping validation while defaults select v2', () => {
  const base = {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'channel',
    key: 'key',
    models: 'alias',
    group: ['default'],
  }
  expect(JSON.parse(base.model_mapping!)).toEqual({ version: 2, rules: [] })
  expect(
    channelFormSchema.safeParse({
      ...base,
      model_mapping: '{"alias":["a","b"]}',
    }).success
  ).toBe(true)
  expect(
    channelFormSchema.safeParse({
      ...base,
      model_mapping: '{"version":2,"rules":[{"id":"x","from":"a","to":"b"}]}',
    }).success
  ).toBe(true)
  expect(
    channelFormSchema.safeParse({ ...base, model_mapping: '{"a":"b","b":"a"}' })
      .success
  ).toBe(false)
  expect(validateModelMapping('null')).toBe(false)
})

test('prototype-shaped model names are ordinary sources and targets', () => {
  const config = parseModelMappingConfig(
    '{"__proto__":"constructor","constructor":"toString","toString":"toString"}'
  )
  expect(mappingCandidates(config, '__proto__')).toEqual(['toString'])
  expect(
    mappingCandidates(
      parseModelMappingConfig('{"alias":"constructor"}'),
      'alias'
    )
  ).toEqual(['constructor'])
  expect(mappingCandidates(upgradeLegacyMapping(config), '__proto__')).toEqual([
    'toString',
  ])
})

test('integral JSON numbers use numeric rather than lexical integer semantics', () => {
  for (const priority of ['1.0', '1e2', '-2147483648', '2147483647']) {
    const raw = `{"version":2.0,"rules":[{"id":"x","from":"a","to":"b","priority":${priority}}]}`
    expect(validateModelMappingJson(raw).valid).toBe(true)
  }
})

test('legacy envelope names, defaults, enabled sources and strict bounds', () => {
  expect(
    parseModelMappingConfig('{"version":"rules","rules":["end"]}').version
  ).toBe(1)
  expect(
    extractMappingSourceModels(
      '{"version":2,"rules":[{"id":"x","from":"A","to":"B","enabled":false}]}'
    )
  ).toEqual([])
  for (const value of [
    'null',
    '{"version":3,"rules":[]}',
    '{"version":2,"rules":[{"id":"x","from":"A","to":"B","priority":2147483648}]}',
    '{"version":2,"rules":[{"id":"x","from":"A","to":"B","priority":1.2}]}',
    '{"version":2.5,"rules":[]}',
    '{"A":[]}',
  ])
    expect(validateModelMappingJson(value).valid).toBe(false)
})
