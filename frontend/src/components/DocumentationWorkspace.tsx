import { type ReactNode, useEffect, useMemo, useRef, useState } from 'react'
import { ChevronDown, ChevronRight, FileText, Loader2, Search, X } from 'lucide-react'
import { createPortal } from 'react-dom'
import { MarkdownContent } from '@/components/MarkdownContent'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { desktop } from '@/lib/bridge'
import type { DocumentationDocument, DocumentationEntry, DocumentationSearchResult } from '@/lib/types'
import { cn } from '@/lib/utils'
import { useTranslation } from 'react-i18next'

const selectedDocumentKey = 'neuropipe.documentation.document.v1'
const collapsedCategoriesKey = 'neuropipe.documentation.collapsed.v1'

interface OutlineItem {
  level: number
  title: string
  anchor: string
}

interface CategoryTree {
  children: Map<string, CategoryTree>
  documents: DocumentationEntry[]
}

function readStoredDocument(): string | undefined {
  try { return window.localStorage.getItem(selectedDocumentKey) || undefined } catch { return undefined }
}

function persistDocument(id: string) {
  try { window.localStorage.setItem(selectedDocumentKey, id) } catch { /* optional local preference */ }
}

function readCollapsedCategories(): Set<string> {
  try {
    const parsed: unknown = JSON.parse(window.localStorage.getItem(collapsedCategoriesKey) ?? '[]')
    return new Set(Array.isArray(parsed) ? parsed.filter((value): value is string => typeof value === 'string') : [])
  } catch { return new Set() }
}

function persistCollapsedCategories(value: Set<string>) {
  try { window.localStorage.setItem(collapsedCategoriesKey, JSON.stringify([...value])) } catch { /* optional local preference */ }
}

function slug(value: string) {
  return value.toLowerCase().replace(/[^\p{L}\p{N}]+/gu, '-').replace(/(^-|-$)/g, '') || 'section'
}

const categoryKeys: Record<string, string> = {
  'Getting started': 'gettingStarted',
  'Core concepts': 'coreConcepts',
  Guides: 'guides',
  'Node reference': 'nodeReference',
  Triggers: 'triggers',
  Data: 'data',
  Flow: 'flow',
  Actions: 'actions',
  Local: 'local',
  AI: 'ai',
  Chat: 'chat',
  Functions: 'functions',
  Canvas: 'canvas',
  Extensions: 'extensions',
  Plugins: 'plugins',
}

function outline(markdown: string): OutlineItem[] {
  const results: OutlineItem[] = []
  const used = new Map<string, number>()
  let fence: string | undefined
  for (const line of markdown.split(/\r?\n/)) {
    const fenceMatch = /^\s*(`{3,}|~{3,})/.exec(line)
    if (fenceMatch) {
      if (!fence) fence = fenceMatch[1]
      else if (fenceMatch[1][0] === fence[0] && fenceMatch[1].length >= fence.length) fence = undefined
      continue
    }
    if (fence) continue
    const match = /^(#{2,3})\s+(.+?)\s*#*\s*$/.exec(line)
    if (!match) continue
    const title = match[2].replace(/[`*_]/g, '').trim()
    const base = slug(title)
    const occurrence = used.get(base) ?? 0
    used.set(base, occurrence + 1)
    results.push({ level: match[1].length, title, anchor: occurrence === 0 ? base : `${base}-${occurrence + 1}` })
  }
  return results
}

function buildTree(entries: DocumentationEntry[]): CategoryTree {
  const root: CategoryTree = { children: new Map(), documents: [] }
  for (const entry of entries) {
    let current = root
    for (const segment of entry.category) {
      const child = current.children.get(segment) ?? { children: new Map(), documents: [] }
      current.children.set(segment, child)
      current = child
    }
    current.documents.push(entry)
  }
  const sortNode = (node: CategoryTree) => {
    node.documents.sort((left, right) => left.title.localeCompare(right.title))
    for (const child of node.children.values()) sortNode(child)
  }
  sortNode(root)
  return root
}

interface DocumentationWorkspaceProps {
  initialDocumentID?: string
  initialAnchor?: string
}

