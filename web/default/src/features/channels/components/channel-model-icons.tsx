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
import { type MouseEvent, useMemo } from 'react'
import { Diamond } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

/**
 * Provider-icon renderer for the channel "Models" column.
 *
 * Rationale: rendering one text badge per model made the column arbitrarily
 * wide and pushed the row action buttons off-screen. Instead we collapse the
 * model list into a small, fixed-width set of provider brand icons (reusing the
 * existing `getLobeIcon` brand set, no new assets) so the column width stays
 * predictable regardless of how many models a channel serves.
 *
 * Behaviour:
 * - OpenAI and Claude always occupy the first two slots. They light up when the
 *   channel serves those providers and render clearly dimmed (dashed border +
 *   grayscale + diagonal slash) when it does not, so the two anchors stay
 *   visually stable across rows.
 * - Every other recognised provider is appended only when present, ordered by
 *   how many models it contributes (desc).
 * - Models that match no known provider collapse into a single generic
 *   "Other" (◇) icon whose popover lists them.
 * - At most 5 icons are ever rendered; any extras collapse into a trailing
 *   "+N" popover.
 */

const MAX_ICONS = 5
const OTHER_KEY = 'other'

/** Fixed leading providers, always rendered (lit when present, dimmed if not). */
const PINNED_KEYS = ['openai', 'anthropic'] as const

interface ProviderDef {
  key: string
  label: string
  /** Lobe brand icon base name; consumed as `${icon}.Color`. */
  icon: string
  test: (model: string) => boolean
}

/**
 * Provider matchers, evaluated in order; the first match wins. More specific
 * providers are listed before broader ones.
 */
const PROVIDER_DEFS: ProviderDef[] = [
  {
    key: 'anthropic',
    label: 'Claude',
    icon: 'Claude',
    test: (m) => m.includes('claude'),
  },
  {
    key: 'openai',
    label: 'OpenAI',
    icon: 'OpenAI',
    test: (m) =>
      /^(gpt|o1|o3|o4|chatgpt|codex|davinci|babbage)/.test(m) ||
      m.includes('gpt') ||
      m.includes('dall-e') ||
      m.includes('dalle') ||
      m.includes('whisper') ||
      m.includes('text-embedding') ||
      m.includes('text-moderation') ||
      m.includes('omni-moderation') ||
      m.includes('sora'),
  },
  {
    key: 'gemini',
    label: 'Gemini',
    icon: 'Gemini',
    test: (m) =>
      m.includes('gemini') ||
      m.includes('gemma') ||
      m.includes('palm') ||
      m.includes('bison') ||
      m.includes('imagen'),
  },
  {
    key: 'deepseek',
    label: 'DeepSeek',
    icon: 'DeepSeek',
    test: (m) => m.includes('deepseek'),
  },
  {
    key: 'xai',
    label: 'Grok',
    icon: 'XAI',
    test: (m) => m.includes('grok'),
  },
  {
    key: 'qwen',
    label: 'Qwen',
    icon: 'Qwen',
    test: (m) => m.includes('qwen') || m.includes('qwq') || m.includes('qvq'),
  },
  {
    key: 'moonshot',
    label: 'Moonshot',
    icon: 'Moonshot',
    test: (m) => m.includes('moonshot') || m.includes('kimi'),
  },
  {
    key: 'zhipu',
    label: 'Zhipu',
    icon: 'Zhipu',
    test: (m) =>
      m.includes('glm') || m.includes('cogview') || m.includes('cogvideo'),
  },
  {
    key: 'doubao',
    label: 'Doubao',
    icon: 'Doubao',
    test: (m) => m.includes('doubao'),
  },
  {
    key: 'hunyuan',
    label: 'Hunyuan',
    icon: 'Hunyuan',
    test: (m) => m.includes('hunyuan'),
  },
  {
    key: 'baidu',
    label: 'ERNIE',
    icon: 'Baidu',
    test: (m) => m.includes('ernie') || m.includes('wenxin'),
  },
  {
    key: 'minimax',
    label: 'MiniMax',
    icon: 'Minimax',
    test: (m) => m.includes('minimax') || m.includes('abab'),
  },
  {
    key: 'mistral',
    label: 'Mistral',
    icon: 'Mistral',
    test: (m) =>
      m.includes('mistral') ||
      m.includes('mixtral') ||
      m.includes('codestral') ||
      m.includes('ministral') ||
      m.includes('pixtral'),
  },
  {
    key: 'cohere',
    label: 'Cohere',
    icon: 'Cohere',
    test: (m) =>
      m.startsWith('command') || m.includes('cohere') || m.includes('rerank'),
  },
  {
    key: 'perplexity',
    label: 'Perplexity',
    icon: 'Perplexity',
    test: (m) =>
      m.includes('sonar') || m.includes('pplx') || m.includes('perplexity'),
  },
  {
    key: 'yi',
    label: 'Yi',
    icon: 'Yi',
    test: (m) => m.includes('yi-'),
  },
  {
    key: 'spark',
    label: 'Spark',
    icon: 'Spark',
    test: (m) => m.includes('spark'),
  },
  {
    key: 'suno',
    label: 'Suno',
    icon: 'Suno',
    test: (m) => m.includes('suno') || m.includes('chirp'),
  },
  {
    key: 'kling',
    label: 'Kling',
    icon: 'Kling',
    test: (m) => m.includes('kling'),
  },
  {
    key: 'midjourney',
    label: 'Midjourney',
    icon: 'Midjourney',
    test: (m) => m.includes('midjourney') || m.startsWith('mj'),
  },
  {
    key: 'meta',
    label: 'Llama',
    icon: 'Meta',
    test: (m) => m.includes('llama'),
  },
  {
    key: 'jina',
    label: 'Jina',
    icon: 'Jina',
    test: (m) => m.includes('jina'),
  },
]

