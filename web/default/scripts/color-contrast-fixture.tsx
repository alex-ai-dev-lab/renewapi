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
import { writeFileSync } from 'node:fs'
import { renderToStaticMarkup } from 'react-dom/server'
import {
  StatusBadge,
  textColorMap,
  type StatusVariant,
} from '../src/components/status-badge'
import { avatarColorMap } from '../src/lib/colors'
import { THEME_PRESETS } from '../src/lib/theme-customization'

// Render actual component markup; the browser runner supplies the production CSS.
const samples = (
  <>
    {Object.keys(textColorMap).map((variant) =>
      (['badge', 'text', 'underline'] as const).map((type) => (
        <StatusBadge
          key={`${variant}-${type}`}
          variant={variant as StatusVariant}
          type={type}
          label={`${variant} status`}
          showDot
          copyable={false}
          data-contrast={`status:${variant}:${type}`}
        />
      ))
    )}
    {Object.entries(avatarColorMap).map(([name, className]) => (
      <span key={name} className={className} data-contrast={`avatar:${name}`}>
        Ab
      </span>
    ))}
  </>
)
writeFileSync(
  process.argv[2],
  JSON.stringify({
    markup: renderToStaticMarkup(samples),
    presets: THEME_PRESETS.map((p) => p.value),
  })
)
