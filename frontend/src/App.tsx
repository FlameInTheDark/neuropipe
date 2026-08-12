import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react'
import { AlertCircle, Loader2 } from 'lucide-react'
import { Sidebar } from '@/components/Sidebar'
import { TitleBar } from '@/components/TitleBar'
import { Button } from '@/components/ui/button'
import { desktop } from '@/lib/bridge'
import type { FunctionSummary, GlobalVariableSummary, NodeDefinition, PipelineSummary, Report, Settings, TriggerBinding } from '@/lib/types'
import { useUIStore } from '@/stores/ui'
import i18n from '@/i18n'
import { localizeNodeDefinitions } from '@/i18n/node-catalog'
import { useTranslation } from 'react-i18next'
import { PipelinesView } from '@/views/PipelinesView'
import { SchedulesView } from '@/views/SchedulesView'
import { SettingsView } from '@/views/SettingsView'
import { TriggersView } from '@/views/TriggersView'
import { FunctionsView } from '@/views/FunctionsView'
import { FunctionEditor } from '@/views/FunctionEditor'
import { VariablesView } from '@/views/VariablesView'
import { DocumentationDialog } from '@/components/DocumentationWorkspace'
import { EventsOn } from '../wailsjs/runtime/runtime'

const PipelineEditor = lazy(() => import('@/views/PipelineEditor').then((module) => ({ default: module.PipelineEditor })))
const ReportsView = lazy(() => import('@/views/ReportsView').then((module) => ({ default: module.ReportsView })))
const TriggerButtonsView = lazy(() => import('@/views/TriggerButtonsView').then((module) => ({ default: module.TriggerButtonsView })))
const ChatView = lazy(() => import('@/views/ChatView').then((module) => ({ default: module.ChatView })))
const DocumentationView = lazy(() => import('@/views/DocumentationView').then((module) => ({ default: module.DocumentationView })))
const MetricsView = lazy(() => import('@/views/MetricsView').then((module) => ({ default: module.MetricsView })))

const workspaceLoadTimeoutMs = 20_000

function withTimeout<T>(promise: Promise<T>, milliseconds: number): Promise<T> {
  let timeout: number | undefined
  const deadline = new Promise<never>((_, reject) => {
    timeout = window.setTimeout(() => reject(new Error('Neuropipe could not load the local workspace. Try restarting the app.')), milliseconds)
  })
  return Promise.race([promise, deadline]).finally(() => {
    if (timeout !== undefined) window.clearTimeout(timeout)
  })
}

