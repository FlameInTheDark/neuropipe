import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import i18n from "@/i18n";
import { desktop } from "@/lib/bridge";
import type { DocumentationDocument, DocumentationEntry, DocumentationSearchResult } from "@/lib/types";
import { usePersistedValue } from "@/lib/prefs";
import { Modal } from "../components/primitives/Modal";
import { SearchInput, ViewShell, EmptyState } from "../components/ViewShell";
import { Icon } from "../components/icons";
import { cn } from "../utils/cn";

interface TreeGroup {
  label: string;
  docs: DocumentationEntry[];
}

/** Local reference pages delivered by the Desktop service (language-aware).
 *  `embedded` renders without the page chrome so it can live inside a modal,
 *  optionally opened on a specific document/anchor (editor node docs). */
export function DocsView({
  embedded = false,
  initialDocumentId = null,
  initialAnchor = null,
}: {
  embedded?: boolean;
  initialDocumentId?: string | null;
  initialAnchor?: string | null;
} = {}) {
  const { t } = useTranslation();
  const [entries, setEntries] = useState<DocumentationEntry[]>([]);
  const [document, setDocument] = useState<DocumentationDocument | null>(null);
  const [query, setQuery] = useState("");
  const [searchResults, setSearchResults] = useState<DocumentationSearchResult[] | null>(null);
  const [collapsed, setCollapsed] = usePersistedValue<Record<string, boolean>>(
    "neuropipe.docs.collapsed.v2",
    {},
  );
  const articleRef = useRef<HTMLElement>(null);
  const pendingAnchor = useRef<string | null>(initialAnchor);

  const language = i18n.resolvedLanguage ?? i18n.language ?? "en";

  /* list + current document reload with the locale */
  useEffect(() => {
    let cancelled = false;
    void desktop
      .listDocumentation(language)
      .then((list) => {
        if (cancelled) return;
        setEntries(list);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [language]);

  /* selection: requested document first, then the first entry */
  const [selectedId, setSelectedId] = useState<string | null>(null);
  useEffect(() => {
    if (!selectedId && entries.length > 0) {
      const wanted =
        initialDocumentId && entries.some((e) => e.id === initialDocumentId)
          ? initialDocumentId
          : entries[0].id;
      setSelectedId(wanted);
    }
  }, [entries, selectedId, initialDocumentId]);

  useEffect(() => {
    if (!selectedId) return;
    let cancelled = false;
    void desktop
      .getDocumentation(language, selectedId)
      .then((doc) => {
        if (!cancelled) setDocument(doc);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [language, selectedId]);

  /* debounced search */
  useEffect(() => {
    const trimmed = query.trim();
    if (!trimmed) {
      setSearchResults(null);
      return;
    }
    const timer = window.setTimeout(() => {
      void desktop
        .searchDocumentation(language, trimmed)
        .then(setSearchResults)
        .catch(() => setSearchResults([]));
    }, 150);
    return () => window.clearTimeout(timer);
  }, [query, language]);

  const tree = useMemo<TreeGroup[]>(() => {
    const groups = new Map<string, DocumentationEntry[]>();
    for (const entry of entries) {
      const key = entry.category[0] ?? t("docs.uncategorized");
      const list = groups.get(key) ?? [];
      list.push(entry);
      groups.set(key, list);
    }
    return [...groups.entries()]
      .sort((a, b) => a[0].localeCompare(b[0]))
      .map(([label, docs]) => ({ label, docs: docs.sort((a, b) => a.title.localeCompare(b.title)) }));
  }, [entries, t]);

  /* outline from ## / ### headings, skipping fenced code blocks */
  const outline = useMemo(() => {
    if (!document) return [] as { level: number; text: string; id: string }[];
    const items: { level: number; text: string; id: string }[] = [];
    let inFence = false;
    for (const line of document.markdown.split("\n")) {
      if (line.trimStart().startsWith("```")) inFence = !inFence;
      if (inFence) continue;
      const match = /^(#{2,3})\s+(.*)$/.exec(line);
      if (match) {
        const text = match[2].trim();
        items.push({ level: match[1].length, text, id: headingSlug(text) });
      }
    }
    return items;
  }, [document]);

  /* scroll to the requested anchor once the document is in the DOM */
  useEffect(() => {
    if (!document || !pendingAnchor.current) return;
    const anchor = pendingAnchor.current;
    pendingAnchor.current = null;
    requestAnimationFrame(() => {
      articleRef.current
        ?.querySelector(`#${CSS.escape(anchor)}`)
        ?.scrollIntoView({ block: "start" });
    });
  }, [document]);

  const body = (
    <div className="flex h-full min-h-0">
      <aside className="w-[248px] shrink-0 overflow-y-auto border-r border-seam p-2.5">
          <SearchInput value={query} onChange={setQuery} placeholder={t("docs.search")} />
          <div className="mt-3 space-y-3">
            {searchResults ? (
              <>
                <p className="px-1.5 pb-1 text-[10px] font-medium tracking-[0.09em] text-ink-500 uppercase">
                  {t("common.search")}
                </p>
                {searchResults.length === 0 && (
                  <p className="px-1.5 py-2 text-[11.5px] text-ink-500">{t("library.noMatches")}</p>
                )}
                {searchResults.map((hit) => (
                  <button
                    key={hit.document.id}
                    onClick={() => {
                      setSelectedId(hit.document.id);
                      setQuery("");
                    }}
                    className={cn(
                      "block w-full rounded-md px-2 py-[6px] text-left transition",
                      selectedId === hit.document.id ? "bg-ink-750" : "hover:bg-ink-850",
                    )}
                  >
                    <span className="block truncate text-[12px] text-ink-100">{hit.document.title}</span>
                    <span className="mt-[2px] line-clamp-2 block text-[10.5px] leading-snug text-ink-500">
                      {hit.excerpt}
                    </span>
                  </button>
                ))}
              </>
            ) : (
              tree.map((group) => {
                const isCollapsed = collapsed[group.label] ?? false;
                return (
                  <div key={group.label}>
                    <button
                      onClick={() => setCollapsed((c) => ({ ...c, [group.label]: !isCollapsed }))}
                      className="flex w-full items-center gap-1 px-1.5 pb-1 text-left"
                    >
                      <Icon
                        name="ChevronRight"
                        className={cn(
                          "h-2.5 w-2.5 shrink-0 text-ink-600 transition-transform",
                          !isCollapsed && "rotate-90",
                        )}
                      />
                      <span className="text-[10px] font-medium tracking-[0.09em] text-ink-500 uppercase">
                        {group.label}
                      </span>
                    </button>
                    {!isCollapsed &&
                      group.docs.map((doc) => (
                        <button
                          key={doc.id}
                          onClick={() => setSelectedId(doc.id)}
                          className={cn(
                            "block w-full truncate rounded-md px-2 py-[5px] pl-5 text-left text-[12px] transition",
                            selectedId === doc.id
                              ? "bg-ink-750 text-ink-50"
                              : "text-ink-400 hover:bg-ink-850 hover:text-ink-100",
                          )}
                        >
                          {doc.title}
                        </button>
                      ))}
                  </div>
                );
              })
            )}
          </div>
        </aside>

        <article ref={articleRef} className="fade-in min-w-0 flex-1 overflow-y-auto px-7 py-6">
          {document ? (
            <div className="mx-auto max-w-[640px] pb-16">
              <p className="font-mono text-[11px] text-ink-500">{document.category.join(" / ")}</p>
              <h1 className="mt-2 text-[22px] font-semibold tracking-tight text-ink-50">{document.title}</h1>

              {/* doc content is authored markdown shipped with the app */}
              <MarkdownBody markdown={document.markdown} />
            </div>
          ) : (
            <EmptyState icon="BookOpen" title={t("docs.loadingDocs")} />
          )}
        </article>

        <aside className="hidden w-[190px] shrink-0 overflow-y-auto border-l border-seam p-3.5 xl:block">
          <p className="mb-2 text-[10px] font-medium tracking-[0.09em] text-ink-500 uppercase">{t("docs.onThisPage")}</p>
          {outline.map((item) => (
            <a
              key={item.id}
              href={`#${item.id}`}
              onClick={(e) => {
                e.preventDefault();
                articleRef.current?.querySelector(`#${CSS.escape(item.id)}`)?.scrollIntoView({ block: "start" });
              }}
              className={cn(
                "block py-[3px] text-[11.5px] text-ink-400 transition hover:text-ink-50",
                item.level === 3 && "pl-3",
              )}
            >
              {item.text}
            </a>
          ))}
        </aside>
      </div>
  );

  if (embedded) return body;
  return (
    <ViewShell title={t("docs.dialog")} subtitle={t("docs.description")} padded={false}>
      {body}
    </ViewShell>
  );
}

/** Documentation opened as an overlay (editor node docs) — never navigates
 *  away from the editor, so unsaved graph work is never at risk. */
export function DocsDialog({
  documentID,
  anchor,
  onClose,
}: {
  documentID?: string | null;
  anchor?: string | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  return (
    <Modal
      title={t("docs.dialog")}
      icon="BookOpen"
      size="full"
      onClose={onClose}
      bodyClassName="p-0 overflow-hidden"
    >
      <DocsView embedded initialDocumentId={documentID} initialAnchor={anchor} />
    </Modal>
  );
}

/** Deterministic anchor id shared by the outline and the markdown renderer. */
function headingSlug(text: string): string {
  return text.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
}

/** Extracts plain text from react-markdown children (string | node array). */
function childText(children: unknown): string {
  if (typeof children === "string") return children;
  if (Array.isArray(children)) return children.map(childText).join("");
  if (children && typeof children === "object" && "props" in (children as Record<string, unknown>)) {
    return childText((children as { props: { children?: unknown } }).props.children);
  }
  return "";
}

function MarkdownBody({ markdown }: { markdown: string }) {
  return (
    <div className="mt-4 space-y-3 text-[13px] leading-relaxed text-ink-200 [&_a]:text-sky-300 [&_blockquote]:border-l-2 [&_blockquote]:border-ink-600 [&_blockquote]:pl-3 [&_code]:rounded [&_code]:bg-ink-800 [&_code]:px-1 [&_code]:font-mono [&_code]:text-[12px] [&_h1]:text-[17px] [&_h1]:font-semibold [&_h1]:text-ink-50 [&_h2]:mt-5 [&_h2]:border-b [&_h2]:border-seam [&_h2]:pb-1 [&_h2]:text-[15px] [&_h2]:font-semibold [&_h2]:text-ink-50 [&_h3]:mt-4 [&_h3]:text-[13px] [&_h3]:font-semibold [&_h3]:text-ink-100 [&_li]:ml-4 [&_li]:list-disc [&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:border [&_pre]:border-ink-700 [&_pre]:bg-ink-950/60 [&_pre]:p-3 [&_table]:w-full [&_td]:border-t [&_td]:border-seam [&_td]:px-2 [&_td]:py-1 [&_th]:px-2 [&_th]:py-1 [&_th]:text-left">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeHighlight]}
        components={{
          h2: ({ children }) => (
            <h2 id={headingSlug(childText(children))}>{children}</h2>
          ),
          h3: ({ children }) => (
            <h3 id={headingSlug(childText(children))}>{children}</h3>
          ),
        }}
      >
        {markdown}
      </ReactMarkdown>
    </div>
  );
}


