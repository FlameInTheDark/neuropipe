import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Bot, ChevronDown, ChevronRight, CircleStop, Clock3, Copy, Loader2, MessageCircle, Pencil, Plus, Send, ShieldAlert, Trash2, User, Workflow } from 'lucide-react'
import { Events } from '@wailsio/runtime'
import { LucideIcon } from '@/components/LucideIconPicker'
import { MarkdownContent } from '@/components/MarkdownContent'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/EmptyState'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { desktop } from '@/lib/bridge'
import type { ChatApproval, ChatConversation, ChatMessage, ChatMode, ChatPipeline, ChatRun, ChatRunEvent } from '@/lib/types'
import { cn, formatDate } from '@/lib/utils'
import { useConfirmationStore } from '@/stores/confirmation'
import { useUIStore } from '@/stores/ui'
import { useTranslation } from 'react-i18next'

interface ActivityProps {
  run: ChatRun
  events: ChatRunEvent[]
  onOpenPipeline: () => void
}

function ActivityDisclosure({ run, events, onOpenPipeline }: ActivityProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [selectedID, setSelectedID] = useState<string>()
  const selected = events.find((event) => event.id === selectedID)
  const active = run.status === 'pending' || run.status === 'running'
  const summary = events.length > 1 ? t('chat.ranTools', { count: events.length }) : events[0]?.summary ?? run.statusText
  return <div className="my-2 overflow-hidden rounded-md border border-zinc-800 bg-zinc-950/45">
    <button type="button" onClick={() => setOpen((current) => !current)} className="flex h-8 w-full items-center gap-2 px-2.5 text-left text-xs text-zinc-400 transition-colors hover:bg-zinc-900 hover:text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500">
      {active ? <Loader2 className="size-3.5 shrink-0 animate-spin text-zinc-500" /> : <span className="flex size-3.5 shrink-0 items-center justify-center rounded-sm border border-zinc-700 font-mono text-[8px] text-zinc-500">›_</span>}
      <span className="min-w-0 flex-1 truncate">{active ? run.statusText : summary}</span>
      {open ? <ChevronDown className="size-3.5 shrink-0" /> : <ChevronRight className="size-3.5 shrink-0" />}
    </button>
    {open ? <div className="border-t border-zinc-800 bg-zinc-950/60 p-1.5">
      {events.length === 0 ? <p className="px-2 py-1.5 text-xs text-zinc-600">{active ? t('chat.waiting') : run.error || t('chat.noActivity')}</p> : events.map((event) => <button key={event.id} type="button" onClick={() => setSelectedID(event.id)} className={cn('flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-xs transition-colors hover:bg-zinc-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-600', selectedID === event.id && 'bg-zinc-900 text-zinc-100')}>
        <span className={cn('size-1.5 shrink-0 rounded-full', event.status === 'failed' ? 'bg-red-400' : event.status === 'running' || event.status === 'pending' ? 'bg-amber-300' : 'bg-zinc-600')} />
        <span className="min-w-0 flex-1 truncate text-zinc-400">{event.summary}</span><ChevronRight className="size-3 shrink-0 text-zinc-600" />
      </button>)}
      {selected ? <div className="mt-1 overflow-hidden rounded border border-zinc-800 bg-zinc-900/70"><div className="flex items-center justify-between border-b border-zinc-800 px-2.5 py-1.5"><span className="text-[10px] font-medium text-zinc-400">{selected.summary}</span>{run.executionId ? <button type="button" onClick={onOpenPipeline} className="text-[10px] text-zinc-500 hover:text-zinc-200">{t('chat.openRun')}</button> : null}</div><pre className="muted-scroll max-h-40 overflow-auto whitespace-pre-wrap break-words p-2.5 font-mono text-[11px] leading-5 text-zinc-400">{selected.detail || t('chat.completed')}</pre></div> : null}
    </div> : null}
  </div>
}

