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
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'

export function RenameGroupDialog(props: {
  name: string
  pending: boolean
  onClose: () => void
  onSubmit: (name: string) => void
}) {
  const { t } = useTranslation()
  const schema = z.object({
    name: z
      .string()
      .trim()
      .regex(
        /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/,
        t(
          'Use 1-64 letters, numbers, underscores or hyphens; start with a letter or number.'
        )
      )
      .refine(
        (name) => name.toLowerCase() !== 'auto',
        t('This group name is reserved.')
      ),
  })
  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: { name: props.name },
  })
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !props.pending) props.onClose()
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Rename group')}</DialogTitle>
          <DialogDescription>
            {t(
              'Subscriptions, users, API keys and channel permissions will follow the new name.'
            )}
          </DialogDescription>
        </DialogHeader>
        <form
          className='flex flex-col gap-4'
          onSubmit={form.handleSubmit((values) => props.onSubmit(values.name))}
        >
          <FieldGroup>
            <Field data-invalid={!!form.formState.errors.name}>
              <FieldLabel htmlFor='rename-group-name'>
                {t('Group name')}
              </FieldLabel>
              <Input
                id='rename-group-name'
                autoFocus
                disabled={props.pending}
                aria-invalid={!!form.formState.errors.name}
                {...form.register('name')}
              />
              <FieldError errors={[form.formState.errors.name]} />
            </Field>
          </FieldGroup>
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={props.pending}
              onClick={props.onClose}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='submit'
              disabled={
                props.pending || form.watch('name').trim() === props.name
              }
            >
              {props.pending ? t('Saving...') : t('Rename group')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export function DeleteGroupDialog(props: {
  name: string
  pending: boolean
  onClose: () => void
  onSubmit: () => void
}) {
  const { t } = useTranslation()
  return (
    <AlertDialog
      open
      onOpenChange={(open) => {
        if (!open && !props.pending) props.onClose()
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {t('Delete group {{name}}?', { name: props.name })}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {t(
              'Groups bound to subscriptions or still in use cannot be deleted.'
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <Button
            variant='outline'
            disabled={props.pending}
            onClick={props.onClose}
          >
            {t('Cancel')}
          </Button>
          <Button
            variant='destructive'
            disabled={props.pending}
            onClick={props.onSubmit}
          >
            {props.pending ? t('Deleting...') : t('Delete')}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
