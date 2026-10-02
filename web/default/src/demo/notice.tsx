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
import { useTranslation } from 'react-i18next'

export function DemoNotice() {
  const { t } = useTranslation()
  return (
    <div className='pointer-events-none fixed inset-x-16 top-3 z-[100] flex justify-center'>
      <span className='bg-background/95 text-foreground border-border max-w-full rounded-full border px-3 py-1 text-center text-[10px] shadow-sm'>
        {t('Demo · Virtual data · Stored only in your browser')}
      </span>
    </div>
  )
}
