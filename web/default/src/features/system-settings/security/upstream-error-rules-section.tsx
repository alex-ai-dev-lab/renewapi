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
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, Pencil, Plus, RefreshCw, Trash2, Upload } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { SettingsSection } from '../components/settings-section'

type ErrorRule = {
  id: number
  enabled: boolean
  description: string
  platforms: string
  upstream_status: number
  keywords: string
  passthrough_code: boolean
  response_code: number
  passthrough_body: boolean
  custom_message: string
  skip_monitoring: boolean
  priority: number
  created_at?: number
  updated_at?: number
}

type ErrorRuleForm = Omit<ErrorRule, 'id' | 'created_at' | 'updated_at'> & {
  id?: number
}

type ErrorRuleImportExportPayload = {
  UpstreamErrorRules?: unknown
}

const EMPTY_RULE: ErrorRuleForm = {
  enabled: true,
  description: '',
  platforms: '',
  upstream_status: 0,
  keywords: '',
  passthrough_code: false,
  response_code: 0,
  passthrough_body: false,
  custom_message: '',
  skip_monitoring: false,
  priority: 100,
}

const DEFAULT_PRIORITY = 100

const IMPORT_CONCURRENCY = 4

// priority 支持 0（0 是升序排序下的最高优先级），所以不能用 `Number(x) || 100`
// 这种写法——它会把 0 当成 falsy 静默换成 100。
function toPriority(value: unknown): number {
  if (value === '' || value === null || value === undefined) {
    return DEFAULT_PRIORITY
  }
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : DEFAULT_PRIORITY
}

function toStatusCode(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback
}

async function listRules() {
  const res = await api.get<{ success: boolean; data: ErrorRule[] }>(
    '/api/compat/error-rules'
  )
  return res.data.data ?? []
}

async function saveRule(values: ErrorRuleForm) {
  if (values.id) {
    const res = await api.put(`/api/compat/error-rules/${values.id}`, values)
    return res.data
  }
  const res = await api.post('/api/compat/error-rules', values)
  return res.data
}

async function deleteRule(id: number) {
  const res = await api.delete(`/api/compat/error-rules/${id}`)
  return res.data
}

async function reloadRules() {
  const res = await api.post('/api/compat/error-rules/reload')
  return res.data
}

function normalizeForm(values: ErrorRuleForm): ErrorRuleForm {
  return {
    ...values,
    description: values.description.trim(),
    platforms: values.platforms.trim(),
    keywords: values.keywords.trim(),
    passthrough_body: false,
    skip_monitoring: false,
    custom_message: values.custom_message.trim(),
    upstream_status: toStatusCode(values.upstream_status),
    response_code: toStatusCode(values.response_code),
    priority: toPriority(values.priority),
  }
}

function exportableRule(rule: ErrorRule): ErrorRuleForm {
  return normalizeForm({
    enabled: Boolean(rule.enabled),
    description: rule.description,
    platforms: rule.platforms,
    upstream_status: rule.upstream_status,
    keywords: rule.keywords,
    passthrough_code: Boolean(rule.passthrough_code),
    response_code: rule.response_code,
    passthrough_body: Boolean(rule.passthrough_body),
    custom_message: rule.custom_message,
    skip_monitoring: Boolean(rule.skip_monitoring),
    priority: rule.priority,
  })
}

function parseImportRules(text: string): ErrorRuleForm[] {
  const raw = JSON.parse(text) as ErrorRuleImportExportPayload | unknown[]
  const payload = Array.isArray(raw)
    ? raw
    : Array.isArray(raw.UpstreamErrorRules)
      ? raw.UpstreamErrorRules
      : []

  return payload.map((item) => {
    if (!item || typeof item !== 'object' || Array.isArray(item)) {
      throw new Error('invalid rule')
    }
    const value = item as Partial<ErrorRuleForm>
    return normalizeForm({
      enabled: value.enabled === undefined ? true : Boolean(value.enabled),
      description:
        typeof value.description === 'string' ? value.description : '',
      platforms: typeof value.platforms === 'string' ? value.platforms : '',
      upstream_status: toStatusCode(value.upstream_status),
      keywords: typeof value.keywords === 'string' ? value.keywords : '',
      passthrough_code: Boolean(value.passthrough_code),
      response_code: toStatusCode(value.response_code),
      passthrough_body: Boolean(value.passthrough_body),
      custom_message:
        typeof value.custom_message === 'string' ? value.custom_message : '',
      skip_monitoring: Boolean(value.skip_monitoring),
      priority: toPriority(value.priority),
    })
  })
}