// DocumentationWorkspace is shared verbatim by the main Documentation route
// and the editor overlay. Keeping it mounted outside route changes preserves
// draft graph state while a user reads a node reference.
export function DocumentationWorkspace({ initialDocumentID, initialAnchor }: DocumentationWorkspaceProps) {
  const { t, i18n } = useTranslation()
  const [entries, setEntries] = useState<DocumentationEntry[]>([])
  const [selectedID, setSelectedID] = useState<string | undefined>(initialDocumentID ?? readStoredDocument())
  const [selected, setSelected] = useState<DocumentationDocument>()
  const [query, setQuery] = useState('')
  const [searchResults, setSearchResults] = useState<DocumentationSearchResult[]>([])
  const [loading, setLoading] = useState(true)
  const [loadingDocument, setLoadingDocument] = useState(false)
  const [error, setError] = useState<string>()
  const [collapsed, setCollapsed] = useState<Set<string>>(readCollapsedCategories)
  const [activeAnchor, setActiveAnchor] = useState<string | undefined>(initialAnchor)
  const articleRef = useRef<HTMLElement>(null)
  const localizeCategory = (label: string) => {
    const key = categoryKeys[label]
    return key ? t(`docs.category.${key}`) : label
  }

  useEffect(() => {
    let cancelled = false
    void desktop.listDocumentation(i18n.resolvedLanguage ?? i18n.language).then((next) => {
      if (cancelled) return
      setEntries(next)
      setSelectedID((current) => current && next.some((entry) => entry.id === current) ? current : next[0]?.id)
    }).catch((reason) => {
      if (!cancelled) setError(reason instanceof Error ? reason.message : t('docs.loadError'))
    }).finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [i18n.language, i18n.resolvedLanguage, t])

  useEffect(() => {
    if (!initialDocumentID) return
    setSelectedID(initialDocumentID)
    setActiveAnchor(initialAnchor)
  }, [initialAnchor, initialDocumentID])

  useEffect(() => {
    if (!selectedID) return
    let cancelled = false
    setLoadingDocument(true)
    void desktop.getDocumentation(i18n.resolvedLanguage ?? i18n.language, selectedID).then((next) => {
      if (!cancelled) {
        setSelected(next)
        setError(undefined)
        persistDocument(next.id)
      }
    }).catch((reason) => {
      if (!cancelled) setError(reason instanceof Error ? reason.message : t('docs.loadError'))
    }).finally(() => { if (!cancelled) setLoadingDocument(false) })
    return () => { cancelled = true }
  }, [i18n.language, i18n.resolvedLanguage, selectedID, t])

  useEffect(() => {
    const normalized = query.trim()
    if (!normalized) { setSearchResults([]); return }
    let cancelled = false
    const timer = window.setTimeout(() => {
      void desktop.searchDocumentation(i18n.resolvedLanguage ?? i18n.language, normalized).then((next) => { if (!cancelled) setSearchResults(next) }).catch(() => { if (!cancelled) setSearchResults([]) })
    }, 120)
    return () => { cancelled = true; window.clearTimeout(timer) }
  }, [i18n.language, i18n.resolvedLanguage, query])

  useEffect(() => {
    if (!selected || !activeAnchor) return
    const timer = window.setTimeout(() => {
      const target = articleRef.current?.querySelector<HTMLElement>(`[id="${activeAnchor}"]`)
      target?.scrollIntoView({ block: 'start' })
    }, 0)
    return () => window.clearTimeout(timer)
  }, [activeAnchor, selected])

  const visibleEntries = useMemo(() => query.trim() ? searchResults.map((result) => result.document) : entries, [entries, query, searchResults])
  const resultExcerpt = useMemo(() => new Map(searchResults.map((result) => [result.document.id, result.excerpt])), [searchResults])
  const tree = useMemo(() => buildTree(visibleEntries), [visibleEntries])
  const pageOutline = useMemo(() => selected ? outline(selected.markdown) : [], [selected])

  const selectDocument = (id: string, anchor?: string) => {
    setActiveAnchor(anchor)
    setSelectedID(id)
    if (!anchor) articleRef.current?.scrollTo({ top: 0 })
  }

  const toggleCategory = (key: string) => setCollapsed((current) => {
    const next = new Set(current)
    next.has(key) ? next.delete(key) : next.add(key)
    persistCollapsedCategories(next)
    return next
  })

  const renderNode = (node: CategoryTree, path: string[], depth: number): ReactNode => {
    const categories = [...node.children.entries()].sort(([left], [right]) => left.localeCompare(right))
    return <>
      {categories.map(([label, child]) => {
        const categoryPath = [...path, label]
        const key = categoryPath.join('/')
        const isCollapsed = collapsed.has(key)
        return <div key={key} className={depth === 0 ? 'mt-2' : 'mt-1'}>
          <button type="button" onClick={() => toggleCategory(key)} aria-expanded={!isCollapsed} className="flex h-8 w-full items-center gap-1.5 rounded px-2 text-left text-xs font-medium text-zinc-300 hover:bg-zinc-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500">
            {isCollapsed ? <ChevronRight className="size-3.5 shrink-0" /> : <ChevronDown className="size-3.5 shrink-0" />}
            <span className="truncate">{localizeCategory(label)}</span>
          </button>
          {!isCollapsed && <div className="ml-2 border-l border-zinc-800 pl-2">{renderNode(child, categoryPath, depth + 1)}</div>}
        </div>
      })}
      {node.documents.map((entry) => <button key={entry.id} type="button" onClick={() => selectDocument(entry.id)} className={cn('mt-0.5 w-full rounded px-2 py-1.5 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500', selectedID === entry.id ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-500 hover:bg-zinc-900 hover:text-zinc-200')}>
        <span className="flex items-center gap-1.5 text-xs"><FileText className="size-3 shrink-0" /><span className="truncate">{entry.title}</span></span>
        {query.trim() && resultExcerpt.get(entry.id) ? <span className="mt-1 block line-clamp-2 pl-[18px] text-[10px] leading-4 text-zinc-600">{resultExcerpt.get(entry.id)}</span> : null}
      </button>)}
    </>
  }

  return <div className="flex min-h-0 flex-1 overflow-hidden bg-zinc-950">
    <aside className="flex w-60 shrink-0 flex-col border-r border-zinc-800 bg-zinc-950/80">
      <div className="border-b border-zinc-800 p-3">
        <label className="relative block"><Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-zinc-600" /><Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('docs.search')} aria-label={t('docs.search')} className="h-8 pl-8 text-xs" /></label>
      </div>
      <nav aria-label={t('docs.categories')} className="min-h-0 flex-1 overflow-y-auto p-2 scrollbar-thin">{loading ? <div className="flex items-center gap-2 p-3 text-xs text-zinc-500"><Loader2 className="size-3.5 animate-spin" />{t('docs.loadingDocs')}</div> : renderNode(tree, [], 0)}</nav>
    </aside>
    <article ref={articleRef} className="min-w-0 flex-1 overflow-y-auto px-7 py-7 scrollbar-thin">
      {error ? <div className="rounded-lg border border-red-500/30 bg-red-500/5 p-4 text-sm text-red-200">{error}</div> : loadingDocument || !selected ? <div className="flex items-center gap-2 text-sm text-zinc-500"><Loader2 className="size-4 animate-spin" />{t('docs.loadingArticle')}</div> : <>
        <div className="mb-7 border-b border-zinc-800 pb-5"><p className="text-xs text-zinc-500">{selected.category.map(localizeCategory).join(' / ')}</p>{selected.summary ? <p className="mt-2 text-sm text-zinc-400">{selected.summary}</p> : null}</div>
        <MarkdownContent markdown={selected.markdown} onDocumentLink={selectDocument} />
      </>}
    </article>
    <aside className="hidden w-52 shrink-0 border-l border-zinc-800 px-4 py-7 xl:block">
      <p className="mb-3 text-[11px] font-medium uppercase tracking-[0.12em] text-zinc-500">{t('docs.onThisPage')}</p>
      <nav aria-label={t('docs.onThisPage')} className="sticky top-2 space-y-1">{pageOutline.map((item) => <button key={item.anchor} type="button" onClick={() => articleRef.current?.querySelector<HTMLElement>(`[id="${item.anchor}"]`)?.scrollIntoView({ block: 'start' })} className={cn('block w-full truncate text-left text-xs leading-5 text-zinc-500 hover:text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500', item.level === 3 && 'pl-3')}>{item.title}</button>)}</nav>
    </aside>
  </div>
}

export function DocumentationDialog({ documentID, anchor, onClose }: { documentID: string; anchor?: string; onClose: () => void }) {
  const { t } = useTranslation()
  const closeRef = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : undefined
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKeyDown)
    const timer = window.setTimeout(() => closeRef.current?.focus(), 0)
    return () => { window.removeEventListener('keydown', onKeyDown); window.clearTimeout(timer); previousFocus?.focus() }
  }, [onClose])

  return createPortal(<div className="fixed inset-0 z-[110] flex items-center justify-center bg-black/70 p-5 backdrop-blur-[2px]" onPointerDown={(event) => { if (event.target === event.currentTarget) onClose() }}>
    <section role="dialog" aria-modal="true" aria-label={t('docs.dialog')} className="flex h-[min(860px,calc(100vh-40px))] w-[min(1240px,calc(100vw-40px))] min-w-0 flex-col overflow-hidden rounded-xl border border-zinc-700 bg-zinc-950 shadow-2xl shadow-black/70">
      <header className="flex h-12 shrink-0 items-center justify-between border-b border-zinc-800 px-4"><span className="text-sm font-semibold text-zinc-100">{t('docs.dialog')}</span><Button ref={closeRef} size="sm" variant="ghost" onClick={onClose}><X className="size-3.5" />{t('common.close')}</Button></header>
      <DocumentationWorkspace initialDocumentID={documentID} initialAnchor={anchor} />
    </section>
  </div>, document.body)
}
