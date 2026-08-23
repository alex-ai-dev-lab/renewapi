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
import { useLocation } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { useStatus } from '@/hooks/use-status'
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
  const { pathname } = useLocation()
  const { t, i18n } = useTranslation()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const user = useAuthStore((state) => state.auth.user)
  const { status } = useStatus()

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

  const isOverview = pathname.startsWith('/dashboard/overview')
  const isUsageLogsCommon = pathname.startsWith('/usage-logs/common')
  const isModelsMetadata = pathname.startsWith('/models/metadata')
  const isUsers = pathname.startsWith('/users')
  const now = new Date()
  const userName =
    user?.display_name ||
    user?.username ||
    t('aurora.common.userFallback', {
      defaultValue: isChinese ? '用户' : 'there',
    })

  let greeting = t('aurora.greeting.evening', {
    defaultValue: isChinese ? '晚安' : 'Good evening',
  })
  if (now.getHours() < 12) {
    greeting = t('aurora.greeting.morning', {
      defaultValue: isChinese ? '早上好' : 'Good morning',
    })
  } else if (now.getHours() < 18) {
    greeting = t('aurora.greeting.afternoon', {
      defaultValue: isChinese ? '下午好' : 'Good afternoon',
    })
  }

  let renderedTitle: ReactNode = title
  let renderedDescription: ReactNode = description

  if (isOverview) {
    renderedTitle = (
      <>
        {greeting}
        {t('aurora.hero.dashboard.separator', {
          defaultValue: isChinese ? '，' : ',',
        })}{' '}
        <span className='text-aurora'>{userName}</span>
        {t('aurora.hero.dashboard.ready', {
          defaultValue: isChinese ? '。网关一切就绪。' : '. Gateway ready.',
        })}
      </>
    )
    const systemName = status?.system_name || 'RenewAPI'
    renderedDescription = (
      <>
        {t('aurora.hero.dashboard.description', {
          defaultValue: isChinese
            ? '{{system}} 运营控制台 · 多供应商 AI 网关'
            : '{{system}} operations · Multi-provider AI gateway',
          system: systemName,
        })}
        {status?.version
          ? t('aurora.hero.dashboard.version', {
              defaultValue: ' · {{version}}',
              version: status.version,
            })
          : null}
      </>
    )
  } else if (isUsageLogsCommon) {
    renderedTitle = (
      <>
        {t('aurora.hero.logs.lead', {
          defaultValue: isChinese ? '调用' : 'Request',
        })}{' '}
        <span className='text-aurora'>
          {t('aurora.hero.logs.accent', {
            defaultValue: isChinese ? '日志' : 'logs',
          })}
        </span>
      </>
    )
    if (renderedDescription == null) {
      renderedDescription = t('aurora.hero.logs.description', {
        defaultValue: isChinese
          ? '每一次中继请求的可观测证据。'
          : 'Per-request observability across routing, latency, token usage and failures.',
      })
    }
  } else if (isModelsMetadata) {
    renderedTitle = (
      <>
        {t('aurora.hero.models.lead', {
          defaultValue: isChinese ? '模型与' : 'Models &',
        })}{' '}
        <span className='text-aurora'>
          {t('aurora.hero.models.accent', {
            defaultValue: isChinese ? '定价' : 'pricing',
          })}
        </span>
      </>
    )
    if (renderedDescription == null) {
      renderedDescription = t('aurora.hero.models.description', {
        defaultValue: isChinese
          ? '上游价格透明同步，倍率一目了然。'
          : 'Model registry, pricing metadata and deployment availability.',
      })
    }
  } else if (isUsers) {
    renderedTitle = (
      <>
        {t('aurora.hero.users.lead', {
          defaultValue: isChinese ? '用户与' : 'Users &',
        })}{' '}
        <span className='text-aurora'>
          {t('aurora.hero.users.accent', {
            defaultValue: isChinese ? '分组' : 'groups',
          })}
        </span>
      </>
    )
    if (renderedDescription == null) {
      renderedDescription = t('aurora.hero.users.description', {
        defaultValue: isChinese
          ? 'JWT · Passkey · OIDC 多模式身份。'
          : 'Identity, groups, quota posture and account administration.',
      })
    }
  }

  return (
    <PageFooterProvider container={footerContainer}>
      <Main className='overflow-y-auto'>
        <PageContainer
          width='fluid'
          className='mx-auto min-h-full w-full max-w-[1240px] min-w-0 flex-1 gap-0 px-4 pt-4 pb-28 sm:px-6 sm:pt-5 lg:px-6 lg:pt-4 lg:pb-32'
        >
          <header className='mb-[26px] flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between'>
            <div className='min-w-0 flex-1'>
              {breadcrumb != null && (
                <div className='text-muted-foreground mb-2 text-xs'>
                  {breadcrumb}
                </div>
              )}
              {renderedTitle != null && (
                <h1 className='text-[34px] leading-[1.05] font-extrabold tracking-[-0.032em] sm:text-[38px]'>
                  {renderedTitle}
                </h1>
              )}
              {renderedDescription != null && (
                <p className='text-muted-foreground mt-1.5 max-w-2xl text-[13px] leading-5'>
                  {renderedDescription}
                </p>
              )}
            </div>
            <div className='flex shrink-0 flex-col items-start gap-2 sm:items-end'>
              <div className='text-muted-foreground text-left text-[11px] leading-5 sm:text-right'>
                <div>
                  {new Intl.DateTimeFormat(undefined, {
                    year: 'numeric',
                    month: 'long',
                    day: 'numeric',
                    weekday: 'short',
                  }).format(now)}
                </div>
                <div className='text-foreground text-[12px] font-semibold'>
                  {new Intl.DateTimeFormat(undefined, {
                    hour: '2-digit',
                    minute: '2-digit',
                  }).format(now)}
                </div>
              </div>
              {!isOverview && actions != null && (
                <div className='flex flex-wrap items-center gap-2'>
                  {actions}
                </div>
              )}
            </div>
          </header>

          <div className='min-h-0 min-w-0 flex-1'>{content}</div>

          <div ref={setFooterContainer} className='shrink-0 empty:hidden' />
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
