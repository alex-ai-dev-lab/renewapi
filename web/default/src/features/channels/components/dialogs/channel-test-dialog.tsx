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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import {
  type ColumnDef,
  type RowSelectionState,
  type Table as TanStackTable,
  flexRender,
  getCoreRowModel,
  getPaginationRowModel,
  useReactTable,
} from '@tanstack/react-table'
import {
  AlertTriangle,
  Check,
  Copy,
  Info,
  Loader2,
  Settings,
  ShieldCheck,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { useIsMobile } from '@/hooks/use-mobile'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { DataTablePagination } from '@/components/data-table/pagination'
import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { StatusBadge } from '@/components/status-badge'
import {
  channelsQueryKeys,
  formatResponseTime,
  handleClearAntiPoisonRisk,
  handleTestChannel,
} from '../../lib'
import type { ChannelTestResponse } from '../../types'
import { useChannels } from '../channels-provider'

type ChannelTestDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

type ModelRow = {
  model: string
}

type TestStatus = 'idle' | 'testing' | 'success' | 'error'

type TestResult = {
  status: TestStatus
  responseTime?: number
  firstByteTime?: number
  totalTime?: number
  httpStatus?: number
  endpointType?: string
  request?: string
  response?: string
  error?: string
  errorCode?: string
}

const endpointTypeOptions: Array<{ value: string; label: string }> = [
  { value: 'auto', label: 'Auto detect (default)' },
  { value: 'openai', label: 'OpenAI (/v1/chat/completions)' },
  { value: 'openai-response', label: 'OpenAI Responses (/v1/responses)' },
  {
    value: 'openai-response-compact',
    label: 'OpenAI Response Compaction (/v1/responses/compact)',
  },
  { value: 'anthropic', label: 'Anthropic (/v1/messages)' },
  {
    value: 'gemini',
    label: 'Gemini (/v1beta/models/{model}:generateContent)',
  },
  { value: 'jina-rerank', label: 'Jina Rerank (/v1/rerank)' },
  {
    value: 'image-generation',
    label: 'Image Generation (/v1/images/generations)',
  },
  { value: 'image-edits', label: 'Image Edits (/v1/images/edits)' },
  { value: 'embeddings', label: 'Embeddings (/v1/embeddings)' },
  { value: 'audio-speech', label: 'Audio Speech (/v1/audio/speech)' },
  {
    value: 'audio-transcription',
    label: 'Audio Transcription (/v1/audio/transcriptions)',
  },
  {
    value: 'audio-translation',
    label: 'Audio Translation (/v1/audio/translations)',
  },
  { value: 'moderations', label: 'Moderations (/v1/moderations)' },
  { value: 'openai-video', label: 'OpenAI Video (/v1/videos)' },
]

const STREAM_INCOMPATIBLE_ENDPOINTS = new Set([
  'embeddings',
  'image-generation',
  'image-edits',
  'audio-speech',
  'audio-transcription',
  'audio-translation',
  'jina-rerank',
  'moderations',
  'openai-video',
  'openai-response-compact',
])

const BATCH_TEST_CONCURRENCY = 6

const MODEL_PRICE_ERROR_CODE = 'model_price_error'
const FAILURE_SUMMARY_MAX_LENGTH = 96

type FailureStatusDisplay = {
  summary: string
  details?: string
}

type FailureDetailsState = {
  model: string
  summary: string
  details: string
  result?: TestResult
}

type ChannelRiskMeta = {
  risk_status?: string
  risk_reason?: string
  risk_time?: number
  risk_evidence_path?: string
}

function normalizeInlineError(errorText: string) {
  return errorText.replace(/\s+/g, ' ').trim()
}

function getFirstErrorLine(errorText: string) {
  return errorText
    .split(/\r?\n/)
    .map((line) => line.trim())
    .find(Boolean)
}

function truncateFailureSummary(summary: string) {
  if (summary.length <= FAILURE_SUMMARY_MAX_LENGTH) {
    return summary
  }

  return `${summary.slice(0, FAILURE_SUMMARY_MAX_LENGTH).trimEnd()}...`
}

function getFailureStatusDisplay({
  errorText,
  fallbackSummary,
  isModelPriceError,
  modelPriceSummary,
}: {
  errorText?: string
  fallbackSummary: string
  isModelPriceError: boolean
  modelPriceSummary: string
}): FailureStatusDisplay {
  const rawError = errorText?.trim()

  if (!rawError) {
    return { summary: fallbackSummary }
  }

  if (isModelPriceError) {
    return {
      summary: modelPriceSummary,
      details: rawError === modelPriceSummary ? undefined : rawError,
    }
  }

  const firstLine = getFirstErrorLine(rawError) ?? rawError
  const summary = truncateFailureSummary(normalizeInlineError(firstLine))
  const normalizedRawError = normalizeInlineError(rawError)

  return {
    summary,
    details: summary === normalizedRawError ? undefined : rawError,
  }
}

function getTestTableColumnClass(columnId: string) {
  switch (columnId) {
    case 'select':
      return 'w-10 min-w-10'
    case 'model':
      return 'w-56 min-w-56 max-w-56 whitespace-nowrap'
    case 'status':
      return 'w-72 min-w-72 max-w-72 whitespace-normal'
    case 'actions':
      return 'bg-card sticky right-0 z-20 w-24 min-w-24 border-l shadow-[-8px_0_8px_-8px_rgb(0_0_0_/_0.2)] whitespace-nowrap sm:w-28 sm:min-w-28'
    default:
      return undefined
  }
}

function secondsToMs(value?: number) {
  return typeof value === 'number' ? value * 1000 : undefined
}

function buildTestResultFromResponse(
  success: boolean,
  responseTime?: number,
  error?: string,
  errorCode?: string,
  response?: ChannelTestResponse
): TestResult {
  const firstByteTime = secondsToMs(
    response?.first_byte_time ?? response?.data?.first_byte_time
  )
  const totalTime =
    secondsToMs(response?.total_time ?? response?.data?.total_time) ??
    responseTime

  return {
    status: success ? 'success' : 'error',
    responseTime: totalTime ?? responseTime,
    firstByteTime,
    totalTime,
    httpStatus: response?.http_status ?? response?.data?.http_status,
    endpointType: response?.endpoint_type ?? response?.data?.endpoint_type,
    request: response?.request ?? response?.data?.request,
    response: response?.response ?? response?.data?.response,
    error,
    errorCode,
  }
}

function hasTestDetails(result: TestResult) {
  return Boolean(
    result.request ||
    result.response ||
    result.error ||
    result.httpStatus ||
    result.endpointType ||
    typeof result.firstByteTime === 'number' ||
    typeof result.totalTime === 'number'
  )
}

function parseChannelRiskMeta(otherInfo: string | null | undefined) {
  if (!otherInfo) return null
  try {
    const parsed = JSON.parse(otherInfo) as ChannelRiskMeta
    if (parsed && typeof parsed === 'object') {
      return parsed
    }
  } catch {
    return null
  }
  return null
}

export function ChannelTestDialog({
  open,
  onOpenChange,
}: ChannelTestDialogProps) {
  const { t } = useTranslation()
  const { currentRow } = useChannels()
  const queryClient = useQueryClient()
  const [endpointType, setEndpointType] = useState('auto')
  const [isStreamTest, setIsStreamTest] = useState(true)
  const userStreamPreferenceRef = useRef(true)
  const [searchTerm, setSearchTerm] = useState('')
  const [testResults, setTestResults] = useState<Record<string, TestResult>>({})
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const [testingModels, setTestingModels] = useState<Set<string>>(
    () => new Set()
  )
  const [isBatchTesting, setIsBatchTesting] = useState(false)
  const [failureDetails, setFailureDetails] =
    useState<FailureDetailsState | null>(null)
  const [riskCleared, setRiskCleared] = useState(false)
  const [isClearingRisk, setIsClearingRisk] = useState(false)
  const [pagination, setPagination] = useState({
    pageIndex: 0,
    pageSize: 100,
  })

  const resetState = useCallback(() => {
    setEndpointType('auto')
    setIsStreamTest(true)
    userStreamPreferenceRef.current = true
    setSearchTerm('')
    setTestResults({})
    setRowSelection({})
    setTestingModels(() => new Set())
    setIsBatchTesting(false)
    setFailureDetails(null)
    setRiskCleared(false)
    setPagination({ pageIndex: 0, pageSize: 100 })
  }, [])

  useEffect(() => {
    if (open && currentRow) {
      resetState()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, currentRow?.id, resetState])

  const streamDisabled = STREAM_INCOMPATIBLE_ENDPOINTS.has(endpointType)

  useEffect(() => {
    if (streamDisabled) {
      setIsStreamTest(false)
    } else {
      setIsStreamTest(userStreamPreferenceRef.current)
    }
  }, [streamDisabled])

  const handleStreamToggle = useCallback((checked: boolean) => {
    userStreamPreferenceRef.current = checked
    setIsStreamTest(checked)
  }, [])

  const modelsValue = currentRow?.models ?? ''
  const defaultTestModel = currentRow?.test_model?.trim()

  const models = useMemo(() => {
    if (!modelsValue) return []
    return modelsValue
      .split(',')
      .map((model) => model.trim())
      .filter(Boolean)
  }, [modelsValue])

  const filteredModels = useMemo(() => {
    if (!searchTerm) return models
    const keyword = searchTerm.toLowerCase()
    return models.filter((model) => model.toLowerCase().includes(keyword))
  }, [models, searchTerm])

  useEffect(() => {
    setPagination((prev) => ({ ...prev, pageIndex: 0 }))
  }, [searchTerm, modelsValue])

  const tableData = useMemo<ModelRow[]>(
    () => filteredModels.map((model) => ({ model })),
    [filteredModels]
  )

  const markModelTesting = useCallback((key: string, isTesting: boolean) => {
    setTestingModels((prev) => {
      const next = new Set(prev)
      if (isTesting) {
        next.add(key)
      } else {
        next.delete(key)
      }
      return next
    })
  }, [])

  const updateTestResult = useCallback((key: string, result: TestResult) => {
    setFailureDetails((current) => (current?.model === key ? null : current))
    setTestResults((prev) => ({
      ...prev,
      [key]: result,
    }))
  }, [])

  const testSingleModel = useCallback(
    async (model: string, silent = false): Promise<TestResult | undefined> => {
      if (!currentRow) return

      markModelTesting(model, true)
      updateTestResult(model, { status: 'testing' })
      let finalResult: TestResult | undefined

      try {
        await handleTestChannel(
          currentRow.id,
          {
            testModel: model,
            endpointType: endpointType === 'auto' ? undefined : endpointType,
            stream: isStreamTest || undefined,
            silent,
          },
          (success, responseTime, error, errorCode, response) => {
            finalResult = buildTestResultFromResponse(
              success,
              responseTime,
              error,
              errorCode,
              response
            )
            updateTestResult(model, finalResult)
          }
        )
      } catch (error: unknown) {
        finalResult = {
          status: 'error',
          error: error instanceof Error ? error.message : t('Test failed'),
        }
        updateTestResult(model, finalResult)
      } finally {
        markModelTesting(model, false)
      }
      return finalResult
    },
    [
      currentRow,
      endpointType,
      isStreamTest,
      markModelTesting,
      t,
      updateTestResult,
    ]
  )

  const handleBatchTest = useCallback(
    async (modelsToTest: string[]) => {
      if (!modelsToTest.length) return

      setIsBatchTesting(true)
      try {
        const results: TestResult[] = []
        let cursor = 0

        const runWorker = async () => {
          while (cursor < modelsToTest.length) {
            const index = cursor
            cursor += 1
            const modelName = modelsToTest[index]
            const result = await testSingleModel(modelName, true)
            if (result) {
              results.push(result)
            }
          }
        }

        const workerCount = Math.min(
          BATCH_TEST_CONCURRENCY,
          modelsToTest.length
        )
        await Promise.all(
          Array.from({ length: workerCount }, () => runWorker())
        )

        const successCount = results.filter(
          (result) => result.status === 'success'
        ).length
        const failedCount = modelsToTest.length - successCount
        if (failedCount > 0) {
          toast.error(
            t(
              'Batch test completed: SS_VAR succeeded, FF_VAR failed'
                .replace('SS_VAR', '{' + '{success}' + '}')
                .replace('FF_VAR', '{' + '{failed}' + '}'),
              {
                success: successCount,
                failed: failedCount,
              }
            )
          )
        } else {
          toast.success(
            t(
              'Batch test completed: CC_VAR succeeded'.replace(
                'CC_VAR',
                '{' + '{count}' + '}'
              ),
              {
                count: successCount,
              }
            )
          )
        }
      } finally {
        setIsBatchTesting(false)
        setRowSelection({})
      }
    },
    [t, testSingleModel]
  )

  const handleClose = () => {
    resetState()
    onOpenChange(false)
  }

  const isAnyTesting = testingModels.size > 0 || isBatchTesting
  const riskMeta = parseChannelRiskMeta(currentRow?.other_info)
  const hasAntiPoisonRisk =
    !riskCleared && riskMeta?.risk_status === 'anti_poison'

  const handleClearRisk = async () => {
    if (!currentRow) return
    setIsClearingRisk(true)
    try {
      await handleClearAntiPoisonRisk(currentRow.id, queryClient, () => {
        setRiskCleared(true)
        queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
      })
    } finally {
      setIsClearingRisk(false)
    }
  }

  const columns = useMemo<ColumnDef<ModelRow>[]>(
    () => [
      {
        id: 'select',
        header: ({ table }) => (
          <Checkbox
            checked={table.getIsAllPageRowsSelected()}
            indeterminate={table.getIsSomePageRowsSelected()}
            onCheckedChange={(value) =>
              table.toggleAllPageRowsSelected(!!value)
            }
            aria-label={t('Select all models')}
          />
        ),
        cell: ({ row }) => (
          <Checkbox
            checked={row.getIsSelected()}
            onCheckedChange={(value) => row.toggleSelected(!!value)}
            aria-label={t(
              'Select model MM_VAR'.replace('MM_VAR', '{' + '{model}' + '}'),
              {
                model: row.original.model,
              }
            )}
          />
        ),
        enableSorting: false,
        enableHiding: false,
        size: 40,
      },
      {
        accessorKey: 'model',
        header: t('Model'),
        cell: ({ row }) => {
          const model = row.original.model
          const isDefault = defaultTestModel === model

          return (
            <div className='flex w-max items-center gap-2 whitespace-nowrap'>
              <span className='font-medium whitespace-nowrap' title={model}>
                {model}
              </span>
              {isDefault && (
                <StatusBadge
                  label={t('Default')}
                  variant='info'
                  size='sm'
                  copyable={false}
                />
              )}
            </div>
          )
        },
      },
      {
        id: 'status',
        header: t('Status'),
        cell: ({ row }) => {
          const model = row.original.model
          const result = testResults[model]
          return (
            <TestStatusCell
              result={result}
              model={model}
              onOpenDetails={setFailureDetails}
            />
          )
        },
        enableSorting: false,
        size: 220,
      },
      {
        id: 'actions',
        header: t('Actions'),
        cell: ({ row }) => {
          const model = row.original.model
          const isTestingModel = testingModels.has(model)

          return (
            <Button
              variant='outline'
              size='sm'
              onClick={() => testSingleModel(model)}
              disabled={isTestingModel || isBatchTesting}
            >
              {isTestingModel && (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              )}
              {t('Test')}
            </Button>
          )
        },
        enableSorting: false,
        size: 120,
      },
    ],
    [
      defaultTestModel,
      isBatchTesting,
      t,
      testResults,
      testingModels,
      testSingleModel,
    ]
  )

  const table = useReactTable({
    data: tableData,
    columns,
    state: {
      rowSelection,
      pagination,
    },
    enableRowSelection: true,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    onRowSelectionChange: setRowSelection,
    onPaginationChange: setPagination,
  })

  if (!currentRow) {
    return null
  }

  return (
    <>
      <Dialog open={open} onOpenChange={handleClose}>
        <DialogContent className='max-h-[90vh] overflow-hidden sm:max-w-3xl'>
          <DialogHeader>
            <DialogTitle>{t('Test Channel Connection')}</DialogTitle>
            <DialogDescription>
              {t('Test connectivity for:')} <strong>{currentRow.name}</strong>
            </DialogDescription>
          </DialogHeader>

          <div className='max-h-[78vh] space-y-4 overflow-y-auto py-4 pr-1'>
            {hasAntiPoisonRisk && (
              <div className='border-destructive/30 bg-destructive/5 text-destructive rounded-md border px-3 py-2'>
                <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
                  <div className='min-w-0 space-y-1'>
                    <div className='flex items-center gap-2 text-sm font-medium'>
                      <AlertTriangle className='size-4 shrink-0' />
                      <span>{t('Anti-poison risk detected')}</span>
                    </div>
                    {riskMeta?.risk_reason && (
                      <p className='text-xs break-words'>
                        {t('Reason:')} {riskMeta.risk_reason}
                      </p>
                    )}
                    {riskMeta?.risk_evidence_path && (
                      <p className='text-xs break-all'>
                        {t('Evidence:')} {riskMeta.risk_evidence_path}
                      </p>
                    )}
                  </div>
                  <Button
                    size='sm'
                    variant='outline'
                    onClick={handleClearRisk}
                    disabled={isClearingRisk}
                    className='shrink-0'
                  >
                    {isClearingRisk ? (
                      <Loader2 className='mr-2 size-4 animate-spin' />
                    ) : (
                      <ShieldCheck className='mr-2 size-4' />
                    )}
                    {t('Clear Anti-poison Risk')}
                  </Button>
                </div>
              </div>
            )}

            <div className='bg-muted/20 flex min-w-0 flex-col gap-4 rounded-lg border p-3 md:flex-row md:items-start'>
              <div className='grid min-w-0 gap-2 md:flex-1'>
                <Label htmlFor='endpoint-type'>{t('Endpoint Type')}</Label>
                <Select
                  items={[
                    ...endpointTypeOptions.map((option) => {
                      const itemValue = option.value
                      return { value: itemValue, label: t(option.label) }
                    }),
                  ]}
                  value={endpointType}
                  onValueChange={(v) => v !== null && setEndpointType(v)}
                >
                  <SelectTrigger id='endpoint-type'>
                    <SelectValue placeholder={t('Auto detect (default)')} />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {endpointTypeOptions.map((option) => {
                        const itemValue = option.value
                        return (
                          <SelectItem key={itemValue} value={itemValue}>
                            {t(option.label)}
                          </SelectItem>
                        )
                      })}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Override the endpoint used for testing. Leave empty to auto detect.'
                  )}
                </p>
              </div>
              <div className='grid min-w-0 gap-2 md:w-48 md:shrink-0'>
                <Label htmlFor='stream-toggle'>{t('Stream Mode')}</Label>
                <div className='flex items-center gap-2'>
                  <Switch
                    id='stream-toggle'
                    checked={isStreamTest}
                    onCheckedChange={handleStreamToggle}
                    disabled={streamDisabled}
                  />
                  <span className='text-sm'>
                    {isStreamTest ? t('Enabled') : t('Disabled')}
                  </span>
                </div>
                <p className='text-muted-foreground text-xs'>
                  {t('Enable streaming mode for the test request.')}
                </p>
              </div>
            </div>

            <div className='min-w-0 space-y-3 max-sm:has-[div[role="toolbar"]]:pb-16'>
              <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                <div>
                  <p className='text-sm font-medium'>{t('Channel models')}</p>
                  <p className='text-muted-foreground text-xs'>
                    {t('Select models to run batch tests.')}
                  </p>
                </div>
                <Input
                  placeholder={t('Filter models...')}
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className='sm:w-64'
                />
              </div>

              <div className='space-y-3'>
                <div
                  className='min-w-0 overflow-hidden rounded-md border'
                  role='region'
                  aria-label={t('Channel models')}
                >
                  <div className='max-h-90 overflow-auto **:data-[slot=table-container]:overflow-visible'>
                    <Table className='w-max min-w-full table-auto'>
                      <colgroup>
                        <col className='w-10 min-w-10' />
                        <col className='w-56' />
                        <col className='w-72' />
                        <col className='w-24 sm:w-28' />
                      </colgroup>
                      <TableHeader>
                        {table.getHeaderGroups().map((headerGroup) => (
                          <TableRow key={headerGroup.id}>
                            {headerGroup.headers.map((header) => (
                              <TableHead
                                key={header.id}
                                className={getTestTableColumnClass(
                                  header.column.id
                                )}
                              >
                                {header.isPlaceholder
                                  ? null
                                  : flexRender(
                                      header.column.columnDef.header,
                                      header.getContext()
                                    )}
                              </TableHead>
                            ))}
                          </TableRow>
                        ))}
                      </TableHeader>
                      <TableBody>
                        {table.getRowModel().rows.length ? (
                          table.getRowModel().rows.map((row) => (
                            <TableRow
                              key={row.id}
                              data-state={
                                row.getIsSelected() ? 'selected' : undefined
                              }
                            >
                              {row.getVisibleCells().map((cell) => (
                                <TableCell
                                  key={cell.id}
                                  className={getTestTableColumnClass(
                                    cell.column.id
                                  )}
                                >
                                  {flexRender(
                                    cell.column.columnDef.cell,
                                    cell.getContext()
                                  )}
                                </TableCell>
                              ))}
                            </TableRow>
                          ))
                        ) : (
                          <TableRow>
                            <TableCell
                              colSpan={table.getVisibleLeafColumns().length}
                              className='text-muted-foreground h-16 text-center text-sm'
                            >
                              {models.length
                                ? 'No models matched your search.'
                                : 'This channel has no configured models.'}
                            </TableCell>
                          </TableRow>
                        )}
                      </TableBody>
                    </Table>
                  </div>
                </div>

                <DataTablePagination table={table} />
              </div>

              <TestModelsBulkActions
                table={table}
                disabled={isAnyTesting}
                onTestSelected={handleBatchTest}
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant='outline' onClick={handleClose}>
              {t('Close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <FailureDetailsSheet
        details={failureDetails}
        onOpenChange={(sheetOpen) => {
          if (!sheetOpen) {
            setFailureDetails(null)
          }
        }}
      />
    </>
  )
}

function TestStatusCell({
  result,
  model,
  onOpenDetails,
}: {
  result?: TestResult
  model: string
  onOpenDetails: (details: FailureDetailsState) => void
}) {
  const { t } = useTranslation()

  if (!result || result.status === 'idle') {
    return (
      <StatusBadge label={t('Not tested')} variant='neutral' copyable={false} />
    )
  }

  if (result.status === 'testing') {
    return (
      <div className='text-muted-foreground flex min-w-0 items-center gap-2 text-sm'>
        <Loader2 className='h-4 w-4 shrink-0 animate-spin' />
        <span className='truncate'>{t('Testing...')}</span>
      </div>
    )
  }

  if (result.status === 'success') {
    return (
      <div className='flex min-w-0 flex-col gap-1.5 text-xs'>
        <StatusBadge label={t('Success')} variant='success' copyable={false} />
        <div className='flex min-w-0 flex-wrap items-center gap-1.5'>
          {typeof result.firstByteTime === 'number' && (
            <span className='bg-muted text-muted-foreground rounded px-1.5 py-0.5'>
              {t('First byte')} {formatResponseTime(result.firstByteTime, t)}
            </span>
          )}
          {typeof result.totalTime === 'number' && (
            <span className='bg-muted text-muted-foreground rounded px-1.5 py-0.5'>
              {t('Total')} {formatResponseTime(result.totalTime, t)}
            </span>
          )}
          {typeof result.totalTime !== 'number' &&
            typeof result.responseTime === 'number' && (
              <span className='text-muted-foreground truncate'>
                {formatResponseTime(result.responseTime, t)}
              </span>
            )}
          {hasTestDetails(result) && (
            <Button
              variant='ghost'
              size='sm'
              className='h-7 w-fit px-2 text-xs'
              aria-haspopup='dialog'
              onClick={() =>
                onOpenDetails({
                  model,
                  summary: t('Channel test succeeded'),
                  details: result.response || result.request || '',
                  result,
                })
              }
            >
              <Info className='mr-1 h-3 w-3 shrink-0' />
              {t('Details')}
            </Button>
          )}
        </div>
      </div>
    )
  }

  return <FailureStatusContent result={result} />
}

function FailureStatusContent({ result }: { result: TestResult }) {
  const { t } = useTranslation()
  const errorText = result.error?.trim()
  const isModelPriceError = result.errorCode === MODEL_PRICE_ERROR_CODE
  const modelPriceSummary = t(
    'Model price is not configured. Please complete model pricing in settings.'
  )
  const { summary, details } = getFailureStatusDisplay({
    errorText,
    fallbackSummary: t('Test failed'),
    isModelPriceError,
    modelPriceSummary,
  })

  return (
    <div className='flex min-w-0 flex-col gap-1.5 text-xs whitespace-normal'>
      <StatusBadge label={t('Failed')} variant='danger' copyable={false} />
      <p className='text-muted-foreground line-clamp-2 min-w-0 leading-snug wrap-break-word'>
        {summary}
      </p>
      <div className='flex min-w-0 flex-wrap items-center gap-1.5'>
        {isModelPriceError && (
          <Button
            variant='outline'
            size='sm'
            className='h-7 w-fit px-2 text-xs'
            onClick={() =>
              window.open('/system-settings/models/model-pricing', '_blank')
            }
          >
            <Settings className='mr-1 h-3 w-3 shrink-0' />
            {t('Go to Settings')}
          </Button>
        )}
        {(details || result.error) && (
          <details className='border-border/70 bg-muted/30 w-full rounded-md border px-2 py-1.5'>
            <summary className='cursor-pointer font-medium select-none'>
              {t('Details')}
            </summary>
            <pre className='text-muted-foreground mt-2 max-h-40 overflow-auto font-mono text-[11px] leading-relaxed break-words whitespace-pre-wrap'>
              {details ?? result.error}
            </pre>
          </details>
        )}
      </div>
    </div>
  )
}

function FailureDetailsSheet({
  details,
  onOpenChange,
}: {
  details: FailureDetailsState | null
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const isMobile = useIsMobile()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })

  return (
    <Sheet open={Boolean(details)} onOpenChange={onOpenChange}>
      <SheetContent
        side={isMobile ? 'bottom' : 'right'}
        className={
          isMobile
            ? sideDrawerContentClassName('h-auto max-h-[85dvh] rounded-t-xl')
            : sideDrawerContentClassName('sm:max-w-lg')
        }
      >
        {details && (
          <>
            <SheetHeader className={sideDrawerHeaderClassName('sm:px-5')}>
              <SheetTitle className='pr-10'>{t('Details')}</SheetTitle>
              <SheetDescription className='pr-10 wrap-break-word'>
                {details.model}
              </SheetDescription>
            </SheetHeader>
            <div className={sideDrawerFormClassName('gap-4 sm:px-5')}>
              <section className='space-y-1'>
                <div className='text-muted-foreground text-xs font-medium'>
                  {t('Model')}
                </div>
                <p className='text-sm font-medium break-all'>{details.model}</p>
              </section>
              <section className='space-y-1'>
                <div className='text-muted-foreground text-xs font-medium'>
                  {details.result?.status === 'success'
                    ? t('Succeeded')
                    : t('Failed')}
                </div>
                <p className='text-muted-foreground text-sm leading-relaxed wrap-break-word'>
                  {details.summary}
                </p>
              </section>
              {details.result && (
                <section className='grid gap-2 text-xs sm:grid-cols-2'>
                  {details.result.endpointType && (
                    <div className='rounded-md border p-2'>
                      <div className='text-muted-foreground font-medium'>
                        {t('Endpoint')}
                      </div>
                      <div className='mt-1 break-all'>
                        {details.result.endpointType}
                      </div>
                    </div>
                  )}
                  {typeof details.result.httpStatus === 'number' && (
                    <div className='rounded-md border p-2'>
                      <div className='text-muted-foreground font-medium'>
                        {t('HTTP status')}
                      </div>
                      <div className='mt-1'>{details.result.httpStatus}</div>
                    </div>
                  )}
                  {typeof details.result.firstByteTime === 'number' && (
                    <div className='rounded-md border p-2'>
                      <div className='text-muted-foreground font-medium'>
                        {t('First byte')}
                      </div>
                      <div className='mt-1'>
                        {formatResponseTime(details.result.firstByteTime, t)}
                      </div>
                    </div>
                  )}
                  {typeof details.result.totalTime === 'number' && (
                    <div className='rounded-md border p-2'>
                      <div className='text-muted-foreground font-medium'>
                        {t('Total')}
                      </div>
                      <div className='mt-1'>
                        {formatResponseTime(details.result.totalTime, t)}
                      </div>
                    </div>
                  )}
                </section>
              )}
              {details.result?.request && (
                <section className='space-y-2'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('Request')}
                  </div>
                  <pre className='bg-muted/30 text-muted-foreground m-0 max-w-full overflow-auto rounded-md border p-3 text-xs leading-relaxed whitespace-pre-wrap'>
                    {details.result.request}
                  </pre>
                </section>
              )}
              {details.result?.response && (
                <section className='space-y-2'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('Response')}
                  </div>
                  <pre className='bg-muted/30 text-muted-foreground m-0 max-w-full overflow-auto rounded-md border p-3 text-xs leading-relaxed whitespace-pre-wrap'>
                    {details.result.response}
                  </pre>
                </section>
              )}
              <section className='space-y-2'>
                <div className='text-muted-foreground text-xs font-medium'>
                  {t('Details')}
                </div>
                <pre className='bg-muted/30 text-muted-foreground m-0 max-w-full rounded-md border p-3 text-xs leading-relaxed wrap-break-word whitespace-pre-wrap'>
                  {details.details}
                </pre>
              </section>
            </div>
            <SheetFooter className={sideDrawerFooterClassName('sm:px-5')}>
              <Button
                variant='outline'
                className='w-full sm:w-auto'
                onClick={() => copyToClipboard(details.details)}
              >
                {copiedText === details.details ? (
                  <Check className='text-success mr-2 h-4 w-4' />
                ) : (
                  <Copy className='mr-2 h-4 w-4' />
                )}
                {t('Copy')}
              </Button>
            </SheetFooter>
          </>
        )}
      </SheetContent>
    </Sheet>
  )
}

function TestModelsBulkActions({
  table,
  disabled,
  onTestSelected,
}: {
  table: TanStackTable<ModelRow>
  disabled?: boolean
  onTestSelected: (models: string[]) => void
}) {
  const { t } = useTranslation()
  const selectedRows = table.getFilteredSelectedRowModel().rows
  const selectedModels = selectedRows.map((row) => row.original.model)

  const buttonLabel =
    selectedModels.length > 0
      ? t('Test CC_VAR selected'.replace('CC_VAR', '{' + '{count}' + '}'), {
          count: selectedModels.length,
        })
      : t('Test selected models')

  return (
    <BulkActionsToolbar table={table} entityName='model'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              size='sm'
              onClick={() => onTestSelected(selectedModels)}
              disabled={disabled || selectedModels.length === 0}
            />
          }
        >
          {disabled ? (
            <>
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              {t('Testing...')}
            </>
          ) : (
            buttonLabel
          )}
        </TooltipTrigger>
        <TooltipContent>
          <p>{t('Run tests for the selected models')}</p>
        </TooltipContent>
      </Tooltip>
    </BulkActionsToolbar>
  )
}
