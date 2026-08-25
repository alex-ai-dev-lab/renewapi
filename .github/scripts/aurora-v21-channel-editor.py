from pathlib import Path
import re

ROOT = Path('.')
SRC = ROOT / 'web/default/src'
OLD = SRC / 'features/channels/components/drawers/channel-mutate-drawer.tsx'
NEW = SRC / 'features/channels/components/editor/channel-editor.tsx'


def read(path: Path) -> str:
    return path.read_text(encoding='utf-8')


def write(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding='utf-8')


def replace_once(text: str, old: str, new: str, label: str) -> str:
    count = text.count(old)
    if count != 1:
        raise SystemExit(f'{label}: expected exactly 1 match, got {count}')
    return text.replace(old, new, 1)


def regex_once(text: str, pattern: str, replacement: str, label: str) -> str:
    value, count = re.subn(pattern, replacement, text, count=1, flags=re.S | re.M)
    if count != 1:
        raise SystemExit(f'{label}: expected exactly 1 match, got {count}')
    return value


if not OLD.exists():
    raise SystemExit(f'missing source editor: {OLD}')

editor = read(OLD)

editor = regex_once(
    editor,
    r"import \{\n  Sheet,\n  SheetClose,\n  SheetContent,\n  SheetDescription,\n  SheetFooter,\n  SheetHeader,\n  SheetTitle,\n\} from '@/components/ui/sheet'\n",
    '',
    'remove sheet imports',
)
editor = replace_once(
    editor,
    "import { useQueryClient } from '@tanstack/react-query'\n",
    "import { useQueryClient } from '@tanstack/react-query'\nimport { useBlocker } from '@tanstack/react-router'\n",
    'add blocker import',
)
editor = replace_once(
    editor,
    "import { MultiSelect } from '@/components/multi-select'\n",
    "import { ConfirmDialog } from '@/components/confirm-dialog'\nimport { MultiSelect } from '@/components/multi-select'\n",
    'add confirm dialog import',
)
editor = regex_once(
    editor,
    r"import \{\n  sideDrawerContentClassName,\n  sideDrawerFooterClassName,\n  sideDrawerFormClassName,\n  sideDrawerHeaderClassName,\n  sideDrawerSectionClassName,\n  sideDrawerSwitchItemClassName,\n\} from '@/components/drawer-layout'\n",
    "import {\n  sideDrawerSectionClassName,\n  sideDrawerSwitchItemClassName,\n} from '@/components/drawer-layout'\n",
    'trim drawer layout imports',
)
editor = editor.replace("import { useChannels } from '../channels-provider'\n", '')
editor = replace_once(editor, "} from './sections'\n", "} from '../drawers/sections'\n", 'section import path')

editor = regex_once(
    editor,
    r"type ChannelMutateDrawerProps = \{\n  open: boolean\n  onOpenChange: \(open: boolean\) => void\n  currentRow\?: Channel \| null\n\}",
    "type ChannelEditorProps = {\n  currentRow?: Channel | null\n  onClose: () => void\n}",
    'editor props',
)
editor = regex_once(
    editor,
    r"export function ChannelMutateDrawer\(\{\n  open,\n  onOpenChange,\n  currentRow,\n\}: ChannelMutateDrawerProps\) \{",
    "export function ChannelEditor({ currentRow, onClose }: ChannelEditorProps) {\n  const open = true",
    'editor signature',
)
editor = editor.replace("  const { setOpen } = useChannels()\n", '')

editor = regex_once(
    editor,
    r"  const handleSuccess = useCallback\(\(\) => \{\n    queryClient.invalidateQueries\(\{ queryKey: channelsQueryKeys.lists\(\) \}\)\n    if \(channelId\) \{\n      queryClient.invalidateQueries\(\{\n        queryKey: channelsQueryKeys.detail\(channelId\),\n      \}\)\n    \}\n    onOpenChange\(false\)\n    setOpen\(null\)\n  \}, \[channelId, queryClient, onOpenChange, setOpen\]\)",
    """  const handleSuccess = useCallback(() => {\n    queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })\n    if (channelId) {\n      queryClient.invalidateQueries({\n        queryKey: channelsQueryKeys.detail(channelId),\n      })\n    }\n    form.reset(form.getValues())\n    if (!isEditing) onClose()\n  }, [channelId, queryClient, form, isEditing, onClose])""",
    'page success behavior',
)

