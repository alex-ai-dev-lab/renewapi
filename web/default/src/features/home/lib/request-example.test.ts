/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import { describe, expect, test } from 'bun:test'
import { buildRequestExample, resolveApiBase } from './request-example'

describe('home request example', () => {
  test('uses the current origin when no server address is configured', () => {
    expect(resolveApiBase(undefined, 'https://gateway.example')).toBe(
      'https://gateway.example/v1'
    )
  })

  test('preserves a configured deployment path without duplicating v1', () => {
    expect(
      resolveApiBase('https://gateway.example/proxy/v1/', 'http://localhost')
    ).toBe('https://gateway.example/proxy/v1')
    expect(resolveApiBase('/proxy/', 'https://gateway.example')).toBe(
      'https://gateway.example/proxy/v1'
    )
  })

  test('rejects unsafe schemes and embedded credentials', () => {
    for (const value of [
      'javascript:alert(1)',
      'https://user:secret@example.com',
      'not a url',
    ]) {
      expect(resolveApiBase(value, 'https://gateway.example')).toBe(
        'https://gateway.example/v1'
      )
    }
  })

  test('does not put query secrets or fragments into the copied example', () => {
    expect(
      resolveApiBase(
        'https://gateway.example/?token=secret#private',
        'http://localhost'
      )
    ).toBe('https://gateway.example/v1')
  })

  test('quotes shell metacharacters and leaves model and API key as explicit placeholders', () => {
    const example = buildRequestExample("https://gateway.example/a'b/v1")
    expect(example).toContain(
      "'https://gateway.example/a'\\''b/v1/chat/completions'"
    )
    expect(example).toContain('$API_KEY')
    expect(example).toContain('YOUR_MODEL')
    expect(example).not.toContain('sk-')
  })
})
