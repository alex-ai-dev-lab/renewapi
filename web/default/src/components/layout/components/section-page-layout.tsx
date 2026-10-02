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
import {
  Children,
  isValidElement,
  useState,
  type ReactElement,
  type ReactNode,
} from 'react'
import { PageContainer } from '@/components/page-primitives'
import { Main } from './main'
import { PageFooterProvider } from './page-footer'

type SlotProps = { children?: ReactNode }

function SectionPageLayoutTitle(_props: SlotProps) {
  return null
}
SectionPageLayoutTitle.displayName = 'SectionPageLayout.Title'

function SectionPageLayoutActions(_props: SlotProps) {
  return null
}
SectionPageLayoutActions.displayName = 'SectionPageLayout.Actions'

function SectionPageLayoutDescription(_props: SlotProps) {
  return null
}
SectionPageLayoutDescription.displayName = 'SectionPageLayout.Description'

function SectionPageLayoutContent(_props: SlotProps) {
  return null
}
SectionPageLayoutContent.displayName = 'SectionPageLayout.Content'

function SectionPageLayoutBreadcrumb(_props: SlotProps) {
  return null
}
SectionPageLayoutBreadcrumb.displayName = 'SectionPageLayout.Breadcrumb'

export type SectionPageLayoutProps = {
  children: ReactNode
}

export function SectionPageLayout(props: SectionPageLayoutProps) {
  const [footerContainer, setFooterContainer] = useState<HTMLDivElement | null>(
    null
  )
  let title: ReactNode = null
  let description: ReactNode = null
  let actions: ReactNode = null
  let content: ReactNode = null
  let breadcrumb: ReactNode = null

  Children.forEach(props.children, (node) => {
    if (!isValidElement(node)) return
    const child = node as ReactElement<SlotProps>
    if (child.type === SectionPageLayoutTitle) title = child.props.children
    else if (child.type === SectionPageLayoutDescription)
      description = child.props.children
    else if (child.type === SectionPageLayoutActions)
      actions = child.props.children
    else if (child.type === SectionPageLayoutContent)
      content = child.props.children
    else if (child.type === SectionPageLayoutBreadcrumb)
      breadcrumb = child.props.children
  })

  return (
    <PageFooterProvider container={footerContainer}>
      <Main className='obsidian-section-page overflow-y-auto'>
        <PageContainer
          width='fluid'
          className='mx-auto min-h-full w-full max-w-[1600px] min-w-0 flex-none gap-4 px-4 py-4 sm:px-6 sm:py-5'
        >
          {(title != null ||
            description != null ||
            actions != null ||
            breadcrumb != null) && (
            <header className='flex shrink-0 flex-col gap-3 sm:flex-row sm:items-end sm:justify-between'>
              <div className='min-w-0 flex-1'>
                {breadcrumb != null && (
                  <div className='text-muted-foreground mb-2 text-xs'>
                    {breadcrumb}
                  </div>
                )}
                {title != null && (
                  <h1 className='text-[26px] leading-tight font-semibold tracking-tight'>
                    {title}
                  </h1>
                )}
                {description != null && (
                  <p className='text-muted-foreground mt-1 max-w-3xl text-[13px] leading-5'>
                    {description}
                  </p>
                )}
              </div>
              {actions != null && (
                <div className='flex min-w-0 shrink-0 flex-wrap items-center gap-2'>
                  {actions}
                </div>
              )}
            </header>
          )}

          <div className='min-w-0 flex-1'>{content}</div>
          {/* Stays in page flow: portal pagination cannot cover the last table row. */}
          <div
            ref={setFooterContainer}
            className='min-w-0 shrink-0 empty:hidden'
          />
        </PageContainer>
      </Main>
    </PageFooterProvider>
  )
}

SectionPageLayout.Title = SectionPageLayoutTitle
SectionPageLayout.Description = SectionPageLayoutDescription
SectionPageLayout.Actions = SectionPageLayoutActions
SectionPageLayout.Content = SectionPageLayoutContent
SectionPageLayout.Breadcrumb = SectionPageLayoutBreadcrumb
