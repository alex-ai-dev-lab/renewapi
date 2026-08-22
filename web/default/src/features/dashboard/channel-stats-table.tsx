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
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { ChannelStat } from './stats-api'
import { useDashboardHealthThresholds } from './use-dashboard-controls'

interface ChannelStatsTableProps {
  data: ChannelStat[]
}

export function ChannelStatsTable({ data }: ChannelStatsTableProps) {
  const healthThresholds = useDashboardHealthThresholds()

  return (
    <Card>
      <CardHeader>
        <CardTitle>Top Channels</CardTitle>
        <CardDescription>Channel performance statistics</CardDescription>
      </CardHeader>
      <CardContent>
        <div className='border-border/60 overflow-hidden rounded-[calc(var(--radius)*1.125)] border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Channel</TableHead>
                <TableHead className='text-right'>Requests</TableHead>
                <TableHead className='text-right'>Success Rate</TableHead>
                <TableHead className='text-right'>Avg First Token</TableHead>
                <TableHead className='text-right'>Cost</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={5}
                    className='text-muted-foreground text-center'
                  >
                    No data available
                  </TableCell>
                </TableRow>
              ) : (
                data.slice(0, 10).map((channel) => (
                  <TableRow key={channel.channel_id}>
                    <TableCell className='font-medium'>
                      {channel.channel_name}
                      <span className='text-muted-foreground ml-2 text-xs'>
                        #{channel.channel_id}
                      </span>
                    </TableCell>
                    <TableCell className='text-right tabular-nums'>
                      {channel.total_requests.toLocaleString()}
                    </TableCell>
                    <TableCell className='text-right'>
                      <Badge
                        variant={
                          channel.success_rate >=
                          healthThresholds.successRateGoodThreshold
                            ? 'default'
                            : channel.success_rate >=
                                healthThresholds.successRateDegradedThreshold
                              ? 'secondary'
                              : 'destructive'
                        }
                        className='font-mono'
                      >
                        {channel.success_rate.toFixed(2)}%
                      </Badge>
                    </TableCell>
                    <TableCell className='text-right font-mono'>
                      {channel.avg_first_token > 0
                        ? `${channel.avg_first_token.toFixed(0)}ms`
                        : 'N/A'}
                    </TableCell>
                    <TableCell className='text-right font-mono'>
                      ${channel.total_cost.toFixed(4)}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  )
}