editor = replace_once(
    editor,
    "  const isSubmitting = channelMutation.isPending\n",
    """  const isSubmitting = channelMutation.isPending\n  const isDirty = form.formState.isDirty\n  const blocker = useBlocker({\n    shouldBlockFn: () => isDirty && !isSubmitting,\n    enableBeforeUnload: isDirty && !isSubmitting,\n    withResolver: true,\n  })\n""",
    'unsaved blocker',
)

editor = regex_once(
    editor,
    r"  // Handle drawer close\n  const handleOpenChange = useCallback\(\n    \(v: boolean\) => \{\n      onOpenChange\(v\)\n      if \(!v\) \{\n        initializedFormTargetRef.current = null\n        setEditingConfigVersion\(undefined\)\n        form.reset\(CHANNEL_FORM_DEFAULT_VALUES\)\n        setAdvancedSettingsOpen\(false\)\n        resetCodexCredential\(\)\n      \}\n    \},\n    \[onOpenChange, form, resetCodexCredential\]\n  \)\n",
    """  const handleCancel = useCallback(() => {\n    onClose()\n  }, [onClose])\n""",
    'replace drawer close handler',
)

advanced_marker = "  const handleAdvancedSettingsOpenChange = useCallback((nextOpen: boolean) => {\n"
shortcut = """  useEffect(() => {\n    const handleShortcut = (event: KeyboardEvent) => {\n      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 's') {\n        event.preventDefault()\n        if (!isSubmitting) void form.handleSubmit(onSubmit)()\n      }\n    }\n    window.addEventListener('keydown', handleShortcut)\n    return () => window.removeEventListener('keydown', handleShortcut)\n  }, [form, isSubmitting, onSubmit])\n\n"""
editor = replace_once(editor, advanced_marker, shortcut + advanced_marker, 'insert save shortcut')

