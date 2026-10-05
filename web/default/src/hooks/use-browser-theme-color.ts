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
import { useEffect } from 'react'

/** Browser chrome follows the rendered page even when no theme switch is mounted. */
export function useBrowserThemeColor() {
  useEffect(() => {
    let frame = 0
    const update = () => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        let meta = document.querySelector<HTMLMetaElement>(
          'meta[name="theme-color"]'
        )
        if (!meta) {
          meta = document.createElement('meta')
          meta.name = 'theme-color'
          document.head.append(meta)
        }
        meta.content = getComputedStyle(document.body).backgroundColor
      })
    }
    const observer = new MutationObserver((records) => {
      const relevant = records.some((record) => {
        if (record.type === 'attributes')
          return (
            record.target === document.body ||
            record.target === document.documentElement
          )
        return [...record.addedNodes, ...record.removedNodes].some(
          (node) =>
            node instanceof Element &&
            (node.matches('.snowapi-auth-shell, .snowapi-auth-route') ||
              node.querySelector('.snowapi-auth-shell, .snowapi-auth-route'))
        )
      })
      if (relevant) update()
    })
    const options = {
      attributes: true,
      attributeFilter: [
        'class',
        'data-site-design',
        'data-theme-preset',
        'data-snowapi-console',
      ],
    }
    observer.observe(document.documentElement, options)
    observer.observe(document.body, {
      ...options,
      childList: true,
      subtree: true,
    })
    document.body.addEventListener('transitionend', update)
    update()
    return () => {
      observer.disconnect()
      document.body.removeEventListener('transitionend', update)
      cancelAnimationFrame(frame)
    }
  }, [])
}
