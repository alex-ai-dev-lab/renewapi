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
import ReactMarkdown from 'react-markdown'
import rehypeRaw from 'rehype-raw'
import remarkGfm from 'remark-gfm'
import { cn } from '@/lib/utils'

interface MarkdownProps {
  children: string
  className?: string
}

type HastNode = {
  type?: string
  tagName?: string
  properties?: Record<string, unknown>
  children?: HastNode[]
}

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

const URL_PROPS = new Set([
  'action',
  'formaction',
  'href',
  'poster',
  'src',
  'xlinkhref',
])

const SAFE_PROTOCOLS = new Set(['http:', 'https:', 'mailto:', 'tel:'])
const SAFE_DATA_IMAGE_RE =
  /^data:image\/(?:gif|jpeg|jpg|png|webp);base64,[a-z0-9+/]+=*$/i

function isSafeUrl(value: string) {
  const trimmed = value.trim()
  if (trimmed === '') return true
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
    const origin =
      typeof window === 'undefined'
        ? 'http://localhost'
        : window.location.origin
    const parsed = new URL(trimmed, origin)
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

function propertyToString(value: unknown) {
  if (Array.isArray(value)) {
    return value.join(' ')
  }
  return typeof value === 'string' ? value : ''
}

function sanitizeHastNode(node: HastNode) {
  if (node.type === 'element') {
    const props = node.properties ?? {}
    for (const key of Object.keys(props)) {
      const value = props[key]
      const keyLower = key.toLowerCase()
      if (
        keyLower.startsWith('on') ||
        keyLower === 'srcdoc' ||
        (URL_PROPS.has(keyLower) && !isSafeUrl(propertyToString(value))) ||
        (keyLower === 'style' && !isSafeStyle(propertyToString(value)))
      ) {
        delete props[key]
      }
    }
    if (node.tagName === 'a' && props.target === '_blank') {
      props.rel = 'noopener noreferrer'
    }
    node.properties = props
  }

  if (!node.children) return
  node.children = node.children.filter((child) => {
    return !(
      child.type === 'element' &&
      child.tagName &&
      BLOCKED_TAGS.has(child.tagName)
    )
  })
  for (const child of node.children) {
    sanitizeHastNode(child)
  }
}

function rehypeSanitizeUserHtml() {
  return (tree: HastNode) => {
    sanitizeHastNode(tree)
  }
}

export function Markdown({ children, className }: MarkdownProps) {
  return (
    <div
      className={cn(
        'prose prose-sm dark:prose-invert max-w-none',
        'prose-headings:font-semibold prose-headings:tracking-tight',
        'prose-h1:text-2xl prose-h2:text-xl prose-h3:text-lg',
        'prose-p:leading-relaxed prose-p:my-2',
        'prose-a:text-primary prose-a:no-underline hover:prose-a:underline',
        'prose-code:bg-muted prose-code:px-1 prose-code:py-0.5 prose-code:rounded prose-code:before:content-none prose-code:after:content-none',
        'prose-pre:bg-muted prose-pre:border',
        'prose-blockquote:border-l-primary prose-blockquote:bg-muted/50 prose-blockquote:py-1',
        'prose-ul:my-2 prose-ol:my-2 prose-li:my-1',
        'prose-table:border prose-thead:bg-muted',
        'prose-td:border prose-th:border prose-td:px-3 prose-th:px-3',
        'prose-img:rounded-lg prose-img:shadow-sm',
        '[&>*:first-child]:mt-0 [&>*:last-child]:mb-0',
        '[overflow-wrap:anywhere] break-words',
        className
      )}
    >
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeRaw, rehypeSanitizeUserHtml]}
        components={{
          // 自定义组件渲染（可选）
          a: ({ node, ...props }) => (
            <a {...props} target='_blank' rel='noopener noreferrer' />
          ),
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  )
}
