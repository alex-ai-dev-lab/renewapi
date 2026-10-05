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
import { useState, type ComponentProps } from 'react'
import { DEFAULT_LOGO } from '@/lib/constants'
import { cn } from '@/lib/utils'

type BrandImageProps = ComponentProps<'img'> & { alt: string }

function BrandImageSource(props: BrandImageProps) {
  const [failure, setFailure] = useState(0)
  const { src, alt, className, onError, ...imageProps } = props
  const source = failure === 0 && src ? src : DEFAULT_LOGO
  if (failure === 2) {
    return (
      <span
        className={cn(
          'inline-flex items-center justify-center rounded font-semibold',
          className
        )}
        role={alt ? 'img' : undefined}
        aria-label={alt || undefined}
        aria-hidden={props['aria-hidden']}
      >
        {alt.trim().slice(0, 1) || 'R'}
      </span>
    )
  }
  return (
    <img
      {...imageProps}
      src={source}
      alt={alt}
      className={className}
      onError={(event) => {
        setFailure(source === DEFAULT_LOGO ? 2 : 1)
        onError?.(event)
      }}
    />
  )
}

/** Keep configured artwork unchanged; failed external assets never leave a broken-image box. */
export function BrandImage(props: BrandImageProps) {
  return <BrandImageSource key={props.src} {...props} />
}
