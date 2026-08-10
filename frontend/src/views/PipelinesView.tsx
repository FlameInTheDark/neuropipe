import { useMemo, useState } from 'react'
import { AlertTriangle, ArrowRight, Copy, Loader2, MoreHorizontal, Plus, Search, Trash2, Workflow } from 'lucide-react'
import { ContextMenu, contextMenuPointFromElement, contextMenuPosition, type ContextMenuPoint, type ContextMenuPosition } from '@/components/ContextMenu'
import { EmptyState } from '@/components/EmptyState'
import { LucideIcon } from '@/components/LucideIconPicker'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tooltip } from '@/components/ui/tooltip'
import { desktop } from '@/lib/bridge'
import { formatDate } from '@/lib/utils'
import type { PipelineSummary } from '@/lib/types'
import { useConfirmationStore } from '@/stores/confirmation'
import { useUIStore } from '@/stores/ui'
import { useTranslation } from 'react-i18next'

interface PipelineMenu {
  pipeline: PipelineSummary
  position: ContextMenuPosition
}

export function PipelinesView({ pipelines, onRefresh }: { pipelines: PipelineSummary[]; onRefresh: () => Promise<void> }) {
  const { t } = useTranslation()
  const { setScreen, setError } = useUIStore()
  const [query, setQuery] = useState('')
  const [creating, setCreating] = useState(false)
  const [name, setName] = useState('')
  const [menu, setMenu] = useState<PipelineMenu>()
  const [actionID, setActionID] = useState('')
  const requestConfirmation = useConfirmationStore((state) => state.ask)
  const filtered = useMemo(() => pipelines.filter((pipeline) => `${pipeline.name} ${pipeline.description}`.toLowerCase().includes(query.toLowerCase())), [pipelines, query])

  const create = async () => {
    try {
      setCreating(true)
      const pipeline = await desktop.createPipeline(name)
      await onRefresh()
      setScreen('editor', pipeline.id)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t('pipelines.createFailed'))
    } finally {
      setCreating(false)
    }
  }

  const openMenu = (point: ContextMenuPoint, pipeline: PipelineSummary) => {
    setMenu({
      pipeline,
      position: contextMenuPosition(point, { width: 192, height: 128 }),
    })
  }

  const duplicate = async (pipeline: PipelineSummary) => {
    try {
      setActionID(pipeline.id)
      const copy = await desktop.duplicatePipeline(pipeline.id)
      setMenu(undefined)
      await onRefresh()
      setScreen('editor', copy.id)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t('pipelines.duplicateFailed'))
    } finally {
      setActionID('')
    }
  }

  const remove = async (pipeline: PipelineSummary) => {
    const confirmed = await requestConfirmation({
      title: t('pipelines.deleteTitle'),
      description: t('pipelines.deleteDescription', { name: pipeline.name }),
      confirmLabel: t('pipelines.deleteConfirm'),
    })
    if (!confirmed) return
    try {
      setActionID(pipeline.id)
      await desktop.deletePipeline(pipeline.id)
      await onRefresh()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t('pipelines.deleteFailed'))
    } finally {
      setActionID('')
    }
  }

  return <section className="flex h-full min-h-0 flex-col">
    <PageHeader
      title={t('pipelines.title')}
      description={t('pipelines.description')}
      actions={<Button onClick={() => void create()} disabled={creating}>{creating ? <Loader2 className="size-4 animate-spin" /> : <Plus className="size-4" />}{t('pipelines.new')}</Button>}
    />
    <div className="muted-scroll min-h-0 flex-1 overflow-y-auto p-8">
      <div className="mb-5 flex max-w-xl items-center gap-3">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-2.5 top-2 size-4 text-zinc-600" />
          <Input value={query} onChange={(event) => setQuery(event.target.value)} className="pl-8" placeholder={t('pipelines.search')} />
        </div>
        <Input value={name} onChange={(event) => setName(event.target.value)} className="max-w-52" placeholder={t('pipelines.name')} />
      </div>
      {pipelines.length === 0 ? <EmptyState icon={Workflow} title={t('pipelines.emptyTitle')} description={t('pipelines.emptyDescription')} action={{ label: t('pipelines.new'), onClick: () => void create() }} /> : <div className="overflow-hidden rounded-xl border border-zinc-800">
        <div className="grid grid-cols-[minmax(0,1fr)_130px_110px_42px] border-b border-zinc-800 bg-zinc-900/50 px-4 py-2 text-xs font-medium text-zinc-500">
          <span>{t('pipelines.pipeline')}</span><span>{t('pipelines.status')}</span><span>{t('pipelines.updated')}</span><span />
        </div>
        {filtered.map((pipeline) => <div
          key={pipeline.id}
          onContextMenu={(event) => { event.preventDefault(); openMenu(event, pipeline) }}
          className="group grid grid-cols-[minmax(0,1fr)_130px_110px_42px] items-center border-b border-zinc-800 px-4 py-4 transition-colors last:border-0 hover:bg-zinc-900"
        >
          <button
            type="button"
            onClick={() => setScreen('editor', pipeline.id)}
            onKeyDown={(event) => {
              if (event.key !== 'ContextMenu' && !(event.shiftKey && event.key === 'F10')) return
              event.preventDefault()
              openMenu(contextMenuPointFromElement(event.currentTarget), pipeline)
            }}
            className="flex min-w-0 items-center gap-3 rounded text-left outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"
          >
            <span className="flex size-8 shrink-0 items-center justify-center rounded-md border border-zinc-800" style={{ color: pipeline.iconColor, backgroundColor: pipeline.iconBackground }}><LucideIcon name={pipeline.icon} className="size-4" /></span>
            <span className="min-w-0"><span className="block truncate text-sm font-medium text-zinc-100">{pipeline.name}</span>
            <span className="mt-1 block truncate text-xs text-zinc-500">{pipeline.description || t('pipelines.triggerCount', { count: pipeline.triggerCount })}</span>
            {pipeline.migrationIssue ? <Tooltip content={pipeline.migrationIssue} side="bottom" align="start"><span className="mt-1 flex items-center gap-1 text-[11px] text-amber-300"><AlertTriangle className="size-3" />{t('pipelines.migrationRequired')}</span></Tooltip> : null}</span>
          </button>
          <span><span className={pipeline.status === 'active' ? 'rounded bg-emerald-500/10 px-2 py-1 text-xs text-emerald-300' : pipeline.status === 'legacy' ? 'rounded bg-amber-500/10 px-2 py-1 text-xs text-amber-300' : 'rounded bg-zinc-800 px-2 py-1 text-xs text-zinc-400'}>{pipeline.status === 'active' ? t('pipelines.published', { version: pipeline.publishedRevision }) : pipeline.status === 'legacy' ? t('pipelines.legacy') : t('pipelines.draft')}</span></span>
          <span className="text-xs text-zinc-500">{formatDate(pipeline.updatedAt)}</span>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            className="size-7 p-0 text-zinc-500 hover:text-zinc-100 focus-visible:text-zinc-100"
            onClick={(event) => openMenu(contextMenuPointFromElement(event.currentTarget), pipeline)}
            aria-label={t('pipelines.options', { name: pipeline.name })}
            aria-haspopup="menu"
            aria-expanded={menu?.pipeline.id === pipeline.id}
          >
            <MoreHorizontal className="size-4" />
          </Button>
        </div>)}
      </div>}
      {pipelines.length > 0 && filtered.length === 0 && <div className="py-16 text-center text-sm text-zinc-500"><Copy className="mx-auto mb-3 size-5" />{t('pipelines.noMatches', { query })}</div>}
      {menu ? <ContextMenu position={menu.position} ariaLabel={t('pipelines.options', { name: menu.pipeline.name })} className="w-48" onClose={() => setMenu(undefined)}>
        <button role="menuitem" className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-xs text-zinc-200 hover:bg-zinc-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500" onClick={() => { setMenu(undefined); setScreen('editor', menu.pipeline.id) }}><ArrowRight className="size-3.5" />{t('pipelines.open')}</button>
        <button role="menuitem" disabled={actionID !== '' || menu.pipeline.status === 'legacy'} className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-xs text-zinc-200 hover:bg-zinc-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500 disabled:cursor-not-allowed disabled:opacity-40" onClick={() => void duplicate(menu.pipeline)}>{actionID === menu.pipeline.id ? <Loader2 className="size-3.5 animate-spin" /> : <Copy className="size-3.5" />}{t('pipelines.duplicate')}</button>
        <div className="my-1 border-t border-zinc-800" />
        <button role="menuitem" disabled={actionID !== ''} className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-xs text-red-300 hover:bg-red-500/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-400/50 disabled:cursor-not-allowed disabled:opacity-40" onClick={() => { void remove(menu.pipeline); setMenu(undefined) }}><Trash2 className="size-3.5" />{t('common.delete')}</button>
      </ContextMenu> : null}
    </div>
  </section>
}
