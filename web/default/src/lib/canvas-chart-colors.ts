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
export type CanvasChartColors = {
  background: string
  foreground: string
  mutedForeground: string
  border: string
  primary: string
  chart1: string
  success: string
  warning: string
  destructive: string
  text: string
  grid: string
  series: string[]
}

const LIGHT_CANVAS_CHART_COLORS: CanvasChartColors = {
  background: 'rgb(255, 255, 255)',
  foreground: 'rgb(23, 23, 23)',
  mutedForeground: 'rgb(115, 115, 115)',
  border: 'rgb(229, 229, 229)',
  primary: 'rgb(23, 23, 23)',
  chart1: 'rgb(0, 112, 243)',
  success: 'rgb(5, 150, 105)',
  warning: 'rgb(217, 119, 6)',
  destructive: 'rgb(220, 38, 38)',
  text: 'rgba(23, 23, 23, 0.58)',
  grid: 'rgba(23, 23, 23, 0.12)',
  // Vercel/Geist-style high-contrast categorical palette (no black/near-black).
  series: [
    'rgb(0, 112, 243)', // #0070F3 blue
    'rgb(121, 40, 202)', // #7928CA purple
    'rgb(245, 166, 35)', // #F5A623 amber
    'rgb(229, 72, 77)', // #E5484D red
    'rgb(18, 165, 148)', // #12A594 teal
    'rgb(235, 54, 127)', // #EB367F pink
    'rgb(80, 227, 194)', // #50E3C2 light teal
    'rgb(249, 115, 22)', // #F97316 orange
    'rgb(139, 92, 246)', // #8B5CF6 violet
    'rgb(12, 206, 107)', // #0CCE6B green
    'rgb(217, 119, 87)', // terracotta
    'rgb(38, 38, 38)', // graphite
  ],
}

const DARK_CANVAS_CHART_COLORS: CanvasChartColors = {
  background: 'rgb(30, 30, 30)',
  foreground: 'rgb(245, 245, 245)',
  mutedForeground: 'rgb(190, 190, 190)',
  border: 'rgba(255, 255, 255, 0.16)',
  primary: 'rgb(245, 245, 245)',
  chart1: 'rgb(56, 153, 255)',
  success: 'rgb(52, 211, 153)',
  warning: 'rgb(251, 191, 36)',
  destructive: 'rgb(248, 113, 113)',
  text: 'rgba(245, 245, 245, 0.68)',
  grid: 'rgba(245, 245, 245, 0.12)',
  // Brightened variants of the same hues for readability on dark backgrounds.
  series: [
    'rgb(56, 153, 255)', // blue (lighter #0070F3)
    'rgb(165, 110, 240)', // purple (lighter #7928CA)
    'rgb(247, 184, 75)', // amber (lighter #F5A623)
    'rgb(240, 110, 115)', // red (lighter #E5484D)
    'rgb(45, 196, 178)', // teal (lighter #12A594)
    'rgb(242, 105, 158)', // pink (lighter #EB367F)
    'rgb(110, 235, 208)', // light teal (lighter #50E3C2)
    'rgb(251, 146, 60)', // orange (lighter #F97316)
    'rgb(167, 139, 250)', // violet (lighter #8B5CF6)
    'rgb(74, 222, 128)', // green (lighter #0CCE6B)
    'rgb(240, 151, 120)', // terracotta
    'rgb(245, 245, 245)', // graphite on dark
  ],
}

export function getCanvasChartColors(theme?: string): CanvasChartColors {
  return theme === 'dark' ? DARK_CANVAS_CHART_COLORS : LIGHT_CANVAS_CHART_COLORS
}
