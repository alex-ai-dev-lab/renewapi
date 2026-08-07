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
/**
 * LobeHub Icon Loader
 * Dynamically load and render icons from @lobehub/icons
 *
 * Supports:
 * - Basic: "OpenAI", "OpenAI.Color"
 * - Chained properties: "OpenAI.Avatar.type={'platform'}"
 * - Size parameter: getLobeIcon("OpenAI", 20)
 */
import { useEffect, useState, type ComponentType, type ReactNode } from 'react'
import { COMMON_LOBE_ICONS } from './lobe-icon-common'

type IconComponent = ComponentType<Record<string, unknown>>
type IconRegistry = Record<string, unknown>

let extensionIcons: IconRegistry | null = null
let extensionIconsPromise: Promise<IconRegistry> | null = null

function loadExtensionIcons(): Promise<IconRegistry> {
  if (!extensionIconsPromise) {
    extensionIconsPromise = import('@lobehub/icons').then((icons) => {
      extensionIcons = icons as IconRegistry
      return extensionIcons
    })
  }
  return extensionIconsPromise
}

function getIconEntry(icons: IconRegistry, key: string): unknown {
  return Object.prototype.hasOwnProperty.call(icons, key)
    ? icons[key]
    : undefined
}

/**
 * Parse a property value from string to appropriate type
 * @param raw - Raw string value
 * @returns Parsed value (boolean, number, or string)
 */
function parseValue(raw: string | undefined | null): string | number | boolean {
  if (raw == null) return true

  let v = String(raw).trim()

  // Remove curly braces
  if (v.startsWith('{') && v.endsWith('}')) {
    v = v.slice(1, -1).trim()
  }

  // Remove quotes
  if (
    (v.startsWith('"') && v.endsWith('"')) ||
    (v.startsWith("'") && v.endsWith("'"))
  ) {
    return v.slice(1, -1)
  }

  // Boolean
  if (v === 'true') return true
  if (v === 'false') return false

  // Number
  if (/^-?\d+(?:\.\d+)?$/.test(v)) return Number(v)

  // Return as string
  return v
}

/**
 * Get LobeHub icon component by name
 * @param iconName - Icon name/description (e.g., "OpenAI", "OpenAI.Color", "Claude.Avatar")
 * @param size - Icon size (default: 20)
 * @returns Icon component or fallback
 *
 * @example
 * getLobeIcon("OpenAI", 24)
 * getLobeIcon("OpenAI.Color", 20)
 * getLobeIcon("Claude.Avatar.type={'platform'}", 32)
 */
function renderFallback(label: string, size: number): ReactNode {
  return (
    <div
      className='bg-muted text-muted-foreground flex items-center justify-center rounded-full text-xs font-medium'
      style={{ width: size, height: size }}
    >
      {label}
    </div>
  )
}

function renderIcon(
  trimmedName: string,
  size: number,
  icons: IconRegistry
): ReactNode {
  // Parse component path and chained properties
  const segments = trimmedName.split('.')
  const baseKey = segments[0]
  const BaseIcon = getIconEntry(icons, baseKey)
  const variantKey = segments[1]
  const VariantIcon =
    BaseIcon && variantKey
      ? getIconEntry(BaseIcon as IconRegistry, variantKey)
      : undefined

  let IconComponent: IconComponent | undefined
  let propStartIndex: number

  if (VariantIcon) {
    IconComponent = VariantIcon as IconComponent
    propStartIndex = 2
  } else {
    IconComponent = BaseIcon as IconComponent | undefined
    propStartIndex = segments.length > 1 && /^[A-Z]/.test(segments[1]) ? 2 : 1
  }

  // Fallback if icon not found
  if (
    !IconComponent ||
    (typeof IconComponent !== 'function' && typeof IconComponent !== 'object')
  ) {
    const firstLetter = trimmedName.charAt(0).toUpperCase()
    return renderFallback(firstLetter, size)
  }

  // Parse chained properties (e.g., "type={'platform'}", "shape='square'")
  const props: Record<string, string | number | boolean> = {}

  for (let i = propStartIndex; i < segments.length; i++) {
    const seg = segments[i]
    if (!seg) continue

    const eqIdx = seg.indexOf('=')
    if (eqIdx === -1) {
      props[seg.trim()] = true
      continue
    }

    const key = seg.slice(0, eqIdx).trim()
    const valRaw = seg.slice(eqIdx + 1).trim()
    props[key] = parseValue(valRaw)
  }

  // Set size if not explicitly specified in the string
  if (props.size == null && size != null) {
    props.size = size
  }

  return <IconComponent {...props} />
}

// eslint-disable-next-line react-refresh/only-export-components
function LobeIconRenderer(props: { iconName: string; size: number }) {
  const baseKey = props.iconName.split('.')[0]
  const commonIcon = getIconEntry(COMMON_LOBE_ICONS, baseKey)
  const [loadedExtensionIcons, setLoadedExtensionIcons] =
    useState<IconRegistry | null>(() => extensionIcons)

  useEffect(() => {
    if (commonIcon || loadedExtensionIcons) return

    let active = true
    void loadExtensionIcons()
      .then((icons) => {
        if (active) setLoadedExtensionIcons(icons)
      })
      .catch(() => {
        if (active) setLoadedExtensionIcons({})
      })

    return () => {
      active = false
    }
  }, [commonIcon, loadedExtensionIcons])

  const icons = commonIcon ? COMMON_LOBE_ICONS : loadedExtensionIcons
  if (!icons) {
    return renderFallback(props.iconName.charAt(0).toUpperCase(), props.size)
  }

  return renderIcon(props.iconName, props.size, icons)
}

export function getLobeIcon(
  iconName: string | undefined | null,
  size: number = 20
): ReactNode {
  if (!iconName || typeof iconName !== 'string') {
    return renderFallback('?', size)
  }

  const trimmedName = iconName.trim()
  if (!trimmedName) {
    return renderFallback('?', size)
  }

  return <LobeIconRenderer iconName={trimmedName} size={size} />
}