nav = r'''
const CHANNEL_EDITOR_NAV_ITEMS = [
  { id: 'channel-basic', label: 'Basic Information' },
  { id: 'channel-api-access', label: 'API Access' },
  { id: 'channel-auth', label: 'Credentials & Authentication' },
  { id: 'channel-models', label: 'Models & Groups' },
  { id: 'channel-endpoints', label: 'Model Endpoints' },
  { id: 'channel-advanced', label: 'Advanced Settings' },
] as const

type ChannelEditorNavProps = {
  onAdvancedOpen: () => void
}

function ChannelEditorSectionNav({ onAdvancedOpen }: ChannelEditorNavProps) {
  const { t } = useTranslation()
  const [activeSection, setActiveSection] = useState(
    CHANNEL_EDITOR_NAV_ITEMS[0].id
  )

  useEffect(() => {
    const elements = CHANNEL_EDITOR_NAV_ITEMS.map((item) =>
      document.getElementById(item.id)
    ).filter((element): element is HTMLElement => Boolean(element))
    if (elements.length === 0) return

    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0]
        if (visible?.target.id) {
          setActiveSection(
            visible.target.id as (typeof CHANNEL_EDITOR_NAV_ITEMS)[number]['id']
          )
        }
      },
      { rootMargin: '-18% 0px -62% 0px', threshold: [0, 0.1, 0.35] }
    )
    elements.forEach((element) => observer.observe(element))
    return () => observer.disconnect()
  }, [])

  const selectSection = (id: string) => {
    if (id === 'channel-advanced') onAdvancedOpen()
    setActiveSection(id as (typeof CHANNEL_EDITOR_NAV_ITEMS)[number]['id'])
    window.requestAnimationFrame(() => {
      document.getElementById(id)?.scrollIntoView({
        behavior: 'smooth',
        block: 'start',
      })
    })
  }

  return (
    <>
      <aside className='hidden lg:block'>
        <nav
          aria-label={t('Channel configuration sections')}
          className='glass-tile sticky top-24 space-y-1 p-3'
        >
          <div className='text-muted-foreground px-2 pb-2 text-[11px] font-semibold tracking-[0.14em] uppercase'>
            {t('Configuration')}
          </div>
          {CHANNEL_EDITOR_NAV_ITEMS.map((item) => (
            <button
              key={item.id}
              type='button'
              aria-current={activeSection === item.id ? 'location' : undefined}
              onClick={() => selectSection(item.id)}
              className={
                activeSection === item.id
                  ? 'bg-primary/10 text-primary flex w-full items-center rounded-xl px-3 py-2 text-left text-sm font-medium'
                  : 'text-muted-foreground hover:bg-muted/50 hover:text-foreground flex w-full items-center rounded-xl px-3 py-2 text-left text-sm transition-colors'
              }
            >
              {t(item.label)}
            </button>
          ))}
        </nav>
      </aside>

      <div className='sticky top-16 z-20 lg:hidden'>
        <div className='glass-tile p-2'>
          <Select value={activeSection} onValueChange={(value) => value && selectSection(value)}>
            <SelectTrigger aria-label={t('Channel configuration section')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                {CHANNEL_EDITOR_NAV_ITEMS.map((item) => (
                  <SelectItem key={item.id} value={item.id}>
                    {t(item.label)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>
      </div>
    </>
  )
}

'''
editor = replace_once(
    editor,
    'export function ChannelEditor({ currentRow, onClose }: ChannelEditorProps) {',
    nav + 'export function ChannelEditor({ currentRow, onClose }: ChannelEditorProps) {',
    'insert section nav',
)

return_marker = "  return (\n    <>\n      <Sheet open={open} onOpenChange={handleOpenChange}>"
return_pos = editor.rfind(return_marker)
if return_pos < 0:
    raise SystemExit('page shell: return Sheet marker not found')
content_marker = "        <SheetContent className={sideDrawerContentClassName('sm:max-w-3xl')}>"
content_pos = editor.find(content_marker, return_pos)
if content_pos < 0:
    raise SystemExit('page shell: SheetContent marker not found')
inner_start = content_pos + len(content_marker)
closing_marker = "\n        </SheetContent>\n      </Sheet>\n\n      {paramOverrideEditorOpen && ("
closing_pos = editor.find(closing_marker, inner_start)
if closing_pos < 0:
    raise SystemExit('page shell: closing Sheet marker not found')
inner = editor[inner_start:closing_pos]
suffix = editor[closing_pos + len("\n        </SheetContent>\n      </Sheet>\n\n"):]