const DEF_BY_KEY = new Map(PROVIDER_DEFS.map((d) => [d.key, d]))

/** Localised "not provided by this channel" hint for dimmed anchor icons. */
const NOT_PROVIDED: Record<string, string> = {
  en: 'Not provided by this channel',
  zh: '本渠道未提供',
  ja: 'このチャンネルでは提供されていません',
  fr: 'Non fourni par ce canal',
  ru: 'Не предоставляется этим каналом',
  vi: 'Kênh này không cung cấp',
}

interface ProviderGroup {
  key: string
  label: string
  icon: string
  models: string[]
}

function classifyModel(model: string): string {
  const m = model.toLowerCase().trim()
  for (const def of PROVIDER_DEFS) {
    if (def.test(m)) return def.key
  }
  return OTHER_KEY
}

/**
 * Collapse a model list into ordered provider groups: pinned anchors first,
 * then present providers by model count (desc), "Other" last.
 */
function buildProviderGroups(models: string[]): ProviderGroup[] {
  const byKey = new Map<string, string[]>()
  for (const model of models) {
    const key = classifyModel(model)
    const bucket = byKey.get(key)
    if (bucket) bucket.push(model)
    else byKey.set(key, [model])
  }

  const groups: ProviderGroup[] = []

  for (const key of PINNED_KEYS) {
    const def = DEF_BY_KEY.get(key)!
    groups.push({
      key,
      label: def.label,
      icon: def.icon,
      models: byKey.get(key) ?? [],
    })
  }

  const rest = [...byKey.keys()].filter(
    (k) =>
      k !== OTHER_KEY &&
      !(PINNED_KEYS as readonly string[]).includes(k)
  )
  rest.sort((a, b) => {
    const diff = byKey.get(b)!.length - byKey.get(a)!.length
    return diff !== 0 ? diff : a.localeCompare(b)
  })
  for (const key of rest) {
    const def = DEF_BY_KEY.get(key)!
    groups.push({
      key,
      label: def.label,
      icon: def.icon,
      models: byKey.get(key)!,
    })
  }

  const others = byKey.get(OTHER_KEY)
  if (others && others.length > 0) {
    groups.push({ key: OTHER_KEY, label: '', icon: '', models: others })
  }

  return groups
}

function renderBrandIcon(icon: string) {
  if (!icon) return <Diamond className='size-4' />
  return getLobeIcon(`${icon}.Color`, 16)
}

const slotBase =
  'relative inline-flex size-6 shrink-0 items-center justify-center rounded-md border transition-colors focus-visible:ring-ring focus-visible:ring-2 focus-visible:outline-none'
const slotLit = 'border-border/60 bg-background hover:bg-accent'
const slotDim =
  'border-dashed border-muted-foreground/40 bg-muted/40 opacity-45 grayscale'

const stop = (e: MouseEvent) => e.stopPropagation()

function CountBadge({ count }: { count: number }) {
  if (count < 2) return null
  return (
    <span className='bg-primary text-primary-foreground absolute -top-1 -right-1 z-10 flex h-3.5 min-w-3.5 items-center justify-center rounded-full px-0.5 text-[9px] leading-none font-semibold'>
      {count > 99 ? '99+' : count}
    </span>
  )
}

function ModelChips({ models }: { models: string[] }) {
  return (
    <div className='flex flex-wrap gap-1'>
      {models.map((model) => (
        <span
          key={model}
          className='bg-muted rounded px-1.5 py-0.5 font-mono text-[11px] leading-tight'
        >
          {model}
        </span>
      ))}
    </div>
  )
}

function AnchorSlot({
  group,
  notProvided,
}: {
  group: ProviderGroup
  notProvided: string
}) {
  const lit = group.models.length > 0
  const ariaLabel = lit
    ? `${group.label}: ${group.models.join(', ')}`
    : `${group.label}: ${notProvided}`

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type='button'
            aria-label={ariaLabel}
            onClick={stop}
            className={cn(slotBase, lit ? slotLit : slotDim)}
          >
            {renderBrandIcon(group.icon)}
            {!lit && (
              <span
                aria-hidden='true'
                className='bg-muted-foreground/70 pointer-events-none absolute top-1/2 left-1/2 h-px w-[150%] -translate-x-1/2 -translate-y-1/2 rotate-45'
              />
            )}
            <CountBadge count={group.models.length} />
          </button>
        }
      />
      <TooltipContent side='top' className='max-w-[280px]'>
        <div className='text-xs font-medium'>{group.label}</div>
        {lit ? (
          <div className='text-muted-foreground mt-1 font-mono text-[11px] break-words'>
            {group.models.join(', ')}
          </div>
        ) : (
          <div className='text-muted-foreground mt-1 text-[11px]'>
            {notProvided}
          </div>
        )}
      </TooltipContent>
    </Tooltip>
  )
}

