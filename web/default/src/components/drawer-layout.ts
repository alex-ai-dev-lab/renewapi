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
import { createElement, type ReactNode } from 'react'
import { cn } from '@/lib/utils'

export const sideDrawerContentClassName = (className?: string) =>
  cn(
    'bg-background/90 text-foreground flex h-dvh w-full flex-col gap-0 overflow-hidden p-0 shadow-none backdrop-blur-xl',
    className
  )

export const sideDrawerHeaderClassName = (className?: string) =>
  cn(
    'border-border/60 bg-background/70 border-b px-4 py-3 text-start backdrop-blur-xl backdrop-saturate-150 sm:px-6 sm:py-4',
    className
  )

export const sideDrawerFormClassName = (className?: string) =>
  cn(
    'flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto overscroll-contain px-4 py-4 sm:gap-5 sm:px-6 sm:py-5',
    className
  )

export const sideDrawerFooterClassName = (className?: string) =>
  cn(
    'border-border/60 bg-background/70 grid grid-cols-2 gap-2 border-t px-4 py-3 backdrop-blur-xl backdrop-saturate-150 sm:flex sm:flex-row sm:justify-end sm:px-6 sm:py-4',
    className
  )

export const sideDrawerSectionClassName = (className?: string) =>
  cn(
    'border-border/60 bg-card/55 flex flex-col gap-4 rounded-[calc(var(--radius)*1.125)] border p-4 shadow-sm sm:p-5',
    className
  )

export const sideDrawerSwitchItemClassName = (className?: string) =>
  cn(
    'border-border/60 bg-muted/20 flex min-h-16 flex-row items-center justify-between gap-3 rounded-[calc(var(--radius)*0.75)] border px-3 py-3',
    className
  )

export function SideDrawerSection(props: {
  children: ReactNode
  className?: string
}) {
  return createElement(
    'section',
    { className: sideDrawerSectionClassName(props.className) },
    props.children
  )
}

export function SideDrawerSectionHeader(props: {
  title: ReactNode
  description?: ReactNode
  icon?: ReactNode
  className?: string
}) {
  return createElement(
    'div',
    { className: cn('flex items-start gap-3', props.className) },
    props.icon
      ? createElement(
          'span',
          {
            className:
              'bg-primary/10 text-primary ring-primary/10 flex size-8 shrink-0 items-center justify-center rounded-[calc(var(--radius)*0.75)] ring-1',
          },
          props.icon
        )
      : null,
    createElement(
      'div',
      { className: 'min-w-0 flex-1' },
      createElement(
        'h3',
        {
          className:
            'text-sm leading-none font-semibold tracking-[-0.01em]',
        },
        props.title
      ),
      props.description
        ? createElement(
            'p',
            { className: 'text-muted-foreground mt-1.5 text-xs leading-5' },
            props.description
          )
        : null
    )
  )
}
