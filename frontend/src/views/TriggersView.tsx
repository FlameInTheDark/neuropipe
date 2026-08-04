import { useState } from 'react'
import { CircleCheck, Clock3, ExternalLink, Loader2, Play, Workflow } from 'lucide-react'
import { EmptyState } from '@/components/EmptyState'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { desktop } from '@/lib/bridge'
import { formatDate } from '@/lib/utils'
import type { TriggerBinding } from '@/lib/types'
import { useUIStore } from '@/stores/ui'
import { useTranslation } from 'react-i18next'

export function TriggersView({ buttons, onRefresh }: { buttons: TriggerBinding[]; onRefresh: () => Promise<void> }) {
  const { t } = useTranslation()
  const { setScreen, setError } = useUIStore()
  const [running, setRunning] = useState<string>()
  const run = async (binding: TriggerBinding) => {
    try {
      setRunning(binding.id)
      await desktop.runTrigger(binding.id)
      await onRefresh()
    } catch (reason) { setError(reason instanceof Error ? reason.message : t('triggers.runFailed')) } finally { setRunning(undefined) }
  }
  return <section className="flex h-full min-h-0 flex-col"><PageHeader eyebrow={t('triggers.eyebrow')} title={t('triggers.title')} description={t('triggers.description')} />
    <div className="muted-scroll min-h-0 flex-1 overflow-y-auto p-8">
      {buttons.length === 0 ? <EmptyState icon={Workflow} title={t('triggers.emptyTitle')} description={t('triggers.emptyDescription')} action={{ label: t('triggers.createPipeline'), onClick: () => setScreen('pipelines') }} /> : <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
        {buttons.map((binding) => <article key={binding.id} className="surface group relative min-h-44 rounded-xl p-4 transition-colors hover:border-zinc-600"><div className="flex items-start justify-between"><div className="flex size-9 items-center justify-center rounded-lg bg-zinc-900" style={{ color: binding.color || '#fff' }}><Play className="size-4 fill-current" /></div>{binding.lastRunStatus === 'completed' && <span className="flex items-center gap-1 text-xs text-emerald-400"><CircleCheck className="size-3.5" />{t('triggers.success')}</span>}</div><h2 className="mt-7 pr-8 text-sm font-medium">{binding.label}</h2><p className="mt-1 text-xs text-zinc-500">{binding.hotkey ? `${binding.hotkey} · ` : ''}{t('triggers.lastRun', { date: formatDate(binding.lastRunAt) })}</p><div className="mt-4 flex items-center gap-2"><Button size="sm" onClick={() => void run(binding)} disabled={running === binding.id}>{running === binding.id ? <Loader2 className="size-3.5 animate-spin" /> : <Play className="size-3.5" />}{t('triggers.run')}</Button><Button size="sm" variant="ghost" aria-label={t('triggers.open', { name: binding.label })} onClick={() => setScreen('editor', binding.pipelineId)}><ExternalLink className="size-3.5" /></Button></div></article>)}
      </div>}
      <p className="mt-7 flex items-center gap-2 text-xs text-zinc-600"><Clock3 className="size-3.5" />{t('triggers.boardNote')}</p>
    </div>
  </section>
}
