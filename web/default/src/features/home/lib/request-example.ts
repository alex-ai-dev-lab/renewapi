/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
export function resolveApiBase(
  configuredAddress: unknown,
  origin: string
): string {
  let base = new URL(origin)
  if (typeof configuredAddress === 'string' && configuredAddress.trim()) {
    const value = configuredAddress.trim()
    try {
      const candidate = new URL(value, origin)
      if (
        (value.startsWith('/') || /^https?:\/\//i.test(value)) &&
        ['http:', 'https:'].includes(candidate.protocol) &&
        !candidate.username &&
        !candidate.password
      ) {
        base = candidate
      }
    } catch {
      // Invalid configuration falls back to this deployment, never a brand domain.
    }
  }
  base.search = ''
  base.hash = ''
  const path = base.pathname.replace(/\/+$/, '')
  base.pathname = path.endsWith('/v1') ? path : `${path}/v1`
  return base.href
}

export function buildRequestExample(apiBase: string): string {
  // Single-quote the URL for POSIX shells, including any apostrophe in its path.
  const endpoint = `'${`${apiBase}/chat/completions`.replaceAll("'", "'\\''")}'`
  return [
    `curl --request POST ${endpoint} \\`,
    '  --header "Authorization: Bearer $API_KEY" \\',
    "  --header 'Content-Type: application/json' \\",
    "  --data '{",
    '    "model": "YOUR_MODEL",',
    '    "messages": [{"role": "user", "content": "Hello"}]',
    "  }'",
  ].join('\n')
}
