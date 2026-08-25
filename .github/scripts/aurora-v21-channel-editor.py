from pathlib import Path
import re

ROOT = Path("web/default/src")
OLD = ROOT / "features/channels/components/drawers/channel-mutate-drawer.tsx"
EDITOR_DIR = ROOT / "features/channels/components/editor"
NEW = EDITOR_DIR / "channel-editor-page.tsx"


def replace_once(text: str, old: str, new: str, label: str) -> str:
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{label}: expected exactly 1 match, got {count}")
    return text.replace(old, new, 1)


def regex_once(text: str, pattern: str, replacement: str, label: str) -> str:
    next_text, count = re.subn(pattern, replacement, text, count=1, flags=re.S | re.M)
    if count != 1:
        raise SystemExit(f"{label}: expected exactly 1 match, got {count}")
    return next_text


def edit(relative: str, replacements: list[tuple[str, str, str]]) -> None:
    path = ROOT / relative
    text = path.read_text()
    for old, new, label in replacements:
        text = replace_once(text, old, new, f"{relative}: {label}")
    path.write_text(text)


def route_header() -> str:
    return """/*
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
"""


if not OLD.exists():
    raise SystemExit(f"missing source editor: {OLD}")

EDITOR_DIR.mkdir(parents=True, exist_ok=True)
text = OLD.read_text()

text = replace_once(
    text,
    "import { useQueryClient } from '@tanstack/react-query'\n",
    "import { useQueryClient } from '@tanstack/react-query'\nimport { useBlocker } from '@tanstack/react-router'\n",
    "router blocker import",
)
text = regex_once(
    text,
    r"import \{\n  Sheet,\n  SheetClose,\n  SheetContent,\n  SheetDescription,\n  SheetFooter,\n  SheetHeader,\n  SheetTitle,\n\} from '@/components/ui/sheet'\n",
    "",
    "sheet imports",
)
text = regex_once(
    text,
    r"import \{\n  sideDrawerContentClassName,\n  sideDrawerFooterClassName,\n  sideDrawerFormClassName,\n  sideDrawerHeaderClassName,\n  sideDrawerSectionClassName,\n  sideDrawerSwitchItemClassName,\n\} from '@/components/drawer-layout'\n",
    "import {\n  sideDrawerSectionClassName,\n  sideDrawerSwitchItemClassName,\n} from '@/components/drawer-layout'\n",
    "drawer layout imports",
)
text = replace_once(text, "import { useChannels } from '../channels-provider'\n", "", "channels context import")
text = replace_once(text, "} from './sections'\n", "} from '../drawers/sections'\n", "section import path")
text = replace_once(
    text,
    "type ChannelMutateDrawerProps = {\n  open: boolean\n  onOpenChange: (open: boolean) => void\n  currentRow?: Channel | null\n}\n",
    "type ChannelEditorPageProps = {\n  channelId?: number | null\n  onNavigateBack: () => void\n}\n",
    "editor props",
)
text = replace_once(
    text,
    "export function ChannelMutateDrawer({\n  open,\n  onOpenChange,\n  currentRow,\n}: ChannelMutateDrawerProps) {\n  const { t } = useTranslation()\n  const queryClient = useQueryClient()\n  const { setOpen } = useChannels()\n",
    "export function ChannelEditorPage({\n  channelId: requestedChannelId,\n  onNavigateBack,\n}: ChannelEditorPageProps) {\n  const { t } = useTranslation()\n  const queryClient = useQueryClient()\n  const open = true\n  const allowNavigationRef = useRef(false)\n  const currentRow = useMemo<Channel | null>(\n    () =>\n      requestedChannelId\n        ? ({ id: requestedChannelId } as Channel)\n        : null,\n    [requestedChannelId]\n  )\n",
    "component signature",
)

# Keep the proven initialization path mounted by retaining open=true. The old
# close-only reset effects are harmless but the Doubao unlock should reset on
# channel changes and unmount in page mode.
text = replace_once(
    text,
    "  useEffect(() => {\n    if (!open) {\n      resetDoubaoApiUnlock()\n    }\n  }, [open, resetDoubaoApiUnlock])\n",
    "  useEffect(() => {\n    resetDoubaoApiUnlock()\n    return resetDoubaoApiUnlock\n  }, [channelId, resetDoubaoApiUnlock])\n",
    "doubao reset effect",
)

