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
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'
import {
  getCoreRowModel,
  useReactTable,
  type SortingState,
  type VisibilityState,
} from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { DataTablePage } from '@/components/data-table'
import { FilterPills } from '@/components/page-primitives'
import { getModels, searchModels, getVendors } from '../api'
import {
  DEFAULT_PAGE_SIZE,
  getModelStatusOptions,
  getSyncStatusOptions,
} from '../constants'
import { modelsQueryKeys, vendorsQueryKeys } from '../lib'
import { DataTableBulkActions } from './data-table-bulk-actions'
import { useModelsColumns } from './models-columns'
import { ModelsPrimaryButtons } from './models-primary-buttons'
import { useModels } from './models-provider'
import { ModelsStats } from './models-stats'

const route = getRouteApi('/_authenticated/models/$section')

export function ModelsTable() {
  const { t } = useTranslation()
  const { selectedVendor } = useModels()

  // Table state
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({
    id: false,
    name_rule: false,
    description: false,
    tags: false,
    endpoints: false,
    bound_channels: false,
    quota_types: false,
    created_time: false,
    updated_time: false,
  })
  const [rowSelection, setRowSelection] = useState({})
  const [managementOpen, setManagementOpen] = useState(false)

  // URL state management
  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: {
      defaultPage: 1,
      defaultPageSize: DEFAULT_PAGE_SIZE,
    },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'status', type: 'array' },
      { columnId: 'vendor_id', searchKey: 'vendor', type: 'array' },
      { columnId: 'sync_official', searchKey: 'sync', type: 'array' },
    ],
  })

  // Extract filters from column filters
  const statusFilter =
    (columnFilters.find((f) => f.id === 'status')?.value as string[]) || []
  const vendorFilter =
    (columnFilters.find((f) => f.id === 'vendor_id')?.value as string[]) || []
  const syncFilter =
    (columnFilters.find((f) => f.id === 'sync_official')?.value as string[]) ||
    []

  // Fetch vendors for filter
  const { data: vendorsData } = useQuery({
    queryKey: vendorsQueryKeys.list(),
    queryFn: () => getVendors({ page_size: 1000 }),
  })

  const vendors = useMemo(
    () => vendorsData?.data?.items || [],
    [vendorsData?.data?.items]
  )

  const vendorOptions = useMemo(() => {
    return vendors.map((v) => ({
      label: v.name,
      value: String(v.id),
    }))
  }, [vendors])

  // Determine whether to use search or regular list API
  const shouldSearch = Boolean(globalFilter?.trim())

  // Apply selected vendor from context or filter
  const activeVendorFilter =
    selectedVendor ||
    (vendorFilter.length > 0 && !vendorFilter.includes('all')
      ? vendorFilter[0]
      : undefined)

  // Fetch models data
  // eslint-disable-next-line @tanstack/query/exhaustive-deps
  const { data, isLoading, isFetching, isError, error } = useQuery({
    queryKey: modelsQueryKeys.list({
      keyword: globalFilter,
      vendor: activeVendorFilter,
      status:
        statusFilter.length > 0 && !statusFilter.includes('all')
          ? statusFilter[0]
          : undefined,
      sync_official:
        syncFilter.length > 0 && !syncFilter.includes('all')
          ? syncFilter[0]
          : undefined,
      p: pagination.pageIndex + 1,
      page_size: pagination.pageSize,
    }),
    queryFn: async () => {
      if (shouldSearch || activeVendorFilter) {
        return searchModels({
          keyword: globalFilter,
          vendor: activeVendorFilter,
          status:
            statusFilter.length > 0 && !statusFilter.includes('all')
              ? statusFilter[0]
              : undefined,
          sync_official:
            syncFilter.length > 0 && !syncFilter.includes('all')
              ? syncFilter[0]
              : undefined,
          p: pagination.pageIndex + 1,
          page_size: pagination.pageSize,
        })
      }

      return getModels({
        status:
          statusFilter.length > 0 && !statusFilter.includes('all')
            ? statusFilter[0]
            : undefined,
        sync_official:
          syncFilter.length > 0 && !syncFilter.includes('all')
            ? syncFilter[0]
            : undefined,
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      })
    },
    placeholderData: (previousData) => previousData,
  })

  const models = data?.data?.items || []
  const totalCount = data?.data?.total || 0
  const vendorCounts = data?.data?.vendor_counts
  const errorDescription = error instanceof Error ? error.message : undefined

  // Columns configuration
  const columns = useModelsColumns(vendors)

  // React Table instance
  const table = useReactTable({
    data: models,
    columns,
    pageCount: Math.ceil(totalCount / pagination.pageSize),
    state: {
      sorting,
      columnFilters,
      columnVisibility,
      rowSelection,
      pagination,
      globalFilter,
    },
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    onSortingChange: setSorting,
    onColumnFiltersChange,
    onColumnVisibilityChange: setColumnVisibility,
    onPaginationChange,
    onGlobalFilterChange,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualSorting: true,
    manualFiltering: true,
  })

  // Ensure page is in range when total count changes
  const pageCount = table.getPageCount()
  useEffect(() => {
    ensurePageInRange(pageCount)
  }, [pageCount, ensurePageInRange])

  // Prepare filter options
  const vendorFilterOptions = [
    {
      label: `${t('All Vendors')}${vendorCounts?.all ? ` (${vendorCounts.all})` : ''}`,
      value: 'all',
    },
    ...vendorOptions.map((option) => ({
      label: `${option.label}${vendorCounts?.[option.value] ? ` (${vendorCounts[option.value]})` : ''}`,
      value: option.value,
    })),
  ]

  const activeSync = syncFilter[0] ?? 'all'
  const syncPillOptions = [
    { value: 'all', label: t('全部同步状态') },
    ...getSyncStatusOptions(t).map((option) => ({
      value: option.value,
      label: option.label,
    })),
  ]

  return (
    <div className='space-y-3 sm:space-y-4'>
      <ModelsStats
        models={models}
        vendors={vendors}
        totalModels={totalCount}
        isLoading={isLoading}
        isError={isError}
        errorDescription={errorDescription}
        managementOpen={managementOpen}
        onManagementToggle={() => setManagementOpen((open) => !open)}
      />

      {managementOpen ? (
        <div className='space-y-3'>
          <div className='flex flex-wrap items-center justify-end gap-2 px-1'>
            <Link
              to='/models/$section'
              params={{ section: 'deployments' }}
              className='border-border/60 bg-background/45 text-muted-foreground hover:bg-background/75 hover:text-foreground inline-flex h-8 items-center rounded-full border px-3 text-xs font-semibold transition-colors'
            >
              {t('Deployments')}
            </Link>
            <ModelsPrimaryButtons />
          </div>

          <DataTablePage
            table={table}
            columns={columns}
            isLoading={isLoading}
            isFetching={isFetching}
            isError={isError}
            errorDescription={errorDescription}
            tableHeaderClassName='bg-background/80 backdrop-blur-md sticky top-0 z-10'
            tableClassName='[&_[data-slot=table]_td]:text-[13px] [&_[data-slot=table]_td_*]:text-[13px] [&_[data-slot=table]_th]:text-[12px] [&_[data-slot=table]_th_*]:text-[12px]'
            emptyTitle={t('No Models Found')}
            emptyDescription={t(
              'No models available. Create your first model to get started.'
            )}
            skeletonKeyPrefix='model-skeleton'
            toolbarProps={{
              searchPlaceholder: t('Filter by model name...'),
              additionalSearch: (
                <FilterPills
                  value={activeSync}
                  options={syncPillOptions}
                  onValueChange={(value) => {
                    onColumnFiltersChange((prev) => {
                      const filtered = prev.filter(
                        (filter) => filter.id !== 'sync_official'
                      )
                      return value === 'all'
                        ? filtered
                        : [...filtered, { id: 'sync_official', value: [value] }]
                    })
                  }}
                  className='min-w-0'
                />
              ),
              filters: [
                {
                  columnId: 'status',
                  title: t('Status'),
                  options: [...getModelStatusOptions(t)],
                  singleSelect: true,
                },
                {
                  columnId: 'vendor_id',
                  title: t('Vendor'),
                  options: vendorFilterOptions,
                  singleSelect: true,
                },
              ],
            }}
            bulkActions={<DataTableBulkActions table={table} />}
          />
        </div>
      ) : null}
    </div>
  )
}