function ApprovalDialog({ approval, onResolve }: { approval: ChatApproval; onResolve: (approved: boolean) => void }) {
  const { t } = useTranslation()
  return <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/65 p-5"><section role="dialog" aria-modal="true" aria-label={t('chat.approval')} className="w-full max-w-md rounded-xl border border-zinc-700 bg-zinc-950 p-5 shadow-2xl"><div className="flex gap-3"><div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-amber-400/10 text-amber-300"><ShieldAlert className="size-4" /></div><div><h2 className="text-sm font-semibold text-zinc-100">{t('chat.approveTitle')}</h2><p className="mt-1 text-xs leading-5 text-zinc-500">{t('chat.modelWants', { action: approval.toolCall.name.replaceAll('_', ' ') })}</p></div></div><pre className="muted-scroll mt-4 max-h-44 overflow-auto rounded-lg border border-zinc-800 bg-zinc-900/60 p-3 font-mono text-[11px] leading-5 text-zinc-400">{JSON.stringify(approval.toolCall.arguments, null, 2)}</pre><div className="mt-5 flex justify-end gap-2"><Button variant="ghost" onClick={() => onResolve(false)}>{t('chat.deny')}</Button><Button onClick={() => onResolve(true)}>{t('chat.allow')}</Button></div></section></div>
}

// TypingIndicator mirrors the assistant message bubble (avatar + bubble) but
// renders three animated dots in place of the content. It is shown when an
// active run exists but no assistant message for that run has streamed yet.
function TypingIndicator() {
  return <div className="mb-5 flex gap-3">
    <div className="flex size-6 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-fuchsia-500 to-violet-500 text-white"><Bot className="size-3.5" /></div>
    <div className="rounded-xl border border-zinc-800 bg-zinc-900">
      <div className="flex items-center gap-1 px-4 py-3">
        <span className="size-1.5 animate-bounce rounded-full bg-zinc-500" style={{ animationDelay: '0ms' }} />
        <span className="size-1.5 animate-bounce rounded-full bg-zinc-500" style={{ animationDelay: '150ms' }} />
        <span className="size-1.5 animate-bounce rounded-full bg-zinc-500" style={{ animationDelay: '300ms' }} />
      </div>
    </div>
  </div>
}

interface ConversationItemProps {
  conversation: ChatConversation
  selected: boolean
  renaming: boolean
  renameValue: string
  onClick: () => void
  onStartRename: () => void
  onRenameChange: (value: string) => void
  onRenameCommit: () => void
  onRenameCancel: () => void
}

function ConversationItem({ conversation, selected, renaming, renameValue, onClick, onStartRename, onRenameChange, onRenameCommit, onRenameCancel }: ConversationItemProps) {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement>(null)
  useEffect(() => {
    if (renaming) {
      // Defer one frame so the input is mounted before we focus/select.
      const id = requestAnimationFrame(() => { inputRef.current?.focus(); inputRef.current?.select() })
      return () => cancelAnimationFrame(id)
    }
  }, [renaming])
  return <div className={cn('group relative flex items-center gap-2 rounded-md transition-colors', selected ? 'bg-zinc-800 border-l-2 border-zinc-300 -ml-px' : 'hover:bg-zinc-900')}>
    {renaming ? <input ref={inputRef} value={renameValue} onChange={(event) => onRenameChange(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); onRenameCommit() } else if (event.key === 'Escape') { event.preventDefault(); onRenameCancel() } }} onBlur={() => onRenameCommit()} aria-label={t('chat.renameInline')} className="m-1.5 min-w-0 flex-1 rounded border border-zinc-600 bg-zinc-950 px-2 py-1 text-xs text-zinc-100 outline-none focus:border-zinc-500 focus:ring-1 focus:ring-zinc-500/30" /> : <>
      <button type="button" onClick={onClick} className="flex min-w-0 flex-1 items-center gap-2 px-2.5 py-2 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"><span className={cn('flex size-5 shrink-0 items-center justify-center', conversation.mode === 'model' ? 'text-fuchsia-300' : 'text-violet-300')}>{conversation.mode === 'model' ? <Bot className="size-3.5" /> : <MessageCircle className="size-3.5" />}</span><span className="min-w-0"><span className="block truncate text-xs font-medium text-zinc-300">{conversation.title}</span><span className="block truncate pt-0.5 text-[10px] text-zinc-600">{formatDate(conversation.updatedAt)}</span></span></button>
      <button type="button" onClick={onStartRename} className="invisible flex size-6 items-center justify-center rounded text-zinc-600 hover:bg-zinc-700 hover:text-zinc-200 group-hover:visible focus-visible:visible focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500" aria-label={t('chat.renameInline')}><Pencil className="size-3.5" /></button>
    </>}
  </div>
}

