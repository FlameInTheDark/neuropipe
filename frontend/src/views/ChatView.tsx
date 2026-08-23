import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Events } from "@wailsio/runtime";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { desktop } from "@/lib/bridge";
import type {
  ChatApproval,
  ChatConversation,
  ChatMessage,
  ChatPipeline,
  ChatRun,
  ChatRunEvent,
  ChatToolCall,
} from "@/lib/types";
import { conversationGroup, formatDateTime } from "@/lib/format";
import { ask } from "@/stores/confirmation";
import { SearchInput } from "../components/ViewShell";
import { Dropdown } from "../components/Dropdown";
import { Modal } from "../components/primitives/Modal";
import { Field } from "../components/primitives/Field";
import { Icon } from "../components/icons";
import { useCtxMenu } from "../components/ContextMenu";
import { cn } from "../utils/cn";

const SUGGESTION_KEYS = [
  "chat.suggestion1",
  "chat.suggestion2",
  "chat.suggestion3",
  "chat.suggestion4",
];

/** Ordered transcript entry: a plain bubble or an inline tool-call group. */
type TranscriptItem =
  | { kind: "message"; msg: ChatMessage }
  | { kind: "tools"; key: string; calls: ToolCallEntry[] };

/* ── sidebar row ── */
function ThreadRow({
  title,
  subtitle,
  pipeline,
  active,
  onClick,
  onCtx,
}: {
  title: string;
  subtitle: string;
  pipeline?: boolean;
  active: boolean;
  onClick: () => void;
  onCtx: (e: React.MouseEvent) => void;
}) {
  return (
    <button
      onClick={onClick}
      onContextMenu={onCtx}
      className={cn(
        "group relative mb-0.5 flex w-full items-start gap-2.5 rounded-lg px-2.5 py-2 text-left transition",
        active ? "bg-ink-750" : "hover:bg-ink-800/70",
      )}
    >
      {active && <span className="absolute top-1/2 left-0 h-4 w-[2px] -translate-y-1/2 rounded-r bg-ink-200" />}
      <span
        className={cn(
          "mt-[1px] grid h-6 w-6 shrink-0 place-items-center rounded-md border",
          pipeline ? "border-violet-500/40 bg-violet-500/10 text-violet-300" : "border-ink-700 bg-ink-850 text-ink-400",
        )}
      >
        <Icon name={pipeline ? "Cable" : "Bot"} className="h-3 w-3" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[12.5px] leading-tight font-medium text-ink-100">{title}</span>
        <span className="mt-[3px] block truncate text-[11px] leading-tight text-ink-500">{subtitle}</span>
      </span>
    </button>
  );
}