inner = regex_once(
    inner,
    r"\n          <SheetHeader className=\{sideDrawerHeaderClassName\(\)\}>[\s\S]*?</SheetHeader>\n",
    r'''
          <div
            data-ui='channel-editor-summary'
            className='glass-tile flex flex-col gap-2 p-5 sm:p-6'
          >
            <div className='flex items-center gap-3'>
              <span className='bg-muted flex size-10 shrink-0 items-center justify-center rounded-xl'>
                {getLobeIcon(`${getChannelTypeIcon(currentType)}.Color`, 24)}
              </span>
              <div className='min-w-0'>
                <h2 className='truncate text-lg font-bold tracking-tight'>
                  {isEditing ? t('Edit Channel') : t('Create Channel')}
                </h2>
                <p className='text-muted-foreground text-sm'>
                  {t(currentTypeLabel)}
                  {channelId ? ` · #${channelId}` : ''}
                </p>
              </div>
            </div>
            <p className='text-muted-foreground text-sm'>
              {isEditing
                ? t("Update channel configuration and click save when you're done.")
                : t('Add a new channel by providing the necessary information.')}
            </p>
          </div>
''',
    'replace Sheet header',
)
inner = replace_once(
    inner,
    "              className={sideDrawerFormClassName('gap-5')}",
    "              className='flex min-w-0 flex-col gap-5'",
    'page form class',
)
inner = regex_once(
    inner,
    r"\n          <SheetFooter className=\{sideDrawerFooterClassName\(\)\}>[\s\S]*?</SheetFooter>\n",
    r'''
          <div
            data-ui='channel-editor-actions'
            className='border-border/60 bg-background/85 sticky bottom-24 z-20 mt-2 flex flex-wrap items-center justify-end gap-2 rounded-2xl border p-3 shadow-lg backdrop-blur-xl sm:bottom-28'
          >
            <Button type='button' variant='outline' disabled={isSubmitting} onClick={handleCancel}>
              {t('Cancel')}
            </Button>
            <Button
              form='channel-form'
              type='submit'
              disabled={isSubmitting || isChannelDetailLoading || isChannelDetailUnavailable}
            >
              {isSubmitting && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {isEditing ? t('Update Channel') : t('Save changes')}
            </Button>
          </div>
''',
    'replace Sheet footer',
)

for component, section_id in [
    ('ChannelBasicSection', 'channel-basic'),
    ('ChannelApiAccessSection', 'channel-api-access'),
    ('ChannelAuthSection', 'channel-auth'),
    ('ChannelModelsSection', 'channel-models'),
]:
    opening = f'<{component}>'
    closing = f'</{component}>'
    inner = replace_once(inner, opening, f"<div id='{section_id}' className='scroll-mt-28'>\n                    {opening}", f'{component} opening anchor')
    inner = replace_once(inner, closing, f"{closing}\n                  </div>", f'{component} closing anchor')

inner = regex_once(
    inner,
    r"(\n                  <ChannelModelEndpointsSection\n                    channelId=\{channelId \?\? undefined\}\n                    models=\{currentModels\}\n                  />)",
    r"\n                  <div id='channel-endpoints' className='scroll-mt-28'>\1\n                  </div>",
    'model endpoints anchor',
)
inner = replace_once(inner, '                  <ChannelAdvancedSection\n', "                  <div id='channel-advanced' className='scroll-mt-28'>\n                    <ChannelAdvancedSection\n", 'advanced opening anchor')
inner = replace_once(inner, '                  </ChannelAdvancedSection>', '                    </ChannelAdvancedSection>\n                  </div>', 'advanced closing anchor')

page_shell = """  return (\n    <>\n      <div data-ui='channel-editor-page' className='grid min-w-0 gap-5 lg:grid-cols-[220px_minmax(0,1fr)]'>\n        <ChannelEditorSectionNav onAdvancedOpen={() => handleAdvancedSettingsOpenChange(true)} />\n        <div className='min-w-0 space-y-5'>""" + inner + """\n        </div>\n      </div>\n\n""" + suffix
editor = editor[:return_pos] + page_shell

confirm_marker = """      <StatusCodeRiskDialog\n        open={statusCodeRiskOpen}\n"""
confirm_ui = """      <ConfirmDialog\n        open={blocker.status === 'blocked'}\n        onOpenChange={(nextOpen) => {\n          if (!nextOpen && blocker.status === 'blocked') blocker.reset()\n        }}\n        title={t('Discard unsaved changes?')}\n        desc={t('You have unsaved channel changes. Leaving now will discard them.')}\n        confirmText={t('Discard changes')}\n        destructive\n        handleConfirm={() => {\n          if (blocker.status === 'blocked') blocker.proceed()\n        }}\n      />\n\n"""
editor = replace_once(editor, confirm_marker, confirm_ui + confirm_marker, 'insert unsaved confirmation')
editor = editor.replace('Close this drawer and try again before saving changes.', 'Return to the channel list and try again before saving changes.')

