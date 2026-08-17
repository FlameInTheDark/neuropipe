import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Activity, ArrowUpRight, BarChart3, Clock3, Cpu, DollarSign, Filter, Loader2, RefreshCw, Server, Workflow, X } from 'lucide-react'
import type { EChartsOption } from 'echarts'
import { DateRangePicker } from '@/components/ui/date-range-picker'
import { MetricsChart } from '@/components/MetricsChart'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { desktop } from '@/lib/bridge'
import i18n from '@/i18n'
import { usePersistedChoice, usePersistedValue } from '@/lib/preferences'
import type { MetricsBreakdown, MetricsFilter, MetricsKPI, MetricsOverview, PipelineSummary, RunStatus, TriggerKind } from '@/lib/types'
import { cn } from '@/lib/utils'
import { useUIStore } from '@/stores/ui'
import { Events } from '@wailsio/runtime'
import { useTranslation } from 'react-i18next'

type RangePreset = 'today' | '7d' | '30d' | '90d' | '12mo' | 'custom'

const rangePresets: readonly RangePreset[] = ['today', '7d', '30d', '90d', '12mo', 'custom']
const chartText = '#a1a1aa'
const orange = '#f97316'
const green = '#34d399'
const red = '#fb7185'

function timeRange(preset: RangePreset, customFrom: string, customTo: string): { from: string; to: string } {
  if (preset === 'custom' && customFrom && customTo) return { from: toISO(customFrom), to: toISO(customTo) }
  const to = new Date()
  const from = new Date(to)
  if (preset === 'today') from.setHours(0, 0, 0, 0)
  if (preset === '7d') from.setDate(from.getDate() - 7)
  if (preset === '30d' || preset === 'custom') from.setDate(from.getDate() - 30)
  if (preset === '90d') from.setDate(from.getDate() - 90)
  if (preset === '12mo') from.setMonth(from.getMonth() - 12)
  return { from: from.toISOString(), to: to.toISOString() }
}

function toISO(value: string): string {
  if (!value) return ''
  const time = Date.parse(value)
  return Number.isFinite(time) ? new Date(time).toISOString() : value
}

function shortDate(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(i18n.resolvedLanguage, { month: 'short', day: 'numeric' }).format(date)
}

function compact(value: number): string {
  return new Intl.NumberFormat(i18n.resolvedLanguage, { notation: 'compact', maximumFractionDigits: 1 }).format(value)
}

function percentage(value: number): string {
  if (!Number.isFinite(value)) return '—'
  return new Intl.NumberFormat(i18n.resolvedLanguage, { style: 'percent', maximumFractionDigits: 1 }).format(value / 100)
}

function list<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

function duration(value: number): string {
  if (!Number.isFinite(value)) return '—'
  const format = (amount: number, unit: Intl.NumberFormatOptions['unit'], maximumFractionDigits: number) => new Intl.NumberFormat(i18n.resolvedLanguage, { style: 'unit', unit, unitDisplay: 'narrow', maximumFractionDigits }).format(amount)
  if (value > 0 && value < 1) return format(value, 'millisecond', value < 0.1 ? 2 : 1)
  if (value < 1_000) return format(Math.round(value), 'millisecond', 0)
  if (value < 60_000) return format(value / 1_000, 'second', value < 10_000 ? 1 : 0)
  return format(value / 60_000, 'minute', 1)
}

function memory(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB']
  let index = 0
  while (value >= 1024 && index < units.length - 1) { value /= 1024; index++ }
  return `${value.toFixed(value >= 10 || index === 0 ? 0 : 1)} ${units[index]}`
}

function delta(kpi: MetricsKPI, invert: boolean, noComparison: string, previous: string): { text: string; tone: string } {
  if (!kpi.available || !Number.isFinite(kpi.previousValue) || kpi.previousValue === 0) return { text: noComparison, tone: 'text-zinc-600' }
  const change = ((kpi.value - kpi.previousValue) / Math.abs(kpi.previousValue)) * 100
  const good = invert ? change <= 0 : change >= 0
  return { text: `${change > 0 ? '+' : ''}${change.toFixed(1)}% ${previous}`, tone: good ? 'text-emerald-400' : 'text-rose-400' }
}