text = replace_once(
    text,
    "  // Handle successful submission\n  const handleSuccess = useCallback(() => {\n    queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })\n    if (channelId) {\n      queryClient.invalidateQueries({\n        queryKey: channelsQueryKeys.detail(channelId),\n      })\n    }\n    onOpenChange(false)\n    setOpen(null)\n  }, [channelId, queryClient, onOpenChange, setOpen])\n",
    "  // Handle successful submission\n  const handleSuccess = useCallback(() => {\n    queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })\n    if (channelId) {\n      queryClient.invalidateQueries({\n        queryKey: channelsQueryKeys.detail(channelId),\n      })\n    }\n    form.reset(form.getValues())\n    if (!isEditing) {\n      allowNavigationRef.current = true\n      onNavigateBack()\n    }\n  }, [channelId, form, isEditing, onNavigateBack, queryClient])\n",
    "success behavior",
)

text = replace_once(
    text,
    "  const isSubmitting = channelMutation.isPending\n\n  // Submit handler\n",
    "  const isSubmitting = channelMutation.isPending\n\n  useBlocker({\n    shouldBlockFn: () => {\n      if (allowNavigationRef.current || !form.formState.isDirty || isSubmitting) {\n        return false\n      }\n      return !window.confirm(\n        t('You have unsaved changes. Leave without saving?')\n      )\n    },\n    enableBeforeUnload: () =>\n      !allowNavigationRef.current && form.formState.isDirty && !isSubmitting,\n  })\n\n  // Submit handler\n",
    "unsaved changes blocker",
)

text = regex_once(
    text,
    r"  const handleOpenChange = useCallback\(\n    \(nextOpen: boolean\) => \{\n      onOpenChange\(nextOpen\)\n      if \(!nextOpen\) \{\n        form\.reset\(CHANNEL_FORM_DEFAULT_VALUES\)\n        initializedFormTargetRef\.current = null\n        resetCodexCredential\(\)\n        resetDoubaoApiUnlock\(\)\n      \}\n    \},\n    \[onOpenChange, form, resetCodexCredential, resetDoubaoApiUnlock\]\n  \)\n\n",
    "",
    "drawer open handler",
)
text = text.replace(
    "Close this drawer and try again from the channel list.",
    "Return to the channel list and try again.",
)

old_start = """  return (
    <>
      <Sheet open={open} onOpenChange={handleOpenChange}>
        <SheetContent className={sideDrawerContentClassName('sm:max-w-3xl')}>
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle className='flex items-center gap-2'>
              {getLobeIcon(
                `${getChannelTypeIcon(currentType)}.Color`,
                18
              )}
              {isEditing ? t('Update Channel') : t('Add Channel')}
              {isEditing && channelId && (
                <Badge variant='outline'>#{channelId}</Badge>
              )}
            </SheetTitle>
            <SheetDescription>
              {isEditing
                ? t(
                    'Update channel configuration. Please save after completing the changes.'
                  )
                : t(
                    'Add a new channel by providing all necessary configuration details.'
                  )}
            </SheetDescription>
          </SheetHeader>

          <Form {...form}>
            <form
              id='channel-form'
              onSubmit={form.handleSubmit(onSubmit)}
              className={sideDrawerFormClassName('gap-5')}
            >
"""
new_start = """  return (
    <>
      <div className='mx-auto w-full max-w-[1240px] px-4 pt-6 pb-32 sm:px-6 lg:px-8'>
        <div className='lg:grid lg:grid-cols-[220px_minmax(0,1fr)] lg:gap-6'>
          <aside className='mb-4 lg:sticky lg:top-24 lg:mb-0 lg:self-start'>
            <nav
              aria-label={t('Channel editor sections')}
              className='glass-tile flex gap-1 overflow-x-auto p-2 lg:flex-col'
            >
              {[
                ['channel-basic', t('Basic Information')],
                ['channel-auth', t('Authentication')],
                ['channel-api', t('API Access')],
                ['channel-models', t('Models')],
                ['channel-endpoints', t('Model Endpoints')],
                ['channel-advanced', t('Advanced Settings')],
              ].map(([id, label]) => (
                <button
                  key={id}
                  type='button'
                  className='text-muted-foreground hover:bg-muted/60 hover:text-foreground rounded-xl px-3 py-2 text-start text-sm font-medium whitespace-nowrap transition-colors'
                  onClick={() => {
                    if (id === 'channel-advanced') {
                      handleAdvancedSettingsOpenChange(true)
                    }
                    window.requestAnimationFrame(() => {
                      document
                        .getElementById(id)
                        ?.scrollIntoView({ behavior: 'smooth', block: 'start' })
                    })
                  }}
                >
                  {label}
                </button>
              ))}
            </nav>
          </aside>

          <div className='min-w-0'>
            <div className='glass-tile mb-5 px-5 py-4 sm:px-6'>
              <div className='mb-3'>
                <Button
                  type='button'
                  variant='ghost'
                  size='sm'
                  onClick={onNavigateBack}
                >
                  {t('Back to Channels')}
                </Button>
              </div>
              <div className='flex items-center gap-2 text-base font-semibold'>
                {getLobeIcon(
                  `${getChannelTypeIcon(currentType)}.Color`,
                  18
                )}
                {isEditing ? t('Update Channel') : t('Add Channel')}
                {isEditing && channelId && (
                  <Badge variant='outline'>#{channelId}</Badge>
                )}
              </div>
              <p className='text-muted-foreground mt-1 text-sm'>
                {isEditing
                  ? t(
                      'Update channel configuration. Please save after completing the changes.'
                    )
                  : t(
                      'Add a new channel by providing all necessary configuration details.'
                    )}
              </p>
            </div>

            <Form {...form}>
              <form
                id='channel-form'
                onSubmit={form.handleSubmit(onSubmit)}
                className='flex min-w-0 flex-col gap-5'
              >
"""
text = replace_once(text, old_start, new_start, "page shell start")