for forbidden in ["from '@/components/ui/sheet'", 'SheetContent', 'SheetFooter', 'SheetHeader', 'SheetClose', 'sideDrawerContentClassName', 'sideDrawerFormClassName', 'sideDrawerHeaderClassName', 'sideDrawerFooterClassName', 'useChannels', 'setOpen(null)']:
    if forbidden in editor:
        raise SystemExit(f'page editor still contains forbidden drawer token: {forbidden}')

write(NEW, editor)

old_test = SRC / 'features/channels/components/drawers/channel-mutate-drawer.test.tsx'
new_test = SRC / 'features/channels/components/editor/channel-editor.test.tsx'
test_text = read(old_test).replace('ChannelMutateDrawer edit config version contract', 'ChannelEditor edit config version contract')
write(new_test, test_text)

write(SRC / 'features/channels/components/editor/channel-editor-page.tsx', r'''/*
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
import type { ReactNode } from 'react'
import { useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { getChannel } from '../../api'
import { channelsQueryKeys } from '../../lib'
import { ChannelEditor } from './channel-editor'

type ChannelEditorPageProps =
  | { mode: 'create'; channelId?: never }
  | { mode: 'edit'; channelId: string }

export function ChannelEditorPage(props: ChannelEditorPageProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const parsedId = props.mode === 'edit' ? Number(props.channelId) : null
  const validId = parsedId !== null && Number.isInteger(parsedId) && parsedId > 0

  const channelQuery = useQuery({
    queryKey: channelsQueryKeys.detail(validId ? parsedId : 0),
    queryFn: ({ signal }) => getChannel(parsedId as number, { signal }),
    enabled: props.mode === 'edit' && validId,
  })

  const goBack = useCallback(() => {
    void navigate({ to: '/channels' })
  }, [navigate])

  let content: ReactNode
  if (props.mode === 'edit' && !validId) {
    content = <ErrorState title={t('Invalid channel ID')} description={t('The requested channel ID is invalid.')} action={<Button variant='outline' size='sm' onClick={goBack}><ArrowLeft className='size-4' />{t('Back to Channels')}</Button>} />
  } else if (props.mode === 'edit' && channelQuery.isLoading) {
    content = <div className='space-y-4'><Skeleton className='h-28 w-full rounded-2xl' /><Skeleton className='h-96 w-full rounded-2xl' /></div>
  } else if (props.mode === 'edit' && (channelQuery.isError || channelQuery.data?.success === false || !channelQuery.data?.data)) {
    content = <ErrorState title={t('Unable to load channel')} description={channelQuery.data?.message || t('The channel could not be loaded. It may have been removed.')} onRetry={() => void channelQuery.refetch()} action={<Button variant='outline' size='sm' onClick={goBack}><ArrowLeft className='size-4' />{t('Back to Channels')}</Button>} />
  } else {
    content = <ChannelEditor currentRow={props.mode === 'edit' ? channelQuery.data?.data : null} onClose={goBack} />
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{props.mode === 'edit' ? t('Edit Channel') : t('Create Channel')} <span className='text-aurora'>{t('Workspace')}</span></SectionPageLayout.Title>
      <SectionPageLayout.Description>{props.mode === 'edit' ? t('Configure provider access, models, routing, security, and advanced channel behavior in one workspace.') : t('Create a channel with provider access, models, routing, and advanced controls in one workspace.')}</SectionPageLayout.Description>
      <SectionPageLayout.Actions><Button variant='outline' size='sm' onClick={goBack}><ArrowLeft className='size-4' />{t('Back to Channels')}</Button></SectionPageLayout.Actions>
      <SectionPageLayout.Content>{content}</SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
''')

route_license = '''/*
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
'''

def guard_route(route_id: str) -> str:
    return route_license + f'''import {{ createFileRoute, redirect }} from '@tanstack/react-router'\nimport {{ useAuthStore }} from '@/stores/auth-store'\nimport {{ ROLE }} from '@/lib/roles'\n\nexport const Route = createFileRoute('{route_id}')({{\n  beforeLoad: () => {{\n    const {{ auth }} = useAuthStore.getState()\n    if (!auth.user || auth.user.role < ROLE.ADMIN) throw redirect({{ to: '/403' }})\n  }},\n}})\n'''