function chartBase(): EChartsOption {
  return {
    animationDuration: 220,
    textStyle: { color: chartText, fontFamily: 'Geist, ui-sans-serif, system-ui' },
    grid: { left: 34, right: 16, top: 20, bottom: 28, containLabel: false },
    tooltip: { trigger: 'axis', backgroundColor: '#18181b', borderColor: '#3f3f46', textStyle: { color: '#f4f4f5' }, confine: true },
    aria: { enabled: true },
    xAxis: { type: 'category', axisLine: { lineStyle: { color: '#27272a' } }, axisTick: { show: false }, axisLabel: { color: '#71717a', fontSize: 10 } },
    yAxis: { type: 'value', splitLine: { lineStyle: { color: '#27272a' } }, axisLabel: { color: '#71717a', fontSize: 10 } },
  }
}

function KPI({ label, value, metric, icon: Icon, suffix = '', invert = false, available = metric.available }: { label: string; value: string; metric: MetricsKPI; icon: typeof Activity; suffix?: string; invert?: boolean; available?: boolean }) {
  const { t } = useTranslation()
  const comparison = delta(metric, invert, t('metricsUI.noComparison'), t('metricsUI.previous'))
  return <article className="surface min-w-0 rounded-xl p-4"><div className="flex items-center justify-between gap-3"><span className="text-xs font-medium text-zinc-500">{label}</span><Icon className="size-4 text-zinc-600" /></div><p className="mt-3 text-2xl font-semibold tracking-tight text-zinc-100">{available ? value : '—'}{available ? suffix : ''}</p><p className={cn('mt-1.5 text-[11px]', comparison.tone)}>{comparison.text}</p><div className="mt-3 h-px w-12 bg-orange-500/70" /></article>
}

interface BreakdownDialogState {
  title: string
  items: MetricsBreakdown[]
  primaryLabel: string
  formatValue: (value: number) => string
}

function BreakdownTable({ title, items, empty, primaryLabel = 'Count', formatValue = compact, onOpen }: { title: string; items: MetricsBreakdown[]; empty: string; primaryLabel?: string; formatValue?: (value: number) => string; onOpen?: () => void }) {
  const { t } = useTranslation()
  return <article className="surface min-w-0 rounded-xl"><div className="flex items-center justify-between gap-3 border-b border-zinc-800 px-4 py-3"><h2 className="text-sm font-medium text-zinc-200">{title}</h2><div className="flex shrink-0 items-center gap-1.5"><span className="text-[10px] uppercase tracking-[0.12em] text-zinc-600">{primaryLabel}</span>{items.length > 0 && onOpen ? <button type="button" onClick={onOpen} aria-label={t('metricsUI.viewAll', { title })} className="flex size-5 items-center justify-center rounded text-zinc-600 transition-colors hover:bg-zinc-800 hover:text-zinc-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"><ArrowUpRight className="size-3.5" /></button> : null}</div></div>{items.length === 0 ? <p className="px-4 py-8 text-center text-xs text-zinc-600">{empty}</p> : <div className="divide-y divide-zinc-800/80">{items.slice(0, 6).map((item) => <div key={item.id} className="flex min-w-0 items-center gap-3 px-4 py-3"><span className="min-w-0 flex-1 truncate text-xs text-zinc-300">{item.label}</span><span className="font-mono text-xs text-zinc-500">{formatValue(item.value)}</span></div>)}</div>}</article>
}

