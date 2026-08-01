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
import { Download, Upload } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import { SettingsPageActionsPortal } from '../../components/settings-page-context'
import { SettingsSection } from '../../components/settings-section'
import { ProviderFormDialog } from './components/provider-form-dialog'
import { ProviderTable } from './components/provider-table'
import {
  useCreateProvider,
  useUpdateProvider,
} from './hooks/use-custom-oauth-mutations'
import { useCustomOAuthProviders } from './hooks/use-custom-oauth-providers'
import { customOAuthFormSchema, type CustomOAuthProvider } from './types'

const REDACTED_SECRET = '__SECRET_REDACTED__'

const REDACTED_SECRET_IMPORT_ERROR =
  'Client secret was redacted on export and this instance has no saved secret for this provider'

type CustomOAuthImportPayload =
  | CustomOAuthProvider[]
  | {
      CustomOAuthProviders?: CustomOAuthProvider[]
      providers?: CustomOAuthProvider[]
    }

type PreparedImport = {
  payload: Omit<CustomOAuthProvider, 'id'>
  existing?: CustomOAuthProvider
}

const redactProviderSecret = (
  provider: CustomOAuthProvider
): CustomOAuthProvider => ({
  ...provider,
  client_secret: provider.client_secret ? REDACTED_SECRET : '',
})

const toProviderPayload = (
  provider: CustomOAuthProvider | Record<string, unknown>,
  existing?: CustomOAuthProvider
): Omit<CustomOAuthProvider, 'id'> => {
  const source = { ...provider } as Partial<CustomOAuthProvider>
  if (
    existing &&
    (source.client_secret === undefined ||
      source.client_secret === REDACTED_SECRET)
  ) {
    source.client_secret = existing.client_secret
  } else if (source.client_secret === REDACTED_SECRET) {
    // There is no saved secret to restore. Writing the placeholder through
    // would store a bogus secret, and blanking it would create a provider that
    // looks enabled but can never complete a token exchange. importConfig
    // reports this case up front; this throw keeps the helper safe for any
    // other caller.
    throw new Error(REDACTED_SECRET_IMPORT_ERROR)
  }

  const parsed = customOAuthFormSchema.parse(source)
  return {
    name: parsed.name,
    slug: parsed.slug,
    icon: parsed.icon,
    enabled: parsed.enabled,
    client_id: parsed.client_id,
    client_secret: parsed.client_secret,
    authorization_endpoint: parsed.authorization_endpoint,
    token_endpoint: parsed.token_endpoint,
    user_info_endpoint: parsed.user_info_endpoint,
    scopes: parsed.scopes,
    user_id_field: parsed.user_id_field,
    username_field: parsed.username_field,
    display_name_field: parsed.display_name_field,
    email_field: parsed.email_field,
    well_known: parsed.well_known,
    auth_style: parsed.auth_style,
    access_policy: parsed.access_policy,
    access_denied_message: parsed.access_denied_message,
  }
}

