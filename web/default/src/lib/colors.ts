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
export type SemanticColor =
  | 'blue'
  | 'green'
  | 'cyan'
  | 'purple'
  | 'pink'
  | 'red'
  | 'orange'
  | 'amber'
  | 'yellow'
  | 'lime'
  | 'light-green'
  | 'teal'
  | 'light-blue'
  | 'indigo'
  | 'violet'
  | 'grey'
  | 'slate'

export const colorToBgClass: Record<SemanticColor, string> = {
  blue: 'bg-blue-500',
  green: 'bg-green-500',
  cyan: 'bg-cyan-500',
  purple: 'bg-purple-500',
  pink: 'bg-pink-500',
  red: 'bg-red-500',
  orange: 'bg-orange-500',
  amber: 'bg-amber-500',
  yellow: 'bg-yellow-500',
  lime: 'bg-lime-500',
  'light-green': 'bg-green-400',
  teal: 'bg-teal-500',
  'light-blue': 'bg-sky-400',
  indigo: 'bg-indigo-500',
  violet: 'bg-violet-500',
  grey: 'bg-gray-400',
  slate: 'bg-slate-500',
}

export const avatarColorMap: Record<SemanticColor, string> = {
  blue: 'bg-tag-blue-text/10 text-tag-blue-text',
  green: 'bg-success-text/10 text-success-text',
  cyan: 'bg-tag-cyan-text/10 text-tag-cyan-text',
  purple: 'bg-tag-purple-text/10 text-tag-purple-text',
  pink: 'bg-tag-pink-text/10 text-tag-pink-text',
  red: 'bg-destructive-text/10 text-destructive-text',
  orange: 'bg-warning-text/10 text-warning-text',
  amber: 'bg-warning-text/10 text-warning-text',
  yellow: 'bg-warning-text/10 text-warning-text',
  lime: 'bg-tag-lime-text/10 text-tag-lime-text',
  'light-green': 'bg-success-text/10 text-success-text',
  teal: 'bg-tag-cyan-text/10 text-tag-cyan-text',
  'light-blue': 'bg-info-text/10 text-info-text',
  indigo: 'bg-tag-blue-text/10 text-tag-blue-text',
  violet: 'bg-tag-purple-text/10 text-tag-purple-text',
  grey: 'bg-muted text-tag-neutral-text',
  slate: 'bg-muted text-tag-neutral-text',
}

export function getAvatarColorClass(name: string): string {
  return avatarColorMap[stringToColor(name)]
}

export function getBgColorClass(color?: string): string {
  if (!color) return colorToBgClass.blue
  return (
    (colorToBgClass as Record<string, string>)[color] || colorToBgClass.blue
  )
}

/**
 * Categorical data colors, independent of the console's monochrome UI theme.
 * Keep hex values so canvas charts and translucent flow links share the palette.
 */
export const CHART_COLORS = [
  '#0084ce',
  '#d55e00',
  '#00986e',
  '#c15c94',
  '#8473dc',
  '#c063b1',
  '#83832c',
  '#398f81',
  '#cc6576',
  '#d24d90',
  '#828282',
  '#af7a46',
] as const

/**
 * Get a chart color by index (cycles through the palette)
 */
export function getChartColor(index: number): string {
  return CHART_COLORS[index % CHART_COLORS.length]
}

/**
 * Semantic colors for tags and badges
 */
const TAG_COLORS = [
  'amber',
  'blue',
  'cyan',
  'green',
  'grey',
  'indigo',
  'light-blue',
  'lime',
  'orange',
  'pink',
  'purple',
  'red',
  'teal',
  'violet',
  'yellow',
] as const

/**
 * Convert string to a stable semantic color
 * Used for model tags, group badges, user avatars, etc.
 * Same string always returns the same color
 *
 * @param str - Input string (model name, group name, username, etc.)
 * @returns Semantic color name from TAG_COLORS
 *
 * @example
 * stringToColor('gpt-4') // 'blue'
 * stringToColor('claude-3') // 'purple'
 * stringToColor('default') // 'green'
 */
export function stringToColor(str: string): SemanticColor {
  let sum = 0
  for (let i = 0; i < str.length; i++) {
    sum += str.charCodeAt(i)
  }
  const index = sum % TAG_COLORS.length
  return TAG_COLORS[index]
}

/** Stable non-color encodings; adjacent series do not rely on hue alone. */
const SERIES_DASHES = [[], [6, 3], [2, 3], [8, 3, 2, 3], [3, 2], [10, 4]]
const SERIES_SYMBOLS = [
  'circle',
  'square',
  'diamond',
  'triangle',
  'cross',
  'star',
]
export function getSeriesAppearance(index: number) {
  const value = Math.max(0, index)
  return {
    lineDash: SERIES_DASHES[value % SERIES_DASHES.length],
    symbolType:
      SERIES_SYMBOLS[
        Math.floor(value / SERIES_DASHES.length) % SERIES_SYMBOLS.length
      ],
  }
}
