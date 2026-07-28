import { describe, expect, test } from 'bun:test'
import { Window } from 'happy-dom'
import { resolveHttpRedirect, resolveInternalRedirect } from './dom-utils'
import {
  MAX_CREDENTIAL_FILE_BYTES,
  sanitizeDownloadFilename,
  validateCredentialFiles,
} from './security-boundaries'

const browser = new Window({ url: 'https://app.example/dashboard' })
Object.defineProperties(globalThis, {
  window: { configurable: true, value: browser },
  document: { configurable: true, value: browser.document },
  Element: { configurable: true, value: browser.Element },
  HTMLAnchorElement: {
    configurable: true,
    value: browser.HTMLAnchorElement,
  },
})

const { sanitizeHtml } = await import('./sanitize-html')

describe('HTML sanitization corpus', () => {
  test.each([
    '<script>alert(1)</script><p>safe</p>',
    '<img src=x onerror="alert(1)">',
    '<a href="javascript:alert(1)">click</a>',
    '<div style="background:url(javascript:alert(1))">x</div>',
    '<iframe srcdoc="<script>alert(1)</script>"></iframe>',
  ])('removes executable markup from %s', (payload) => {
    const clean = sanitizeHtml(payload).toLowerCase()
    expect(clean).not.toContain('<script')
    expect(clean).not.toContain('onerror')
    expect(clean).not.toContain('javascript:')
    expect(clean).not.toContain('srcdoc')
    expect(clean).not.toContain('<iframe')
  })

  test('does not preserve an unsafe opener relationship', () => {
    const clean = sanitizeHtml(
      '<a href="https://example.com" target="_blank">x</a>'
    )
    expect(
      !clean.includes('target="_blank"') ||
        clean.includes('rel="noopener noreferrer"')
    ).toBe(true)
  })
})

describe('redirect corpus', () => {
  test.each([
    'https://evil.example/path',
    '//evil.example/path',
    'javascript:alert(1)',
    'data:text/html,<script>alert(1)</script>',
    '/\\evil.example/path',
  ])('rejects external or executable internal redirect %s', (target) => {
    expect(resolveInternalRedirect(target, '/dashboard')).toBe('/dashboard')
  })

  test('preserves same-origin paths and query strings', () => {
    expect(resolveInternalRedirect('/keys?page=2#active')).toBe(
      '/keys?page=2#active'
    )
    expect(resolveInternalRedirect('https://app.example/profile')).toBe(
      '/profile'
    )
  })

  test.each(['javascript:alert(1)', 'data:text/html,x', '/relative'])(
    'rejects non-http checkout target %s',
    (target) => expect(resolveHttpRedirect(target)).toBeNull()
  )
})

describe('upload and download corpus', () => {
  test('rejects oversized and non-JSON credential files', () => {
    expect(
      validateCredentialFiles([
        {
          name: 'large.json',
          size: MAX_CREDENTIAL_FILE_BYTES + 1,
          type: 'application/json',
        },
      ])
    ).toMatchObject({ valid: false, reason: 'size' })
    expect(
      validateCredentialFiles([
        { name: 'credential.exe', size: 10, type: 'application/octet-stream' },
      ])
    ).toMatchObject({ valid: false, reason: 'type' })
    expect(
      validateCredentialFiles([
        { name: 'credential.json', size: 10, type: 'application/octet-stream' },
      ])
    ).toMatchObject({ valid: false, reason: 'type' })
    expect(
      validateCredentialFiles([{ name: 'credential.exe', size: 10, type: '' }])
    ).toMatchObject({ valid: false, reason: 'type' })
  })

  test('accepts bounded JSON credential files', () => {
    expect(
      validateCredentialFiles([
        { name: 'credential.json', size: 512, type: 'application/json' },
      ])
    ).toEqual({ valid: true })
  })

  test.each(['../../secret.txt', 'bad\u0000name.txt', 'CON<>:"/\\|?*.txt'])(
    'normalizes unsafe download filename %s',
    (name) => {
      const safe = sanitizeDownloadFilename(name)
      expect(
        Array.from(safe).some((character) => {
          const codePoint = character.codePointAt(0) ?? 0
          return codePoint <= 0x1f || codePoint === 0x7f
        })
      ).toBe(false)
      expect(safe).not.toMatch(/[<>:"/\\|?*]/)
      expect(safe.length).toBeGreaterThan(0)
    }
  )
})