function BreakdownDialog({ state, onClose }: { state: BreakdownDialogState; onClose: () => void }) {
  const { t } = useTranslation()
  const titleID = useId()
  const closeRef = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : undefined
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKeyDown)
    const timer = window.setTimeout(() => closeRef.current?.focus(), 0)
    return () => { window.removeEventListener('keydown', onKeyDown); window.clearTimeout(timer); previousFocus?.focus() }
  }, [onClose])

  return createPortal(<div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/70 p-5 backdrop-blur-[2px]" onPointerDown={(event) => { if (event.target === event.currentTarget) onClose() }}>
    <section role="dialog" aria-modal="true" aria-labelledby={titleID} className="flex max-h-[min(720px,calc(100vh-40px))] w-full max-w-2xl flex-col overflow-hidden rounded-xl border border-zinc-700 bg-zinc-950 shadow-2xl shadow-black/70">
      <header className="flex items-center justify-between gap-4 border-b border-zinc-800 px-5 py-4"><div><h2 id={titleID} className="text-sm font-semibold text-zinc-100">{state.title}</h2><p className="mt-1 text-xs text-zinc-500">{t('metricsUI.results', { count: state.items.length })}</p></div><Button ref={closeRef} size="sm" variant="ghost" onClick={onClose}><X className="size-3.5" />{t('common.close')}</Button></header>
      <div className="muted-scroll min-h-0 overflow-auto"><table className="w-full text-left text-sm"><thead className="sticky top-0 bg-zinc-950 text-[10px] uppercase tracking-[0.12em] text-zinc-600"><tr><th className="px-5 py-3 font-medium">{t('metricsUI.name')}</th><th className="px-5 py-3 text-right font-medium">{state.primaryLabel}</th></tr></thead><tbody className="divide-y divide-zinc-800/80">{state.items.map((item) => <tr key={item.id}><td className="max-w-0 truncate px-5 py-3 text-zinc-300">{item.label}</td><td className="whitespace-nowrap px-5 py-3 text-right font-mono text-xs text-zinc-400">{state.formatValue(item.value)}</td></tr>)}</tbody></table></div>
    </section>
  </div>, document.body)
}

function ChartFallback({ rows, valueLabel }: { rows: { label: string; value: number }[]; valueLabel: string }) {
  const { t } = useTranslation()
  return <div className="max-h-52 overflow-y-auto px-1"><table className="w-full text-left text-xs"><thead className="text-zinc-600"><tr><th className="pb-2 font-medium">{t('metricsUI.period')}</th><th className="pb-2 text-right font-medium">{valueLabel}</th></tr></thead><tbody className="divide-y divide-zinc-800">{rows.map((row) => <tr key={row.label}><td className="py-2 text-zinc-400">{row.label}</td><td className="py-2 text-right font-mono text-zinc-200">{compact(row.value)}</td></tr>)}</tbody></table></div>
}