export function App() {
  const { screen, pipelineID, functionID, error, sidebarCollapsed, documentationDialog, closeDocumentation, setError, setScreen } = useUIStore()
  const { t } = useTranslation()
  const [pipelines, setPipelines] = useState<PipelineSummary[]>([])
  const [triggers, setTriggers] = useState<TriggerBinding[]>([])
  const [schedules, setSchedules] = useState<TriggerBinding[]>([])
  const [reports, setReports] = useState<Report[]>([])
  const [nodes, setNodes] = useState<NodeDefinition[]>([])
  const [functions, setFunctions] = useState<FunctionSummary[]>([])
  const [variables, setVariables] = useState<GlobalVariableSummary[]>([])
  const [settings, setSettings] = useState<Settings | null>(null)
  const [loading, setLoading] = useState(true)
  const reportRefreshSequence = useRef(0)

  const refresh = useCallback(async () => {
    try {
      setLoading(true)
      const [nextPipelines, nextTriggers, nextSchedules, nextReports, nextNodes, nextFunctions, nextVariables, nextSettings] = await withTimeout(Promise.all([desktop.listPipelines(), desktop.listTriggers(), desktop.listSchedules(), desktop.listReports(), desktop.listNodes(), desktop.listFunctions(), desktop.listGlobalVariables(), desktop.getSettings()]), workspaceLoadTimeoutMs)
      const language = nextSettings.language || 'en'
      if (i18n.resolvedLanguage !== language) await i18n.changeLanguage(language)
      setPipelines(nextPipelines); setTriggers(nextTriggers); setSchedules(nextSchedules); setReports(nextReports); setNodes(localizeNodeDefinitions(nextNodes, language)); setFunctions(nextFunctions); setVariables(nextVariables); setSettings(nextSettings)
      setError()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : i18n.t('app.unavailable'))
    } finally { setLoading(false) }
  }, [setError])

  const refreshReports = useCallback(async () => {
    const sequence = ++reportRefreshSequence.current
    try {
      const nextReports = await withTimeout(desktop.listReports(), workspaceLoadTimeoutMs)
      if (sequence !== reportRefreshSequence.current) return
      setReports(nextReports)
      setError()
    } catch (reason) {
      if (sequence !== reportRefreshSequence.current) return
      setError(reason instanceof Error ? reason.message : i18n.t('app.unavailable'))
    }
  }, [setError])

  const refreshVariables = useCallback(async () => {
    try {
      const nextVariables = await withTimeout(desktop.listGlobalVariables(), workspaceLoadTimeoutMs)
      setVariables(nextVariables)
      setError()
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : i18n.t('app.unavailable'))
    }
  }, [setError])

  useEffect(() => { void refresh() }, [refresh])

  useEffect(() => {
    const disableBrowserContextMenu = (event: MouseEvent) => event.preventDefault()
    document.addEventListener('contextmenu', disableBrowserContextMenu)
    return () => document.removeEventListener('contextmenu', disableBrowserContextMenu)
  }, [])

  useEffect(() => EventsOn('app.open.settings', () => setScreen('settings')), [setScreen])

  useEffect(() => {
    void desktop.configureTrayMenu({
      show: t('tray.show'),
      settings: t('tray.settings'),
      hide: t('tray.hide'),
      close: t('tray.close'),
    }).catch(() => undefined)
  }, [t])

  const content = () => {
    if (loading && screen !== 'editor') return <div className="flex h-full items-center justify-center text-sm text-zinc-500"><Loader2 className="mr-2 size-4 animate-spin" />{t('app.opening')}</div>
    switch (screen) {
      case 'pipelines': return <PipelinesView pipelines={pipelines} onRefresh={refresh} />
      case 'functions': return <FunctionsView functions={functions} onRefresh={refresh} />
      case 'variables': return <VariablesView variables={variables} onRefresh={refreshVariables} />
      case 'function-editor': return functionID && <FunctionEditor functionID={functionID} definitions={nodes} onRefresh={refresh} />
      case 'reports': return <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-zinc-500"><Loader2 className="mr-2 size-4 animate-spin" />{t('app.loadingReports')}</div>}><ReportsView reports={reports} onRefresh={refreshReports} /></Suspense>
      case 'metrics': return <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-zinc-500"><Loader2 className="mr-2 size-4 animate-spin" />{t('app.loadingMetrics')}</div>}><MetricsView pipelines={pipelines} /></Suspense>
      case 'schedules': return <SchedulesView schedules={schedules} onRefresh={refresh} />
      case 'settings': return settings && <SettingsView settings={settings} onSettingsChange={setSettings} onRefresh={refresh} />
      case 'button-board': return <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-zinc-500"><Loader2 className="mr-2 size-4 animate-spin" />{t('app.loadingBoard')}</div>}><TriggerButtonsView buttons={triggers.filter((trigger) => trigger.kind === 'button')} pipelines={pipelines} onRefresh={refresh} /></Suspense>
      case 'chat': return <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-zinc-500"><Loader2 className="mr-2 size-4 animate-spin" />{t('app.loadingChat')}</div>}><ChatView /></Suspense>
      case 'documentation': return <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-zinc-500"><Loader2 className="mr-2 size-4 animate-spin" />{t('app.loadingDocumentation')}</div>}><DocumentationView /></Suspense>
      case 'editor': return pipelineID && <Suspense fallback={<div className="flex h-full items-center justify-center text-sm text-zinc-500"><Loader2 className="mr-2 size-4 animate-spin" />{t('app.loadingEditor')}</div>}><PipelineEditor pipelineID={pipelineID} definitions={nodes} onRefresh={refresh} /></Suspense>
      default: return <TriggersView triggers={triggers} onRefresh={refresh} />
    }
  }

  return <div className={sidebarCollapsed ? "app-window app-window-sidebar-collapsed" : "app-window"}><TitleBar /><Sidebar /><main className="app-content min-w-0 min-h-0 overflow-hidden">{error && <div className="absolute right-5 top-14 z-50 flex max-w-md items-center gap-3 rounded-lg border border-red-500/30 bg-zinc-900 px-3 py-2 text-sm text-red-200 shadow-2xl"><AlertCircle className="size-4 shrink-0" /><span>{error}</span><Button size="sm" variant="ghost" onClick={() => setError()}>{t('app.dismiss')}</Button></div>}{content()}</main>{documentationDialog ? <DocumentationDialog documentID={documentationDialog.documentID} anchor={documentationDialog.anchor} onClose={closeDocumentation} /> : null}</div>
}
