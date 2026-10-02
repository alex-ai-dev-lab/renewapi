/* Copyright (C) 2026 RenewAPI contributors. SPDX-License-Identifier: AGPL-3.0-or-later */
import { describe, expect, test } from 'bun:test'
import { readFileSync } from 'node:fs'
import {
  mappingCandidates,
  mappingSources,
  parseModelMappingConfig,
  validateModelMappingJson,
} from './model-mapping-validation'

type Fixture = { name: string; raw: string } & (
  | { valid: false }
  | {
      valid: true
      version: 1 | 2
      sources: string[]
      candidates: Record<string, string[]>
    }
)
const fixtures: Fixture[] = JSON.parse(
  readFileSync(
    new URL(
      '../../../../../../common/testdata/model-mapping-contract.json',
      import.meta.url
    ),
    'utf8'
  )
)

describe('shared frontend/backend model mapping contract', () => {
  for (const fixture of fixtures) {
    test(fixture.name, () => {
      expect(validateModelMappingJson(fixture.raw).valid).toBe(fixture.valid)
      if (!fixture.valid) return
      const config = parseModelMappingConfig(fixture.raw)
      expect(config.version).toBe(fixture.version)
      expect(mappingSources(config)).toEqual(fixture.sources)
      for (const [source, targets] of Object.entries(
        fixture.candidates || {}
      )) {
        expect(mappingCandidates(config, source)).toEqual(targets)
      }
    })
  }
})
