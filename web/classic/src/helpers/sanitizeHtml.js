/*
Copyright (C) 2025 QuantumNous

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

import DOMPurify from 'dompurify';

const BLOCKED_TAGS = [
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
];

const SAFE_STYLE_BLOCKLIST = ['expression(', 'url(', '@import', '-moz-binding'];

function isSafeStyle(value) {
  const normalized = String(value || '').toLowerCase();
  return !SAFE_STYLE_BLOCKLIST.some((item) => normalized.includes(item));
}

if (typeof window !== 'undefined') {
  DOMPurify.addHook('afterSanitizeAttributes', (node) => {
    if (!(node instanceof Element)) return;
    const style = node.getAttribute('style');
    if (style && !isSafeStyle(style)) {
      node.removeAttribute('style');
    }
    if (
      node instanceof HTMLAnchorElement &&
      node.target.toLowerCase() === '_blank'
    ) {
      node.rel = 'noopener noreferrer';
    }
  });
}

export function sanitizeHtml(html) {
  if (!html || typeof window === 'undefined') {
    return html || '';
  }

  return DOMPurify.sanitize(html, {
    FORBID_TAGS: BLOCKED_TAGS,
    FORBID_ATTR: ['srcdoc'],
    ALLOWED_URI_REGEXP:
      /^(?:(?:(?:f|ht)tps?|mailto|tel):|data:image\/(?:gif|jpeg|jpg|png|webp);base64,|[^a-z]|[a-z+.-]+(?:[^a-z+.-:]|$))/i,
  });
}
