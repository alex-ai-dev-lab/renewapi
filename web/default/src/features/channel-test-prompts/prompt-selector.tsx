/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import { useTranslation } from 'react-i18next'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useChannelTestPrompts } from './api'

export function PromptSelector(props: {
  value?: number
  onChange: (id: number) => void
  id?: string
}) {
  const { t } = useTranslation()
  const prompts = useChannelTestPrompts()
  const selected = prompts.data?.find((prompt) => prompt.id === props.value)
  return (
    <div className='space-y-1'>
      <Select
        value={String(props.value ?? 0)}
        onValueChange={(value) => props.onChange(Number(value ?? 0))}
        disabled={prompts.isPending || prompts.isError}
      >
        <SelectTrigger id={props.id}>
          <SelectValue>
            {selected?.name ?? t('Default test prompt')}
          </SelectValue>
        </SelectTrigger>
        <SelectContent>
          <SelectItem value='0'>{t('Default test prompt')}</SelectItem>
          {prompts.data?.map((prompt) => (
            <SelectItem
              key={prompt.id}
              value={String(prompt.id)}
              disabled={!prompt.enabled}
            >
              {prompt.name}
              {!prompt.enabled && ` (${t('Disabled')})`}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {prompts.isError && (
        <p className='text-destructive text-sm'>
          {t('Failed to load test prompts')}
        </p>
      )}
      {selected && !selected.enabled && (
        <p className='text-muted-foreground text-sm'>
          {t('Disabled prompts fall back to the default prompt')}
        </p>
      )}
    </div>
  )
}
