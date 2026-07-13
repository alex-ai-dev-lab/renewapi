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
import type { ReactNode } from 'react'

export function CardHeading(props: { title: string; icon?: ReactNode }) {
  return (
    <div className='flex items-center gap-3'>
      {props.icon && (
        <span className='bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-md'>
          {props.icon}
        </span>
      )}
      <h3 className='text-sm font-semibold tracking-tight'>{props.title}</h3>
    </div>
  )
}

export function SubHeading(props: { title: string; icon?: ReactNode }) {
  return (
    <div className='flex items-center gap-2'>
      {props.icon && (
        <span className='text-muted-foreground'>{props.icon}</span>
      )}
      <h4 className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>
        {props.title}
      </h4>
    </div>
  )
}
