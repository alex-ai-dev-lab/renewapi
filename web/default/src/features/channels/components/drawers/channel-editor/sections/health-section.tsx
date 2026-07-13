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
import type { UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import type { ChannelFormValues } from '../../../../lib'
import { getChannelEditorSection } from '../../../../lib/channel-editor-sections'

type ChannelHealthSectionProps = {
  form: UseFormReturn<ChannelFormValues>
}

export function ChannelHealthSection(props: ChannelHealthSectionProps) {
  const { t } = useTranslation()
  const form = props.form

  return (
    <>
      <div
        id={getChannelEditorSection('health').anchorId}
        className='scroll-mt-4'
        aria-hidden='true'
      />

      <FormField
        control={form.control}
        name='normalize_upstream_errors'
        render={({ field }) => (
          <FormItem className='flex items-center justify-between rounded-md border p-3'>
            <div className='space-y-0.5'>
              <FormLabel>{t('Normalize upstream errors')}</FormLabel>
              <FormDescription>
                {t(
                  'Return fixed client-facing error messages while keeping sanitized logs.'
                )}
              </FormDescription>
            </div>
            <FormControl>
              <Switch
                checked={field.value !== false}
                onCheckedChange={field.onChange}
              />
            </FormControl>
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='anti_poison_enabled'
        render={({ field }) => (
          <FormItem className='flex items-center justify-between rounded-md border p-3'>
            <div className='space-y-0.5'>
              <FormLabel>{t('Anti-Poison validation')}</FormLabel>
              <FormDescription>
                {t(
                  'Enabled by default. Disable only for channels that cannot follow guard validation.'
                )}
              </FormDescription>
            </div>
            <FormControl>
              <Switch
                checked={field.value !== false}
                onCheckedChange={field.onChange}
              />
            </FormControl>
          </FormItem>
        )}
      />

      <div className='grid gap-3 rounded-md border p-3 md:grid-cols-2'>
        <FormField
          control={form.control}
          name='anti_poison_profile'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Risk Profile')}</FormLabel>
              <Select
                value={field.value || 'inherit'}
                onValueChange={field.onChange}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value='inherit'>
                    {t('Inherit by channel id')}
                  </SelectItem>
                  <SelectItem value='trusted'>{t('trusted')}</SelectItem>
                  <SelectItem value='unknown'>{t('unknown')}</SelectItem>
                  <SelectItem value='probation'>{t('probation')}</SelectItem>
                  <SelectItem value='quarantine'>{t('quarantine')}</SelectItem>
                </SelectContent>
              </Select>
              <FormDescription>
                {t(
                  'Use probation for channel 101, quarantine for ad-only channels, trusted for stable channels.'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='anti_poison_answer_envelope'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Answer Envelope')}</FormLabel>
              <Select
                value={field.value || 'inherit'}
                onValueChange={field.onChange}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value='inherit'>{t('Inherit')}</SelectItem>
                  <SelectItem value='off'>{t('off')}</SelectItem>
                  <SelectItem value='auto'>{t('auto')}</SelectItem>
                  <SelectItem value='required'>{t('required')}</SelectItem>
                  <SelectItem value='required_non_stream'>
                    {t('required_non_stream')}
                  </SelectItem>
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='anti_poison_response_proof'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Response Proof')}</FormLabel>
              <Select
                value={field.value || 'inherit'}
                onValueChange={field.onChange}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value='inherit'>{t('Inherit')}</SelectItem>
                  <SelectItem value='off'>{t('off')}</SelectItem>
                  <SelectItem value='warn'>{t('warn')}</SelectItem>
                  <SelectItem value='auto'>{t('auto')}</SelectItem>
                  <SelectItem value='required'>{t('required')}</SelectItem>
                  <SelectItem value='required_non_stream'>
                    {t('required_non_stream')}
                  </SelectItem>
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='anti_poison_tool_call_guard'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Tool Call Guard')}</FormLabel>
              <Select
                value={field.value || 'inherit'}
                onValueChange={field.onChange}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value='inherit'>{t('Inherit')}</SelectItem>
                  <SelectItem value='off'>{t('off')}</SelectItem>
                  <SelectItem value='warn'>{t('warn')}</SelectItem>
                  <SelectItem value='auto'>{t('auto')}</SelectItem>
                  <SelectItem value='strict'>{t('strict')}</SelectItem>
                  <SelectItem value='strict_when_tools'>
                    {t('strict_when_tools')}
                  </SelectItem>
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='anti_poison_opaque_scan'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Opaque Payload Scanner')}</FormLabel>
              <Select
                value={field.value || 'inherit'}
                onValueChange={field.onChange}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value='inherit'>{t('Inherit')}</SelectItem>
                  <SelectItem value='off'>{t('off')}</SelectItem>
                  <SelectItem value='warn'>{t('warn')}</SelectItem>
                  <SelectItem value='score'>{t('score')}</SelectItem>
                  <SelectItem value='score_strict'>
                    {t('score_strict')}
                  </SelectItem>
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='anti_poison_stream_mode'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Stream Mode')}</FormLabel>
              <Select
                value={field.value || 'inherit'}
                onValueChange={field.onChange}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectItem value='inherit'>{t('Inherit')}</SelectItem>
                  <SelectItem value='direct_stream_light_scan'>
                    {t('direct_stream_light_scan')}
                  </SelectItem>
                  <SelectItem value='preflight_probe_first_bytes_buffer'>
                    {t('preflight_probe_first_bytes_buffer')}
                  </SelectItem>
                  <SelectItem value='aggregate_then_replay'>
                    {t('aggregate_then_replay')}
                  </SelectItem>
                  <SelectItem value='disabled'>{t('disabled')}</SelectItem>
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='anti_poison_hard_failures_to_quarantine'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Hard failures to quarantine')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={0}
                  {...field}
                  onChange={(event) =>
                    field.onChange(Number(event.target.value || 0))
                  }
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='anti_poison_soft_failures_to_degrade'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Soft failures to degrade')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={0}
                  {...field}
                  onChange={(event) =>
                    field.onChange(Number(event.target.value || 0))
                  }
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>

      <FormField
        control={form.control}
        name='anti_poison_probe_before_every_request'
        render={({ field }) => (
          <FormItem className='flex items-center justify-between rounded-md border p-3'>
            <div className='space-y-0.5'>
              <FormLabel>{t('Probe before every request')}</FormLabel>
              <FormDescription>
                {t(
                  'Use for probation channels such as 101. Probe traffic is separated from real user requests.'
                )}
              </FormDescription>
            </div>
            <FormControl>
              <Switch
                checked={field.value === true}
                onCheckedChange={field.onChange}
              />
            </FormControl>
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='anti_poison_failure_mode'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Anti-Poison Failure Mode')}</FormLabel>
            <Select
              value={field.value || 'inherit'}
              onValueChange={field.onChange}
            >
              <FormControl>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
              </FormControl>
              <SelectContent>
                <SelectItem value='inherit'>{t('Inherit')}</SelectItem>
                <SelectItem value='block'>{t('Block')}</SelectItem>
                <SelectItem value='warn'>{t('Warn only')}</SelectItem>
              </SelectContent>
            </Select>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='anti_poison_response_proof_enabled'
        render={({ field }) => (
          <FormItem className='flex items-center justify-between rounded-md border p-3'>
            <div className='space-y-0.5'>
              <FormLabel>{t('Response proof validation')}</FormLabel>
              <FormDescription>
                {t(
                  'Grey rollout only. The upstream must echo a hidden nonce at the beginning of normal text responses, otherwise the channel is blocked and marked risky.'
                )}
              </FormDescription>
            </div>
            <FormControl>
              <Switch
                checked={field.value === true}
                onCheckedChange={field.onChange}
              />
            </FormControl>
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='anti_poison_canary_echo_enabled'
        render={({ field }) => (
          <FormItem className='flex items-center justify-between rounded-md border p-3'>
            <div className='space-y-0.5'>
              <FormLabel>{t('Canary echo validation')}</FormLabel>
              <FormDescription>
                {t(
                  'Legacy real-request canary. Keep disabled for exact-output, JSON-only, and tool-only prompts; use profile probe instead.'
                )}
              </FormDescription>
            </div>
            <FormControl>
              <Switch
                checked={field.value === true}
                onCheckedChange={field.onChange}
              />
            </FormControl>
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='anti_poison_shape_check_enabled'
        render={({ field }) => (
          <FormItem className='flex items-center justify-between rounded-md border p-3'>
            <div className='space-y-0.5'>
              <FormLabel>{t('Response shape validation')}</FormLabel>
              <FormDescription>
                {t(
                  'Validate response id/model/object/finish_reason fingerprint. Catches malformed or mismatched responses.'
                )}
              </FormDescription>
            </div>
            <FormControl>
              <Switch
                checked={field.value === true}
                onCheckedChange={field.onChange}
              />
            </FormControl>
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='requires_codex_identity'
        render={({ field }) => (
          <FormItem>
            <div className='space-y-2 rounded-md border p-3'>
              <FormLabel>{t('Require Codex identity')}</FormLabel>
              <FormDescription>
                {t(
                  'Enabled by default for OpenAI-style requests. Force disabled only for channels that reject Codex client metadata.'
                )}
              </FormDescription>
              <Select
                value={field.value || 'auto'}
                onValueChange={(value) => field.onChange(value ?? 'auto')}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value='auto'>{t('Auto infer')}</SelectItem>
                    <SelectItem value='true'>{t('Force enabled')}</SelectItem>
                    <SelectItem value='false'>{t('Force disabled')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name='supports_claude_thinking'
        render={({ field }) => (
          <FormItem>
            <div className='space-y-2 rounded-md border p-3'>
              <FormLabel>{t('Supports Claude thinking')}</FormLabel>
              <FormDescription>
                {t(
                  'Auto infers support from channel type. Explicit values override inference.'
                )}
              </FormDescription>
              <Select
                value={field.value || 'auto'}
                onValueChange={(value) => field.onChange(value ?? 'auto')}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value='auto'>{t('Auto infer')}</SelectItem>
                    <SelectItem value='true'>{t('Force supported')}</SelectItem>
                    <SelectItem value='false'>
                      {t('Force unsupported')}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>
            <FormMessage />
          </FormItem>
        )}
      />

      <div className='grid gap-4 border-t pt-4 sm:grid-cols-3'>
        <FormField
          control={form.control}
          name='auto_test_interval'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Test interval minutes')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={0}
                  {...field}
                  onChange={(event) =>
                    field.onChange(Number(event.target.value))
                  }
                />
              </FormControl>
              <FormDescription>{t('0 uses global interval.')}</FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='auto_test_retry_count'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Attempts per test')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={1}
                  {...field}
                  onChange={(event) =>
                    field.onChange(Number(event.target.value))
                  }
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='auto_test_retry_threshold'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Disable after failures')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={1}
                  {...field}
                  onChange={(event) =>
                    field.onChange(Number(event.target.value))
                  }
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='auto_test_time_window_start'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Test window start')}</FormLabel>
              <FormControl>
                <Input placeholder='23:00' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='auto_test_time_window_end'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Test window end')}</FormLabel>
              <FormControl>
                <Input placeholder='07:00' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='auto_test_timezone'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Test timezone')}</FormLabel>
              <FormControl>
                <Input placeholder='Asia/Taipei' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>
    </>
  )
}
