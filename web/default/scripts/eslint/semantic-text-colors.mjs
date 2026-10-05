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
export default {
  meta: {
    type: 'problem',
    schema: [],
    messages: {
      fillText:
        'Use text-{{color}}-text for readable text; {{color}} is a fill token.',
    },
  },
  create(context) {
    const check = (node, value) => {
      if (typeof value !== 'string') return
      for (const match of value.matchAll(
        /(?<![\w-])text-(success|warning|info|destructive)(?![\w-])/g
      )) {
        context.report({
          node,
          messageId: 'fillText',
          data: { color: match[1] },
        })
      }
    }
    return {
      Literal(node) {
        check(node, node.value)
      },
      TemplateElement(node) {
        check(node, node.value.raw)
      },
    }
  },
}
