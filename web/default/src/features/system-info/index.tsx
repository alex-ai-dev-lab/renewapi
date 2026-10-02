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
import { useStatus } from '@/hooks/use-status'
import { Table, TableBody, TableCell, TableRow } from '@/components/ui/table'
import { SectionPageLayout } from '@/components/layout'

export function SystemInfo() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const fields: [string, unknown][] = [
    ['Version', status?.version],
    ['System name', status?.system_name],
    ['Server address', status?.server_address],
  ]
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('System Info')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <Table>
          <TableBody>
            {fields.map(([name, value]) => (
              <TableRow key={name}>
                <TableCell>{t(name || '')}</TableCell>
                <TableCell>{String(value ?? '—')}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
