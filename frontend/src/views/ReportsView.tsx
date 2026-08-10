import { useEffect, useMemo, useState } from 'react'
import { Columns2, Clock3, FileText, Newspaper, Search, Tag, Trash2, Workflow } from 'lucide-react'
import { ContextMenu, contextMenuPointFromElement, contextMenuPosition, type ContextMenuPoint, type ContextMenuPosition } from '@/components/ContextMenu'
import { MarkdownContent } from '@/components/MarkdownContent'
import { Button } from '@/components/ui/button'
import { DateRangePicker } from '@/components/ui/date-range-picker'
import { EmptyState } from '@/components/EmptyState'
import { PageHeader } from '@/components/PageHeader'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Tooltip } from '@/components/ui/tooltip'
import { desktop } from '@/lib/bridge'
import { usePersistedChoice } from '@/lib/preferences'
import type { Report } from '@/lib/types'
import { cn, formatDate } from '@/lib/utils'
import { useConfirmationStore } from '@/stores/confirmation'
import { useUIStore } from '@/stores/ui'
import { useTranslation } from 'react-i18next'
import { EventsOn } from '../../wailsjs/runtime/runtime'

type ReportView = 'split' | 'posts'
type ReportSort = 'newest' | 'oldest' | 'tag'

interface ReportMenu {
  report: Report
  position: ContextMenuPosition
}

const reportViews = ['split', 'posts'] as const

function ReportMarkdown({ markdown }: { markdown: string }) {
  return <MarkdownContent markdown={markdown} />
}

function reportPreview(markdown: string, fallback: string): string {
  const text = markdown
    .replace(/```[\s\S]*?```/g, ' code block ')
    .replace(/!?(?:\[[^\]]*\]\([^)]*\))/g, ' ')
    .replace(/[#>*_`~-]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
  return text.length > 180 ? `${text.slice(0, 177).trimEnd()}…` : text || fallback
}

function dateTimeBoundary(value: string, endOfDay = false): number | undefined {
	const dateOnly = /^\d{4}-\d{2}-\d{2}$/.test(value)
	const dateTime = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)
	if (!dateOnly && !dateTime) return undefined
	const time = Date.parse(dateOnly ? `${value}T${endOfDay ? '23:59:59.999' : '00:00:00'}` : value)
  return Number.isNaN(time) ? undefined : time
}

function ReportTags({ report, onSelect }: { report: Report; onSelect: (tag: string) => void }) {
  if (report.tags.length === 0) return null
  return <div className="mt-3 flex flex-wrap gap-1.5">{report.tags.map((tag) => <button key={tag.toLowerCase()} type="button" onClick={() => onSelect(tag)} className="inline-flex items-center gap-1 rounded bg-zinc-800 px-2 py-1 text-[10px] font-medium text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-zinc-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"><Tag className="size-3" />{tag}</button>)}</div>
}

function ReportMetadata({ report, onOpenPipeline }: { report: Report; onOpenPipeline: () => void }) {
  const { t } = useTranslation()
  return <div className="mt-3 flex flex-wrap gap-x-4 gap-y-2 text-xs text-zinc-500"><span className="flex items-center gap-1.5"><Clock3 className="size-3.5" />{t('reports.created', { date: formatDate(report.createdAt) })}</span><span className="flex items-center gap-1.5"><Clock3 className="size-3.5" />{t('reports.pipelineStarted', { date: formatDate(report.executionStartedAt) })}</span><button type="button" onClick={onOpenPipeline} className="flex items-center gap-1.5 hover:text-zinc-200"><Workflow className="size-3.5" />{report.pipelineName}</button></div>
}

