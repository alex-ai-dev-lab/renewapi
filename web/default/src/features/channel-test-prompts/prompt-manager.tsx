/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { SettingsSection } from '@/features/system-settings/components/settings-section'
import {
  useChannelTestPrompts,
  usePromptMutation,
  type ChannelTestPrompt,
} from './api'
import { PromptEditor } from './prompt-editor'

export function PromptManager() {
  const { t } = useTranslation()
  const prompts = useChannelTestPrompts()
  const mutation = usePromptMutation()
  const [editing, setEditing] = useState<ChannelTestPrompt | null | undefined>()
  const [deleting, setDeleting] = useState<ChannelTestPrompt | null>(null)
  const saved = () => {
    setEditing(undefined)
    toast.success(t('Saved successfully'))
  }
  return (
    <SettingsSection title={t('Test prompt profiles')}>
      <h2 className='text-base font-semibold'>{t('Test prompt profiles')}</h2>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Each channel can select its own prompt. Anti-poison appends a nonce check.'
        )}
      </p>
      <Button
        type='button'
        variant='outline'
        onClick={() => setEditing(null)}
        disabled={mutation.isPending}
      >
        {t('New prompt')}
      </Button>
      {editing !== undefined && (
        <PromptEditor
          key={editing?.id ?? 'new'}
          initial={editing ?? undefined}
          pending={mutation.isPending}
          onCancel={() => setEditing(undefined)}
          onSave={(input) =>
            mutation.mutate(
              { action: 'save', id: editing?.id, input },
              { onSuccess: saved }
            )
          }
        />
      )}
      {prompts.isPending && <p>{t('Loading...')}</p>}
      {prompts.isError && (
        <Button
          type='button'
          variant='outline'
          onClick={() => void prompts.refetch()}
        >
          {t('Failed to load test prompts')}
        </Button>
      )}
      {prompts.data?.length === 0 && (
        <p className='text-muted-foreground text-sm'>
          {t('No prompt profiles. The legacy prompt is used.')}
        </p>
      )}
      <div className='space-y-3'>
        {prompts.data?.map((prompt) => (
          <article key={prompt.id} className='space-y-2 rounded-lg border p-3'>
            <div className='flex flex-wrap items-center gap-2'>
              <span className='font-medium break-all'>{prompt.name}</span>
              {prompt.is_default && <Badge>{t('Default')}</Badge>}
              {!prompt.enabled && (
                <Badge variant='secondary'>{t('Disabled')}</Badge>
              )}
              <span className='text-muted-foreground text-sm'>
                {t('Referenced by {{count}} channels', {
                  count: prompt.reference_count,
                })}
              </span>
            </div>
            {prompt.description && (
              <p className='text-muted-foreground text-sm break-words'>
                {prompt.description}
              </p>
            )}
            <p className='line-clamp-3 text-sm break-words whitespace-pre-wrap'>
              {prompt.prompt}
            </p>
            <div className='flex flex-wrap gap-2'>
              <Button
                type='button'
                size='sm'
                variant='outline'
                disabled={mutation.isPending}
                onClick={() => setEditing(prompt)}
              >
                {t('Edit')}
              </Button>
              <Button
                type='button'
                size='sm'
                variant='outline'
                disabled={mutation.isPending}
                onClick={() =>
                  mutation.mutate({
                    action: 'save',
                    id: prompt.id,
                    input: { ...prompt, enabled: !prompt.enabled },
                  })
                }
              >
                {prompt.enabled ? t('Disable') : t('Enable')}
              </Button>
              <Button
                type='button'
                size='sm'
                variant='outline'
                disabled={
                  mutation.isPending || !prompt.enabled || prompt.is_default
                }
                onClick={() =>
                  mutation.mutate({
                    action: 'save',
                    id: prompt.id,
                    input: { ...prompt, is_default: true },
                  })
                }
              >
                {t('Set as default')}
              </Button>
              <Button
                type='button'
                size='sm'
                variant='destructive'
                disabled={mutation.isPending || prompt.reference_count > 0}
                onClick={() => setDeleting(prompt)}
              >
                {t('Delete')}
              </Button>
            </div>
          </article>
        ))}
      </div>
      <ConfirmDialog
        open={!!deleting}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
        title={t('Delete prompt')}
        desc={deleting?.name ?? ''}
        destructive
        isLoading={mutation.isPending}
        confirmText={t('Delete')}
        handleConfirm={() => {
          if (deleting)
            mutation.mutate(
              { action: 'delete', id: deleting.id },
              { onSuccess: () => setDeleting(null) }
            )
        }}
      />
    </SettingsSection>
  )
}
