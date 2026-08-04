import { useState } from 'react'
import { CalendarClock, CircleAlert, Loader2, ShieldCheck } from 'lucide-react'
import { EmptyState } from '@/components/EmptyState'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { desktop } from '@/lib/bridge'
import { formatDate } from '@/lib/utils'
import type { TriggerBinding } from '@/lib/types'
import { useUIStore } from '@/stores/ui'
import { useTranslation } from 'react-i18next'

export function SchedulesView({ schedules, onRefresh }: { schedules: TriggerBinding[]; onRefresh: () => Promise<void> }) {
  const { t } = useTranslation()
  const { setScreen, setError } = useUIStore()
  const [busy, setBusy] = useState<string>()
  const trust = async (schedule: TriggerBinding) => { try { setBusy(schedule.id); await desktop.trustRevision(schedule.pipelineId, schedule.revision); await onRefresh() } catch (reason) { setError(reason instanceof Error ? reason.message : t('schedules.trustFailed')) } finally { setBusy(undefined) } }
  const toggle = async (schedule: TriggerBinding, enabled: boolean) => { try { setBusy(schedule.id); await desktop.setScheduleEnabled(schedule.id, enabled); await onRefresh() } catch (reason) { setError(reason instanceof Error ? reason.message : t('schedules.updateFailed')) } finally { setBusy(undefined) } }
  return <section className="flex h-full min-h-0 flex-col"><PageHeader title={t('schedules.title')} description={t('schedules.description')} />
    <div className="muted-scroll min-h-0 flex-1 overflow-y-auto p-8">{schedules.length === 0 ? <EmptyState icon={CalendarClock} title={t('schedules.emptyTitle')} description={t('schedules.emptyDescription')} action={{ label: t('schedules.openPipelines'), onClick: () => setScreen('pipelines') }} /> : <div className="space-y-3">{schedules.map((schedule) => <article key={schedule.id} className="surface flex flex-col gap-4 rounded-xl p-4 lg:flex-row lg:items-center"><div className="flex min-w-0 flex-1 items-start gap-3"><div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg bg-zinc-900"><CalendarClock className="size-4 text-zinc-300" /></div><div className="min-w-0"><button className="truncate text-sm font-medium hover:underline" onClick={() => setScreen('editor', schedule.pipelineId)}>{schedule.label}</button><p className="mt-1 font-mono text-xs text-zinc-400">{schedule.cron} <span className="font-sans text-zinc-600">·</span> {schedule.timezone || t('schedules.localTimezone')}</p><p className="mt-2 text-xs text-zinc-600">{t('schedules.nextLast', { next: formatDate(schedule.nextRunAt), last: formatDate(schedule.lastRunAt) })}</p></div></div><div className="flex items-center gap-3 border-t border-zinc-800 pt-3 lg:border-l lg:border-t-0 lg:pl-4 lg:pt-0">{schedule.trusted ? <span className="flex items-center gap-1.5 text-xs text-emerald-300"><ShieldCheck className="size-4" />{t('schedules.trusted', { version: schedule.revision })}</span> : <Button size="sm" variant="outline" disabled={busy === schedule.id} onClick={() => void trust(schedule)}>{busy === schedule.id ? <Loader2 className="size-3.5 animate-spin" /> : <CircleAlert className="size-3.5" />}{t('schedules.trust')}</Button>}<Switch label={t('schedules.enable', { name: schedule.label })} checked={schedule.enabled} disabled={!schedule.trusted || busy === schedule.id} onCheckedChange={(enabled) => void toggle(schedule, enabled)} /></div></article>)}</div>}</div>
  </section>
}
