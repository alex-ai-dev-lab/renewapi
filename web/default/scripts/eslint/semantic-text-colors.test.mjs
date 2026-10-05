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
import { RuleTester } from 'eslint'
import rule from './semantic-text-colors.mjs'

new RuleTester().run('semantic-text-colors', rule, {
  valid: [
    "const cls = 'text-success-text hover:text-warning-text'",
    "const cls = 'text-success-foreground bg-success/10'",
    'const cls = `text-info-text ${value}`',
  ],
  invalid: [
    { code: "const cls = 'text-success'", errors: [{ messageId: 'fillText' }] },
    {
      code: "const cls = 'hover:text-warning/80'",
      errors: [{ messageId: 'fillText' }],
    },
    {
      code: 'const cls = `text-destructive ${value}`',
      errors: [{ messageId: 'fillText' }],
    },
  ],
})
