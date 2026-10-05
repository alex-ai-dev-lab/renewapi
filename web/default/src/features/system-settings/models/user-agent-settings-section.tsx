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
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, Pencil, Plus, Trash2, Upload } from 'lucide-react'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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

type UserAgentCategory = 'openai' | 'claude' | 'grok' | 'gemini' | 'other'

type UserAgent = {
  id: number
  name: string
  value: string
  model_category: UserAgentCategory
  is_global: boolean
  enabled: boolean
  sort_order: number
  remark?: string
}

type UserAgentForm = Omit<UserAgent, 'id'> & { id?: number }

const CATEGORIES: UserAgentCategory[] = [
  'openai',
  'claude',
  'grok',
  'gemini',
  'other',
]

const EMPTY_FORM: UserAgentForm = {
  name: '',
  value: '',
  model_category: 'openai',
  is_global: false,
  enabled: true,
  sort_order: 0,
  remark: '',
}

async function listUserAgents() {
  const res = await api.get<{ success: boolean; data: UserAgent[] }>(
    '/api/user-agents/'
  )
  return res.data.data || []
}

async function saveUserAgent(values: UserAgentForm) {
  if (values.id) {
    const res = await api.put(`/api/user-agents/${values.id}`, values)
    return res.data
  }
  const res = await api.post('/api/user-agents/', values)
  return res.data
}

async function deleteUserAgent(id: number) {
  const res = await api.delete(`/api/user-agents/${id}`)
  return res.data
}

