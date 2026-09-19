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
import { type Table } from '@tanstack/react-table'
import {
  ChevronLeft as ChevronLeftIcon,
  ChevronRight as ChevronRightIcon,
  ChevronsLeft as DoubleArrowLeftIcon,
  ChevronsRight as DoubleArrowRightIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn, getPageNumbers } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

type DataTablePaginationProps<TData> = {
  table: Table<TData>
}

export function DataTablePagination<TData>({
  table,
}: DataTablePaginationProps<TData>) {
  const { t } = useTranslation()
  const currentPage = table.getState().pagination.pageIndex + 1
  const totalPages = table.getPageCount()
  const pageNumbers = getPageNumbers(currentPage, totalPages)

  return (
    <div
      className={cn(
        'flex w-full flex-col-reverse items-center justify-between gap-2',
        'sm:flex-row sm:gap-3'
      )}
    >
      <div className='flex w-full items-center justify-between gap-2 sm:w-auto'>
        <div className='flex min-w-0 items-center text-xs font-medium whitespace-nowrap sm:min-w-[130px] sm:text-sm lg:hidden'>
          {t('第 {{current}} 页，共 {{total}} 页', {
            current: currentPage,
            total: totalPages,
          })}
        </div>
        <div className='flex items-center gap-2'>
          <Select
            items={[
              ...[10, 20, 30, 40, 50, 100].map((pageSize) => ({
                value: `${pageSize}`,
                label: pageSize,
              })),
            ]}
            value={`${table.getState().pagination.pageSize}`}
            onValueChange={(value) => {
              table.setPageSize(Number(value))
            }}
          >
            <SelectTrigger
              className='h-7 w-[60px] sm:w-[66px]'
              aria-label={t('每页行数')}
            >
              <SelectValue placeholder={table.getState().pagination.pageSize} />
            </SelectTrigger>
            <SelectContent side='top' alignItemWithTrigger={false}>
              <SelectGroup>
                {[10, 20, 30, 40, 50, 100].map((pageSize) => (
                  <SelectItem key={pageSize} value={`${pageSize}`}>
                    {pageSize}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
          <p className='hidden text-xs font-medium sm:block'>{t('每页行数')}</p>
        </div>
      </div>

      <div className='flex items-center sm:space-x-6 lg:space-x-8'>
        <div className='hidden min-w-[120px] items-center text-xs font-medium whitespace-nowrap lg:flex'>
          {t('第 {{current}} 页，共 {{total}} 页', {
            current: currentPage,
            total: totalPages,
          })}
        </div>
        <div className='flex items-center space-x-1.5 sm:space-x-2'>
          <Button
            variant='outline'
            className='hidden size-7 p-0 sm:inline-flex'
            onClick={() => table.setPageIndex(0)}
            disabled={!table.getCanPreviousPage()}
          >
            <span className='sr-only'>{t('前往首页')}</span>
            <DoubleArrowLeftIcon className='h-3.5 w-3.5' />
          </Button>
          <Button
            variant='outline'
            className='size-7 p-0'
            onClick={() => table.previousPage()}
            disabled={!table.getCanPreviousPage()}
          >
            <span className='sr-only'>{t('前往上一页')}</span>
            <ChevronLeftIcon className='h-3.5 w-3.5' />
          </Button>

          {/* Page number buttons */}
          {pageNumbers.map((pageNumber, index) => (
            <div key={`${pageNumber}-${index}`} className='flex items-center'>
              {pageNumber === '...' ? (
                <span className='text-muted-foreground px-1 text-sm'>...</span>
              ) : (
                <Button
                  variant={currentPage === pageNumber ? 'default' : 'outline'}
                  className='h-7 min-w-7 px-2 text-xs'
                  onClick={() => table.setPageIndex((pageNumber as number) - 1)}
                >
                  <span className='sr-only'>
                    {t('前往第 {{page}} 页', { page: pageNumber })}
                  </span>
                  {pageNumber}
                </Button>
              )}
            </div>
          ))}

          <Button
            variant='outline'
            className='size-7 p-0'
            onClick={() => table.nextPage()}
            disabled={!table.getCanNextPage()}
          >
            <span className='sr-only'>{t('前往下一页')}</span>
            <ChevronRightIcon className='h-3.5 w-3.5' />
          </Button>
          <Button
            variant='outline'
            className='hidden size-7 p-0 sm:inline-flex'
            onClick={() => table.setPageIndex(table.getPageCount() - 1)}
            disabled={!table.getCanNextPage()}
          >
            <span className='sr-only'>{t('前往末页')}</span>
            <DoubleArrowRightIcon className='h-3.5 w-3.5' />
          </Button>
        </div>
      </div>
    </div>
  )
}