export function MetricsView({ pipelines }: { pipelines: PipelineSummary[] }) {
  const { t } = useTranslation()
  const { setError } = useUIStore()
  const [preset, setPreset] = usePersistedChoice<RangePreset>('neuropipe.metrics.range.v1', rangePresets, '30d')
  const [customRange, setCustomRange] = usePersistedValue('neuropipe.metrics.custom-range.v1', { from: '', to: '' })
  const [pipelineID, setPipelineID] = usePersistedValue('neuropipe.metrics.pipeline.v1', '')
  const [trigger, setTrigger] = usePersistedValue<'' | TriggerKind>('neuropipe.metrics.trigger.v1', '')
  const [status, setStatus] = usePersistedValue<'' | RunStatus>('neuropipe.metrics.status.v1', '')
  const [model, setModel] = usePersistedValue('neuropipe.metrics.model.v1', '')
  const [breakdown, setBreakdown] = useState<BreakdownDialogState>()
  const [overview, setOverview] = useState<MetricsOverview>()
  const [loading, setLoading] = useState(true)
  const rangeOptions = useMemo(() => [
    { value: 'today', label: t('metricsUI.today') }, { value: '7d', label: t('metricsUI.last7') }, { value: '30d', label: t('metricsUI.last30') }, { value: '90d', label: t('metricsUI.last90') }, { value: '12mo', label: t('metricsUI.last12') }, { value: 'custom', label: t('metricsUI.custom') },
  ], [t])
  const triggerOptions = useMemo<{ value: '' | TriggerKind; label: string }[]>(() => [
    { value: '', label: t('metricsUI.allTriggers') }, { value: 'button', label: t('metricsUI.button') }, { value: 'cron', label: t('metricsUI.schedule') }, { value: 'file', label: t('metricsUI.fileWatch') }, { value: 'hotkey', label: t('metricsUI.hotkey') }, { value: 'webhook', label: t('metricsUI.webhook') }, { value: 'chat', label: t('metricsUI.chat') },
  ], [t])
  const statusOptions = useMemo<{ value: '' | RunStatus; label: string }[]>(() => [
    { value: '', label: t('metricsUI.allOutcomes') }, { value: 'completed', label: t('metricsUI.completed') }, { value: 'failed', label: t('metricsUI.failed') }, { value: 'skipped', label: t('metricsUI.skipped') }, { value: 'cancelled', label: t('metricsUI.cancelled') },
  ], [t])
  const dates = useMemo(() => timeRange(preset, customRange.from, customRange.to), [customRange.from, customRange.to, preset])
  const filter = useMemo<MetricsFilter>(() => ({ from: dates.from, to: dates.to, pipelineIds: pipelineID ? [pipelineID] : undefined, triggerKinds: trigger ? [trigger] : undefined, statuses: status ? [status] : undefined, models: model ? [model] : undefined }), [dates.from, dates.to, model, pipelineID, status, trigger])
  const refresh = useCallback(async () => {
    try {
      setLoading(true)
      setOverview(await desktop.getMetricsOverview(filter))
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t('metricsUI.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [filter, setError, t])

  useEffect(() => { void refresh() }, [refresh])
  useEffect(() => Events.On('metrics.updated', () => { void refresh() }), [refresh])

  const modelOptions = useMemo(() => {
    const available = list(overview?.models)
    return [{ value: '', label: t('metricsUI.allModels') }, ...available.map((entry) => ({ value: entry.label.split(' · ').at(-1) ?? entry.label, label: entry.label }))]
  }, [overview?.models, t])
  const activeFilters = Boolean(pipelineID || trigger || status || model || preset === 'custom')
  const clearFilters = () => { setPipelineID(''); setTrigger(''); setStatus(''); setModel(''); setPreset('30d') }

  const runOption = useMemo<EChartsOption>(() => {
    const base = chartBase()
    const points = overview?.runSeries ?? []
    return { ...base, aria: { enabled: true, description: t('metricsUI.pipelineOutcomeAria') }, legend: { top: 0, right: 0, textStyle: { color: '#a1a1aa', fontSize: 10 }, itemWidth: 9, itemHeight: 9 }, xAxis: { ...(base.xAxis as object), data: points.map((point) => shortDate(point.at)) }, yAxis: base.yAxis, series: [
      { name: t('metricsUI.completed'), type: 'bar', stack: 'runs', data: points.map((point) => point.completed), itemStyle: { color: green } },
      { name: t('metricsUI.failed'), type: 'bar', stack: 'runs', data: points.map((point) => point.failed), itemStyle: { color: red } },
      { name: t('metricsUI.skipped'), type: 'bar', stack: 'runs', data: points.map((point) => point.skipped), itemStyle: { color: '#a1a1aa' } },
      { name: t('metricsUI.cancelled'), type: 'bar', stack: 'runs', data: points.map((point) => point.cancelled), itemStyle: { color: '#71717a' } },
    ] }
  }, [overview?.runSeries, t])

  const durationOption = useMemo<EChartsOption>(() => {
    const base = chartBase()
    const points = overview?.durationSeries ?? []
    return { ...base, aria: { enabled: true, description: t('metricsUI.durationAria') }, tooltip: { trigger: 'axis', backgroundColor: '#18181b', borderColor: '#3f3f46', textStyle: { color: '#f4f4f5' }, confine: true, valueFormatter: (value: unknown) => duration(Number(Array.isArray(value) ? value[0] : value)) }, xAxis: { ...(base.xAxis as object), data: points.map((point) => shortDate(point.at)) }, yAxis: { ...(base.yAxis as object), axisLabel: { color: '#71717a', formatter: (value: number) => duration(value) } }, series: [{ name: t('metricsUI.averageDuration'), type: 'line', smooth: true, symbol: 'none', data: points.map((point) => point.value), lineStyle: { color: orange, width: 2 }, areaStyle: { color: 'rgba(249,115,22,0.08)' }, markLine: { symbol: 'none', label: { color: '#71717a', fontSize: 10 }, lineStyle: { color: '#52525b', type: 'dashed' }, data: [{ yAxis: 1_000 }] } }] }
  }, [overview?.durationSeries, t])

  const tokensOption = useMemo<EChartsOption>(() => {
    const base = chartBase()
    const points = overview?.llmSeries ?? []
    return { ...base, aria: { enabled: true, description: t('metricsUI.tokensAria') }, legend: { top: 0, right: 0, textStyle: { color: '#a1a1aa', fontSize: 10 }, itemWidth: 9, itemHeight: 9 }, xAxis: { ...(base.xAxis as object), data: points.map((point) => shortDate(point.at)) }, yAxis: base.yAxis, series: [{ name: t('metricsUI.input'), type: 'bar', stack: 'tokens', data: points.map((point) => point.value), itemStyle: { color: '#fb923c' } }, { name: t('metricsUI.output'), type: 'bar', stack: 'tokens', data: points.map((point) => point.value2 ?? 0), itemStyle: { color: '#fdba74' } }] }
  }, [overview?.llmSeries, t])

  const resourceOption = useMemo<EChartsOption>(() => {
    const base = chartBase()
    const points = overview?.resources ?? []
    const labels = [...new Set(points.map((point) => point.at))]
    const byProcess = (process: string, field: 'cpuPercent' | 'workingSet') => labels.map((at) => points.find((point) => point.at === at && point.process === process)?.[field] ?? null)
    return { ...base, aria: { enabled: true, description: t('metricsUI.resourcesAria') }, legend: { top: 0, right: 0, textStyle: { color: '#a1a1aa', fontSize: 10 }, itemWidth: 9, itemHeight: 9 }, tooltip: { trigger: 'axis', backgroundColor: '#18181b', borderColor: '#3f3f46', textStyle: { color: '#f4f4f5' }, confine: true, valueFormatter: (value: unknown) => `${Number(Array.isArray(value) ? value[0] : value ?? 0).toFixed(2)}%` }, xAxis: { ...(base.xAxis as object), data: labels.map(shortDate) }, yAxis: { ...(base.yAxis as object), axisLabel: { color: '#71717a', formatter: '{value}%' } }, series: [{ name: 'Neuropipe', type: 'line', smooth: true, symbol: 'none', data: byProcess('Neuropipe', 'cpuPercent'), lineStyle: { color: orange } }, { name: 'llama.cpp', type: 'line', smooth: true, symbol: 'none', data: byProcess('llama.cpp', 'cpuPercent'), lineStyle: { color: '#a78bfa' } }] }
  }, [overview?.resources, t])

  const healthRows = useMemo(() => list(overview?.pipelines), [overview?.pipelines])
  const healthOption = useMemo<EChartsOption>(() => {
    const base = chartBase()
    const names = [...new Set(healthRows.map((item) => item.name))]
    const dates = [...new Set(healthRows.map((item) => shortDate(item.at)))]
    return { ...base, aria: { enabled: true, description: t('metricsUI.healthAria') }, grid: { left: 110, right: 20, top: 20, bottom: 34 }, tooltip: { trigger: 'item', backgroundColor: '#18181b', borderColor: '#3f3f46', textStyle: { color: '#f4f4f5' }, valueFormatter: (value: unknown) => percentage(Number(Array.isArray(value) ? value.at(-1) : value)) }, visualMap: { min: 0, max: 100, calculable: false, orient: 'horizontal', left: 'center', bottom: 0, inRange: { color: ['#be123c', '#f97316', '#10b981'] }, textStyle: { color: '#71717a', fontSize: 10 } }, xAxis: { ...(base.xAxis as object), data: dates }, yAxis: { type: 'category', data: names, axisLine: { lineStyle: { color: '#27272a' } }, axisTick: { show: false }, axisLabel: { color: '#a1a1aa', fontSize: 10, width: 96, overflow: 'truncate' } }, series: [{ name: t('metricsUI.pipelineHealth'), type: 'heatmap', data: healthRows.map((item) => { const total = item.completed + item.failed; return [dates.indexOf(shortDate(item.at)), names.indexOf(item.name), total > 0 ? item.completed * 100 / total : 0] }), label: { show: false }, itemStyle: { borderColor: '#09090b', borderWidth: 2 } }] }
  }, [healthRows, t])

  const resourceSummary = useMemo(() => {
    const current = overview?.resources ?? []
    const app = current.filter((item) => item.process === 'Neuropipe').at(-1)
    const llama = current.filter((item) => item.process === 'llama.cpp').at(-1)
    return { app, llama }
  }, [overview?.resources])
  const averageQueueWait = useMemo(() => {
    const points = overview?.queueSeries ?? []
    const weighted = points.reduce((total, item) => total + item.value * (item.value3 || 1), 0)
    const count = points.reduce((total, item) => total + (item.value3 || 1), 0)
    return count ? weighted / count : 0
  }, [overview?.queueSeries])

  return <section className="flex h-full min-h-0 flex-col">
    <PageHeader title={t('metrics.title')} description={t('metrics.description')} actions={<Button size="sm" variant="outline" onClick={() => void refresh()} disabled={loading}>{loading ? <Loader2 className="size-3.5 animate-spin" /> : <RefreshCw className="size-3.5" />}{t('metrics.refresh')}</Button>} />
    <div className="muted-scroll min-h-0 flex-1 overflow-y-auto p-8">
      <div className="mx-auto max-w-[1500px] space-y-5 pb-8">
        <div className="grid gap-3 rounded-xl border border-zinc-800 bg-zinc-950/50 p-3 lg:grid-cols-[10rem_minmax(13rem,1fr)_10rem_10rem_12rem_auto]">
          <Select value={preset} onValueChange={(value) => setPreset(value as RangePreset)} options={rangeOptions} ariaLabel={t('metricsUI.timeRange')} />
          <Select value={pipelineID} onValueChange={setPipelineID} options={[{ value: '', label: t('metricsUI.allPipelines') }, ...pipelines.map((pipeline) => ({ value: pipeline.id, label: pipeline.name }))]} ariaLabel={t('metricsUI.filterPipeline')} />
          <Select value={trigger} onValueChange={(value) => setTrigger(value as '' | TriggerKind)} options={triggerOptions} ariaLabel={t('metricsUI.filterTrigger')} />
          <Select value={status} onValueChange={(value) => setStatus(value as '' | RunStatus)} options={statusOptions} ariaLabel={t('metricsUI.filterOutcome')} />
          <Select value={model} onValueChange={setModel} options={modelOptions} ariaLabel={t('metricsUI.filterModel')} />
          <Button size="sm" variant="ghost" onClick={clearFilters} disabled={!activeFilters}><Filter className="size-3.5" />{t('metricsUI.clear')}</Button>
          {preset === 'custom' ? <div className="lg:col-span-6"><DateRangePicker from={customRange.from} to={customRange.to} onChange={setCustomRange} className="w-full max-w-sm" /></div> : null}
        </div>

        {loading && !overview ? <div className="surface flex h-52 items-center justify-center rounded-xl text-sm text-zinc-500"><Loader2 className="mr-2 size-4 animate-spin" />{t('metricsUI.loading')}</div> : overview ? <>
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <KPI label={t('metricsUI.totalRuns')} value={compact(overview.runs.value)} metric={overview.runs} icon={Workflow} />
            <KPI label={t('metricsUI.successRate')} value={overview.successRate.value.toFixed(1)} metric={overview.successRate} icon={Activity} suffix="%" />
            <KPI label={t('metricsUI.p95Duration')} value={duration(overview.p95DurationMs.value)} metric={overview.p95DurationMs} icon={Clock3} invert />
            <KPI label={t('metricsUI.estimatedCost')} value={overview.estimatedCostUsd.value > 0 ? `$${overview.estimatedCostUsd.value.toFixed(2)}` : overview.localCalls > 0 ? t('metricsUI.local') : t('metricsUI.unpriced')} metric={overview.estimatedCostUsd} available={overview.estimatedCostUsd.available || overview.unpricedCalls > 0} icon={DollarSign} />
          </div>

          <div className="grid gap-5 xl:grid-cols-[minmax(0,1.65fr)_minmax(19rem,.75fr)]">
            <article className="surface min-w-0 rounded-xl p-5"><div className="mb-4"><h2 className="text-sm font-medium text-zinc-200">{t('metricsUI.runHealth')}</h2><p className="mt-1 text-xs text-zinc-500">{t('metricsUI.runHealthDescription')}</p></div><MetricsChart option={runOption} ariaLabel={t('metricsUI.runHealth')} className="h-64" fallback={<ChartFallback valueLabel={t('metricsUI.totalRuns')} rows={list(overview.runSeries).map((point) => ({ label: shortDate(point.at), value: point.completed + point.failed + point.skipped + point.cancelled }))} />} /></article>
            <article className="surface min-w-0 rounded-xl p-5"><h2 className="text-sm font-medium text-zinc-200">{t('metricsUI.queuePerformance')}</h2><dl className="mt-5 space-y-4"><div className="flex items-baseline justify-between gap-3"><dt className="text-xs text-zinc-500">{t('metricsUI.averageDuration')}</dt><dd className="font-mono text-sm text-zinc-200">{duration(overview.averageDurationMs.value)}</dd></div><div className="flex items-baseline justify-between gap-3"><dt className="text-xs text-zinc-500">{t('metricsUI.p95Duration')}</dt><dd className="font-mono text-sm text-zinc-200">{overview.p95DurationMs.available ? duration(overview.p95DurationMs.value) : t('metricsUI.detailedRangeOnly')}</dd></div><div className="flex items-baseline justify-between gap-3"><dt className="text-xs text-zinc-500">{t('metricsUI.averageQueueWait')}</dt><dd className="font-mono text-sm text-zinc-200">{duration(averageQueueWait)}</dd></div></dl><p className="mt-7 border-t border-zinc-800 pt-4 text-[11px] leading-5 text-zinc-600">{t('metricsUI.queueHint')}</p></article>
          </div>

          <div className="grid gap-5 xl:grid-cols-2">
            <article className="surface min-w-0 rounded-xl p-5"><div className="mb-4"><h2 className="text-sm font-medium text-zinc-200">{t('metricsUI.pipelineDuration')}</h2><p className="mt-1 text-xs text-zinc-500">{t('metricsUI.durationDescription', { granularity: overview.granularity })}</p></div><MetricsChart option={durationOption} ariaLabel={t('metricsUI.pipelineDuration')} className="h-60" fallback={<ChartFallback valueLabel={t('metricsUI.averageDuration')} rows={list(overview.durationSeries).map((point) => ({ label: shortDate(point.at), value: point.value }))} />} /></article>
            <article className="surface min-w-0 rounded-xl p-5"><div className="mb-4 flex items-start justify-between gap-4"><div><h2 className="text-sm font-medium text-zinc-200">{t('metricsUI.llmUsage')}</h2><p className="mt-1 text-xs text-zinc-500">{t('metricsUI.llmDescription')}</p></div><span className="font-mono text-xs text-zinc-500">{compact(overview.llmCalls.value)} {t('metricsUI.calls')}</span></div><MetricsChart option={tokensOption} ariaLabel={t('metricsUI.llmUsage')} className="h-60" fallback={<ChartFallback valueLabel={t('metricsUI.input')} rows={list(overview.llmSeries).map((point) => ({ label: shortDate(point.at), value: point.value }))} />} /><div className="mt-3 flex flex-wrap gap-3 text-[11px] text-zinc-500"><span>{t('metricsUI.input')} {overview.promptTokens.available ? compact(overview.promptTokens.value) : t('metricsUI.notReported')}</span><span>{t('metricsUI.output')} {overview.completionTokens.available ? compact(overview.completionTokens.value) : t('metricsUI.notReported')}</span>{overview.tokensUnavailable ? <span>{t('metricsUI.usageMissing', { count: overview.tokensUnavailable })}</span> : null}</div></article>
          </div>

          <article className="surface min-w-0 rounded-xl p-5"><div className="mb-4"><h2 className="text-sm font-medium text-zinc-200">{t('metricsUI.pipelineHealth')}</h2><p className="mt-1 text-xs text-zinc-500">{t('metricsUI.healthDescription', { granularity: overview.granularity })}</p></div>{healthRows.length ? <><MetricsChart option={healthOption} ariaLabel={t('metricsUI.pipelineHealth')} className="h-72" fallback={<ChartFallback valueLabel={t('metricsUI.completed')} rows={healthRows.map((item) => ({ label: `${item.name} · ${shortDate(item.at)}`, value: item.completed }))} />} /><div className="mt-4 flex flex-wrap gap-2">{[...new Map(healthRows.map((item) => [item.pipelineId, item.name])).entries()].map(([id, name]) => <button key={id} type="button" aria-pressed={pipelineID === id} onClick={() => setPipelineID(pipelineID === id ? '' : id)} className={cn('rounded-md border px-2.5 py-1.5 text-xs transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500', pipelineID === id ? 'border-orange-500/50 bg-orange-500/10 text-orange-200' : 'border-zinc-800 text-zinc-500 hover:bg-zinc-900 hover:text-zinc-300')}>{name}</button>)}</div></> : <p className="py-12 text-center text-xs text-zinc-600">{t('metricsUI.healthEmpty')}</p>}</article>

          <div className="grid gap-5 xl:grid-cols-4">
            <BreakdownTable title={t('metricsUI.needsAttention')} items={list(overview.failures)} empty={t('metricsUI.noFailures')} primaryLabel={t('metricsUI.failures')} onOpen={() => setBreakdown({ title: t('metricsUI.needsAttention'), items: list(overview.failures), primaryLabel: t('metricsUI.failures'), formatValue: compact })} />
            <BreakdownTable title={t('metricsUI.slowestNodes')} items={list(overview.slowNodes)} empty={t('metricsUI.noTiming')} primaryLabel={t('metricsUI.averageTime')} formatValue={duration} onOpen={() => setBreakdown({ title: t('metricsUI.slowestNodes'), items: list(overview.slowNodes), primaryLabel: t('metricsUI.averageTime'), formatValue: duration })} />
            <BreakdownTable title={t('metricsUI.providerActivity')} items={list(overview.models)} empty={t('metricsUI.noModels')} primaryLabel={t('metricsUI.calls')} onOpen={() => setBreakdown({ title: t('metricsUI.providerActivity'), items: list(overview.models), primaryLabel: t('metricsUI.calls'), formatValue: compact })} />
            <BreakdownTable title={t('metricsUI.triggerActivity')} items={[...list(overview.triggers), ...list(overview.activity)]} empty={t('metricsUI.noActivity')} primaryLabel={t('metricsUI.count')} onOpen={() => setBreakdown({ title: t('metricsUI.triggerActivity'), items: [...list(overview.triggers), ...list(overview.activity)], primaryLabel: t('metricsUI.count'), formatValue: compact })} />
          </div>

          <div className="grid gap-5 xl:grid-cols-[minmax(0,1.65fr)_minmax(19rem,.75fr)]">
            <article className="surface min-w-0 rounded-xl p-5"><div className="mb-4"><h2 className="text-sm font-medium text-zinc-200">{t('metricsUI.runtimeHealth')}</h2><p className="mt-1 text-xs text-zinc-500">{t('metricsUI.runtimeDescription')}</p></div>{list(overview.resources).length ? <MetricsChart option={resourceOption} ariaLabel={t('metricsUI.runtimeHealth')} className="h-56" fallback={<ChartFallback valueLabel="CPU %" rows={list(overview.resources).map((item) => ({ label: `${item.process} · ${shortDate(item.at)}`, value: item.cpuPercent }))} />} /> : <p className="flex h-56 items-center justify-center text-xs text-zinc-600">{t('metricsUI.noSamples')}</p>}</article>
            <article className="surface min-w-0 rounded-xl p-5"><div className="flex items-center gap-2"><Cpu className="size-4 text-zinc-500" /><h2 className="text-sm font-medium text-zinc-200">{t('metricsUI.latestSample')}</h2></div><dl className="mt-5 space-y-5"><div><dt className="text-xs text-zinc-500">Neuropipe</dt><dd className="mt-1 font-mono text-sm text-zinc-200">{resourceSummary.app ? `${resourceSummary.app.cpuPercent.toFixed(1)}% · ${memory(resourceSummary.app.workingSet)}` : t('metricsUI.unavailable')}</dd></div><div><dt className="text-xs text-zinc-500">Managed llama.cpp</dt><dd className="mt-1 font-mono text-sm text-zinc-200">{resourceSummary.llama ? `${resourceSummary.llama.cpuPercent.toFixed(1)}% · ${memory(resourceSummary.llama.workingSet)}` : t('metricsUI.notRunning')}</dd></div></dl><div className="mt-8 flex items-center gap-2 border-t border-zinc-800 pt-4 text-[11px] leading-5 text-zinc-600"><Server className="size-3.5 shrink-0" />{t('metricsUI.privacy')}</div></article>
          </div>

          <p className="px-1 text-center text-[11px] leading-5 text-zinc-600">{t('metricsUI.retention')} {overview.localCalls ? `${t('metricsUI.localBilling', { count: overview.localCalls })} ` : ''}{t('metricsUI.costNote')}</p>
        </> : null}
        {breakdown ? <BreakdownDialog state={breakdown} onClose={() => setBreakdown(undefined)} /> : null}
      </div>
    </div>
  </section>
}
