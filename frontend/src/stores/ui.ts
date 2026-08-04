import { create } from 'zustand'

export type Screen = 'button-board' | 'chat' | 'triggers' | 'pipelines' | 'functions' | 'function-editor' | 'reports' | 'metrics' | 'schedules' | 'settings' | 'documentation' | 'editor'

const persistedScreenKey = 'neuropipe.screen.v1'
const persistedSidebarCollapsedKey = 'neuropipe.sidebar.collapsed.v1'
const persistentScreens: readonly Screen[] = ['button-board', 'chat', 'triggers', 'pipelines', 'functions', 'reports', 'metrics', 'schedules', 'settings', 'documentation']

function initialScreen(): Screen {
  try {
    const stored = window.localStorage.getItem(persistedScreenKey)
    return persistentScreens.includes(stored as Screen) ? stored as Screen : 'triggers'
  } catch {
    return 'triggers'
  }
}

function persistScreen(screen: Screen) {
  if (!persistentScreens.includes(screen)) return
  try { window.localStorage.setItem(persistedScreenKey, screen) } catch { /* Optional local preference. */ }
}

function initialSidebarCollapsed(): boolean {
  try { return window.localStorage.getItem(persistedSidebarCollapsedKey) === 'true' } catch { return false }
}

function persistSidebarCollapsed(collapsed: boolean) {
  try { window.localStorage.setItem(persistedSidebarCollapsedKey, String(collapsed)) } catch { /* Optional local preference. */ }
}

interface UIState {
  screen: Screen
  pipelineID?: string
  functionID?: string
  error?: string
  documentationDialog?: { documentID: string; anchor?: string }
  sidebarCollapsed: boolean
  setScreen: (screen: Screen, resourceID?: string) => void
  setError: (error?: string) => void
  toggleSidebar: () => void
  openDocumentation: (documentID: string, anchor?: string) => void
  closeDocumentation: () => void
}

export const useUIStore = create<UIState>((set) => ({
  screen: initialScreen(),
  sidebarCollapsed: initialSidebarCollapsed(),
  setScreen: (screen, resourceID) => {
    persistScreen(screen)
    set({ screen, pipelineID: screen === 'editor' ? resourceID : undefined, functionID: screen === 'function-editor' ? resourceID : undefined })
  },
  setError: (error) => set({ error }),
  toggleSidebar: () => set((current) => {
    const sidebarCollapsed = !current.sidebarCollapsed
    persistSidebarCollapsed(sidebarCollapsed)
    return { sidebarCollapsed }
  }),
  openDocumentation: (documentID, anchor) => set({ documentationDialog: { documentID, anchor } }),
  closeDocumentation: () => set({ documentationDialog: undefined }),
}))
