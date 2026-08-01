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
import { api } from './api'

// ============================================================================
// OAuth URL Builders
// ============================================================================

// 所有 builder 统一用 URL + searchParams 构造，避免手写字符串拼接时漏掉
// encodeURIComponent 导致 clientId / state 里的 & # = 破坏查询串结构。

/**
 * Build GitHub OAuth URL
 *
 * 注意：GitHub 的回调地址由 OAuth App 配置决定，这里不传 redirect_uri。
 * scope 序列化后为 user%3Aemail，GitHub 侧会正常百分号解码。
 */
export function buildGitHubOAuthUrl(clientId: string, state: string): string {
  const url = new URL('https://github.com/login/oauth/authorize')
  url.searchParams.set('client_id', clientId)
  url.searchParams.set('state', state)
  url.searchParams.set('scope', 'user:email')
  return url.toString()
}

/**
 * Build Discord OAuth URL
 */
export function buildDiscordOAuthUrl(clientId: string, state: string): string {
  const url = new URL('https://discord.com/oauth2/authorize')
  url.searchParams.set('client_id', clientId)
  url.searchParams.set(
    'redirect_uri',
    `${window.location.origin}/oauth/discord`
  )
  url.searchParams.set('response_type', 'code')
  // OAuth2 的 scope 是空格分隔。这里原本写成 'identify+openid'，
  // 而 URLSearchParams 会把 '+' 编码成 %2B，最终请求的是一个名为
  // "identify+openid" 的单个 scope，Discord 侧并不存在。
  url.searchParams.set('scope', 'identify openid')
  url.searchParams.set('state', state)
  return url.toString()
}

/**
 * Build OIDC OAuth URL
 */
export function buildOIDCOAuthUrl(
  authUrl: string,
  clientId: string,
  state: string
): string {
  const url = new URL(authUrl)
  url.searchParams.set('client_id', clientId)
  url.searchParams.set('redirect_uri', `${window.location.origin}/oauth/oidc`)
  url.searchParams.set('response_type', 'code')
  url.searchParams.set('scope', 'openid profile email')
  url.searchParams.set('state', state)
  return url.toString()
}

/**
 * Build LinuxDO OAuth URL
 */
export function buildLinuxDOOAuthUrl(clientId: string, state: string): string {
  const url = new URL('https://connect.linux.do/oauth2/authorize')
  url.searchParams.set('response_type', 'code')
  url.searchParams.set('client_id', clientId)
  url.searchParams.set('state', state)
  return url.toString()
}

// ============================================================================
// OAuth Helper Functions
// ============================================================================

/**
 * 打开 OAuth 授权页。
 *
 * 必须带 noopener：否则被打开的第三方页面可以通过 window.opener 把本站
 * 当前标签页导航到钓鱼页（reverse tabnabbing）。
 */
function openOAuthWindow(url: string): void {
  window.open(url, '_blank', 'noopener,noreferrer')
}

/**
 * Get OAuth state token
 * Includes affiliate code from localStorage if available
 */
export async function getOAuthState(): Promise<string | null> {
  try {
    let path = '/api/oauth/state'
    const affCode = localStorage.getItem('aff')
    if (affCode && affCode.length > 0) {
      // aff 来自 localStorage，可被用户任意写入，必须编码后再拼进查询串
      path += `?aff=${encodeURIComponent(affCode)}`
    }
    const res = await api.get(path)
    if (res.data.success) {
      return res.data.data
    }
    return null
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('Failed to get OAuth state:', error)
    return null
  }
}

/**
 * Handle GitHub OAuth binding/login
 */
export async function handleGitHubOAuth(clientId: string): Promise<void> {
  const state = await getOAuthState()
  if (!state) return

  openOAuthWindow(buildGitHubOAuthUrl(clientId, state))
}

/**
 * Handle Discord OAuth binding/login
 */
export async function handleDiscordOAuth(clientId: string): Promise<void> {
  const state = await getOAuthState()
  if (!state) return

  openOAuthWindow(buildDiscordOAuthUrl(clientId, state))
}

/**
 * Handle OIDC OAuth binding/login
 */
export async function handleOIDCOAuth(
  authUrl: string,
  clientId: string
): Promise<void> {
  const state = await getOAuthState()
  if (!state) return

  openOAuthWindow(buildOIDCOAuthUrl(authUrl, clientId, state))
}

/**
 * Handle LinuxDO OAuth binding/login
 */
export async function handleLinuxDOOAuth(clientId: string): Promise<void> {
  const state = await getOAuthState()
  if (!state) return

  openOAuthWindow(buildLinuxDOOAuthUrl(clientId, state))
}