export function ReportsView({ reports, onRefresh }: { reports: Report[]; onRefresh: () => Promise<void> }) {
  const { t } = useTranslation()
  const { setScreen, setError } = useUIStore()
  const requestConfirmation = useConfirmationStore((state) => state.ask)
  const [selectedID, setSelectedID] = useState<string>()
  const [view, setView] = usePersistedChoice<ReportView>('neuropipe.reports.view.v1', reportViews, 'split')
  const [query, setQuery] = useState('')
  const [tag, setTag] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [sort, setSort] = useState<ReportSort>('newest')
  const [menu, setMenu] = useState<ReportMenu>()
  const [deletingID, setDeletingID] = useState('')

  const availableTags = useMemo(() => Array.from(new Map(reports.flatMap((report) => report.tags.map((item) => [item.toLowerCase(), item] as const))).values()).sort((left, right) => left.localeCompare(right)), [reports])
  const filteredReports = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase()
    const tagKey = tag.toLowerCase()
    const fromTime = dateTimeBoundary(from)
    const toTime = dateTimeBoundary(to, true)
    const matches = reports.filter((report) => {
      if (normalizedQuery && !`${report.title} ${report.pipelineName}`.toLowerCase().includes(normalizedQuery)) return false
      if (tagKey && !report.tags.some((item) => item.toLowerCase() === tagKey)) return false
      const createdAt = Date.parse(report.createdAt)
      if (!Number.isFinite(createdAt)) return false
      if (fromTime !== undefined && createdAt < fromTime) return false
      return toTime === undefined || createdAt <= toTime
    })
    return [...matches].sort((left, right) => {
      if (sort === 'tag') {
        const byTag = (left.tags[0] ?? t('reports.uncategorized')).localeCompare(right.tags[0] ?? t('reports.uncategorized'))
        if (byTag !== 0) return byTag
      }
      const leftTime = Date.parse(left.createdAt)
      const rightTime = Date.parse(right.createdAt)
      return sort === 'oldest' ? leftTime - rightTime : rightTime - leftTime
    })
  }, [from, query, reports, sort, t, tag, to])
  const selected = filteredReports.find((report) => report.id === selectedID) ?? filteredReports[0]
  const filtersActive = query !== '' || tag !== '' || from !== '' || to !== ''
  const openPipeline = (report: Report) => setScreen('editor', report.pipelineId)
  const clearFilters = () => { setQuery(''); setTag(''); setFrom(''); setTo('') }

  useEffect(() => {
    void onRefresh()
  }, [onRefresh])

  useEffect(() => EventsOn('reports.updated', () => { void onRefresh() }), [onRefresh])

  useEffect(() => {
    if (selectedID && filteredReports.some((report) => report.id === selectedID)) return
    setSelectedID(filteredReports[0]?.id)
  }, [filteredReports, selectedID])

  const openMenu = (point: ContextMenuPoint, report: Report) => {
    setMenu({ report, position: contextMenuPosition(point, { width: 176, height: 48 }) })
  }

  const remove = async (report: Report) => {
    setMenu(undefined)
    const confirmed = await requestConfirmation({
      title: t('reports.deleteTitle'),
      description: t('reports.deleteDescription', { name: report.title }),
      confirmLabel: t('reports.deleteConfirm'),
    })
    if (!confirmed) return
    try {
      setDeletingID(report.id)
      await desktop.deleteReport(report.id)
      await onRefresh()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t('reports.deleteFailed'))
    } finally {
      setDeletingID('')
    }
  }

  const reportMenu = (report: Report, event: { clientX: number; clientY: number; preventDefault: () => void }) => {
    event.preventDefault()
    openMenu(event, report)
  }

  return <section className="flex h-full min-h-0 flex-col">
    <PageHeader title={t('reports.title')} description={t('reports.description')} actions={<div className="flex rounded-md border border-zinc-800 bg-zinc-950 p-0.5"><Tooltip content={t('reports.splitTitle')} side="bottom"><Button size="sm" variant={view === 'split' ? 'secondary' : 'ghost'} onClick={() => setView('split')}><Columns2 className="size-3.5" />{t('reports.split')}</Button></Tooltip><Tooltip content={t('reports.postsTitle')} side="bottom"><Button size="sm" variant={view === 'posts' ? 'secondary' : 'ghost'} onClick={() => setView('posts')}><Newspaper className="size-3.5" />{t('reports.posts')}</Button></Tooltip></div>} />
    <div className="min-h-0 flex-1 p-8">
      {reports.length === 0 ? <EmptyState icon={FileText} title={t('reports.emptyTitle')} description={t('reports.emptyDescription')} action={{ label: t('reports.openPipelines'), onClick: () => setScreen('pipelines') }} /> : <>
        <div className="mb-5 grid gap-3 rounded-xl border border-zinc-800 bg-zinc-950/40 p-3 lg:grid-cols-[minmax(15rem,1fr)_11rem_minmax(15rem,.8fr)_9rem_auto]">
          <div className="relative"><Search className="pointer-events-none absolute left-2.5 top-2 size-4 text-zinc-600" /><Input value={query} onChange={(event) => setQuery(event.target.value)} className="pl-8" placeholder={t('reports.search')} /></div>
          <Select value={tag} onValueChange={setTag} options={[{ value: '', label: t('reports.allTags') }, ...availableTags.map((item) => ({ value: item, label: item }))]} ariaLabel={t('reports.filterTag')} />
          <DateRangePicker from={from} to={to} onChange={(range) => { setFrom(range.from); setTo(range.to) }} />
          <Select value={sort} onValueChange={(value) => setSort(value as ReportSort)} options={[{ value: 'newest', label: t('reports.newest') }, { value: 'oldest', label: t('reports.oldest') }, { value: 'tag', label: t('reports.tagAZ') }]} ariaLabel={t('reports.sort')} />
          <Button size="sm" variant="ghost" onClick={clearFilters} disabled={!filtersActive}>{t('reports.clear')}</Button>
        </div>
        {filteredReports.length === 0 ? <div className="surface flex h-48 flex-col items-center justify-center rounded-xl text-center"><Search className="mb-3 size-5 text-zinc-600" /><p className="text-sm font-medium text-zinc-300">{t('reports.noMatching')}</p><p className="mt-1 text-xs text-zinc-500">{t('reports.noMatchingDescription')}</p><Button className="mt-4" size="sm" variant="outline" onClick={clearFilters}>{t('reports.clearFilters')}</Button></div> : view === 'split' ? <div className="grid h-[calc(100%-4.5rem)] min-h-0 gap-5 lg:grid-cols-[minmax(19rem,.85fr)_minmax(0,1.5fr)]">
          <div className="surface muted-scroll min-h-0 overflow-y-auto rounded-xl p-2">
            {filteredReports.map((report) => <article key={report.id} onContextMenu={(event) => reportMenu(report, event)} className={cn('rounded-lg border border-transparent p-3 transition-colors', selected?.id === report.id ? 'border-zinc-700 bg-zinc-900' : 'hover:bg-zinc-900/70')}>
              <button type="button" className="w-full text-left" onClick={() => setSelectedID(report.id)} onKeyDown={(event) => { if (event.key !== 'ContextMenu' && !(event.shiftKey && event.key === 'F10')) return; event.preventDefault(); openMenu(contextMenuPointFromElement(event.currentTarget), report) }} aria-current={selected?.id === report.id ? 'true' : undefined}>
                <h2 className="truncate text-sm font-medium text-zinc-100">{report.title}</h2>
                <p className="mt-1.5 line-clamp-2 text-xs leading-5 text-zinc-500">{reportPreview(report.markdown, t('reports.noContent'))}</p>
              </button>
              <ReportTags report={report} onSelect={setTag} />
              <ReportMetadata report={report} onOpenPipeline={() => openPipeline(report)} />
            </article>)}
          </div>
          {selected ? <article onContextMenu={(event) => reportMenu(selected, event)} className="surface muted-scroll min-h-0 overflow-y-auto rounded-xl p-6">
            <div className="border-b border-zinc-800 pb-5"><h2 className="text-lg font-semibold text-zinc-100">{selected.title}</h2><ReportTags report={selected} onSelect={setTag} /><ReportMetadata report={selected} onOpenPipeline={() => openPipeline(selected)} /></div>
            <div className="mt-6"><ReportMarkdown markdown={selected.markdown} /></div>
          </article> : null}
        </div> : <div className="muted-scroll h-[calc(100%-4.5rem)] min-h-0 overflow-y-auto"><div className="mx-auto max-w-3xl space-y-5 pb-8">{filteredReports.map((report) => <article key={report.id} tabIndex={0} onContextMenu={(event) => reportMenu(report, event)} onKeyDown={(event) => { if (event.key !== 'ContextMenu' && !(event.shiftKey && event.key === 'F10')) return; event.preventDefault(); openMenu(contextMenuPointFromElement(event.currentTarget), report) }} className="surface rounded-xl p-7 outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"><h2 className="text-xl font-semibold tracking-tight text-zinc-100">{report.title}</h2><ReportTags report={report} onSelect={setTag} /><ReportMetadata report={report} onOpenPipeline={() => openPipeline(report)} /><div className="mt-7 border-t border-zinc-800 pt-6"><ReportMarkdown markdown={report.markdown} /></div></article>)}</div></div>}
      </>}
      {menu ? <ContextMenu position={menu.position} ariaLabel={t('reports.options', { name: menu.report.title })} className="w-44" onClose={() => setMenu(undefined)}><button role="menuitem" disabled={deletingID !== ''} className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-xs text-red-300 hover:bg-red-500/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-400/50 disabled:cursor-not-allowed disabled:opacity-40" onClick={() => void remove(menu.report)}><Trash2 className="size-3.5" />{t('reports.deleteConfirm')}</button></ContextMenu> : null}
    </div>
  </section>
}