write(SRC / 'routes/_authenticated/channels/new.tsx', guard_route('/_authenticated/channels/new'))
write(SRC / 'routes/_authenticated/channels/$channelId/edit.tsx', guard_route('/_authenticated/channels/$channelId/edit'))
write(SRC / 'routes/_authenticated/channels/new.lazy.tsx', route_license + r'''import { createLazyFileRoute } from '@tanstack/react-router'
import { ChannelEditorPage } from '@/features/channels/components/editor/channel-editor-page'

export const Route = createLazyFileRoute('/_authenticated/channels/new')({ component: ChannelCreatePage })
function ChannelCreatePage() { return <ChannelEditorPage mode='create' /> }
''')
write(SRC / 'routes/_authenticated/channels/$channelId/edit.lazy.tsx', route_license + r'''import { createLazyFileRoute } from '@tanstack/react-router'
import { ChannelEditorPage } from '@/features/channels/components/editor/channel-editor-page'

export const Route = createLazyFileRoute('/_authenticated/channels/$channelId/edit')({ component: ChannelEditPage })
function ChannelEditPage() { const { channelId } = Route.useParams(); return <ChannelEditorPage mode='edit' channelId={channelId} /> }
''')

primary = SRC / 'features/channels/components/channels-primary-buttons.tsx'
text = read(primary)
text = replace_once(text, "import { useQueryClient } from '@tanstack/react-query'\n", "import { useQueryClient } from '@tanstack/react-query'\nimport { useNavigate } from '@tanstack/react-router'\n", 'primary navigate import')
text = replace_once(text, "  const { t, i18n } = useTranslation()\n", "  const { t, i18n } = useTranslation()\n  const navigate = useNavigate()\n", 'primary navigate hook')
text = replace_once(text, "    setOpen,\n    setCurrentRow,\n", '', 'remove create drawer setters')
text = replace_once(text, """            onClick={() => {\n              setCurrentRow(null)\n              setOpen('create-channel')\n            }}""", """            onClick={() => {\n              void navigate({ to: '/channels/new' })\n            }}""", 'create route navigation')
write(primary, text)

row_actions = SRC / 'features/channels/components/data-table-row-actions.tsx'
text = read(row_actions)
text = replace_once(text, "import { useQueryClient } from '@tanstack/react-query'\n", "import { useQueryClient } from '@tanstack/react-query'\nimport { useNavigate } from '@tanstack/react-router'\n", 'row navigate import')
text = replace_once(text, "  const { t } = useTranslation()\n", "  const { t } = useTranslation()\n  const navigate = useNavigate()\n", 'row navigate hook')
text = replace_once(text, """  const handleEdit = () => {\n    setCurrentRow(channel)\n    setOpen('update-channel')\n  }""", """  const handleEdit = () => {\n    void navigate({\n      to: '/channels/$channelId/edit',\n      params: { channelId: String(channel.id) },\n    })\n  }""", 'edit route navigation')
write(row_actions, text)

provider = SRC / 'features/channels/components/channels-provider.tsx'
text = read(provider)
text = replace_once(text, "  | 'create-channel'\n", '', 'remove create dialog type')
text = replace_once(text, "  | 'update-channel'\n", '', 'remove update dialog type')
write(provider, text)

dialogs = SRC / 'features/channels/components/channels-dialogs.tsx'
text = read(dialogs)
text = replace_once(text, "import { ChannelMutateDrawer } from './drawers/channel-mutate-drawer'\n", '', 'remove drawer import')
text = regex_once(text, r"\n      \{\/\* Channel Create\/Update Drawer \*\/\}\n      <ChannelMutateDrawer[\s\S]*?\n      />\n", '\n', 'remove drawer render')
write(dialogs, text)

OLD.unlink()
old_test.unlink()
print('Channel Editor migration source generated successfully.')
