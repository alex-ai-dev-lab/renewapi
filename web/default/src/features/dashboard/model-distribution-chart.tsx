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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { ModelStat } from './stats-api'

interface ModelDistributionChartProps {
  data: ModelStat[]
}

export function ModelDistributionChart(props: ModelDistributionChartProps) {
  const { t, i18n } = useTranslation()
  const totalRequests = props.data.reduce(
    (sum, model) => sum + model.total_requests,
    0
  )
  return (
    <section className='obsidian-panel'>
      <header className='obsidian-panel-heading'>
        <h2>{t('Model Distribution')}</h2>
        <p className='text-muted-foreground text-xs'>
          {t('Request distribution by model')}
        </p>
      </header>
      <div className='max-h-96 overflow-auto'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='px-4 text-xs'>{t('Model')}</TableHead>
              <TableHead className='text-right text-xs'>
                {t('Requests')}
              </TableHead>
              <TableHead className='pr-4 text-right text-xs'>
                {t('Share')}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.data.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={3}
                  className='text-muted-foreground h-24 text-center'
                >
                  {t('No data available')}
                </TableCell>
              </TableRow>
            ) : (
              props.data.map((model) => {
                const share =
                  totalRequests > 0
                    ? (model.total_requests / totalRequests) * 100
                    : 0
                return (
                  <TableRow key={model.model_name}>
                    <TableCell className='px-4 py-3 text-xs font-medium'>
                      {model.model_name}
                    </TableCell>
                    <TableCell className='text-right text-xs tabular-nums'>
                      {model.total_requests.toLocaleString(
                        i18n.resolvedLanguage
                      )}
                    </TableCell>
                    <TableCell className='pr-4 text-right text-xs tabular-nums'>
                      <div className='flex items-center justify-end gap-3'>
                        <meter
                          className='obsidian-share-meter'
                          min={0}
                          max={100}
                          value={share}
                          aria-label={t('Request share for {{model}}', {
                            model: model.model_name,
                          })}
                        />
                        <span className='min-w-12'>{share.toFixed(1)}%</span>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>
    </section>
  )
}