export function CustomOAuthSection() {
  const { t } = useTranslation()
  const { data: providers = [], isLoading } = useCustomOAuthProviders()
  const createProvider = useCreateProvider()
  const updateProvider = useUpdateProvider()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [importText, setImportText] = useState('')
  const [editingProvider, setEditingProvider] =
    useState<CustomOAuthProvider | null>(null)

  const handleCreate = () => {
    setEditingProvider(null)
    setDialogOpen(true)
  }

  const handleEdit = (provider: CustomOAuthProvider) => {
    setEditingProvider(provider)
    setDialogOpen(true)
  }

  const handleDialogChange = (open: boolean) => {
    setDialogOpen(open)
    if (!open) {
      setEditingProvider(null)
    }
  }

  const exportConfig = async () => {
    const safeProviders = providers.map(redactProviderSecret)
    const text = JSON.stringify(
      {
        CustomOAuthProviders: safeProviders,
        providers: safeProviders,
      },
      null,
      2
    )
    const blob = new Blob([text], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'renewapi-custom-oauth-providers.json'
    link.click()
    URL.revokeObjectURL(url)

    try {
      await navigator.clipboard.writeText(text)
      toast.success(t('Custom OAuth providers JSON exported and copied'))
    } catch {
      toast.success(t('Custom OAuth providers JSON exported'))
    }
  }

  const openImportDialog = () => {
    const safeProviders = providers.map(redactProviderSecret)
    setImportText(
      JSON.stringify(
        {
          CustomOAuthProviders: safeProviders,
        },
        null,
        2
      )
    )
    setImportOpen(true)
  }

  const importConfig = async () => {
    try {
      const raw = JSON.parse(importText) as CustomOAuthImportPayload
      const importProviders = Array.isArray(raw)
        ? raw
        : (raw.CustomOAuthProviders ?? raw.providers ?? [])

      if (!Array.isArray(importProviders)) {
        throw new Error(t('Custom OAuth JSON must contain a providers array'))
      }

      const existingBySlug = new Map(
        providers.map((provider) => [provider.slug, provider])
      )

      // Every provider is written with its own request, so a failure halfway
      // through leaves the earlier ones already persisted. Validate the whole
      // payload before writing any of it.
      const pending: PreparedImport[] = []
      const missingSecrets: string[] = []
      const invalidProviders: string[] = []

      for (const provider of importProviders) {
        const label =
          (typeof provider?.slug === 'string' && provider.slug) ||
          (typeof provider?.name === 'string' && provider.name) ||
          t('Unnamed provider')
        const existing =
          typeof provider?.slug === 'string'
            ? existingBySlug.get(provider.slug)
            : undefined

        if (!existing && provider?.client_secret === REDACTED_SECRET) {
          missingSecrets.push(label)
          continue
        }

        try {
          pending.push({
            payload: toProviderPayload(provider, existing),
            existing,
          })
        } catch {
          invalidProviders.push(label)
        }
      }

      if (missingSecrets.length > 0) {
        throw new Error(
          t(
            'These providers do not exist on this instance and their client secrets were redacted on export. Fill in the client secrets before importing: {{list}}',
            { list: missingSecrets.join(', ') }
          )
        )
      }

      if (invalidProviders.length > 0) {
        throw new Error(
          t('These providers are missing required fields: {{list}}', {
            list: invalidProviders.join(', '),
          })
        )
      }

      let created = 0
      let updated = 0

      for (const item of pending) {
        if (item.existing) {
          await updateProvider.mutateAsync({
            id: item.existing.id,
            data: item.payload,
          })
          updated += 1
        } else {
          await createProvider.mutateAsync(item.payload)
          created += 1
        }
      }

      setImportOpen(false)
      toast.success(
        t('Custom OAuth imported: {{created}} created, {{updated}} updated', {
          created,
          updated,
        })
      )
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Invalid Custom OAuth JSON')
      )
    }
  }

  if (isLoading) {
    return (
      <SettingsSection title={t('Custom OAuth Providers')}>
        <div className='text-muted-foreground py-8 text-center text-sm'>
          {t('Loading...')}
        </div>
      </SettingsSection>
    )
  }

  return (
    <SettingsSection title={t('Custom OAuth Providers')}>
      <SettingsPageActionsPortal>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={exportConfig}
        >
          <Download data-icon='inline-start' />
          <span>{t('Export JSON')}</span>
        </Button>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={openImportDialog}
        >
          <Upload data-icon='inline-start' />
          <span>{t('Import JSON')}</span>
        </Button>
      </SettingsPageActionsPortal>
      <ProviderTable
        providers={providers}
        onEdit={handleEdit}
        onCreate={handleCreate}
      />

      <ProviderFormDialog
        open={dialogOpen}
        onOpenChange={handleDialogChange}
        provider={editingProvider}
      />

      <Dialog open={importOpen} onOpenChange={setImportOpen}>
        <DialogContent className='sm:max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{t('Import Custom OAuth providers JSON')}</DialogTitle>
            <DialogDescription>
              {t(
                'Paste an exported Custom OAuth providers payload. Providers are matched by slug; redacted client secrets keep existing saved secrets.'
              )}
            </DialogDescription>
          </DialogHeader>
          <Textarea
            value={importText}
            onChange={(event) => setImportText(event.target.value)}
            className='min-h-80 font-mono text-xs'
            spellCheck={false}
          />
          <DialogFooter>
            <Button variant='outline' onClick={() => setImportOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button
              onClick={importConfig}
              disabled={createProvider.isPending || updateProvider.isPending}
            >
              {t('Import JSON')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SettingsSection>
  )
}
