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
import type { UserProfile, UserSettings } from '../types'

// ============================================================================
// Profile Formatting Utilities
// ============================================================================

/**
 * Parse user settings from JSON string
 */
export function parseUserSettings(settingsJson?: string): UserSettings {
  if (!settingsJson) return {}

  try {
    return JSON.parse(settingsJson) as UserSettings
  } catch {
    return {}
  }
}

/**
 * Get display name or fallback to username
 */
export function getDisplayName(user?: UserProfile): string {
  if (!user) return ''
  return user.display_name || user.username
}

/**
 * Get user initials for avatar
 */
export function getUserInitials(user?: UserProfile): string {
  if (!user) return '?'
  const name = getDisplayName(user)
  if (!name) return '?'

  const parts = name.trim().split(/\s+/)
  if (parts.length >= 2) {
    return `${parts[0][0]}${parts[1][0]}`.toUpperCase()
  }
  return name.slice(0, 2).toUpperCase()
}

/**
 * Format a quota/rate limit value.
 *
 * Returns `null` when the value is unknown so callers can render an
 * unavailable state instead of inventing a finite or infinite value.
 * `0` means unlimited and renders as the infinity symbol.
 */
export function formatLimit(value: number | undefined | null): string | null {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return null
  }
  if (value <= 0) {
    return '∞'
  }
  return value.toLocaleString()
}
