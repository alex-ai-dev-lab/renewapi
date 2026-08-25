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
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import {
  ArrowUpFromLine,
  DollarSign,
  MoreHorizontal,
  Plus,
  RefreshCw,
  Settings2,
  SortAsc,
  Tags,
  TestTube,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  handleDeleteAllDisabled,
  handleFixAbilities,
  handleTestAllChannels,
  handleUpdateAllBalances,
} from '../lib'
import { useChannels } from './channels-provider'

type ChannelsPrimaryButtonsProps = {
  variant?: 'all' | 'create' | 'tools'
}

export function ChannelsPrimaryButtons(props: ChannelsPrimaryButtonsProps) {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const isChinese = i18n.resolvedLanguage?.startsWith('zh') ?? false
  const { enableTagMode, setEnableTagMode, idSort, setIdSort, upstream } =
    useChannels()
  const queryClient = useQueryClient()
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const variant = props.variant ?? 'all'
  const showCreate = variant !== 'tools'
  const showTools = variant !== 'create'

  const handleTagModeToggle = (checked: boolean) => {
    localStorage.setItem('enable-tag-mode', String(checked))
    setEnableTagMode(checked)
  }

  const handleIdSortToggle = (checked: boolean) => {
    localStorage.setItem('channels-id-sort', String(checked))
    setIdSort(checked)
  }

  return (
    <>
      <div className='flex flex-wrap items-center gap-2'>
        {showTools ? (
          <>
            <div className='hidden items-center gap-2 rounded-md border px-2.5 py-1 sm:flex'>
              <Tags className='text-muted-foreground size-3.5' />
              <Label htmlFor='tag-mode' className='cursor-pointer text-xs'>
                {t('Tag Mode')}
              </Label>
              <Switch
                id='tag-mode'
                checked={enableTagMode}
                onCheckedChange={handleTagModeToggle}
              />
            </div>

            <div className='hidden items-center gap-2 rounded-md border px-2.5 py-1 sm:flex'>
              <SortAsc className='text-muted-foreground size-3.5' />
              <Label htmlFor='id-sort' className='cursor-pointer text-xs'>
                {t('Sort by ID')}
              </Label>
              <Switch
                id='id-sort'
                checked={idSort}
                onCheckedChange={handleIdSortToggle}
              />
            </div>
          </>
        ) : null}

        {showCreate ? (
          <Button
            onClick={() => {
              void navigate({ to: '/channels/new' })
            }}
            size='sm'
          >
            <Plus className='size-4' />
            <span className='max-sm:hidden'>
              {t('aurora.channels.create', {
                defaultValue: isChinese ? '新增渠道' : 'Create Channel',
              })}
            </span>
            <span className='sm:hidden'>{t('Create')}</span>
          </Button>
        ) : null}

        {showTools ? (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  variant='outline'
                  size='sm'
                  aria-label={t('Open menu')}
                />
              }
            >
              <MoreHorizontal className='size-4' />
            </DropdownMenuTrigger>
            <DropdownMenuContent align='end' className='w-56'>
              <DropdownMenuCheckboxItem
                className='sm:hidden'
                checked={enableTagMode}
                onCheckedChange={handleTagModeToggle}
              >
                <Tags className='mr-2 size-4' />
                {t('Tag Mode')}
              </DropdownMenuCheckboxItem>

              <DropdownMenuCheckboxItem
                className='sm:hidden'
                checked={idSort}
                onCheckedChange={handleIdSortToggle}
              >
                <SortAsc className='mr-2 size-4' />
                {t('Sort by ID')}
              </DropdownMenuCheckboxItem>

              <DropdownMenuSeparator className='sm:hidden' />

              <DropdownMenuItem
                onClick={() => {
                  handleTestAllChannels(queryClient)
                }}
              >
                {t('Test All Channels')}
                <DropdownMenuShortcut>
                  <TestTube className='size-4' />
                </DropdownMenuShortcut>
              </DropdownMenuItem>

              <DropdownMenuItem
                onClick={() => {
                  handleUpdateAllBalances(queryClient)
                }}
              >
                {t('Update All Balances')}
                <DropdownMenuShortcut>
                  <DollarSign className='size-4' />
                </DropdownMenuShortcut>
              </DropdownMenuItem>

              <DropdownMenuSeparator />

              <DropdownMenuItem
                onClick={() => upstream.detectAllUpdates()}
                disabled={upstream.detectAllLoading}
              >
                {t('Detect All Upstream Updates')}
                <DropdownMenuShortcut>
                  <RefreshCw className='size-4' />
                </DropdownMenuShortcut>
              </DropdownMenuItem>

              <DropdownMenuItem
                onClick={() => upstream.applyAllUpdates()}
                disabled={upstream.applyAllLoading}
              >
                {t('Apply All Upstream Updates')}
                <DropdownMenuShortcut>
                  <ArrowUpFromLine className='size-4' />
                </DropdownMenuShortcut>
              </DropdownMenuItem>

              <DropdownMenuSeparator />

              <DropdownMenuItem
                onClick={() => {
                  handleFixAbilities(queryClient, (_result) => {
                    // eslint-disable-next-line no-console
                    console.log('Fix abilities result:', _result)
                  })
                }}
              >
                {t('Fix Abilities')}
                <DropdownMenuShortcut>
                  <Settings2 className='size-4' />
                </DropdownMenuShortcut>
              </DropdownMenuItem>

              <DropdownMenuSeparator />

              <DropdownMenuItem
                onSelect={(event) => {
                  event.preventDefault()
                  setShowDeleteDialog(true)
                }}
                className='text-destructive focus:text-destructive'
              >
                {t('Delete All Disabled')}
                <DropdownMenuShortcut>
                  <Trash2 className='size-4' />
                </DropdownMenuShortcut>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        ) : null}
      </div>

      {showTools ? (
        <ConfirmDialog
          open={showDeleteDialog}
          onOpenChange={setShowDeleteDialog}
          title={t('Delete All Disabled Channels?')}
          desc={t(
            'This will permanently delete all manually and automatically disabled channels. This action cannot be undone.'
          )}
          destructive
          handleConfirm={() => {
            handleDeleteAllDisabled(queryClient, (_count) => {
              // eslint-disable-next-line no-console
              console.log(`Deleted ${_count} channels`)
            })
            setShowDeleteDialog(false)
          }}
        />
      ) : null}
    </>
  )
}
