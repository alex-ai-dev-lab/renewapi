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

const BLOCKED_TAGS = new Set([
  'base',
  'embed',
  'form',
  'iframe',
  'link',
  'meta',
  'object',
  'script',
  'style',
  'template',
])

const URL_ATTRS = new Set([
  'action',
  'formaction',
  'href',
  'poster',
  'src',
  'xlink:href',
])

const SAFE_PROTOCOLS = new Set(['http:', 'https:', 'mailto:', 'tel:'])
const SAFE_DATA_IMAGE_RE =
  /^data:image\/(?:gif|jpeg|jpg|png|webp);base64,[a-z0-9+/]+=*$/i

function isSafeUrl(value: string) {
  const trimmed = value.trim()
  if (trimmed === '') {
    return true
  }
  if (
    trimmed.startsWith('#') ||
    trimmed.startsWith('/') ||
    trimmed.startsWith('./') ||
    trimmed.startsWith('../') ||
    trimmed.startsWith('?')
  ) {
    return true
  }
  if (SAFE_DATA_IMAGE_RE.test(trimmed)) {
    return true
  }

  try {
    const parsed = new URL(trimmed, window.location.origin)
    return SAFE_PROTOCOLS.has(parsed.protocol)
  } catch {
    return false
  }
}

function isSafeStyle(value: string) {
  const normalized = value.toLowerCase()
  return !(
    normalized.includes('expression(') ||
    normalized.includes('url(') ||
    normalized.includes('@import') ||
    normalized.includes('-moz-binding')
  )
}

function sanitizeElement(element: Element) {
  const tagName = element.tagName.toLowerCase()
  if (BLOCKED_TAGS.has(tagName)) {
    element.remove()
    return
  }

  for (const attr of Array.from(element.attributes)) {
    const name = attr.name.toLowerCase()
    const value = attr.value
    if (
      name.startsWith('on') ||
      name === 'srcdoc' ||
      (URL_ATTRS.has(name) && !isSafeUrl(value)) ||
      (name === 'style' && !isSafeStyle(value))
    ) {
      element.removeAttribute(attr.name)
    }
  }

  if (
    element instanceof HTMLAnchorElement &&
    element.target.toLowerCase() === '_blank'
  ) {
    element.rel = 'noopener noreferrer'
  }

  for (const child of Array.from(element.children)) {
    sanitizeElement(child)
  }
}

export function sanitizeHtml(html: string) {
  if (!html || typeof document === 'undefined') {
    return html || ''
  }

  const template = document.createElement('template')
  template.innerHTML = html
  for (const child of Array.from(template.content.children)) {
    sanitizeElement(child)
  }
  return template.innerHTML
}