function ProviderSlot({ group }: { group: ProviderGroup }) {
  const ariaLabel = `${group.label}: ${group.models.join(', ')}`
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type='button'
            aria-label={ariaLabel}
            onClick={stop}
            className={cn(slotBase, slotLit)}
          >
            {renderBrandIcon(group.icon)}
            <CountBadge count={group.models.length} />
          </button>
        }
      />
      <TooltipContent side='top' className='max-w-[280px]'>
        <div className='text-xs font-medium'>{group.label}</div>
        <div className='text-muted-foreground mt-1 font-mono text-[11px] break-words'>
          {group.models.join(', ')}
        </div>
      </TooltipContent>
    </Tooltip>
  )
}

function OtherSlot({
  group,
  label,
}: {
  group: ProviderGroup
  label: string
}) {
  const ariaLabel = `${label}: ${group.models.join(', ')}`
  return (
    <Popover>
      <PopoverTrigger
        render={
          <button
            type='button'
            aria-label={ariaLabel}
            onClick={stop}
            className={cn(slotBase, slotLit, 'text-muted-foreground')}
          >
            <Diamond className='size-4' />
            <CountBadge count={group.models.length} />
          </button>
        }
      />
      <PopoverContent
        side='top'
        align='center'
        className='max-h-56 w-56 overflow-y-auto'
        onClick={stop}
      >
        <div className='text-muted-foreground mb-1 text-xs font-medium'>
          {label}
        </div>
        <ModelChips models={group.models} />
      </PopoverContent>
    </Popover>
  )
}

function OverflowSlot({
  groups,
  otherLabel,
}: {
  groups: ProviderGroup[]
  otherLabel: string
}) {
  const names = groups.map((g) => (g.key === OTHER_KEY ? otherLabel : g.label))
  return (
    <Popover>
      <PopoverTrigger
        render={
          <button
            type='button'
            aria-label={names.join(', ')}
            onClick={stop}
            className='border-border/60 bg-background hover:bg-accent text-muted-foreground inline-flex h-6 min-w-6 shrink-0 items-center justify-center rounded-md border px-1 text-[11px] font-medium transition-colors focus-visible:ring-ring focus-visible:ring-2 focus-visible:outline-none'
          >
            {`+${groups.length}`}
          </button>
        }
      />
      <PopoverContent
        side='top'
        align='center'
        className='max-h-64 w-64 overflow-y-auto'
        onClick={stop}
      >
        <div className='flex flex-col gap-2'>
          {groups.map((group) => (
            <div key={group.key} className='flex items-start gap-2'>
              <span className='mt-0.5 flex size-5 shrink-0 items-center justify-center'>
                {renderBrandIcon(group.icon)}
              </span>
              <div className='min-w-0 flex-1'>
                <div className='text-xs font-medium'>
                  {group.key === OTHER_KEY ? otherLabel : group.label}
                </div>
                <ModelChips models={group.models} />
              </div>
            </div>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  )
}

/**
 * Compact provider-icon summary of a channel's served models.
 */
export function ChannelModelIcons({ models }: { models: string[] }) {
  const { t, i18n } = useTranslation()

  const modelsKey = models.join('\u0001')
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const groups = useMemo(() => buildProviderGroups(models), [modelsKey])

  if (groups.length === 0) {
    return <span className='text-muted-foreground text-xs'>-</span>
  }

  const lang = (i18n.language || 'en').split('-')[0]
  const notProvided = NOT_PROVIDED[lang] ?? NOT_PROVIDED.en
  const otherLabel = t('Other')

  const hasOverflow = groups.length > MAX_ICONS
  const visible = hasOverflow ? groups.slice(0, MAX_ICONS - 1) : groups
  const overflow = hasOverflow ? groups.slice(MAX_ICONS - 1) : []

  return (
    <TooltipProvider delay={100}>
      <div className='flex items-center gap-1' onClick={stop}>
        {visible.map((group) => {
          if (group.key === OTHER_KEY) {
            return <OtherSlot key={group.key} group={group} label={otherLabel} />
          }
          if ((PINNED_KEYS as readonly string[]).includes(group.key)) {
            return (
              <AnchorSlot
                key={group.key}
                group={group}
                notProvided={notProvided}
              />
            )
          }
          return <ProviderSlot key={group.key} group={group} />
        })}
        {hasOverflow && (
          <OverflowSlot groups={overflow} otherLabel={otherLabel} />
        )}
      </div>
    </TooltipProvider>
  )
}