export function UpstreamErrorRulesSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState('')
  const [importProgress, setImportProgress] = useState<{
    done: number
    total: number
  } | null>(null)
  const [formValues, setFormValues] = useState<ErrorRuleForm>(EMPTY_RULE)
  const [deleteTarget, setDeleteTarget] = useState<ErrorRule | null>(null)

  const { data: rules = [], isLoading } = useQuery({
    queryKey: ['upstream-error-rules'],
    queryFn: listRules,
  })

  const sortedRules = useMemo(
    () =>
      [...rules].sort((a, b) =>
        a.priority === b.priority ? a.id - b.id : a.priority - b.priority
      ),
    [rules]
  )

  const saveMutation = useMutation({
    mutationFn: saveRule,
    onSuccess: () => {
      toast.success(t('Saved'))
      setFormOpen(false)
      queryClient.invalidateQueries({ queryKey: ['upstream-error-rules'] })
    },
    onError: (error: unknown) => {
      toast.error(errorMessage(error, t('Failed to save error rule')))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteRule,
    onSuccess: () => {
      toast.success(t('Deleted'))
      setDeleteTarget(null)
      queryClient.invalidateQueries({ queryKey: ['upstream-error-rules'] })
    },
    onError: (error: unknown) => {
      toast.error(errorMessage(error, t('Failed to delete error rule')))
    },
  })

  const reloadMutation = useMutation({
    mutationFn: reloadRules,
    onSuccess: () => {
      toast.success(t('Rules reloaded'))
      queryClient.invalidateQueries({ queryKey: ['upstream-error-rules'] })
    },
    onError: (error: unknown) => {
      toast.error(errorMessage(error, t('Failed to reload rules')))
    },
  })

  const importMutation = useMutation({
    mutationFn: async (values: ErrorRuleForm[]) => {
      let nextIndex = 0
      let failed = 0
      setImportProgress({ done: 0, total: values.length })

      async function worker() {
        for (;;) {
          const index = nextIndex
          nextIndex += 1
          if (index >= values.length) {
            return
          }
          try {
            await saveRule(values[index])
          } catch {
            failed += 1
          } finally {
            setImportProgress((prev) =>
              prev ? { ...prev, done: prev.done + 1 } : prev
            )
          }
        }
      }

      const workers = Array.from(
        { length: Math.min(IMPORT_CONCURRENCY, values.length) },
        () => worker()
      )
      await Promise.all(workers)
      if (failed > 0) {
        throw new Error(
          t('Failed to import {{count}} rules', { count: failed })
        )
      }
    },
    onSuccess: (_data, values) => {
      toast.success(t('Imported {{count}} rules', { count: values.length }))
      setImportOpen(false)
      setImportText('')
      setImportProgress(null)
      queryClient.invalidateQueries({ queryKey: ['upstream-error-rules'] })
    },
    onError: (error: unknown) => {
      toast.error(errorMessage(error, t('Failed to import rules')))
      setImportProgress(null)
      // 导入不是原子的：失败之前已经创建成功的规则依然存在，必须刷新列表，
      // 否则管理员看到的是一个不包含这些新规则的陈旧快照。
      queryClient.invalidateQueries({ queryKey: ['upstream-error-rules'] })
    },
  })

  const openCreate = () => {
    setFormValues(EMPTY_RULE)
    setFormOpen(true)
  }

  const openEdit = (rule: ErrorRule) => {
    setFormValues({ ...rule })
    setFormOpen(true)
  }

  const handleSubmit = () => {
    const normalized = normalizeForm(formValues)
    if (!normalized.description) {
      toast.error(t('Description is required'))
      return
    }
    saveMutation.mutate(normalized)
  }

  const exportConfig = async () => {
    const text = JSON.stringify(
      { UpstreamErrorRules: sortedRules.map(exportableRule) },
      null,
      2
    )

    try {
      await navigator.clipboard?.writeText(text)
    } catch {
      /* Clipboard can be unavailable on non-secure origins. */
    }

    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'renewapi-upstream-error-rules.json'
    link.click()
    URL.revokeObjectURL(url)
    toast.success(t('Upstream error rules downloaded and copied when possible'))
  }

  const openImportDialog = () => {
    setImportText('')
    setImportProgress(null)
    setImportOpen(true)
  }

  const importConfig = () => {
    try {
      const imported = parseImportRules(importText)
      if (imported.length === 0) {
        toast.error(t('No rules found in JSON'))
        return
      }
      const invalid = imported.find((rule) => !rule.description)
      if (invalid) {
        toast.error(t('Description is required'))
        return
      }
      importMutation.mutate(imported)
    } catch {
      toast.error(t('Invalid upstream error rules JSON'))
    }
  }

  return (
    <SettingsSection title={t('Upstream Error Rules')}>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Normalize upstream error responses before they are shown to clients. Backend logs keep masked originals.'
          )}
        </p>
        <div className='flex flex-wrap gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={exportConfig}
          >
            <Download className='mr-2 h-4 w-4' />
            {t('Export JSON')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={openImportDialog}
          >
            <Upload className='mr-2 h-4 w-4' />
            {t('Import JSON')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => reloadMutation.mutate()}
            disabled={reloadMutation.isPending}
          >
            <RefreshCw className='mr-2 h-4 w-4' />
            {t('Reload')}
          </Button>
          <Button type='button' size='sm' onClick={openCreate}>
            <Plus className='mr-2 h-4 w-4' />
            {t('Add Rule')}
          </Button>
        </div>
      </div>

      <div className='hidden rounded-md border md:block'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Priority')}</TableHead>
              <TableHead>{t('Description')}</TableHead>
              <TableHead>{t('Match')}</TableHead>
              <TableHead>{t('Response')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6}>{t('Loading...')}</TableCell>
              </TableRow>
            ) : sortedRules.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className='text-muted-foreground'>
                  {t(
                    'No custom rules. Built-in fixed messages are still active.'
                  )}
                </TableCell>
              </TableRow>
            ) : (
              sortedRules.map((rule) => (
                <TableRow key={rule.id}>
                  <TableCell className='font-mono text-xs'>
                    {rule.priority}
                  </TableCell>
                  <TableCell>
                    <div
                      className='max-w-[260px] truncate font-medium'
                      title={rule.description}
                    >
                      {rule.description}
                    </div>
                  </TableCell>
                  <TableCell className='text-xs'>
                    <div>
                      {t('Platform')}: {rule.platforms || t('Any')}
                    </div>
                    <div>
                      {t('HTTP')}: {rule.upstream_status || t('Any')}
                    </div>
                    <div
                      className='max-w-[240px] truncate'
                      title={rule.keywords || t('Any')}
                    >
                      {t('Keywords')}: {rule.keywords || t('Any')}
                    </div>
                  </TableCell>
                  <TableCell className='text-xs'>
                    <div>
                      {rule.passthrough_code
                        ? t('Keep status')
                        : `${t('Status')} ${rule.response_code || t('Upstream')}`}
                    </div>
                    <div
                      className='max-w-[260px] truncate'
                      title={rule.custom_message || t('Built-in fixed message')}
                    >
                      {rule.custom_message || t('Built-in fixed message')}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={rule.enabled ? 'default' : 'secondary'}>
                      {rule.enabled ? t('Enabled') : t('Disabled')}
                    </Badge>
                  </TableCell>
                  <TableCell className='text-right'>
                    <div className='flex justify-end gap-1'>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        aria-label={t('Edit')}
                        onClick={() => openEdit(rule)}
                      >
                        <Pencil className='h-4 w-4' />
                      </Button>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        aria-label={t('Delete')}
                        onClick={() => setDeleteTarget(rule)}
                      >
                        <Trash2 className='h-4 w-4' />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      <div className='grid gap-3 md:hidden'>
        {isLoading ? (
          <div className='text-muted-foreground rounded-md border p-3 text-sm'>
            {t('Loading...')}
          </div>
        ) : sortedRules.length === 0 ? (
          <div className='text-muted-foreground rounded-md border p-3 text-sm'>
            {t('No custom rules. Built-in fixed messages are still active.')}
          </div>
        ) : (
          sortedRules.map((rule) => (
            <div key={rule.id} className='rounded-md border p-3'>
              <div className='flex items-start justify-between gap-3'>
                <div className='min-w-0'>
                  <div className='text-sm font-medium break-words'>
                    {rule.description}
                  </div>
                  <div className='text-muted-foreground mt-1 font-mono text-xs'>
                    {t('Priority')} {rule.priority}
                  </div>
                </div>
                <Badge variant={rule.enabled ? 'default' : 'secondary'}>
                  {rule.enabled ? t('Enabled') : t('Disabled')}
                </Badge>
              </div>

              <div className='mt-3 grid gap-2 text-xs'>
                <div className='bg-muted/40 rounded-md p-2'>
                  <div className='font-medium'>{t('Match')}</div>
                  <div className='text-muted-foreground mt-1 space-y-1 break-words'>
                    <div>
                      {t('Platform')}: {rule.platforms || t('Any')}
                    </div>
                    <div>
                      {t('HTTP')}: {rule.upstream_status || t('Any')}
                    </div>
                    <div>
                      {t('Keywords')}: {rule.keywords || t('Any')}
                    </div>
                  </div>
                </div>
                <div className='bg-muted/40 rounded-md p-2'>
                  <div className='font-medium'>{t('Response')}</div>
                  <div className='text-muted-foreground mt-1 space-y-1 break-words'>
                    <div>
                      {rule.passthrough_code
                        ? t('Keep status')
                        : `${t('Status')} ${rule.response_code || t('Upstream')}`}
                    </div>
                    <div>
                      {rule.custom_message || t('Built-in fixed message')}
                    </div>
                  </div>
                </div>
              </div>

              <div className='mt-3 flex justify-end gap-1'>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  aria-label={t('Edit')}
                  onClick={() => openEdit(rule)}
                >
                  <Pencil className='h-4 w-4' />
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  aria-label={t('Delete')}
                  onClick={() => setDeleteTarget(rule)}
                >
                  <Trash2 className='h-4 w-4' />
                </Button>
              </div>
            </div>
          ))
        )}
      </div>

      <Dialog open={formOpen} onOpenChange={setFormOpen}>
        <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>
              {formValues.id ? t('Edit Error Rule') : t('Add Error Rule')}
            </DialogTitle>
          </DialogHeader>
          <div className='grid gap-4 py-2'>
            <label className='space-y-2'>
              <span className='text-sm font-medium'>{t('Description')}</span>
              <Input
                value={formValues.description}
                onChange={(event) =>
                  setFormValues((prev) => ({
                    ...prev,
                    description: event.target.value,
                  }))
                }
              />
            </label>

            <div className='grid gap-4 md:grid-cols-3'>
              <label className='space-y-2'>
                <span className='text-sm font-medium'>{t('Priority')}</span>
                <Input
                  type='number'
                  value={formValues.priority}
                  onChange={(event) =>
                    setFormValues((prev) => ({
                      ...prev,
                      priority: toPriority(event.target.value),
                    }))
                  }
                />
                <p className='text-muted-foreground text-xs'>
                  {t('Lower values are matched first. 0 is the highest.')}
                </p>
              </label>
              <label className='space-y-2'>
                <span className='text-sm font-medium'>
                  {t('Upstream HTTP Status')}
                </span>
                <Input
                  type='number'
                  value={formValues.upstream_status}
                  onChange={(event) =>
                    setFormValues((prev) => ({
                      ...prev,
                      upstream_status: toStatusCode(event.target.value),
                    }))
                  }
                />
              </label>
              <label className='space-y-2'>
                <span className='text-sm font-medium'>
                  {t('Response HTTP Status')}
                </span>
                <Input
                  type='number'
                  value={formValues.response_code}
                  onChange={(event) =>
                    setFormValues((prev) => ({
                      ...prev,
                      response_code: toStatusCode(event.target.value),
                    }))
                  }
                />
              </label>
            </div>

            <label className='space-y-2'>
              <span className='text-sm font-medium'>{t('Platforms')}</span>
              <Input
                value={formValues.platforms}
                placeholder='1,14'
                onChange={(event) =>
                  setFormValues((prev) => ({
                    ...prev,
                    platforms: event.target.value,
                  }))
                }
              />
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Comma-separated channel type IDs. Empty means all platforms.'
                )}
              </p>
            </label>

            <label className='space-y-2'>
              <span className='text-sm font-medium'>{t('Keywords')}</span>
              <Input
                value={formValues.keywords}
                placeholder='invalid api key, access restricted'
                onChange={(event) =>
                  setFormValues((prev) => ({
                    ...prev,
                    keywords: event.target.value,
                  }))
                }
              />
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Comma-separated case-insensitive body keywords. Empty matches all.'
                )}
              </p>
            </label>

            <label className='space-y-2'>
              <span className='text-sm font-medium'>{t('Custom Message')}</span>
              <Textarea
                rows={3}
                value={formValues.custom_message}
                placeholder={t(
                  'Leave empty to use the built-in fixed message.'
                )}
                onChange={(event) =>
                  setFormValues((prev) => ({
                    ...prev,
                    custom_message: event.target.value,
                  }))
                }
              />
            </label>

            <div className='grid gap-3 rounded-md border p-3 md:grid-cols-2'>
              {[
                { key: 'enabled', label: t('Enabled') },
                {
                  key: 'passthrough_code',
                  label: t('Keep upstream status code'),
                },
                {
                  key: 'passthrough_body',
                  label: t('Keep upstream response body'),
                  deprecated: true,
                },
                {
                  key: 'skip_monitoring',
                  label: t('Skip monitoring'),
                  deprecated: true,
                },
              ].map(({ key, label, deprecated }) => (
                <label
                  key={key}
                  className='flex items-center justify-between gap-4 text-sm'
                >
                  <span>
                    {label}
                    {deprecated ? (
                      <span className='text-muted-foreground ml-2 text-xs'>
                        {t('Deprecated')}
                      </span>
                    ) : null}
                  </span>
                  <Switch
                    checked={
                      deprecated
                        ? false
                        : Boolean(formValues[key as keyof ErrorRuleForm])
                    }
                    disabled={deprecated}
                    onCheckedChange={(checked) =>
                      setFormValues((prev) => ({
                        ...prev,
                        [key]: checked,
                      }))
                    }
                  />
                </label>
              ))}
            </div>
          </div>
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => setFormOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              onClick={handleSubmit}
              disabled={saveMutation.isPending}
            >
              {t('Save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Import upstream error rules')}</DialogTitle>
          </DialogHeader>
          <div className='space-y-2'>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Imported rules are appended as new custom rules. Existing rules are not deleted or overwritten.'
              )}
            </p>
            <Textarea
              rows={14}
              value={importText}
              onChange={(event) => setImportText(event.target.value)}
              placeholder='{ "UpstreamErrorRules": [] }'
              className='font-mono text-xs'
              disabled={importMutation.isPending}
            />
            {importProgress ? (
              <div className='space-y-1'>
                <div className='text-muted-foreground flex justify-between text-xs'>
                  <span>{t('Import progress')}</span>
                  <span>
                    {importProgress.done}/{importProgress.total}
                  </span>
                </div>
                <div className='bg-muted h-2 overflow-hidden rounded'>
                  <div
                    className='bg-primary h-full transition-all'
                    style={{
                      width: `${Math.round(
                        (importProgress.done / importProgress.total) * 100
                      )}%`,
                    }}
                  />
                </div>
              </div>
            ) : null}
          </div>
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => setImportOpen(false)}
              disabled={importMutation.isPending}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              onClick={importConfig}
              disabled={importMutation.isPending}
            >
              {importMutation.isPending && importProgress
                ? t('Importing {{done}}/{{total}}', importProgress)
                : t('Import')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t('Delete error rule?')}
        desc={deleteTarget?.description ?? ''}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (deleteTarget) deleteMutation.mutate(deleteTarget.id)
        }}
      />
    </SettingsSection>
  )
}
