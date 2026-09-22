/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import type { PromptInput } from './api'

const schema = z.object({
  name: z.string().trim().min(1).max(100),
  prompt: z.string().trim().min(1).max(65536),
  description: z.string().max(2000),
  enabled: z.boolean(),
  is_default: z.boolean(),
})

export function PromptEditor(props: {
  initial?: PromptInput
  pending: boolean
  onSave: (input: PromptInput) => void
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const form = useForm<PromptInput>({
    resolver: zodResolver(schema),
    defaultValues: props.initial ?? {
      name: '',
      prompt: '',
      description: '',
      enabled: true,
      is_default: false,
    },
  })
  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(props.onSave)}
        className='space-y-4 rounded-lg border p-4'
      >
        <FormField
          control={form.control}
          name='name'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Name')}</FormLabel>
              <FormControl>
                <Input {...field} maxLength={100} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='prompt'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Test prompt')}</FormLabel>
              <FormControl>
                <Textarea {...field} className='min-h-32' />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='description'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Description')}</FormLabel>
              <FormControl>
                <Input {...field} maxLength={2000} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='enabled'
          render={({ field }) => (
            <FormItem className='flex items-center justify-between'>
              <FormLabel>{t('Enabled')}</FormLabel>
              <FormControl>
                <Switch
                  checked={field.value}
                  onCheckedChange={(checked) => {
                    field.onChange(checked)
                    if (!checked) form.setValue('is_default', false)
                  }}
                />
              </FormControl>
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='is_default'
          render={({ field }) => (
            <FormItem className='flex items-center justify-between'>
              <FormLabel>{t('Set as default')}</FormLabel>
              <FormControl>
                <Switch
                  checked={field.value}
                  disabled={!form.watch('enabled')}
                  onCheckedChange={field.onChange}
                />
              </FormControl>
            </FormItem>
          )}
        />
        <div className='flex gap-2'>
          <Button type='submit' disabled={props.pending}>
            {t('Save')}
          </Button>
          <Button
            type='button'
            variant='outline'
            disabled={props.pending}
            onClick={props.onCancel}
          >
            {t('Cancel')}
          </Button>
        </div>
      </form>
    </Form>
  )
}