old_footer = """          <SheetFooter className={sideDrawerFooterClassName()}>
            <SheetClose
              render={<Button variant='outline' disabled={isSubmitting} />}
            >
              {t('Cancel')}
            </SheetClose>
            <Button
              form='channel-form'
              type='submit'
              disabled={
                isSubmitting ||
                isChannelDetailLoading ||
                isChannelDetailUnavailable
              }
            >
              {isSubmitting && (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              )}
              {isEditing ? t('Update Channel') : t('Save changes')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
"""
new_footer = """            <div className='glass-tile sticky bottom-24 z-20 mt-5 flex flex-wrap items-center justify-end gap-2 px-4 py-3'>
              <Button
                type='button'
                variant='outline'
                disabled={isSubmitting}
                onClick={onNavigateBack}
              >
                {t('Cancel')}
              </Button>
              <Button
                form='channel-form'
                type='submit'
                disabled={
                  isSubmitting ||
                  isChannelDetailLoading ||
                  isChannelDetailUnavailable
                }
              >
                {isSubmitting && (
                  <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                )}
                {isEditing ? t('Update Channel') : t('Save changes')}
              </Button>
            </div>
          </div>
        </div>
      </div>
"""
text = replace_once(text, old_footer, new_footer, "page shell footer")

anchors = {
    "ChannelBasicSection": "channel-basic",
    "ChannelAuthSection": "channel-auth",
    "ChannelApiAccessSection": "channel-api",
    "ChannelModelsSection": "channel-models",
    "ChannelModelEndpointsSection": "channel-endpoints",
    "ChannelAdvancedSection": "channel-advanced",
}
for component, anchor in anchors.items():
    pattern = re.compile(rf"^(\s*)<{component}(?=[ >])", re.M)
    text, count = pattern.subn(
        rf"\1<span id='{anchor}' className='block scroll-mt-28' />\n\1<{component}",
        text,
        count=1,
    )
    if count != 1:
        raise SystemExit(f"anchor {component}: expected 1 JSX match, got {count}")

for forbidden in ("<Sheet", "SheetContent", "SheetFooter", "ChannelMutateDrawer"):
    if forbidden in text:
        raise SystemExit(f"drawer presentation token remains: {forbidden}")

NEW.write_text(text)
OLD.unlink()

old_test = ROOT / "features/channels/components/drawers/channel-mutate-drawer.test.tsx"
if old_test.exists():
    old_test.rename(EDITOR_DIR / "channel-editor-page.test.tsx")

