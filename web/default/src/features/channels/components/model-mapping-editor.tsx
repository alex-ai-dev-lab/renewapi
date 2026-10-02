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
import { useEffect, useId, useRef, useState } from 'react'
import { ArrowDown, ArrowUp, Code, Plus, Table, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import {
  EMPTY_RULE_MAPPING,
  mappingCandidates,
  mappingSources,
  parseModelMappingConfig,
  upgradeLegacyMapping,
  validateModelMappingJson,
  type ModelMappingRule,
} from '../lib/model-mapping-validation'

export type MappingEditorState = { dirty: boolean; errors: string[] }
type ModelMappingEditorProps = {
  value: string
  onChange: (value: string) => void
  onStateChange?: (state: MappingEditorState) => void
  /** Change on an explicit reset or record switch, even when value is unchanged. */
  resetKey?: string | number
  /** Batch mode: deleting drafts returns to no update; only Clear emits rules:[]. */
  emptyMeansUnchanged?: boolean
  disabled?: boolean
  sourceModelOptions?: string[]
  targetModelOptions?: string[]
}
type MappingRow = ModelMappingRule & { key: string; array: boolean }

export function ModelMappingEditor(props: ModelMappingEditorProps) {
  const { t } = useTranslation()
  const sourceListId = useId()
  const targetListId = useId()
  const [mode, setMode] = useState<'visual' | 'json'>('visual')
  const [version, setVersion] = useState<1 | 2>(2)
  const [rows, setRows] = useState<MappingRow[]>([])
  const [jsonValue, setJsonValue] = useState(props.value)
  const [errors, setErrors] = useState<string[]>([])
  const [upgrade, setUpgrade] = useState<string | null>(null)
  const nextId = useRef(0)
  const emitted = useRef<string | null>(null)
  const lastReset = useRef(props.resetKey)
  const focusKey = useRef<string | null>(null)
  const createKey = () => `mapping-${++nextId.current}`
  const report = (nextErrors: string[], dirty: boolean) => {
    setErrors(nextErrors)
    props.onStateChange?.({ dirty, errors: nextErrors })
  }
  const load = (raw: string): boolean => {
    try {
      const config = parseModelMappingConfig(raw || EMPTY_RULE_MAPPING)
      setVersion(config.version)
      if (config.version === 2)
        setRows(
          config.rules.map((rule) => ({
            ...rule,
            key: createKey(),
            array: false,
          }))
        )
      else
        setRows(
          Object.entries(config.legacy).map(([from, targets]) => {
            const original = JSON.parse(raw)[from]
            const array = Array.isArray(original) || targets.length > 1
            const key = createKey()
            return {
              key,
              id: key,
              from,
              to: array ? JSON.stringify(targets) : targets[0],
              array,
              priority: 0,
              enabled: true,
            }
          })
        )
      return true
    } catch {
      return false
    }
  }
  // An external reset deliberately replaces local drafts; own echoes are excluded.
  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    const reset = lastReset.current !== props.resetKey
    lastReset.current = props.resetKey
    if (!reset && emitted.current === props.value) {
      emitted.current = null
      return
    }
    emitted.current = null
    setJsonValue(props.value)
    setUpgrade(null)
    const loaded = load(props.value)
    if (!loaded) {
      setRows([])
      setMode('json')
    }
    // Existing invalid legacy records may be retained unchanged by the parent.
    report([], false)
    // This effect only responds to external value/reset changes, never row edits.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.value, props.resetKey])
  /* eslint-enable react-hooks/set-state-in-effect */

  const emit = (raw: string) => {
    emitted.current = raw
    setJsonValue(raw)
    props.onChange(raw)
  }
  const rowErrors = (next: MappingRow[]) => {
    const result: string[] = []
    if (next.some((row) => !row.from.trim() || !row.to.trim()))
      result.push('Complete or remove unfinished mapping rows')
    if (
      version === 1 &&
      new Set(next.map((r) => r.from.trim())).size !== next.length
    )
      result.push('Duplicate source model mappings are not allowed')
    return result
  }
  const serialize = (next: MappingRow[]) => {
    if (version === 2)
      return JSON.stringify(
        {
          version: 2,
          rules: next.map(({ id, from, to, priority, enabled }) => ({
            id,
            from,
            to,
            priority,
            enabled,
          })),
        },
        null,
        2
      )
    const legacy: Record<string, string | string[]> = Object.create(null)
    for (const row of next) {
      if (!row.from.trim()) continue
      legacy[row.from.trim()] = row.array ? JSON.parse(row.to) : row.to
    }
    return JSON.stringify(legacy, null, 2)
  }
  const sync = (next: MappingRow[]) => {
    setRows(next)
    setUpgrade(null)
    if (props.emptyMeansUnchanged && next.length === 0) {
      emit('')
      report([], false)
      return
    }
    const nextErrors = rowErrors(next)
    try {
      const raw = serialize(next)
      const validation = validateModelMappingJson(raw)
      if (!validation.valid) nextErrors.push(validation.error!)
      // Never publish a lossy legacy duplicate map or invalid array draft.
      if (
        !nextErrors.length ||
        version === 2 ||
        next.every((r) => !r.from.trim())
      )
        emit(raw)
    } catch {
      nextErrors.push('Legacy target arrays must be valid JSON string arrays')
    }
    report([...new Set(nextErrors)], true)
  }
  const change = (key: string, patch: Partial<MappingRow>) =>
    sync(rows.map((row) => (row.key === key ? { ...row, ...patch } : row)))
  const move = (index: number, direction: number) => {
    const next = [...rows]
    ;[next[index], next[index + direction]] = [
      next[index + direction],
      next[index],
    ]
    sync(next)
  }
  const changeJson = (raw: string) => {
    setUpgrade(null)
    emit(raw)
    const validation = validateModelMappingJson(raw)
    let nextErrors = validation.valid ? [] : [validation.error!]
    if (
      version === 2 &&
      raw.trim() &&
      validation.valid &&
      parseModelMappingConfig(raw).version !== 2
    )
      nextErrors = ['Rules cannot be downgraded to legacy mappings']
    if (version === 2 && !raw.trim() && !props.emptyMeansUnchanged)
      nextErrors = ['Use Clear mapping to explicitly remove all rules']
    if (!nextErrors.length && raw.trim())
      setVersion(parseModelMappingConfig(raw).version)
    report(nextErrors, true)
  }
  let preview: { source: string; targets: string[] }[] = []
  try {
    const config = parseModelMappingConfig(jsonValue)
    const budget = { remaining: 65536 }
    preview = mappingSources(config).map((source) => ({
      source,
      targets: mappingCandidates(config, source, budget),
    }))
  } catch {
    /* invalid drafts have field errors, not a misleading preview */
  }
  const duplicateTargets =
    version === 2 &&
    rows
      .filter((r) => r.enabled)
      .some((row, index, all) =>
        all
          .slice(0, index)
          .some(
            (r) =>
              r.from.trim() === row.from.trim() && r.to.trim() === row.to.trim()
          )
      )

  return (
    <div data-slot='model-mapping-editor' className='space-y-2'>
      <Tabs
        value={mode}
        onValueChange={(next) => {
          if (next !== 'visual' && next !== 'json') return
          if (errors.length) return
          if (next === 'visual' && !load(jsonValue)) return
          setMode(next)
        }}
        className='space-y-2'
      >
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <TabsList>
            <TabsTrigger value='visual'>
              <Table className='h-4 w-4' aria-hidden='true' />
              {t('Visual')}
            </TabsTrigger>
            <TabsTrigger value='json'>
              <Code className='h-4 w-4' aria-hidden='true' />
              {t('JSON')}
            </TabsTrigger>
          </TabsList>
          <Button
            type='button'
            variant='link'
            size='sm'
            disabled={props.disabled}
            onClick={() => {
              const raw =
                version === 2
                  ? JSON.stringify(
                      {
                        version: 2,
                        rules: [
                          {
                            id: createKey(),
                            from: 'original-model',
                            to: 'replacement-model',
                            priority: 0,
                            enabled: true,
                          },
                        ],
                      },
                      null,
                      2
                    )
                  : '{"original-model":"replacement-model"}'
              load(raw)
              emit(raw)
              report([], true)
            }}
          >
            {t('Fill Template')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={props.disabled}
            onClick={() => {
              load(EMPTY_RULE_MAPPING)
              emit(EMPTY_RULE_MAPPING)
              report([], true)
            }}
          >
            {t('Clear mapping')}
          </Button>
        </div>
        <p className='text-muted-foreground text-xs'>
          {version === 2
            ? t(
                'Direct rules: higher priority first; ties follow row order. Unmatched models pass through.'
              )
            : t(
                'Legacy mappings follow chains. Upgrade explicitly to direct rules.'
              )}
        </p>
        {errors.length > 0 && (
          <Alert variant='destructive'>
            <AlertDescription>
              {errors.map((error) => (
                <div key={error}>{t(error)}</div>
              ))}
            </AlertDescription>
          </Alert>
        )}
        {duplicateTargets && (
          <Alert>
            <AlertDescription>
              {t(
                'Repeated targets are tried only once, not as duplicate calls.'
              )}
            </AlertDescription>
          </Alert>
        )}
        <TabsContent value='visual' className='space-y-2'>
          {rows.map((row, index) => (
            <div
              key={row.key}
              data-mapping-rule-id={row.id}
              className='space-y-2 rounded-md border p-2'
            >
              <div className='grid grid-cols-1 gap-2 sm:grid-cols-[1fr_1fr_auto]'>
                <div>
                  <Input
                    aria-label={t('Original Model')}
                    aria-invalid={!row.from.trim()}
                    value={row.from}
                    disabled={props.disabled}
                    list={sourceListId}
                    placeholder={t('Original Model')}
                    ref={(node) => {
                      if (node && focusKey.current === row.key) {
                        node.focus()
                        focusKey.current = null
                      }
                    }}
                    onChange={(e) => change(row.key, { from: e.target.value })}
                  />
                  {!row.from.trim() && (
                    <p className='text-destructive text-xs'>
                      {t('Source is required')}
                    </p>
                  )}
                </div>
                <div>
                  <Input
                    aria-label={t('Replacement Model')}
                    aria-invalid={!row.to.trim()}
                    value={row.to}
                    disabled={props.disabled}
                    list={row.array ? undefined : targetListId}
                    placeholder={t('Replacement Model')}
                    onChange={(e) => change(row.key, { to: e.target.value })}
                  />
                  {!row.to.trim() && (
                    <p className='text-destructive text-xs'>
                      {t('Target is required')}
                    </p>
                  )}
                </div>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  disabled={props.disabled}
                  aria-label={t('Delete mapping')}
                  onClick={() => sync(rows.filter((r) => r.key !== row.key))}
                >
                  <Trash2 className='h-4 w-4' aria-hidden='true' />
                </Button>
              </div>
              {version === 2 ? (
                <div className='flex flex-wrap items-center gap-2'>
                  <Input
                    className='w-40'
                    aria-label={t('Rule ID')}
                    value={row.id}
                    disabled={props.disabled}
                    onChange={(e) => change(row.key, { id: e.target.value })}
                  />
                  <label className='flex items-center gap-2 text-xs'>
                    {t('Priority')}
                    <Input
                      className='w-28'
                      aria-label={t('Priority')}
                      type='number'
                      min={-2147483648}
                      max={2147483647}
                      step={1}
                      value={Number.isNaN(row.priority) ? '' : row.priority}
                      disabled={props.disabled}
                      onChange={(e) =>
                        change(row.key, {
                          priority:
                            e.target.value === ''
                              ? NaN
                              : Number(e.target.value),
                        })
                      }
                    />
                  </label>
                  <label className='flex items-center gap-2 text-xs'>
                    <input
                      type='checkbox'
                      checked={row.enabled}
                      disabled={props.disabled}
                      onChange={(e) =>
                        change(row.key, { enabled: e.target.checked })
                      }
                    />
                    {t('Enabled')}
                  </label>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    aria-label={t('Move rule up')}
                    disabled={props.disabled || index === 0}
                    onClick={() => move(index, -1)}
                  >
                    <ArrowUp className='h-4 w-4' aria-hidden='true' />
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    aria-label={t('Move rule down')}
                    disabled={props.disabled || index === rows.length - 1}
                    onClick={() => move(index, 1)}
                  >
                    <ArrowDown className='h-4 w-4' aria-hidden='true' />
                  </Button>
                </div>
              ) : (
                <label className='flex items-center gap-2 text-xs'>
                  <input
                    type='checkbox'
                    checked={row.array}
                    disabled={props.disabled}
                    onChange={(e) =>
                      change(row.key, {
                        array: e.target.checked,
                        to: e.target.checked
                          ? JSON.stringify([row.to])
                          : (() => {
                              try {
                                return JSON.parse(row.to)[0] || ''
                              } catch {
                                return ''
                              }
                            })(),
                      })
                    }
                  />
                  {t('Ordered targets (JSON string array)')}
                </label>
              )}
            </div>
          ))}
          {!rows.length && (
            <div className='text-muted-foreground flex h-24 items-center justify-center rounded-md border border-dashed text-sm'>
              {t(
                'No model mappings configured. Click "Add Mapping" to get started.'
              )}
            </div>
          )}
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='w-full'
            disabled={props.disabled}
            onClick={() => {
              const key = createKey()
              let id = key
              while (rows.some((row) => row.id === id)) id = createKey()
              focusKey.current = key
              sync([
                ...rows,
                {
                  key,
                  id,
                  from: '',
                  to: '',
                  priority: 0,
                  enabled: true,
                  array: false,
                },
              ])
            }}
          >
            <Plus className='mr-2 h-4 w-4' aria-hidden='true' />
            {t('Add Mapping')}
          </Button>
        </TabsContent>
        <TabsContent value='json'>
          <Textarea
            aria-label={t('Model mapping JSON')}
            value={jsonValue}
            onChange={(e) => changeJson(e.target.value)}
            disabled={props.disabled}
            rows={8}
            className='font-mono text-sm'
            aria-invalid={errors.length > 0}
          />
        </TabsContent>
      </Tabs>
      {version === 1 && (
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={props.disabled || errors.length > 0}
          onClick={() => {
            try {
              const validation = validateModelMappingJson(jsonValue)
              if (!validation.valid) throw new Error(validation.error)
              setUpgrade(
                JSON.stringify(
                  upgradeLegacyMapping(parseModelMappingConfig(jsonValue)),
                  null,
                  2
                )
              )
            } catch (error) {
              report(
                [
                  error instanceof Error
                    ? error.message
                    : 'Invalid model mapping',
                ],
                true
              )
            }
          }}
        >
          {t('Preview upgrade to direct rules')}
        </Button>
      )}
      {upgrade && (
        <div className='space-y-2 rounded-md border p-3'>
          <p className='text-sm'>
            {t(
              'Upgrade preserves expanded legacy candidates and their order. Review before confirming.'
            )}
          </p>
          <pre className='max-h-64 overflow-auto text-xs whitespace-pre-wrap'>
            {upgrade}
          </pre>
          <Button
            type='button'
            disabled={props.disabled}
            onClick={() => {
              load(upgrade)
              emit(upgrade)
              setUpgrade(null)
              report([], true)
            }}
          >
            {t('Confirm upgrade')}
          </Button>
          <Button
            type='button'
            variant='ghost'
            onClick={() => setUpgrade(null)}
          >
            {t('Cancel')}
          </Button>
        </div>
      )}
      {!errors.length && preview.length > 0 && (
        <div className='space-y-1 text-xs'>
          <p className='font-medium'>{t('Candidate order preview')}</p>
          {preview.map(({ source, targets }) => (
            <div key={source} className='break-all'>
              {source} → {targets.join(' → ')}
            </div>
          ))}
        </div>
      )}
      {!!props.sourceModelOptions?.length && (
        <datalist id={sourceListId}>
          {props.sourceModelOptions.map((model) => (
            <option key={model} value={model} />
          ))}
        </datalist>
      )}
      {!!props.targetModelOptions?.length && (
        <datalist id={targetListId}>
          {props.targetModelOptions.map((model) => (
            <option key={model} value={model} />
          ))}
        </datalist>
      )}
    </div>
  )
}
