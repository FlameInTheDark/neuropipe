import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Bot, ChevronDown, ChevronRight, CircleStop, Clock3, Loader2, MessageCircle, Pencil, Plus, Send, ShieldAlert, Trash2, Workflow } from 'lucide-react'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { LucideIcon } from '@/components/LucideIconPicker'
import { MarkdownContent } from '@/components/MarkdownContent'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/EmptyState'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Tooltip } from '@/components/ui/tooltip'
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

function RenameDialog({ value, onClose, onSave }: { value: string; onClose: () => void; onSave: (value: string) => void }) {
  const { t } = useTranslation()
  const [title, setTitle] = useState(value)
  return <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/65 p-5"><form onSubmit={(event) => { event.preventDefault(); onSave(title) }} className="w-full max-w-sm rounded-xl border border-zinc-700 bg-zinc-950 p-5 shadow-2xl"><h2 className="text-sm font-semibold text-zinc-100">{t('chat.renameTitle')}</h2><Input autoFocus className="mt-4" value={title} onChange={(event) => setTitle(event.target.value)} /><div className="mt-5 flex justify-end gap-2"><Button type="button" variant="ghost" onClick={onClose}>{t('common.cancel')}</Button><Button type="submit" disabled={!title.trim()}>{t('common.save')}</Button></div></form></div>
}

function ApprovalDialog({ approval, onResolve }: { approval: ChatApproval; onResolve: (approved: boolean) => void }) {
  const { t } = useTranslation()
  return <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/65 p-5"><section role="dialog" aria-modal="true" aria-label={t('chat.approval')} className="w-full max-w-md rounded-xl border border-zinc-700 bg-zinc-950 p-5 shadow-2xl"><div className="flex gap-3"><div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-amber-400/10 text-amber-300"><ShieldAlert className="size-4" /></div><div><h2 className="text-sm font-semibold text-zinc-100">{t('chat.approveTitle')}</h2><p className="mt-1 text-xs leading-5 text-zinc-500">{t('chat.modelWants', { action: approval.toolCall.name.replaceAll('_', ' ') })}</p></div></div><pre className="muted-scroll mt-4 max-h-44 overflow-auto rounded-lg border border-zinc-800 bg-zinc-900/60 p-3 font-mono text-[11px] leading-5 text-zinc-400">{JSON.stringify(approval.toolCall.arguments, null, 2)}</pre><div className="mt-5 flex justify-end gap-2"><Button variant="ghost" onClick={() => onResolve(false)}>{t('chat.deny')}</Button><Button onClick={() => onResolve(true)}>{t('chat.allow')}</Button></div></section></div>
}