edit(
    "features/channels/components/channels-primary-buttons.tsx",
    [
        (
            "import { useQueryClient } from '@tanstack/react-query'\n",
            "import { useQueryClient } from '@tanstack/react-query'\nimport { useNavigate } from '@tanstack/react-router'\n",
            "navigate import",
        ),
        (
            "  const { t, i18n } = useTranslation()\n",
            "  const { t, i18n } = useTranslation()\n  const navigate = useNavigate()\n",
            "navigate hook",
        ),
        (
            "    setOpen,\n    setCurrentRow,\n",
            "",
            "remove create drawer setters",
        ),
        (
            "            onClick={() => {\n              setCurrentRow(null)\n              setOpen('create-channel')\n            }}\n",
            "            onClick={() => void navigate({ to: '/channels/new' })}\n",
            "create route navigation",
        ),
    ],
)

edit(
    "features/channels/components/data-table-row-actions.tsx",
    [
        (
            "import { useQueryClient } from '@tanstack/react-query'\n",
            "import { useQueryClient } from '@tanstack/react-query'\nimport { useNavigate } from '@tanstack/react-router'\n",
            "navigate import",
        ),
        (
            "  const { t } = useTranslation()\n  const channel = row.original\n",
            "  const { t } = useTranslation()\n  const navigate = useNavigate()\n  const channel = row.original\n",
            "navigate hook",
        ),
        (
            "  const handleEdit = () => {\n    setCurrentRow(channel)\n    setOpen('update-channel')\n  }\n",
            "  const handleEdit = () => {\n    void navigate({\n      to: '/channels/$channelId/edit',\n      params: { channelId: String(channel.id) },\n    })\n  }\n",
            "edit route navigation",
        ),
    ],
)

provider = ROOT / "features/channels/components/channels-provider.tsx"
provider_text = provider.read_text()
for token in ("  | 'create-channel'\n", "  | 'update-channel'\n"):
    provider_text = replace_once(provider_text, token, "", f"provider remove {token.strip()}")
provider.write_text(provider_text)

dialogs = ROOT / "features/channels/components/channels-dialogs.tsx"
dialog_text = dialogs.read_text()
dialog_text = replace_once(
    dialog_text,
    "import { ChannelMutateDrawer } from './drawers/channel-mutate-drawer'\n",
    "",
    "dialog editor import",
)
dialog_text = regex_once(
    dialog_text,
    r"\n\s*\{/\* Channel Create/Update Drawer \*/\}\n\s*<ChannelMutateDrawer[\s\S]*?/>\n",
    "\n",
    "dialog editor render",
)
dialogs.write_text(dialog_text)

routes = ROOT / "routes/_authenticated/channels"
(routes / "$channelId").mkdir(parents=True, exist_ok=True)
header = route_header()
(routes / "new.tsx").write_text(
    header
    + """import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'

export const Route = createFileRoute('/_authenticated/channels/new')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
})
"""
)
(routes / "new.lazy.tsx").write_text(
    header
    + """import { createLazyFileRoute, useNavigate } from '@tanstack/react-router'
import { ChannelEditorPage } from '@/features/channels/components/editor/channel-editor-page'

export const Route = createLazyFileRoute('/_authenticated/channels/new')({
  component: ChannelCreatePage,
})

function ChannelCreatePage() {
  const navigate = useNavigate()
  return (
    <ChannelEditorPage
      onNavigateBack={() => void navigate({ to: '/channels' })}
    />
  )
}
"""
)
(routes / "$channelId/edit.tsx").write_text(
    header
    + """import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'

export const Route = createFileRoute('/_authenticated/channels/$channelId/edit')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
})
"""
)
(routes / "$channelId/edit.lazy.tsx").write_text(
    header
    + """import { createLazyFileRoute, useNavigate } from '@tanstack/react-router'
import { ChannelEditorPage } from '@/features/channels/components/editor/channel-editor-page'
import { NotFoundError } from '@/features/errors/not-found-error'

export const Route = createLazyFileRoute(
  '/_authenticated/channels/$channelId/edit'
)({
  component: ChannelEditPage,
})

function ChannelEditPage() {
  const { channelId } = Route.useParams()
  const navigate = useNavigate()
  const numericChannelId = Number(channelId)

  if (!Number.isInteger(numericChannelId) || numericChannelId <= 0) {
    return <NotFoundError />
  }

  return (
    <ChannelEditorPage
      channelId={numericChannelId}
      onNavigateBack={() => void navigate({ to: '/channels' })}
    />
  )
}
"""
)

print("channel editor migration prepared")
