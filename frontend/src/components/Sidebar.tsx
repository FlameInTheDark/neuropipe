import { BarChart3, BookOpen, Braces, Clock3, FileText, LayoutGrid, MessageCircle, PanelLeft, Settings2, Workflow } from 'lucide-react'
import { Tooltip } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'
import { type Screen, useUIStore } from '@/stores/ui'
import { useTranslation } from 'react-i18next'

const navigation: { id: Exclude<Screen, 'editor' | 'function-editor'>; labelKey: string; icon: typeof PanelLeft }[] = [
  { id: 'button-board', labelKey: 'nav.buttonBoard', icon: LayoutGrid },
  { id: 'chat', labelKey: 'nav.chat', icon: MessageCircle },
  { id: 'triggers', labelKey: 'nav.triggers', icon: PanelLeft },
  { id: 'pipelines', labelKey: 'nav.pipelines', icon: Workflow },
  { id: 'functions', labelKey: 'nav.functions', icon: Braces },
  { id: 'reports', labelKey: 'nav.reports', icon: FileText },
  { id: 'metrics', labelKey: 'nav.metrics', icon: BarChart3 },
  { id: 'documentation', labelKey: 'nav.documentation', icon: BookOpen },
  { id: 'schedules', labelKey: 'nav.schedules', icon: Clock3 },
  { id: 'settings', labelKey: 'nav.settings', icon: Settings2 },
]

export function Sidebar() {
  const { screen, setScreen, sidebarCollapsed } = useUIStore()
  const { t } = useTranslation()
  return <aside className={cn('app-sidebar flex min-h-0 flex-col border-r border-zinc-800 bg-zinc-950/70 py-4 transition-[padding] duration-150', sidebarCollapsed ? 'px-2' : 'px-3')}>
    <nav className="space-y-1">
      {navigation.map(({ id, labelKey, icon: Icon }) => {
        const label = t(labelKey)
        const button = <button type="button" onClick={() => setScreen(id)} aria-label={sidebarCollapsed ? label : undefined} className={cn('flex h-9 w-full items-center rounded-md text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500', sidebarCollapsed ? 'justify-center px-0' : 'gap-3 px-3', screen === id ? 'bg-zinc-800 text-zinc-50' : 'text-zinc-500 hover:bg-zinc-900 hover:text-zinc-200')}><Icon className="size-4 shrink-0" />{sidebarCollapsed ? null : <span className="truncate">{label}</span>}</button>
        return sidebarCollapsed
          ? <span key={id} className="block w-full"><Tooltip content={label} side="right" triggerClassName="flex w-full" className="whitespace-nowrap">{button}</Tooltip></span>
          : <span key={id}>{button}</span>
      })}
    </nav>
  </aside>
}
