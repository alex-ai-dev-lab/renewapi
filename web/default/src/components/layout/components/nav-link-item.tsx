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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import type { TopNavLink } from '../types'

interface NavLinkItemProps {
  link: TopNavLink
  className?: string
  isActive?: boolean
  onClick?: React.MouseEventHandler<HTMLAnchorElement>
}

/** Shared internal, external and hash link semantics for both navigation sizes. */
export function NavLinkItem(props: NavLinkItemProps) {
  const { t } = useTranslation()
  const linkClassName = cn(
    'text-muted-foreground hover:text-foreground transition-colors',
    props.isActive && 'bg-accent text-accent-foreground',
    props.link.disabled && 'pointer-events-none opacity-50',
    props.className
  )
  const linkProps = {
    className: linkClassName,
    onClick: props.onClick,
    'aria-disabled': props.link.disabled,
    'aria-current': props.isActive ? ('page' as const) : undefined,
    tabIndex: props.link.disabled ? -1 : undefined,
  }

  if (props.link.external || props.link.href.startsWith('#')) {
    return (
      <a
        href={props.link.href}
        target={props.link.external ? '_blank' : undefined}
        rel={props.link.external ? 'noopener noreferrer' : undefined}
        {...linkProps}
      >
        {t(props.link.title)}
      </a>
    )
  }

  return (
    <Link to={props.link.href} disabled={props.link.disabled} {...linkProps}>
      {t(props.link.title)}
    </Link>
  )
}

interface NavLinkListProps {
  links: TopNavLink[]
  className?: string
  itemClassName?: string
}

/**
 * Renders a list of navigation links
 * Used in both desktop and mobile navigation
 */
export function NavLinkList({
  links,
  className,
  itemClassName,
}: NavLinkListProps) {
  return (
    <>
      {links.map((link, index) => (
        <NavLinkItem
          key={index}
          link={link}
          className={cn(className, itemClassName)}
        />
      ))}
    </>
  )
}