function ConversationItem({ conversation, selected, onClick, onMenu }: { conversation: ChatConversation; selected: boolean; onClick: () => void; onMenu: () => void }) {
  const { t } = useTranslation()
  return <div className={cn('group flex items-center gap-2 rounded-md pr-1 transition-colors', selected ? 'bg-zinc-800' : 'hover:bg-zinc-900')}><button type="button" onClick={onClick} className="flex min-w-0 flex-1 items-center gap-2 px-2.5 py-2 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"><span className={cn('flex size-5 shrink-0 items-center justify-center', conversation.mode === 'model' ? 'text-fuchsia-300' : 'text-violet-300')}>{conversation.mode === 'model' ? <Bot className="size-3.5" /> : <MessageCircle className="size-3.5" />}</span><span className="min-w-0 flex-1"><span className="block truncate text-xs font-medium text-zinc-300">{conversation.title}</span><span className="block truncate pt-0.5 text-[10px] text-zinc-600">{formatDate(conversation.updatedAt)}</span></span></button><button type="button" onClick={onMenu} className="invisible flex size-6 items-center justify-center rounded text-zinc-600 hover:bg-zinc-700 hover:text-zinc-200 group-hover:visible focus-visible:visible focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500" aria-label={t('chat.options', { name: conversation.title })}><ChevronDown className="size-3.5" /></button></div>
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
  const [rename, setRename] = useState<ChatConversation>()
  const feedRef = useRef<HTMLDivElement>(null)

  const selected = conversations.find((item) => item.id === selectedID)
  const activeRun = runs.find((run) => run.status === 'pending' || run.status === 'running')
  const visibleConversations = useMemo(() => conversations.filter((item) => item.mode === mode), [conversations, mode])
  const visiblePipelines = useMemo(() => {
    const query = search.trim().toLowerCase()
    return pipelines.filter((item) => !query || `${item.pipelineName} ${item.label}`.toLowerCase().includes(query))
  }, [pipelines, search])

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
    const stopChat = EventsOn('chat.updated', refresh)
    const stopRun = EventsOn('chat.run.updated', refresh)
    const stopApproval = EventsOn('chat.approval.requested', refresh)
    return () => { stopChat(); stopRun(); stopApproval() }
  }, [loadDetails, selectedID])
  useEffect(() => { feedRef.current?.scrollTo({ top: feedRef.current.scrollHeight, behavior: 'smooth' }) }, [messages, runs])
  useEffect(() => {
    if (selected?.mode === mode) return
    const next = conversations.find((item) => item.mode === mode)
    setSelectedID(next?.id)
    if (next) void loadDetails(next.id)
    else { setMessages([]); setRuns([]); setEvents({}); setApprovals([]) }
  }, [conversations, loadDetails, mode, selected])

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
  const saveConversation = async (conversation: ChatConversation) => { try { const saved = await desktop.saveChatConversation(conversation); setConversations((items) => items.map((item) => item.id === saved.id ? saved : item)); setRename(undefined) } catch (reason) { setError(reason instanceof Error ? reason.message : t('chat.saveFailed')) } }
  const remove = async (conversation: ChatConversation) => { const confirmed = await askConfirmation({ title: t('chat.deleteTitle'), description: t('chat.deleteDescription', { name: conversation.title }), confirmLabel: t('chat.deleteConfirm') }); if (!confirmed) return; try { await desktop.deleteChatConversation(conversation.id); const remaining = conversations.filter((item) => item.id !== conversation.id); setConversations(remaining); if (selectedID === conversation.id) { const next = remaining.find((item) => item.mode === mode); setSelectedID(next?.id); if (next) await loadDetails(next.id); else { setMessages([]); setRuns([]); setEvents({}); setApprovals([]) } } } catch (reason) { setError(reason instanceof Error ? reason.message : t('chat.deleteFailed')) } }
  const resolveApproval = async (approval: ChatApproval, approved: boolean) => { try { await desktop.resolveChatApproval(approval.id, approved); if (selectedID) await loadDetails(selectedID) } catch (reason) { setError(reason instanceof Error ? reason.message : t('chat.resolveFailed')) } }
  const updatePolicy = async (value: string) => { if (!selected) return; await saveConversation({ ...selected, actionPolicy: value === 'always' ? 'always' : 'ask' }) }
  const openRun = async (run: ChatRun) => {
    try {
      if (run.executionId) { const execution = await desktop.getExecution(run.executionId); setScreen('editor', execution.pipelineId); return }
      if (selected?.pipelineId) setScreen('editor', selected.pipelineId)
    } catch (reason) { setError(reason instanceof Error ? reason.message : t('chat.openRunFailed')) }
  }
  const seenRuns = new Set<string>()

  return <section className="flex h-full min-h-0 flex-col"><PageHeader title={t('chat.title')} description={t('chat.description')} actions={<>
    <div className="flex rounded-md border border-zinc-800 bg-zinc-950 p-0.5"><Button size="sm" variant={mode === 'model' ? 'secondary' : 'ghost'} onClick={() => setMode('model')}><Bot className="size-3.5" />{t('chat.model')}</Button><Button size="sm" variant={mode === 'pipeline' ? 'secondary' : 'ghost'} onClick={() => setMode('pipeline')}><Workflow className="size-3.5" />{t('chat.pipelines')}</Button></div>
    <Button size="sm" variant="ghost" className="size-7 px-0" onClick={() => { if (selected) setRename(selected) }} aria-label={t('chat.rename')} disabled={!selected}><Pencil className="size-3.5" /></Button>
    <Button size="sm" variant="ghost" className="size-7 px-0" onClick={() => { if (selected) void remove(selected) }} aria-label={t('chat.delete')} disabled={!selected}><Trash2 className="size-3.5 text-red-300" /></Button>
  </>} />
    <div className="grid min-h-0 flex-1 grid-cols-[15.5rem_minmax(0,1fr)]">
      <aside className="min-h-0 border-r border-zinc-800 bg-zinc-950/35 p-3"><div className="mb-3 flex items-center gap-2">{mode === 'model' ? <Button className="flex-1" size="sm" onClick={() => void create('model')}><Plus className="size-3.5" />{t('chat.new')}</Button> : <div className="relative flex-1"><Input value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t('chat.findPipeline')} className="h-7 text-xs" /></div>}</div>
        <div className="muted-scroll h-[calc(100%-2.75rem)] overflow-y-auto"><p className="px-2 pb-1.5 text-[10px] font-medium uppercase tracking-[.12em] text-zinc-600">{mode === 'model' ? t('chat.conversations') : t('chat.chatPipelines')}</p>{mode === 'model' ? visibleConversations.map((conversation) => <ConversationItem key={conversation.id} conversation={conversation} selected={conversation.id === selectedID} onClick={() => void choose(conversation)} onMenu={() => setRename(conversation)} />) : <>{visiblePipelines.map((pipeline) => <button key={pipeline.bindingId} type="button" onClick={() => void create('pipeline', pipeline.bindingId)} className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left transition-colors hover:bg-zinc-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"><span className="flex size-6 shrink-0 items-center justify-center rounded-md" style={{ color: pipeline.color || '#e4e4e7', backgroundColor: `${pipeline.color || '#27272a'}20` }}><LucideIcon name={pipeline.icon} className="size-3.5" /></span><span className="min-w-0"><span className="block truncate text-xs font-medium text-zinc-300">{pipeline.pipelineName}</span><span className="block truncate pt-0.5 text-[10px] text-zinc-600">{pipeline.label}</span></span></button>)}{visiblePipelines.length === 0 ? <p className="px-2 py-4 text-xs leading-5 text-zinc-600">{t('chat.noPipelines')}</p> : null}<div className="mt-4 border-t border-zinc-800 pt-3">{visibleConversations.map((conversation) => <ConversationItem key={conversation.id} conversation={conversation} selected={conversation.id === selectedID} onClick={() => void choose(conversation)} onMenu={() => setRename(conversation)} />)}</div></>}</div>
      </aside>
      <main className="flex min-w-0 min-h-0 flex-col">{loading ? <div className="flex h-full items-center justify-center text-sm text-zinc-500"><Loader2 className="mr-2 size-4 animate-spin" />{t('chat.loading')}</div> : !selected ? <EmptyState icon={MessageCircle} title={mode === 'model' ? t('chat.startModel') : t('chat.choosePipeline')} description={mode === 'model' ? t('chat.startModelDescription') : t('chat.choosePipelineDescription')} action={mode === 'model' ? { label: t('chat.new'), onClick: () => void create('model') } : undefined} /> : <><div ref={feedRef} className="muted-scroll min-h-0 flex-1 overflow-y-auto"><div className="mx-auto max-w-3xl px-6 py-7">{messages.map((message) => {
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
        const bubble = <article className={cn('mb-5 flex gap-3', message.role === 'user' ? 'justify-end' : 'justify-start')}><div className={cn('max-w-[85%] rounded-xl px-4 py-3 text-sm leading-6', message.role === 'user' ? 'bg-zinc-100 text-zinc-900' : 'border border-zinc-800 bg-zinc-900/55 text-zinc-300')}><div className="mb-1.5 flex items-center gap-1.5 text-[10px] font-medium uppercase tracking-[.1em] opacity-50">{message.role === 'user' ? <MessageCircle className="size-3" /> : <Bot className="size-3" />}{message.role === 'user' ? 'You' : selected.mode === 'pipeline' ? selected.title : 'Neuropipe'}</div>{message.role === 'user' ? <p className="whitespace-pre-wrap break-words">{message.content}</p> : <MarkdownContent markdown={message.content} />}</div></article>
        return <div key={message.id}>{pipelineReply ? activity : null}{bubble}{toolCall ? activity : null}</div>
      })}{runs.filter((run) => !seenRuns.has(run.id)).map((run) => <ActivityDisclosure key={run.id} run={run} events={events[run.id] ?? []} onOpenPipeline={() => void openRun(run)} />)}</div></div>
        <div className="border-t border-zinc-800 bg-zinc-950/30 p-4"><div className="mx-auto flex max-w-3xl items-end gap-2 rounded-xl border border-zinc-700 bg-zinc-950 p-2 shadow-inner shadow-black/20"><textarea value={draft} onChange={(event) => setDraft(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); void send() } }} rows={1} placeholder={t('chat.message')} className="max-h-28 min-h-8 flex-1 resize-none bg-transparent px-2 py-1.5 text-sm text-zinc-100 outline-none placeholder:text-zinc-600" />{activeRun ? <Tooltip content={t('chat.stop')} side="top"><Button variant="ghost" onClick={() => void stop()} aria-label={t('chat.stop')}><CircleStop className="size-4 text-red-300" /></Button></Tooltip> : null}<Button onClick={() => void send()} disabled={!draft.trim() || sending} aria-label={t('chat.send')}>{sending ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}</Button></div><div className="mx-auto mt-2 flex max-w-3xl items-center justify-between px-1 text-[10px] text-zinc-600"><span>{selected.mode === 'model' ? modelLabel : t('chat.publishedPipeline')}</span>{selected.mode === 'model' ? <Select className="w-40" menuPlacement="top" value={selected.actionPolicy} onValueChange={(value) => void updatePolicy(value)} options={[{ value: 'ask', label: t('chat.ask') }, { value: 'always', label: t('chat.always') }]} ariaLabel={t('chat.approval')} /> : <span className="flex items-center gap-1"><Clock3 className="size-3" />{t('chat.storedLocally')}</span>}</div></div></>}</main>
    </div>
    {rename ? <RenameDialog value={rename.title} onClose={() => setRename(undefined)} onSave={(title) => void saveConversation({ ...rename, title })} /> : null}
    {approvals[0] ? <ApprovalDialog approval={approvals[0]} onResolve={(approved) => void resolveApproval(approvals[0], approved)} /> : null}
  </section>
}
