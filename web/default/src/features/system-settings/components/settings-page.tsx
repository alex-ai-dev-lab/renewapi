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
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { useParams, useSearch } from '@tanstack/react-router'
import { getApiErrorMessage } from '@/lib/api-errors'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { useSystemOptions, getOptionValue } from '../hooks/use-system-options'
import { useSystemSettingsTranslation } from '../lib/i18n'
import type { SystemOption } from '../types'
import { SettingsPageProvider } from './settings-page-context'

type SettingsPageProps<
  TSettings extends Record<string, string | number | boolean | unknown[]>,
  TSectionId extends string,
  TExtraArgs extends unknown[] = [],
> = {
  routePath: string
  defaultSettings: TSettings
  defaultSection: TSectionId
  getSectionContent: (
    sectionId: TSectionId,
    settings: TSettings,
    ...extraArgs: TExtraArgs
  ) => ReactNode
  getSectionMeta: (sectionId: TSectionId) => {
    titleKey: string
  }
  extraArgs?: TExtraArgs
  loadingMessage?: string
  resolveSettings?: (
    settings: TSettings,
    raw: SystemOption[] | undefined
  ) => TSettings
}

type SettingsPageFrameProps = {
  title: ReactNode
  children: ReactNode
}

function SettingsPageFrame(props: SettingsPageFrameProps) {
  const [actionsContainer, setActionsContainer] =
    useState<HTMLDivElement | null>(null)
  const [titleStatusContainer, setTitleStatusContainer] =
    useState<HTMLSpanElement | null>(null)
  const actionsContainerRef = useRef<HTMLDivElement | null>(null)
  const titleStatusContainerRef = useRef<HTMLSpanElement | null>(null)

  useLayoutEffect(() => {
    setActionsContainer(actionsContainerRef.current)
    setTitleStatusContainer(titleStatusContainerRef.current)
  }, [])

  return (
    <SettingsPageProvider
      actionsContainer={actionsContainer}
      actionsContainerReady={Boolean(actionsContainer)}
      titleStatusContainer={titleStatusContainer}
    >
      <SectionPageLayout>
        <SectionPageLayout.Title>
          <span className='inline-flex max-w-full min-w-0 items-center gap-2 align-middle'>
            <span className='truncate font-extrabold tracking-[-0.02em]'>
              {props.title}
            </span>
            <span
              ref={titleStatusContainerRef}
              className='inline-flex shrink-0'
            />
          </span>
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <div
            ref={actionsContainerRef}
            className='flex flex-wrap items-center justify-end gap-2'
          />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-5xl flex-col gap-4 sm:gap-5'>
            {props.children}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
    </SettingsPageProvider>
  )
}

export function SettingsPage<
  TSettings extends Record<string, string | number | boolean | unknown[]>,
  TSectionId extends string,
  TExtraArgs extends unknown[] = [],
>({
  defaultSettings,
  defaultSection,
  getSectionContent,
  getSectionMeta,
  extraArgs,
  loadingMessage = 'Loading settings...',
  resolveSettings,
}: SettingsPageProps<TSettings, TSectionId, TExtraArgs>) {
  const { t, ts } = useSystemSettingsTranslation()
  const { data, error, isError, isLoading, refetch } = useSystemOptions()
  const params = useParams({ strict: false }) as { section?: string }
  const search = useSearch({ strict: false }) as { section?: string }
  const activeSection = (params?.section ??
    search?.section ??
    defaultSection) as TSectionId
  const sectionMeta = getSectionMeta(activeSection)

  const settings = useMemo(() => {
    const baseSettings = getOptionValue(
      data?.data,
      defaultSettings
    ) as TSettings
    return resolveSettings
      ? resolveSettings(baseSettings, data?.data)
      : baseSettings
  }, [data?.data, defaultSettings, resolveSettings])

  if (isLoading) {
    return (
      <SettingsPageFrame title={t(sectionMeta.titleKey)}>
        <div className='border-border/60 bg-card/55 text-muted-foreground flex min-h-40 items-center justify-center rounded-[calc(var(--radius)*1.125)] border text-sm'>
          {ts('settings.common.loading', {
            defaultValue: loadingMessage,
          })}
        </div>
      </SettingsPageFrame>
    )
  }

  if (isError) {
    return (
      <SettingsPageFrame title={t(sectionMeta.titleKey)}>
        <ErrorState
          title={ts('settings.common.loadErrorTitle', {
            defaultValue: 'Unable to load settings',
          })}
          description={getApiErrorMessage(
            error,
            ts('settings.common.loadError', {
              defaultValue: 'The settings could not be loaded. Please retry.',
            })
          )}
          onRetry={() => void refetch()}
        />
      </SettingsPageFrame>
    )
  }

  const sectionContent = getSectionContent(
    activeSection,
    settings,
    ...((extraArgs ?? []) as TExtraArgs)
  )

  return (
    <SettingsPageFrame title={t(sectionMeta.titleKey)}>
      {sectionContent}
    </SettingsPageFrame>
  )
}