type DateGroupKey = 'today' | 'yesterday' | 'previous7Days' | 'older'

function dateGroupKey(updatedAt: string): DateGroupKey {
  const now = Date.now()
  const startOfToday = new Date(); startOfToday.setHours(0, 0, 0, 0)
  const startOfTodayMs = startOfToday.getTime()
  const oneDay = 24 * 60 * 60 * 1000
  const startOfYesterday = startOfTodayMs - oneDay
  const start7DaysAgo = startOfTodayMs - 7 * oneDay
  const ts = new Date(updatedAt).getTime()
  if (Number.isNaN(ts) || ts > now) return 'today'
  if (ts >= startOfTodayMs) return 'today'
  if (ts >= startOfYesterday) return 'yesterday'
  if (ts >= start7DaysAgo) return 'previous7Days'
  return 'older'
}

export function ChatView() {
  const { setError, setScreen } = useUIStore()
  const { t } = useTranslation()
  const askConfirmation = useConfirmationStore((state) => state.ask)
  const [mode, setMode] = useState<ChatMode>('model')
  const [conversations, setConversations] = useState<ChatConversation[]>([])
  const [pipelines, setPipelines] = useState<ChatPipeline[]>([])
  const [selectedID, setSelectedID] = useState<string>()
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [runs, setRuns] = useState<ChatRun[]>([])
  const [events, setEvents] = useState<Record<string, ChatRunEvent[]>>({})
  const [approvals, setApprovals] = useState<ChatApproval[]>([])
  const [modelLabel, setModelLabel] = useState(t('chat.activeModel'))
  const [draft, setDraft] = useState('')
  const [search, setSearch] = useState('')
  const [loading, setLoading] = useState(true)
  const [sending, setSending] = useState(false)
  const [renamingID, setRenamingID] = useState<string | null>(null)
  const [renameValue, setRenameValue] = useState('')
  const [isAtBottom, setIsAtBottom] = useState(true)
  const feedRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const isAtBottomRef = useRef(true)
  const pendingScrollToBottomRef = useRef(true)
  const initialFocusDoneRef = useRef<string | undefined>(undefined)

  const selected = conversations.find((item) => item.id === selectedID)
  const activeRun = runs.find((run) => run.status === 'pending' || run.status === 'running')
  const searchQuery = search.trim().toLowerCase()
  const visibleConversations = useMemo(() => conversations.filter((item) => {
    if (item.mode !== mode) return false
    if (mode === 'model' && searchQuery && !item.title.toLowerCase().includes(searchQuery)) return false
    return true
  }), [conversations, mode, searchQuery])
  const visiblePipelines = useMemo(() => {
    const query = search.trim().toLowerCase()
    return pipelines.filter((item) => !query || `${item.pipelineName} ${item.label}`.toLowerCase().includes(query))
  }, [pipelines, search])
  const groupedConversations = useMemo(() => {
    const groups: Record<DateGroupKey, ChatConversation[]> = { today: [], yesterday: [], previous7Days: [], older: [] }
    for (const conv of visibleConversations) groups[dateGroupKey(conv.updatedAt)].push(conv)
    return (Object.keys(groups) as DateGroupKey[]).map((key) => ({ key, items: groups[key] })).filter((group) => group.items.length > 0)
  }, [visibleConversations])
  const suggestions = useMemo(() => [t('chat.suggestion1'), t('chat.suggestion2'), t('chat.suggestion3'), t('chat.suggestion4')], [t])
  const showTyping = activeRun ? !messages.some((m) => m.role === 'assistant' && m.chatRunId === activeRun.id) : false

  const loadDetails = useCallback(async (conversationID: string) => {
    const [nextMessages, nextRuns, nextApprovals] = await Promise.all([desktop.listChatMessages(conversationID), desktop.listChatRuns(conversationID), desktop.listPendingChatApprovals(conversationID)])
    const entries = await Promise.all(nextRuns.map(async (run) => [run.id, await desktop.listChatRunEvents(run.id)] as const))
    setMessages(nextMessages ?? []); setRuns(nextRuns ?? []); setApprovals(nextApprovals ?? []); setEvents(Object.fromEntries(entries))
  }, [])
  const load = useCallback(async () => {
    try {
      setLoading(true)
      const [nextConversations, nextPipelines, settings] = await Promise.all([desktop.listChatConversations(), desktop.listChatPipelines(), desktop.getSettings()])
      setConversations(nextConversations ?? []); setPipelines(nextPipelines ?? [])
      const provider = settings.providers.find((item) => item.id === settings.defaultProviderId)
      setModelLabel(provider ? `${provider.name}${provider.model ? ` · ${provider.model}` : ''}` : t('chat.configureModel'))
      const current = nextConversations.some((item) => item.id === selectedID) ? selectedID : nextConversations.find((item) => item.mode === mode)?.id
      setSelectedID(current)
      if (current) await loadDetails(current)
      else { setMessages([]); setRuns([]); setEvents({}); setApprovals([]) }
    } catch (reason) { setError(reason instanceof Error ? reason.message : t('chat.loadFailed')) } finally { setLoading(false) }
  }, [loadDetails, mode, selectedID, setError, t])

  useEffect(() => { void load() }, []) // Initial data fetch is deliberately independent from view filtering.
  useEffect(() => {
    const refresh = () => { if (selectedID) void loadDetails(selectedID); void desktop.listChatConversations().then(setConversations).catch(() => undefined) }
    const stopChat = Events.On('chat.updated', refresh)
    const stopRun = Events.On('chat.run.updated', refresh)
    const stopApproval = Events.On('chat.approval.requested', refresh)
    return () => { stopChat(); stopRun(); stopApproval() }
  }, [loadDetails, selectedID])
  useEffect(() => {
    if (selected?.mode === mode) return
    const next = conversations.find((item) => item.mode === mode)
    setSelectedID(next?.id)
    if (next) void loadDetails(next.id)
    else { setMessages([]); setRuns([]); setEvents({}); setApprovals([]) }
  }, [conversations, loadDetails, mode, selected])
  // Auto-resize textarea to fit content up to ~6 rows (max-h-40 = 10rem = 160px).
  useEffect(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 160)}px`
  }, [draft])
  // On conversation switch: focus the textarea and mark a pending scroll-to-bottom.
  useEffect(() => {
    if (initialFocusDoneRef.current === selectedID) return
    initialFocusDoneRef.current = selectedID
    textareaRef.current?.focus()
    pendingScrollToBottomRef.current = true
  }, [selectedID])
  // Track whether the message feed is scrolled to the bottom; used to decide
  // whether new content should auto-scroll or wait for the user to opt in.
  const handleFeedScroll = useCallback(() => {
    const el = feedRef.current
    if (!el) return
    const threshold = 32
    const next = el.scrollHeight - el.scrollTop - el.clientHeight < threshold
    isAtBottomRef.current = next
    setIsAtBottom(next)
  }, [])
  useEffect(() => {
    const el = feedRef.current
    if (!el) return
    el.addEventListener('scroll', handleFeedScroll, { passive: true })
    return () => el.removeEventListener('scroll', handleFeedScroll)
  }, [handleFeedScroll])
  // Smart auto-scroll: only follow new content when the user is already at the
  // bottom, or when we just switched conversations (pendingScrollToBottom).
  useEffect(() => {
    const el = feedRef.current
    if (!el) return
    if (isAtBottomRef.current || pendingScrollToBottomRef.current) {
      el.scrollTo({ top: el.scrollHeight, behavior: pendingScrollToBottomRef.current ? 'auto' : 'smooth' })
      pendingScrollToBottomRef.current = false
    }
  }, [messages, runs])
  // Reset scroll state when leaving a conversation so the next one starts at the bottom.
  useEffect(() => {
    isAtBottomRef.current = true
    setIsAtBottom(true)
    pendingScrollToBottomRef.current = true
  }, [selectedID])

  const scrollToBottom = useCallback(() => {
    const el = feedRef.current
    if (!el) return
    el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
    isAtBottomRef.current = true
    setIsAtBottom(true)
  }, [])

  const create = async (nextMode: ChatMode, bindingID = '') => {
    try {
      const conversation = await desktop.createChatConversation(nextMode, bindingID)
      setConversations((items) => [conversation, ...items]); setMode(nextMode); setSelectedID(conversation.id); setMessages([]); setRuns([]); setEvents({}); setApprovals([])
    } catch (reason) { setError(reason instanceof Error ? reason.message : t('chat.createFailed')) }
  }
  const choose = async (conversation: ChatConversation) => { setSelectedID(conversation.id); setMode(conversation.mode); try { await loadDetails(conversation.id) } catch (reason) { setError(reason instanceof Error ? reason.message : t('chat.openFailed')) } }
  const send = async () => {
    if (!selected || !draft.trim() || sending) return
    try { setSending(true); await desktop.sendChatMessage(selected.id, draft); setDraft(''); await loadDetails(selected.id); await desktop.listChatConversations().then(setConversations) } catch (reason) { setError(reason instanceof Error ? reason.message : t('chat.sendFailed')) } finally { setSending(false) }
  }
  const stop = async () => {
    if (!activeRun || !selected) return
    try { await desktop.cancelChatRun(activeRun.id); await loadDetails(selected.id) } catch (reason) { setError(reason instanceof Error ? reason.message : t('chat.stopFailed')) }
  }
  const saveConversation = async (conversation: ChatConversation) => { try { const saved = await desktop.saveChatConversation(conversation); setConversations((items) => items.map((item) => item.id === saved.id ? saved : item)) } catch (reason) { setError(reason instanceof Error ? reason.message : t('chat.saveFailed')) } }
  const remove = async (conversation: ChatConversation) => { const confirmed = await askConfirmation({ title: t('chat.deleteTitle'), description: t('chat.deleteDescription', { name: conversation.title }), confirmLabel: t('chat.deleteConfirm') }); if (!confirmed) return; try { await desktop.deleteChatConversation(conversation.id); const remaining = conversations.filter((item) => item.id !== conversation.id); setConversations(remaining); if (selectedID === conversation.id) { const next = remaining.find((item) => item.mode === mode); setSelectedID(next?.id); if (next) await loadDetails(next.id); else { setMessages([]); setRuns([]); setEvents({}); setApprovals([]) } } } catch (reason) { setError(reason instanceof Error ? reason.message : t('chat.deleteFailed')) } }
  const resolveApproval = async (approval: ChatApproval, approved: boolean) => { try { await desktop.resolveChatApproval(approval.id, approved); if (selectedID) await loadDetails(selectedID) } catch (reason) { setError(reason instanceof Error ? reason.message : t('chat.resolveFailed')) } }
  const updatePolicy = async (value: string) => { if (!selected) return; await saveConversation({ ...selected, actionPolicy: value === 'always' ? 'always' : 'ask' }) }
  const openRun = async (run: ChatRun) => {
    try {
      if (run.executionId) { const execution = await desktop.getExecution(run.executionId); setScreen('editor', execution.pipelineId); return }
      if (selected?.pipelineId) setScreen('editor', selected.pipelineId)
    } catch (reason) { setError(reason instanceof Error ? reason.message : t('chat.openRunFailed')) }
  }
  const startRename = useCallback((conversation: ChatConversation) => {
    setRenamingID(conversation.id)
    setRenameValue(conversation.title)
  }, [])
  const commitRename = useCallback(async () => {
    const id = renamingID
    const next = renameValue.trim()
    setRenamingID(null)
    setRenameValue('')
    if (!id || !next) return
    const conversation = conversations.find((item) => item.id === id)
    if (!conversation || next === conversation.title) return
    await saveConversation({ ...conversation, title: next })
  }, [renamingID, renameValue, conversations])
  const cancelRename = useCallback(() => {
    setRenamingID(null)
    setRenameValue('')
  }, [])
  const seenRuns = new Set<string>()

  return <section className="flex h-full min-h-0 flex-col"><PageHeader title={t('chat.title')} description={t('chat.description')} actions={<>
    <div className="flex rounded-md border border-zinc-800 bg-zinc-950 p-0.5"><Button size="sm" variant={mode === 'model' ? 'secondary' : 'ghost'} onClick={() => setMode('model')}><Bot className="size-3.5" />{t('chat.model')}</Button><Button size="sm" variant={mode === 'pipeline' ? 'secondary' : 'ghost'} onClick={() => setMode('pipeline')}><Workflow className="size-3.5" />{t('chat.pipelines')}</Button></div>
    <Button size="sm" variant="ghost" className="size-7 px-0" onClick={() => { if (selected) startRename(selected) }} aria-label={t('chat.rename')} disabled={!selected}><Pencil className="size-3.5" /></Button>
    <Button size="sm" variant="ghost" className="size-7 px-0" onClick={() => { if (selected) void remove(selected) }} aria-label={t('chat.delete')} disabled={!selected}><Trash2 className="size-3.5 text-red-300" /></Button>
  </>} />
    <div className="grid min-h-0 flex-1 grid-cols-[15.5rem_minmax(0,1fr)]">
      <aside className="flex min-h-0 flex-col border-r border-zinc-800 bg-zinc-950/35 p-3">
        <div className={cn('mb-3 flex gap-2', mode === 'model' ? 'flex-col' : 'items-center')}>
          {mode === 'model' ? <>
            <Button size="sm" className="w-full" onClick={() => void create('model')}><Plus className="size-3.5" />{t('chat.new')}</Button>
            <Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t('common.search')} className="h-7 text-xs" />
          </> : <div className="relative flex-1"><Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t('chat.findPipeline')} className="h-7 text-xs" /></div>}
        </div>
        <div className="muted-scroll min-h-0 flex-1 overflow-y-auto">
          {mode === 'model' ? <>
            {groupedConversations.length === 0 ? <p className="px-2 py-4 text-xs leading-5 text-zinc-600">{search ? t('pipelines.noMatches', { query: search }) : t('chat.conversations')}</p> : groupedConversations.map((group) => <div key={group.key}>
              <div className="px-2 pt-3 pb-1.5 text-[10px] font-medium uppercase tracking-[.12em] text-zinc-600">{t(`chat.${group.key}`)}</div>
              {group.items.map((conversation) => <ConversationItem key={conversation.id} conversation={conversation} selected={conversation.id === selectedID} renaming={renamingID === conversation.id} renameValue={renamingID === conversation.id ? renameValue : conversation.title} onClick={() => void choose(conversation)} onStartRename={() => startRename(conversation)} onRenameChange={setRenameValue} onRenameCommit={() => void commitRename()} onRenameCancel={cancelRename} />)}
            </div>)}
          </> : <>
            <p className="px-2 pb-1.5 text-[10px] font-medium uppercase tracking-[.12em] text-zinc-600">{t('chat.chatPipelines')}</p>
            {visiblePipelines.map((pipeline) => <button key={pipeline.bindingId} type="button" onClick={() => void create('pipeline', pipeline.bindingId)} className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left transition-colors hover:bg-zinc-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"><span className="flex size-6 shrink-0 items-center justify-center rounded-md" style={{ color: pipeline.color || '#e4e4e7', backgroundColor: `${pipeline.color || '#27272a'}20` }}><LucideIcon name={pipeline.icon} className="size-3.5" /></span><span className="min-w-0"><span className="block truncate text-xs font-medium text-zinc-300">{pipeline.pipelineName}</span><span className="block truncate pt-0.5 text-[10px] text-zinc-600">{pipeline.label}</span></span></button>)}
            {visiblePipelines.length === 0 ? <p className="px-2 py-4 text-xs leading-5 text-zinc-600">{t('chat.noPipelines')}</p> : null}
            {groupedConversations.length > 0 ? <div className="mt-4 border-t border-zinc-800 pt-3">
              {groupedConversations.map((group) => <div key={group.key}>
                <div className="px-2 pt-3 pb-1.5 text-[10px] font-medium uppercase tracking-[.12em] text-zinc-600">{t(`chat.${group.key}`)}</div>
                {group.items.map((conversation) => <ConversationItem key={conversation.id} conversation={conversation} selected={conversation.id === selectedID} renaming={renamingID === conversation.id} renameValue={renamingID === conversation.id ? renameValue : conversation.title} onClick={() => void choose(conversation)} onStartRename={() => startRename(conversation)} onRenameChange={setRenameValue} onRenameCommit={() => void commitRename()} onRenameCancel={cancelRename} />)}
              </div>)}
            </div> : null}
          </>}
        </div>
      </aside>
      <main className="relative flex min-w-0 min-h-0 flex-col">{loading ? <div className="flex h-full items-center justify-center text-sm text-zinc-500"><Loader2 className="mr-2 size-4 animate-spin" />{t('chat.loading')}</div> : !selected ? <EmptyState variant="plain" icon={MessageCircle} title={mode === 'model' ? t('chat.startModel') : t('chat.choosePipeline')} description={mode === 'model' ? t('chat.startModelDescription') : t('chat.choosePipelineDescription')} action={mode === 'model' ? { label: t('chat.new'), onClick: () => void create('model') } : undefined} /> : <>
        <div ref={feedRef} className="muted-scroll relative min-h-0 flex-1 overflow-y-auto">
          <div className="mx-auto max-w-3xl px-6 py-7">{messages.length === 0 && !activeRun ? <div className="flex flex-col items-center gap-2 py-8">
            <p className="text-sm text-zinc-400">{t('chat.emptyPrompt')}</p>
            {suggestions.map((suggestion) => <button key={suggestion} type="button" onClick={() => { setDraft(suggestion); textareaRef.current?.focus() }} className="w-full max-w-md rounded-lg border border-zinc-800 bg-zinc-900/40 p-3 text-left text-sm text-zinc-300 transition-colors hover:border-zinc-700 hover:bg-zinc-900">{suggestion}</button>)}
          </div> : <>
            {messages.map((message) => {
              if (message.role === 'tool') return null
              const chatRunID = message.chatRunId
              const run = runs.find((item) => item.id === chatRunID)
              const toolCall = message.role === 'assistant' && (message.toolCalls?.length ?? 0) > 0
              const pipelineReply = selected.mode === 'pipeline' && message.role === 'assistant'
              const renderActivity = Boolean(run && chatRunID && !seenRuns.has(chatRunID) && (toolCall || pipelineReply))
              if (chatRunID && renderActivity) seenRuns.add(chatRunID)
              const activity = renderActivity && run ? <ActivityDisclosure run={run} events={events[run.id] ?? []} onOpenPipeline={() => void openRun(run)} /> : null
              const emptyToolCall = toolCall && !message.content.trim()
              if (emptyToolCall) return <div key={message.id}>{activity}</div>
              const isUser = message.role === 'user'
              const headerLabel = isUser ? 'You' : selected.mode === 'pipeline' ? selected.title : 'Neuropipe'
              const bubble = <article className="group mb-5 flex gap-3" title={formatDate(message.createdAt)}>
                <div className={cn('flex size-6 shrink-0 items-center justify-center rounded-full', isUser ? 'bg-zinc-300 text-zinc-900' : 'bg-gradient-to-br from-fuchsia-500 to-violet-500 text-white')} aria-hidden>{isUser ? <User className="size-3.5" /> : <Bot className="size-3.5" />}</div>
                <div className="flex min-w-0 flex-1 flex-col">
                  <div className={cn('max-w-[85%] rounded-xl px-4 py-3 text-sm leading-6', isUser ? 'bg-zinc-100 text-zinc-900' : 'border border-zinc-800 bg-zinc-900 text-zinc-300')}>
                    <div className="mb-1.5 flex items-center gap-1.5 text-[11px] font-medium text-zinc-500">{isUser ? <MessageCircle className="size-3" /> : <Bot className="size-3" />}<span>{headerLabel}</span></div>
                    {isUser ? <p className="whitespace-pre-wrap break-words">{message.content}</p> : <MarkdownContent markdown={message.content} />}
                  </div>
                  <div className="mt-1 flex items-center justify-end gap-2 px-1">
                    <span className="text-[10px] text-zinc-600 opacity-0 transition-opacity group-hover:opacity-100">{formatDate(message.createdAt)}</span>
                    {!isUser ? <button type="button" onClick={() => void navigator.clipboard.writeText(message.content)} aria-label={t('chat.copyMessage')} className="text-zinc-500 opacity-0 transition-colors hover:text-zinc-100 group-hover:opacity-100"><Copy className="size-3.5" /></button> : null}
                  </div>
                </div>
              </article>
              return <div key={message.id}>{pipelineReply ? activity : null}{bubble}{toolCall ? activity : null}</div>
            })}
            {showTyping ? <TypingIndicator /> : null}
            {runs.filter((run) => !seenRuns.has(run.id)).map((run) => <ActivityDisclosure key={run.id} run={run} events={events[run.id] ?? []} onOpenPipeline={() => void openRun(run)} />)}
          </>}
          </div>
          {!isAtBottom ? <div className="pointer-events-none absolute bottom-20 left-1/2 z-10 -translate-x-1/2"><Button size="sm" variant="outline" className="pointer-events-auto size-8 rounded-full p-0" onClick={scrollToBottom} aria-label={t('chat.scrollToBottom')}><ChevronDown className="size-4" /></Button></div> : null}
        </div>
        {activeRun ? <div className="flex justify-center py-2"><Button variant="ghost" size="sm" onClick={() => void stop()} className="text-zinc-400"><CircleStop className="size-3.5" /> {t('chat.stop')}</Button></div> : null}
        <div className="border-t border-zinc-800 bg-zinc-950/50 px-4 pb-4 pt-3">
          <div className="mx-auto max-w-3xl">
            <div className="flex items-center gap-2 px-1 pb-2">
              <span className="inline-flex h-5 items-center gap-1 rounded-full border border-zinc-800 bg-zinc-900 px-2 text-[10px] text-zinc-400">
                <span className="size-1.5 rounded-full bg-emerald-400" />
                {selected.mode === 'model' ? modelLabel : t('chat.publishedPipeline')}
              </span>
            </div>
            <div className="group relative flex items-end gap-2 rounded-2xl border border-zinc-700/80 bg-zinc-900/80 p-2 shadow-lg shadow-black/20 backdrop-blur-sm transition-all focus-within:border-zinc-500 focus-within:shadow-zinc-500/5">
              <textarea ref={textareaRef} value={draft} onChange={(event) => setDraft(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); void send() } }} placeholder={t('chat.message')} className="max-h-40 min-h-9 flex-1 resize-none bg-transparent px-3 py-2 text-sm leading-6 text-zinc-100 outline-none placeholder:text-zinc-600" />
              <button type="button" onClick={() => void send()} disabled={!draft.trim() || sending} aria-label={t('chat.send')} className={`flex size-8 shrink-0 items-center justify-center rounded-lg transition-all ${draft.trim() && !sending ? 'bg-zinc-100 text-zinc-900 hover:bg-zinc-200' : 'bg-zinc-800 text-zinc-600 cursor-not-allowed'}`}>
                {sending ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}
              </button>
            </div>
            <div className="mt-2 flex items-center justify-between px-2">
              {selected.mode === 'model' ? <Select className="h-6 w-36 text-[10px]" menuPlacement="top" value={selected.actionPolicy} onValueChange={(value) => void updatePolicy(value)} options={[{ value: 'ask', label: t('chat.ask') }, { value: 'always', label: t('chat.always') }]} ariaLabel={t('chat.approval')} /> : <span className="flex items-center gap-1 text-[10px] text-zinc-600"><Clock3 className="size-3" />{t('chat.storedLocally')}</span>}
              <span className="text-[10px] text-zinc-700">{t('chat.enterToSend', 'Enter to send · Shift+Enter for newline')}</span>
            </div>
          </div>
        </div>
      </>}</main>
    </div>
    {approvals[0] ? <ApprovalDialog approval={approvals[0]} onResolve={(approved) => void resolveApproval(approvals[0], approved)} /> : null}
  </section>
}
