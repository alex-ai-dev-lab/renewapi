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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { ContentLoading } from '@/components/content-loading'

export function RouteLoading() {
  const { t } = useTranslation()
  const [slow, setSlow] = useState(false)
  useEffect(() => {
    const timer = window.setTimeout(() => setSlow(true), 10000)
    return () => window.clearTimeout(timer)
  }, [])
  return (
    <div className='bg-background text-foreground flex min-h-64 flex-col items-center justify-center gap-3 px-4 py-8 text-center'>
      <ContentLoading className='min-h-16' />
      <p>{t('Loading...')}</p>
      {slow && (
        <>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Loading is taking longer than expected. Check your connection or reload this page.'
            )}
          </p>
          <Button variant='outline' onClick={() => window.location.reload()}>
            {t('Reload page')}
          </Button>
        </>
      )}
    </div>
  )
}
