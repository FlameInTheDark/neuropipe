import { useMemo, useState } from 'react'
import { ArrowRight, Braces, ChevronDown, Loader2, Plus, Search } from 'lucide-react'
import { EmptyState } from '@/components/EmptyState'
import { LucideIcon } from '@/components/LucideIconPicker'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { desktop } from '@/lib/bridge'
import { formatDate } from '@/lib/utils'
import type { FunctionSummary } from '@/lib/types'
import { useUIStore } from '@/stores/ui'
import { useTranslation } from 'react-i18next'

export function FunctionsView({ functions, onRefresh }: { functions: FunctionSummary[]; onRefresh: () => Promise<void> }) {
  const { t } = useTranslation()
  const { setError, setScreen } = useUIStore()
  const [query, setQuery] = useState('')
  const [name, setName] = useState('')
  const [mode, setMode] = useState<'pure' | 'impure'>('impure')
  const [creating, setCreating] = useState(false)
  const groups = useMemo(() => functions.filter((item) => `${item.name} ${item.description} ${item.category}`.toLowerCase().includes(query.toLowerCase())).reduce<Record<string, FunctionSummary[]>>((all, item) => { (all[item.category] ??= []).push(item); return all }, {}), [functions, query])
  const create = async () => {
    try { setCreating(true); const created = await desktop.createFunction(name, mode); await onRefresh(); setScreen('function-editor', created.id) }
    catch (reason) { setError(reason instanceof Error ? reason.message : t('functions.createFailed')) }
    finally { setCreating(false) }
  }
  return <section className="flex h-full min-h-0 flex-col"><PageHeader title={t('functions.title')} description={t('functions.description')} actions={<Button onClick={() => void create()} disabled={creating}>{creating ? <Loader2 className="size-4 animate-spin" /> : <Plus className="size-4" />}{t('functions.new')}</Button>} />
    <div className="muted-scroll min-h-0 flex-1 overflow-y-auto p-8"><div className="mb-6 flex max-w-3xl gap-3"><div className="relative min-w-0 flex-1"><Search className="pointer-events-none absolute left-2.5 top-2 size-4 text-zinc-600" /><Input value={query} onChange={(event) => setQuery(event.target.value)} className="pl-8" placeholder={t('functions.search')} /></div><Input value={name} onChange={(event) => setName(event.target.value)} className="w-56" placeholder={t('functions.name')} /><Select value={mode} onValueChange={(value) => setMode(value as 'pure' | 'impure')} ariaLabel={t('functions.mode')} options={[{ value: 'impure', label: t('functions.impure') }, { value: 'pure', label: t('functions.pure') }]} /><Button variant="outline" onClick={() => void create()} disabled={creating}>{t('functions.create')}</Button></div>
      {functions.length === 0 ? <EmptyState icon={Braces} title={t('functions.emptyTitle')} description={t('functions.emptyDescription')} action={{ label: t('functions.new'), onClick: () => void create() }} /> : Object.entries(groups).map(([category, items]) => <section key={category} className="mb-7"><h2 className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[.14em] text-zinc-500"><ChevronDown className="size-3.5" />{category}</h2><div className="overflow-hidden rounded-xl border border-zinc-800">{items.map((item) => <button key={item.id} onClick={() => setScreen('function-editor', item.id)} className="grid w-full grid-cols-[minmax(0,1fr)_90px_120px_28px] items-center gap-4 border-b border-zinc-800 px-4 py-3.5 text-left last:border-0 hover:bg-zinc-900"><span className="flex min-w-0 items-center gap-3"><span className="flex size-8 shrink-0 items-center justify-center rounded-md border border-zinc-800" style={{ color: item.iconColor, backgroundColor: item.iconBackground }}><LucideIcon name={item.icon} className="size-4" /></span><span className="min-w-0"><span className="block truncate text-sm font-medium text-zinc-100">{item.name}</span><span className="mt-1 block truncate text-xs text-zinc-500">{item.description || t('functions.noDescription')}</span></span></span><span className={item.mode === 'pure' ? 'text-xs text-emerald-300' : 'text-xs text-violet-300'}>{item.mode === 'pure' ? t('functions.pure') : t('functions.impure')}</span><span className="text-xs text-zinc-500">{item.publishedRevision ? t('functions.published', { version: item.publishedRevision }) : t('functions.draft')} · {formatDate(item.updatedAt)}</span><ArrowRight className="size-4 text-zinc-600" /></button>)}</div></section>)}
    </div>
  </section>
}