export function UserAgentSettingsSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const [formOpen, setFormOpen] = useState(false)
  const [formValues, setFormValues] = useState<UserAgentForm>(EMPTY_FORM)
  const [deleteTarget, setDeleteTarget] = useState<UserAgent | null>(null)

  const { data: userAgents = [], isLoading } = useQuery({
    queryKey: ['user-agents'],
    queryFn: listUserAgents,
    retry: 1,
  })

  const grouped = useMemo(
    () =>
      CATEGORIES.map((category) => ({
        category,
        items: userAgents.filter((ua) => ua.model_category === category),
      })),
    [userAgents]
  )

  const saveMutation = useMutation({
    mutationFn: saveUserAgent,
    onSuccess: () => {
      toast.success(t('Saved'))
      setFormOpen(false)
      queryClient.invalidateQueries({ queryKey: ['user-agents'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteUserAgent,
    onSuccess: () => {
      toast.success(t('Deleted'))
      setDeleteTarget(null)
      queryClient.invalidateQueries({ queryKey: ['user-agents'] })
    },
  })

  const importMutation = useMutation({
    mutationFn: async (payload: { mode: string; user_agents: UserAgent[] }) => {
      const res = await api.post('/api/user-agents/import', payload)
      return res.data
    },
    onSuccess: () => {
      toast.success(t('Imported'))
      queryClient.invalidateQueries({ queryKey: ['user-agents'] })
    },
  })

  const openCreate = () => {
    setFormValues(EMPTY_FORM)
    setFormOpen(true)
  }

  const openEdit = (ua: UserAgent) => {
    setFormValues({ ...ua })
    setFormOpen(true)
  }

  const handleSubmit = () => {
    if (!formValues.name.trim() || !formValues.value.trim()) {
      toast.error(t('Name and User-Agent are required'))
      return
    }
    saveMutation.mutate({
      ...formValues,
      name: formValues.name.trim(),
      value: formValues.value.trim(),
      remark: formValues.remark?.trim() || '',
    })
  }

  const handleExport = async () => {
    const res = await api.get('/api/user-agents/export')
    const blob = new Blob([JSON.stringify(res.data.data, null, 2)], {
      type: 'application/json',
    })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'renewapi-user-agents.json'
    a.click()
    URL.revokeObjectURL(url)
  }

  const handleImportFile = async (file: File | undefined) => {
    if (!file) return
    const text = await file.text()
    const parsed = JSON.parse(text)
    const userAgentsToImport = Array.isArray(parsed)
      ? parsed
      : parsed.user_agents || []
    importMutation.mutate({
      mode: 'append',
      user_agents: userAgentsToImport,
    })
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  useEffect(() => {
    // External dialog closure also discards its local draft.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (!formOpen) setFormValues(EMPTY_FORM)
  }, [formOpen])

  return (
    <SettingsSection title={t('User-Agent Management')}>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Channel UA overrides model-category global UA, then built-in defaults are used.'
          )}
        </p>
        <div className='flex flex-wrap gap-2'>
          <input
            ref={fileInputRef}
            type='file'
            accept='application/json'
            className='hidden'
            onChange={(event) => handleImportFile(event.target.files?.[0])}
          />
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => fileInputRef.current?.click()}
          >
            <Upload className='mr-2 h-4 w-4' />
            {t('Import')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={handleExport}
          >
            <Download className='mr-2 h-4 w-4' />
            {t('Export')}
          </Button>
          <Button type='button' size='sm' onClick={openCreate}>
            <Plus className='mr-2 h-4 w-4' />
            {t('Add UA')}
          </Button>
        </div>
      </div>

      <div className='rounded-md border'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Category')}</TableHead>
              <TableHead>{t('Name')}</TableHead>
              <TableHead>{t('User-Agent')}</TableHead>
              <TableHead>{t('Sort')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell
                  colSpan={6}
                  className='text-muted-foreground py-8 text-center'
                >
                  {t('Loading')}
                </TableCell>
              </TableRow>
            ) : userAgents.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={6}
                  className='text-muted-foreground py-8 text-center'
                >
                  {t('No User-Agent entries configured.')}
                </TableCell>
              </TableRow>
            ) : (
              grouped.flatMap(({ category, items }) =>
                items.map((ua) => (
                  <TableRow key={ua.id}>
                    <TableCell>
                      <Badge variant='outline'>{category}</Badge>
                    </TableCell>
                    <TableCell className='font-medium'>
                      <div className='flex items-center gap-2'>
                        {ua.name}
                        {ua.is_global && <Badge>{t('Global')}</Badge>}
                      </div>
                    </TableCell>
                    <TableCell className='max-w-[360px] truncate font-mono text-xs'>
                      {ua.value}
                    </TableCell>
                    <TableCell>{ua.sort_order}</TableCell>
                    <TableCell>
                      <Badge variant={ua.enabled ? 'default' : 'secondary'}>
                        {ua.enabled ? t('Enabled') : t('Disabled')}
                      </Badge>
                    </TableCell>
                    <TableCell className='text-right'>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        onClick={() => openEdit(ua)}
                      >
                        <Pencil className='h-4 w-4' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon-sm'
                        onClick={() => setDeleteTarget(ua)}
                      >
                        <Trash2 className='text-destructive-text h-4 w-4' />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )
            )}
          </TableBody>
        </Table>
      </div>

      <Dialog open={formOpen} onOpenChange={setFormOpen}>
        <DialogContent className='sm:max-w-xl'>
          <DialogHeader>
            <DialogTitle>
              {formValues.id ? t('Edit User-Agent') : t('Add User-Agent')}
            </DialogTitle>
          </DialogHeader>
          <div className='grid gap-4'>
            <div className='grid gap-4 sm:grid-cols-2'>
              <Input
                placeholder={t('Name')}
                value={formValues.name}
                onChange={(event) =>
                  setFormValues((prev) => ({
                    ...prev,
                    name: event.target.value,
                  }))
                }
              />
              <Select
                value={formValues.model_category}
                onValueChange={(value) =>
                  setFormValues((prev) => ({
                    ...prev,
                    model_category: value as UserAgentCategory,
                  }))
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {CATEGORIES.map((category) => (
                    <SelectItem key={category} value={category}>
                      {category}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Textarea
              rows={4}
              placeholder='codex_exec/0.136.0 (Windows 10.0.26200; x86_64)'
              value={formValues.value}
              onChange={(event) =>
                setFormValues((prev) => ({
                  ...prev,
                  value: event.target.value,
                }))
              }
            />
            <div className='grid gap-4 sm:grid-cols-2'>
              <Input
                type='number'
                placeholder={t('Sort')}
                value={formValues.sort_order}
                onChange={(event) =>
                  setFormValues((prev) => ({
                    ...prev,
                    sort_order: Number(event.target.value),
                  }))
                }
              />
              <Input
                placeholder={t('Remark')}
                value={formValues.remark || ''}
                onChange={(event) =>
                  setFormValues((prev) => ({
                    ...prev,
                    remark: event.target.value,
                  }))
                }
              />
            </div>
            <div className='grid gap-3 sm:grid-cols-2'>
              <label className='flex items-center justify-between rounded-md border p-3 text-sm'>
                <span>{t('Global for category')}</span>
                <Switch
                  checked={formValues.is_global}
                  onCheckedChange={(checked) =>
                    setFormValues((prev) => ({ ...prev, is_global: checked }))
                  }
                />
              </label>
              <label className='flex items-center justify-between rounded-md border p-3 text-sm'>
                <span>{t('Enabled')}</span>
                <Switch
                  checked={formValues.enabled}
                  onCheckedChange={(checked) =>
                    setFormValues((prev) => ({ ...prev, enabled: checked }))
                  }
                />
              </label>
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setFormOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button onClick={handleSubmit} disabled={saveMutation.isPending}>
              {t('Save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t('Delete User-Agent')}
        desc={t('Delete this User-Agent entry?')}
        confirmText={t('Delete')}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() =>
          deleteTarget && deleteMutation.mutate(deleteTarget.id)
        }
      />
    </SettingsSection>
  )
}