/* ── run activity disclosure ── */
function ActivityDisclosure({ run, events }: { run: ChatRun; events: ChatRunEvent[] }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const active = run.status === "pending" || run.status === "running";

  return (
    <div className="rounded-xl border border-ink-700/70 bg-ink-850/60">
      <button onClick={() => setOpen((v) => !v)} className="flex w-full items-center gap-2 px-3 py-2 text-left">
        <Icon name="ChevronRight" className={cn("h-3 w-3 text-ink-500 transition-transform", open && "rotate-90")} />
        <Icon name="Activity" className={cn("h-3.5 w-3.5", active ? "animate-pulse text-sky-300" : "text-ink-400")} />
        <span className="min-w-0 flex-1 truncate text-[12px] text-ink-300">
          {run.statusText || t("chat.waiting")}
        </span>
        {active && (
          <span className="h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-sky-300" />
        )}
      </button>
      {open && (
        <ul className="border-t border-seam px-3 py-2">
          {events.length === 0 && <li className="py-1 text-[11px] text-ink-500">{t("chat.noActivity")}</li>}
          {events.map((ev) => (
            <li key={ev.id} className="flex items-start gap-2 py-1">
              <span
                className={cn(
                  "mt-[6px] h-1.5 w-1.5 shrink-0 rounded-full",
                  ev.status === "failed" ? "bg-rose-400" : ev.status === "completed" ? "bg-emerald-400/80" : "bg-ink-500",
                )}
              />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-[11.5px] text-ink-200">{ev.summary}</span>
                {ev.detail && (
                  <pre className="mt-1 max-h-[90px] overflow-auto whitespace-pre-wrap rounded-md border border-ink-700 bg-ink-950/60 px-2 py-1 font-mono text-[10px] text-ink-400">
                    {ev.detail}
                  </pre>
                )}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/* ── main view ── */
export default function ChatView() {
  const { t } = useTranslation();
  const [conversations, setConversations] = useState<ChatConversation[]>([]);
  const [pipelines, setPipelines] = useState<ChatPipeline[]>([]);
  const [modelLabel, setModelLabel] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [runs, setRuns] = useState<ChatRun[]>([]);
  const [events, setEvents] = useState<Record<string, ChatRunEvent[]>>({});
  const [approvals, setApprovals] = useState<ChatApproval[]>([]);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [q, setQ] = useState("");
  const [newMode, setNewMode] = useState<"model" | "pipeline">("model");
  const [newBindingId, setNewBindingId] = useState("");
  const [renameTarget, setRenameTarget] = useState<ChatConversation | null>(null);
  const [renameDraft, setRenameDraft] = useState("");
  const bottomRef = useRef<HTMLDivElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  /** true while the user is scrolled to (or near) the conversation bottom */
  const pinnedRef = useRef(true);
  const ctx = useCtxMenu();

  const selected = conversations.find((c) => c.id === selectedId) ?? null;

  /* ---------- transcript ---------- */

  /**
   * Walks messages in stored order and lifts tool activity into inline
   * cards: an assistant turn carrying tool_calls is followed by its result
   * card(s), matching how the harness renders a trajectory. Orphan results
   * keep their chronological slot instead of disappearing.
   */
  const transcript = useMemo<TranscriptItem[]>(() => {
    const items: TranscriptItem[] = [];
    for (const msg of messages) {
      if (msg.role === "tool") {
        const last = items[items.length - 1];
        const entry =
          last?.kind === "tools" ? last.calls.find((c) => c.call.id && c.call.id === msg.toolCallId) : undefined;
        if (last?.kind === "tools" && entry) {
          entry.result = msg;
          continue;
        }
        items.push({
          kind: "tools",
          key: msg.id,
          calls: [{ call: { id: msg.toolCallId ?? msg.id, name: msg.toolName ?? "tool", arguments: {} }, result: msg }],
        });
        continue;
      }
      if (msg.role === "assistant" && msg.toolCalls && msg.toolCalls.length > 0) {
        // some models emit prose alongside the calls — keep it, then the cards
        if (msg.content.trim() !== "") items.push({ kind: "message", msg });
        items.push({ kind: "tools", key: msg.id, calls: msg.toolCalls.map((call) => ({ call })) });
        continue;
      }
      if (msg.role === "assistant" && msg.content.trim() === "") continue; // nothing to show
      items.push({ kind: "message", msg });
    }
    return items;
  }, [messages]);

  /* ---------- data loading ---------- */

  /** Sequence guard: only the newest loadDetails may commit its results,
   * so rapid conversation switches can never paint stale history. */
  const detailSeq = useRef(0);
  const [loadError, setLoadError] = useState(false);

  const loadDetails = useCallback(async (conversationId: string) => {
    const ticket = ++detailSeq.current;
    setLoadError(false);
    try {
      const [msgs, rs, aps] = await Promise.all([
        desktop.listChatMessages(conversationId),
        desktop.listChatRuns(conversationId),
        desktop.listPendingChatApprovals(conversationId),
      ]);
      const eventEntries = await Promise.all(
        rs.map(async (r) => {
          try {
            return [r.id, await desktop.listChatRunEvents(r.id)] as const;
          } catch {
            return [r.id, [] as ChatRunEvent[]] as const;
          }
        }),
      );
      if (ticket !== detailSeq.current) return;
      setMessages(msgs);
      setRuns(rs);
      setApprovals(aps);
      setEvents(Object.fromEntries(eventEntries));
    } catch {
      if (ticket === detailSeq.current) setLoadError(true);
    }
  }, []);

  const refreshList = useCallback(async () => {
    const [convs, pipes, settings] = await Promise.all([
      desktop.listChatConversations(),
      desktop.listChatPipelines(),
      desktop.getSettings(),
    ]);
    // newest conversation first regardless of backend ordering
    convs.sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
    setConversations(convs);
    setPipelines(pipes);
    const provider = settings.providers.find((p) => p.id === settings.defaultProviderId);
    setModelLabel(provider ? `${provider.name} · ${provider.model}` : null);
    return convs;
  }, []);

  useEffect(() => {
    void (async () => {
      const convs = await refreshList().catch(() => [] as ChatConversation[]);
      if (convs.length > 0) setSelectedId(convs[0].id);
    })();
  }, [refreshList]);

  /* drop the previous conversation's content immediately so a slow load
     can never show history belonging to another thread */
  useEffect(() => {
    setMessages([]);
    setRuns([]);
    setApprovals([]);
    setEvents({});
    pinnedRef.current = true;
    if (selectedId) void loadDetails(selectedId);
  }, [selectedId, loadDetails]);

  /* live updates pushed from the backend while a run executes */
  useEffect(() => {
    const offs = [
      Events.On("chat.updated", () => {
        void refreshList();
        if (selectedId) void loadDetails(selectedId);
      }),
      Events.On("chat.run.updated", () => {
        if (selectedId) void loadDetails(selectedId);
      }),
      Events.On("chat.approval.requested", () => {
        if (selectedId) void loadDetails(selectedId);
      }),
    ];
    return () => offs.forEach((off) => off());
  }, [selectedId, refreshList, loadDetails]);

  /* auto-scroll only while the user is pinned to the bottom; polling during
     a tool round must never yank the view if they scrolled up to read */
  const transcriptKey = `${selectedId}:${messages.length}:${runs.map((r) => r.status).join(",")}`;
  useEffect(() => {
    if (!pinnedRef.current) return;
    bottomRef.current?.scrollIntoView({ behavior: "instant" });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [transcriptKey]);

  /* ---------- derived ---------- */

  const grouped = useMemo(() => {
    const groups: Record<string, ChatConversation[]> = { today: [], yesterday: [], week: [], older: [] };
    for (const c of conversations.filter((x) => x.mode === (selected?.mode ?? newMode))) {
      groups[conversationGroup(c.updatedAt)].push(c);
    }
    // newest first inside every group
    for (const list of Object.values(groups)) {
      list.sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
    }
    return groups;
  }, [conversations, selected?.mode, newMode]);

  const filterFn = (c: ChatConversation) => c.title.toLowerCase().includes(q.toLowerCase());
  const visibleGroups = useMemo(
    () =>
      Object.entries(grouped)
        .map(([key, list]) => ({ key, list: list.filter(filterFn) }))
        .filter((g) => g.list.length > 0),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [grouped, q],
  );

  const activeRun = runs.find((r) => r.status === "pending" || r.status === "running");
  const approval = approvals[0];

  /* ---------- actions ---------- */

  const createConversation = async (mode: "model" | "pipeline", bindingId = "") => {
    try {
      const conv = await desktop.createChatConversation(mode, bindingId);
      await refreshList();
      setSelectedId(conv.id);
    } catch {
      /* surfaced by backend errors elsewhere */
    }
  };

  const send = async () => {
    const text = draft.trim();
    if (!text || sending) return;
    let target = selected;
    try {
      if (!target) {
        // first message in a fresh chat creates the conversation implicitly
        const conv = await desktop.createChatConversation(
          newMode,
          newMode === "pipeline" ? newBindingId : "",
        );
        target = conv;
        setSelectedId(conv.id);
        await refreshList();
      }
      if (!target) return;
      setSending(true);
      setDraft("");
      await desktop.sendChatMessage(target.id, text);
      await loadDetails(target.id);
      await refreshList();
    } finally {
      setSending(false);
    }
  };

  const cancelRun = async () => {
    if (!activeRun) return;
    await desktop.cancelChatRun(activeRun.id);
    if (selectedId) await loadDetails(selectedId);
  };

  const removeConversation = async (conv: ChatConversation) => {
    const ok = await ask({
      title: t("chat.deleteTitle"),
      description: t("chat.deleteDescription", { name: conv.title }),
      confirmLabel: t("chat.deleteConfirm"),
      danger: true,
    });
    if (!ok) return;
    await desktop.deleteChatConversation(conv.id);
    setSelectedId(null);
    await refreshList();
  };

  const commitRename = async () => {
    if (!renameTarget) return;
    const next = renameDraft.trim();
    setRenameTarget(null);
    if (!next || next === renameTarget.title) return;
    await desktop.saveChatConversation({ ...renameTarget, title: next });
    await refreshList();
  };

  const setPolicy = async (policy: string) => {
    if (!selected) return;
    await desktop.saveChatConversation({ ...selected, actionPolicy: policy as "ask" | "always" });
    await refreshList();
  };

  const resolveApproval = async (approved: boolean) => {
    if (!approval) return;
    await desktop.resolveChatApproval(approval.id, approved);
    if (selectedId) await loadDetails(selectedId);
  };

  const onThreadCtx = (e: React.MouseEvent, conv: ChatConversation) => {
    setSelectedId(conv.id);
    ctx(e, [
      { label: t("chat.rename"), icon: "Pencil", onSelect: () => { setRenameTarget(conv); setRenameDraft(conv.title); } },
      {
        label: t("common.delete"),
        icon: "Trash2",
        danger: true,
        onSelect: () => void removeConversation(conv),
      },
    ]);
  };

  const onBubbleCtx = (e: React.MouseEvent, text: string) =>
    ctx(e, [
      {
        label: t("chat.copyMessage"),
        icon: "Copy",
        hint: "⌘C",
        onSelect: () => navigator.clipboard?.writeText(text),
      },
    ]);

  const mode = selected?.mode ?? newMode;
  const headerPipeline =
    selected?.pipelineId != null ? pipelines.find((p) => p.pipelineId === selected.pipelineId)?.pipelineName : null;

  return (
    <div className="flex h-full min-h-0 overflow-hidden">
      {/* ── sidebar ── */}
      <aside className="flex w-[228px] shrink-0 flex-col border-r border-seam bg-ink-900/50">
        <div className="space-y-2 p-2.5 pb-2">
          <button
            onClick={() => {
              setSelectedId(null);
              setNewMode(mode);
            }}
            className="flex w-full items-center justify-center gap-1.5 rounded-md bg-ink-50 py-1.5 text-[12.5px] font-medium text-ink-950 transition hover:bg-white"
          >
            <Icon name="Plus" className="h-3.5 w-3.5" />
            {t("chat.new")}
          </button>
          <SearchInput value={q} onChange={setQ} placeholder={t("chat.searchChats")} />
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-1.5 pb-2">
          {visibleGroups.map((group) => (
            <div key={group.key}>
              <p className="mb-1 px-2 py-1.5 text-[10px] font-medium tracking-[0.09em] text-ink-500 uppercase">
                {t(`chat.group_${group.key}`)}
              </p>
              {group.list.map((c) => (
                  <ThreadRow
                    key={c.id}
                    title={c.title}
                    subtitle={formatDateTime(c.updatedAt)}
                    pipeline={c.mode === "pipeline"}
                    active={selectedId === c.id}
                    onClick={() => setSelectedId(c.id)}
                    onCtx={(e) => onThreadCtx(e, c)}
                  />
                ))}
            </div>
          ))}

          {/* pipeline bindings for quick-start */}
          {mode === "pipeline" && pipelines.length > 0 && !selected && (
            <div className="mt-3">
              <p className="mb-1 px-2 py-1.5 text-[10px] font-medium tracking-[0.09em] text-ink-500 uppercase">
                {t("chat.chatPipelines")}
              </p>
              {pipelines.map((p) => (
                <ThreadRow
                  key={p.bindingId}
                  title={p.label}
                  subtitle={t("chat.publishedPipeline")}
                  pipeline
                  active={false}
                  onClick={() => void createConversation("pipeline", p.bindingId)}
                  onCtx={() => undefined}
                />
              ))}
            </div>
          )}

          {loadError && conversations.length === 0 && (
            <p className="flex items-center justify-center gap-1.5 px-2 py-4 text-center text-[12px] text-rose-300">
              <Icon name="AlertTriangle" className="h-3.5 w-3.5 shrink-0" />
              {t("chat.loadFailed")}
            </p>
          )}

          {!loadError && conversations.length === 0 && (
            <p className="px-2 py-4 text-center text-[12px] text-ink-500">{t("chat.noChats")}</p>
          )}
        </div>
      </aside>

      {/* ── main area ── */}
      <div className="flex min-w-0 flex-1 flex-col">
        {/* header */}
        <div className="flex h-11 shrink-0 items-center gap-2 border-b border-seam px-4">
          {selected ? (
            <>
              <h2 className="truncate text-[13.5px] font-semibold text-ink-50">{selected.title}</h2>
              <div className="ml-auto flex items-center gap-2">
                <span className="flex items-center gap-1.5 rounded-md border border-ink-700 bg-ink-850 px-2 py-1 text-[11px] text-ink-300">
                  <Icon name={selected.mode === "pipeline" ? "Cable" : "Bot"} className="h-3 w-3 shrink-0 text-ink-500" />
                  {selected.mode === "pipeline"
                    ? headerPipeline ?? t("chat.publishedPipeline")
                    : modelLabel ?? t("chat.configureModel")}
                </span>
                <Dropdown
                  compact
                  value={selected.actionPolicy}
                  onChange={(v) => void setPolicy(v)}
                  options={[
                    { value: "ask", label: t("chat.ask") },
                    { value: "always", label: t("chat.always") },
                  ]}
                />
                <button
                  onClick={() => { setRenameTarget(selected); setRenameDraft(selected.title); }}
                  title={t("chat.rename")}
                  className="grid h-7 w-7 place-items-center rounded-md text-ink-500 transition hover:bg-ink-800 hover:text-ink-100"
                >
                  <Icon name="Pencil" className="h-[15px] w-[15px]" />
                </button>
                <button
                  onClick={() => void removeConversation(selected)}
                  title={t("chat.delete")}
                  className="grid h-7 w-7 place-items-center rounded-md text-ink-500 transition hover:bg-ink-800 hover:text-rose-300"
                >
                  <Icon name="Trash2" className="h-[15px] w-[15px]" />
                </button>
              </div>
            </>
          ) : (
            <>
              <h2 className="text-[13.5px] font-semibold text-ink-50">{t("chat.new")}</h2>
              <div className="ml-auto flex items-center gap-2">
                <div className="flex items-center gap-0.5 rounded-md border border-ink-700 bg-ink-850 p-0.5">
                  {(["model", "pipeline"] as const).map((m) => (
                    <button
                      key={m}
                      onClick={() => setNewMode(m)}
                      className={cn(
                        "flex h-[22px] items-center gap-1.5 rounded px-2 text-[11px] transition",
                        newMode === m ? "bg-ink-700 text-ink-50" : "text-ink-400 hover:text-ink-100",
                      )}
                    >
                      <Icon name={m === "model" ? "Bot" : "Cable"} className="h-3 w-3" />
                      {m === "model" ? t("chat.model") : t("chat.pipelines")}
                    </button>
                  ))}
                </div>
                {newMode === "pipeline" && (
                  <Dropdown
                    value={newBindingId}
                    onChange={setNewBindingId}
                    className="w-[180px]"
                    placeholder={t("chat.choosePipeline")}
                    options={[
                      { value: "", label: t("chat.choosePipeline") },
                      ...pipelines.map((p) => ({ value: p.bindingId, label: p.pipelineName, icon: "Cable" })),
                    ]}
                  />
                )}
              </div>
            </>
          )}
        </div>

        {/* messages */}
        <div
          ref={scrollRef}
          onScroll={() => {
            const el = scrollRef.current;
            if (!el) return;
            pinnedRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 120;
          }}
          className="min-h-0 flex-1 overflow-y-auto"
        >
          {selected ? (
            <div className="mx-auto max-w-[640px] space-y-5 px-4 py-6">
              {/* live status + activity feed for the run in flight; finished
                  runs keep their story in the inline tool cards below */}
              {activeRun && (
                <ActivityDisclosure
                  key={`disc-${activeRun.id}`}
                  run={activeRun}
                  events={events[activeRun.id] ?? []}
                />
              )}

              {transcript.map((item) =>
                item.kind === "tools" ? (
                  <div key={item.key} className="space-y-1.5">
                    {item.calls.map((entry) => (
                      <ToolCallCard key={entry.call.id || entry.result?.id || item.key} entry={entry} />
                    ))}
                  </div>
                ) : (
                  <TranscriptMessage key={item.msg.id} msg={item.msg} pipelineMode={selected.mode === "pipeline"} onCtx={onBubbleCtx} />
                ),
              )}

              {(activeRun || sending) && (
                <div className="flex gap-3">
                  <span className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-ink-200">
                    <Icon name={selected.mode === "pipeline" ? "Cable" : "Bot"} className="h-3.5 w-3.5" />
                  </span>
                  <div className="flex items-center gap-1 rounded-2xl bg-ink-850 px-4 py-3">
                    {[0, 1, 2].map((i) => (
                      <span
                        key={i}
                        className="h-1.5 w-1.5 animate-bounce rounded-full bg-ink-400"
                        style={{ animationDelay: `${i * 140}ms` }}
                      />
                    ))}
                  </div>
                </div>
              )}

              {activeRun && (
                <div className="flex justify-center">
                  <button
                    onClick={() => void cancelRun()}
                    className="flex items-center gap-1.5 rounded-full border border-ink-600 bg-ink-800 px-3 py-1.5 text-[11.5px] text-ink-200 transition hover:border-rose-400/40 hover:text-rose-200"
                  >
                    <Icon name="Square" className="h-3 w-3" />
                    {t("chat.stop")}
                  </button>
                </div>
              )}
              <div ref={bottomRef} />
            </div>
          ) : (
            /* empty state for a new chat */
            <div className="mx-auto flex max-w-[440px] flex-col items-center px-4 pt-14">
              <span
                className={cn(
                  "grid h-12 w-12 place-items-center rounded-2xl border text-ink-200",
                  newMode === "pipeline" ? "border-violet-500/40 bg-violet-500/10 text-violet-300" : "border-ink-700 bg-ink-850",
                )}
              >
                <Icon name={newMode === "pipeline" ? "Cable" : "Sparkles"} className="h-5 w-5" />
              </span>
              <h2 className="mt-3 text-[15px] font-semibold text-ink-50">
                {newMode === "pipeline" ? t("chat.chatWithPipeline") : t("chat.emptyPrompt")}
              </h2>
              <p className="mt-1 max-w-[340px] text-center text-[12px] leading-relaxed text-ink-500">
                {newMode === "pipeline"
                  ? t("chat.chatWithPipelineHint")
                  : modelLabel ?? t("chat.startModelDescription")}
              </p>

              {newMode === "model" && (
                <div className="mt-6 w-full space-y-2">
                  {SUGGESTION_KEYS.map((key) => (
                    <button
                      key={key}
                      onClick={() => setDraft(t(key))}
                      className="group flex w-full items-center gap-2 rounded-xl border border-ink-700/80 bg-ink-850/60 px-4 py-2.5 text-left text-[13px] text-ink-300 transition hover:border-ink-500 hover:bg-ink-800 hover:text-ink-50"
                    >
                      <span className="flex-1">{t(key)}</span>
                      <Icon name="ArrowUpRight" className="h-3.5 w-3.5 text-ink-600 group-hover:text-ink-300" />
                    </button>
                  ))}
                </div>
              )}

              {newMode === "pipeline" && (
                <CtaButton onClick={() => void createConversation("pipeline", newBindingId)} disabled={!newBindingId}>
                  {t("chat.startPipelineChat")}
                </CtaButton>
              )}
            </div>
          )}
        </div>

        {/* ── composer ── */}
        <div className="shrink-0 px-4 pb-4 pt-2">
          <div className="mx-auto max-w-[640px]">
            <div
              className={cn(
                "rounded-2xl border bg-ink-850 transition focus-within:border-ink-500",
                mode === "pipeline" ? "border-violet-500/30 focus-within:border-violet-400/60" : "border-ink-700",
              )}
            >
              <textarea
                rows={2}
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                    void send();
                  }
                }}
                aria-label={t("chat.message")}
                placeholder={t("chat.message")}
                className="w-full resize-none bg-transparent px-4 py-3 text-[13px] text-ink-50 placeholder:text-ink-500"
              />
              <div className="flex items-center gap-2 px-3 pb-2.5">
                <span
                  className={cn(
                    "flex items-center gap-1.5 text-[11px]",
                    mode === "pipeline" ? "text-violet-400/70" : "text-ink-500",
                  )}
                >
                  <Icon name={mode === "pipeline" ? "Cable" : "Sparkles"} className="h-3 w-3" />
                  {mode === "pipeline"
                    ? selected?.mode === "pipeline"
                      ? headerPipeline
                      : pipelines.find((p) => p.bindingId === newBindingId)?.pipelineName
                    : modelLabel ?? t("chat.configureModel")}
                </span>

                <span className="ml-auto text-[10.5px] text-ink-600">{t("chat.enterToSend")}</span>
                <button
                  onClick={() => void send()}
                  disabled={!draft.trim() || sending}
                  aria-label={t("chat.send")}
                  className={cn(
                    "grid h-7 w-7 shrink-0 place-items-center rounded-xl transition",
                    draft.trim() && !sending
                      ? mode === "pipeline"
                        ? "bg-violet-500 text-white hover:bg-violet-400"
                        : "bg-ink-50 text-ink-950 hover:bg-white"
                      : "cursor-not-allowed bg-ink-800 text-ink-500",
                  )}
                >
                  <Icon name={sending ? "Loader2" : "ArrowUpRight"} className={cn("h-4 w-4", sending && "animate-spin")} />
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* approval dialog */}
      {approval && (
        <Modal
          title={t("chat.approveTitle")}
          icon="ShieldCheck"
          onClose={() => void resolveApproval(false)}
          footer={
            <div className="ml-auto flex items-center gap-2">
              <button
                onClick={() => void resolveApproval(false)}
                className="h-7 rounded-md border border-ink-700 bg-ink-850 px-3 text-[11.5px] text-ink-200 transition hover:bg-ink-750"
              >
                {t("chat.deny")}
              </button>
              <button
                onClick={() => void resolveApproval(true)}
                className="h-7 rounded-md bg-ink-50 px-3 text-[11.5px] font-medium text-ink-950 transition hover:bg-white"
              >
                {t("chat.allow")}
              </button>
            </div>
          }
        >
          <div className="space-y-3">
            <Field label={t("chat.modelWants", { action: approval.toolCall.name.replace(/_/g, " ") })}>
              <pre className="max-h-[220px] overflow-auto rounded-md border border-ink-700 bg-ink-950/60 p-3 font-mono text-[11px] text-ink-200">
                {JSON.stringify(approval.toolCall.arguments, null, 2)}
              </pre>
            </Field>
          </div>
        </Modal>
      )}

      {/* rename dialog */}
      {renameTarget && (
        <Modal
          title={t("chat.renameTitle")}
          icon="Pencil"
          size="sm"
          onClose={() => setRenameTarget(null)}
          footer={
            <div className="ml-auto flex items-center gap-2">
              <button
                onClick={() => setRenameTarget(null)}
                className="h-7 rounded-md border border-ink-700 bg-ink-850 px-3 text-[11.5px] text-ink-200 transition hover:bg-ink-750"
              >
                {t("common.cancel")}
              </button>
              <button
                onClick={() => void commitRename()}
                className="h-7 rounded-md bg-ink-50 px-3 text-[11.5px] font-medium text-ink-950 transition hover:bg-white"
              >
                {t("common.save")}
              </button>
            </div>
          }
        >
          <Field label={t("chat.rename")}>
            <input
              autoFocus
              value={renameDraft}
              onChange={(e) => setRenameDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") void commitRename();
              }}
              className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[12.5px] text-ink-100 focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
            />
          </Field>
        </Modal>
      )}
    </div>
  );
}

/** One non-tool transcript row: user or assistant bubble. */
function TranscriptMessage({
  msg,
  pipelineMode,
  onCtx,
}: {
  msg: ChatMessage;
  pipelineMode: boolean;
  onCtx: (e: React.MouseEvent, text: string) => void;
}) {
  return (
    <div className={cn("flex gap-3", msg.role === "user" ? "justify-end" : "justify-start")}>
      {msg.role !== "user" && (
        <span className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-ink-200">
          <Icon name={pipelineMode ? "Cable" : "Bot"} className="h-3.5 w-3.5" />
        </span>
      )}
      <div
        onContextMenu={(e) => onCtx(e, msg.content)}
        className={cn(
          "max-w-[80%] cursor-default rounded-2xl px-4 py-2.5 text-[13px] leading-relaxed",
          msg.role === "user" ? "bg-ink-50 text-ink-950" : "bg-ink-850 text-ink-100",
        )}
      >
        {msg.role === "user" ? msg.content : <MarkdownBody content={msg.content} />}
      </div>
    </div>
  );
}

type ToolCallEntry = { call: ChatToolCall; result?: ChatMessage };

/**
 * One tool invocation rendered inline in the transcript: name plus status
 * while running, expandable arguments and raw result once it finished.
 * Auto-expanded while the result has not arrived yet so progress is visible.
 */
function ToolCallCard({ entry }: { entry: ToolCallEntry }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(!entry.result);
  const running = !entry.result;
  const prettyArguments = useMemo(() => {
    try {
      return JSON.stringify(entry.call.arguments ?? {}, null, 2);
    } catch {
      return String(entry.call.arguments ?? "");
    }
  }, [entry.call.arguments]);
  const resultText = entry.result?.content ?? "";

  return (
    <div className="overflow-hidden rounded-xl border border-ink-700/70 bg-ink-850/50">
      <button onClick={() => setOpen((v) => !v)} className="flex w-full items-center gap-2 px-3 py-2 text-left">
        <Icon name="ChevronRight" className={cn("h-3 w-3 shrink-0 text-ink-500 transition-transform", open && "rotate-90")} />
        <Icon name="Braces" className="h-3.5 w-3.5 shrink-0 text-violet-300/80" />
        <span className="min-w-0 flex-1 truncate font-mono text-[11.5px] text-ink-200">
          {entry.call.name.replace(/_/g, " ")}
        </span>
        {running ? (
          <span className="flex shrink-0 items-center gap-1.5 text-[10.5px] text-sky-300">
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-sky-300" />
            {t("chat.toolRunning")}
          </span>
        ) : (
          <Icon name="Check" className="h-3.5 w-3.5 shrink-0 text-emerald-400/80" />
        )}
      </button>
      {open && (
        <div className="space-y-2 border-t border-seam px-3 py-2">
          <div>
            <p className="mb-1 text-[10px] uppercase tracking-[0.09em] text-ink-500">{t("chat.toolArguments")}</p>
            <pre className="max-h-[140px] overflow-auto whitespace-pre-wrap rounded-md border border-ink-700 bg-ink-950/60 px-2 py-1.5 font-mono text-[10.5px] leading-relaxed text-ink-300">
              {prettyArguments}
            </pre>
          </div>
          {entry.result && (
            <div>
              <p className="mb-1 text-[10px] uppercase tracking-[0.09em] text-ink-500">{t("chat.toolResult")}</p>
              <pre className="max-h-[220px] overflow-auto whitespace-pre-wrap rounded-md border border-ink-700 bg-ink-950/60 px-2 py-1.5 font-mono text-[10.5px] leading-relaxed text-ink-300">
                {resultText}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

const MarkdownBody = memo(function MarkdownBody({ content }: { content: string }) {
  return (
    <div className="[&_a]:text-sky-300 [&_code]:rounded [&_code]:bg-ink-800 [&_code]:px-1 [&_code]:font-mono [&_code]:text-[12px] [&_li]:ml-4 [&_li]:list-disc [&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:border [&_pre]:border-ink-700 [&_pre]:bg-ink-950/60 [&_pre]:p-3">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
    </div>
  );
});

/** Call-to-action for starting a pipeline chat from the empty state. */
function CtaButton({ children, onClick, disabled }: { children: React.ReactNode; onClick: () => void; disabled?: boolean }) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={cn(
        "mt-6 h-8 rounded-md px-4 text-[12.5px] font-medium transition",
        disabled ? "cursor-not-allowed bg-ink-800 text-ink-500" : "bg-ink-50 text-ink-950 hover:bg-white",
      )}
    >
      {children}
    </button>
  );
}
