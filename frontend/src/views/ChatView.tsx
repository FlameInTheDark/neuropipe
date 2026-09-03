import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Events } from "@wailsio/runtime";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { desktop } from "@/lib/bridge";
import { extractPayload } from "@/App";
import type {
  ChatApproval,
  ChatConversation,
  ChatMessage,
  ChatPipeline,
  ChatRun,
  ChatRunEvent,
  ChatToolCall,
  FunctionSummary,
  ProviderConfig,
} from "@/lib/types";
import { conversationGroup, formatDateTime } from "@/lib/format";
import { ask } from "@/stores/confirmation";
import { SearchInput } from "../components/ViewShell";
import { Dropdown, type DropdownOption } from "../components/Dropdown";
import { ToolsPicker, type ToolsPickerTool } from "../components/ToolsPicker";
import { Modal } from "../components/primitives/Modal";
import { Field } from "../components/primitives/Field";
import { Icon } from "../components/icons";
import { useCtxMenu } from "../components/ContextMenu";
import { Button } from "../components/ui";
import { cn } from "../utils/cn";
import { useToast } from "../hooks/useToast";
import { Toaster } from "../components/layout/Toaster";

/** rows fetched per transcript page; older history loads on demand */
const TRANSCRIPT_PAGE = 100;

const SUGGESTION_KEYS = [
  "chat.suggestion1",
  "chat.suggestion2",
  "chat.suggestion3",
  "chat.suggestion4",
];

/** Reasoning effort levels offered per conversation; "" keeps the provider default. */
const REASONING_LEVELS = ["none", "minimal", "low", "medium", "high"] as const;

/** Composite model-picker value: `""` = app default, `providerId::modelId` otherwise. */
function modelValue(providerId?: string, model?: string): string {
  return providerId ? `${providerId}::${model ?? ""}` : "";
}

function splitModelValue(value: string): { providerId: string; model: string } {
  const index = value.indexOf("::");
  if (index === -1) return { providerId: "", model: "" };
  return { providerId: value.slice(0, index), model: value.slice(index + 2) };
}

/* Survives ChatView unmounts (navigating away and back) so reopening the
   chat page restores the exact conversation that was open, including its
   live running state pulled fresh from the backend. */
let lastViewedId: string | null = null;
let lastViewedMode: "model" | "pipeline" | null = null;

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
  /** right-click menu; omitted for action rows (binding quick-starts) */
  onCtx?: (e: React.MouseEvent) => void;
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
          pipeline ? "border-violet-500/40 bg-violet-500/10 text-violet-300" : "border-ink-700 bg-ink-850 text-fg-subtle",
        )}
      >
        <Icon name={pipeline ? "Cable" : "Bot"} className="h-3 w-3" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[12.5px] leading-tight font-medium text-fg">{title}</span>
        <span className="mt-[3px] block truncate text-[11px] leading-tight text-fg-faint">{subtitle}</span>
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
        <Icon name="ChevronRight" className={cn("h-3 w-3 text-fg-faint transition-transform", open && "rotate-90")} />
        <Icon name="Activity" className={cn("h-3.5 w-3.5", active ? "animate-pulse text-info-fg" : "text-fg-subtle")} />
        <span className="min-w-0 flex-1 truncate text-[12px] text-fg-subtle">
          {run.statusText || t("chat.waiting")}
        </span>
        {active && (
          <span className="h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-info" />
        )}
      </button>
      {open && (
        <ul className="border-t border-seam px-3 py-2">
          {events.length === 0 && <li className="py-1 text-[11px] text-fg-faint">{t("chat.noActivity")}</li>}
          {events.map((ev) => (
            <li key={ev.id} className="flex items-start gap-2 py-1">
              <span
                className={cn(
                  "mt-[6px] h-1.5 w-1.5 shrink-0 rounded-full",
                  ev.status === "failed" ? "bg-danger" : ev.status === "completed" ? "bg-success/80" : "bg-ink-500",
                )}
              />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-[11.5px] text-fg-muted">{ev.summary}</span>
                {ev.detail && (
                  <pre className="mt-1 max-h-[90px] overflow-auto whitespace-pre-wrap rounded-md border border-ink-700 bg-ink-950/60 px-2 py-1 font-mono text-[10px] text-fg-subtle">
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

/* ── failed run card ── */

/** Inline error for a finished-but-unsuccessful run: collapsed shows a
 *  one-line reason, expanding reveals the full error text; retry re-runs
 *  the last user prompt as a fresh turn. */
function FailedRunCard({
  error,
  onRetry,
}: {
  error?: string;
  onRetry?: () => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const hasDetails = Boolean(error);

  return (
    <div className="rounded-xl border border-danger/30 bg-danger/10 px-3.5 py-2.5">
      <div className="flex items-center gap-2.5">
        <Icon name="AlertTriangle" className="h-3.5 w-3.5 shrink-0 text-danger-fg" />
        <button
          type="button"
          onClick={() => hasDetails && setOpen((v) => !v)}
          disabled={!hasDetails}
          aria-expanded={open}
          className={cn("min-w-0 flex-1 text-left", hasDetails ? "cursor-pointer" : "cursor-default")}
        >
          <span className="block text-[12px] text-danger-fg">{t("chat.runFailedStrip")}</span>
          {/* collapsed preview only - the expanded body below carries the
              full text, so it never renders twice */}
          {hasDetails && !open && (
            <span className="mt-0.5 block truncate font-mono text-[10.5px] text-danger-fg/70">
              {error}
            </span>
          )}
        </button>
        {hasDetails && (
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            aria-label={t("chat.toggleDetails")}
            title={t("chat.toggleDetails")}
            className="grid h-6 w-6 shrink-0 place-items-center rounded-md text-danger-fg/70 transition hover:bg-danger/15 hover:text-danger-fg"
          >
            <Icon name="ChevronDown" className={cn("h-3.5 w-3.5 transition-transform", open && "rotate-180")} />
          </button>
        )}
        {onRetry && (
          <Button variant="ghost" icon="RefreshCw" onClick={onRetry}>
            {t("chat.retry")}
          </Button>
        )}
      </div>
      {open && hasDetails && (
        <pre className="mt-2 max-h-[220px] overflow-auto whitespace-pre-wrap break-words rounded-md border border-danger/20 bg-ink-950/60 px-2 py-1.5 font-mono text-[10.5px] leading-relaxed text-danger-fg/90">
          {error}
        </pre>
      )}
    </div>
  );
}

/* ── main view ── */
export default function ChatView() {
  const { t } = useTranslation();
  const { toast, notify } = useToast();
  const [conversations, setConversations] = useState<ChatConversation[]>([]);
  const [pipelines, setPipelines] = useState<ChatPipeline[]>([]);
  const [modelLabel, setModelLabel] = useState<string | null>(null);
  const [providers, setProviders] = useState<ProviderConfig[]>([]);
  const [toolFunctions, setToolFunctions] = useState<ToolsPickerTool[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(lastViewedId);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [runs, setRuns] = useState<ChatRun[]>([]);
  const [events, setEvents] = useState<Record<string, ChatRunEvent[]>>({});
  const [approvals, setApprovals] = useState<ChatApproval[]>([]);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  /** live assistant text for the run currently streaming; keyed by run so a
   *  stale or foreign turn can never paint into the wrong transcript. Once a
   *  round finishes (chat.token.end) the draft freezes in place - it keeps
   *  rendering its own bubble until the persisted transcript row replaces
   *  it, so consecutive rounds never melt into one growing element. */
  const [liveReply, setLiveReply] = useState<{ chatRunId: string; text: string; finished?: boolean } | null>(null);
  /** optimistically painted user turn between hitting Enter and the backend
   *  round-trip: the user's message must be its own bubble immediately, not
   *  something that pops in together with the reply later. */
  const [pendingUser, setPendingUser] = useState<{ id: string; text: string; at: number } | null>(null);
  const [q, setQ] = useState("");
  const [newMode, setNewMode] = useState<"model" | "pipeline">(lastViewedMode ?? "model");
  const [renameTarget, setRenameTarget] = useState<ChatConversation | null>(null);
  const [renameDraft, setRenameDraft] = useState("");
  const bottomRef = useRef<HTMLDivElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  /** true while the user is scrolled to (or near) the conversation bottom */
  const pinnedRef = useRef(true);
  const ctx = useCtxMenu();

  const selected = conversations.find((c) => c.id === selectedId) ?? null;

  /* remember the open thread across page switches */
  useEffect(() => {
    lastViewedId = selectedId;
    lastViewedMode = newMode;
  }, [selectedId, newMode]);

  /** Pipeline chosen for a brand-new pipeline chat (before it exists). */
  const [pendingBindingId, setPendingBindingId] = useState("");
  useEffect(() => {
    if (!pendingBindingId && pipelines.length > 0) setPendingBindingId(pipelines[0].bindingId);
  }, [pipelines, pendingBindingId]);
  const newPipelineBinding = pendingBindingId || pipelines[0]?.bindingId || "";

  /** Model, reasoning, and tool selection for a brand-new model chat, applied
   *  when the first message implicitly creates the conversation. */
  const [newModelValue, setNewModelValue] = useState("");
  const [newReasoning, setNewReasoning] = useState("");
  const [newToolIds, setNewToolIds] = useState<string[]>([]);

  /* ---------- transcript ---------- */

  /**
   * Walks messages in stored order and lifts tool activity into inline
   * cards: an assistant turn carrying tool_calls is followed by its result
   * card(s), matching how the harness renders a trajectory. Orphan results
   * keep their chronological slot instead of disappearing. The not-yet-
   * persisted user turn is appended so every send is visibly its own row.
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
    if (pendingUser) {
      // drop the optimistic row as soon as the persisted copy exists
      const persisted = messages.some(
        (m) => m.role === "user" && m.content === pendingUser.text && Date.parse(m.createdAt) >= pendingUser.at,
      );
      if (!persisted) {
        items.push({
          kind: "message",
          msg: {
            id: pendingUser.id,
            conversationId: selectedId ?? "",
            role: "user",
            content: pendingUser.text,
            createdAt: new Date(pendingUser.at).toISOString(),
          },
        });
      }
    }
    return items;
  }, [messages, pendingUser, selectedId]);

  /* ---------- data loading ---------- */

  /** Sequence guard: only the newest loadDetails may commit its results,
   * so rapid conversation switches can never paint stale history. */
  const detailSeq = useRef(0);
  const [loadError, setLoadError] = useState(false);
  /** older pages exist beyond the loaded window */
  const [hasMore, setHasMore] = useState(false);
  const [loadingOlder, setLoadingOlder] = useState(false);
  /** how many transcript rows are currently kept in state */
  const loadedCount = useRef(TRANSCRIPT_PAGE);
  /** newest-first total from the last paged read; offsets anchor against it */
  const totalRef = useRef(0);

  const loadDetails = useCallback(async (conversationId: string) => {
    const ticket = ++detailSeq.current;
    setLoadError(false);
    try {
      // keep every row already on screen while live events refresh the tail;
      // offsets count backwards from the newest row, so the probe anchors the
      // window at the conversation's end and anything older is prepended.
      const want = Math.max(TRANSCRIPT_PAGE, loadedCount.current);
      const probe = await desktop.listChatMessagesPage(conversationId, 0, TRANSCRIPT_PAGE);
      let msgs = probe.messages;
      const missing = Math.min(probe.total, want) - msgs.length;
      if (missing > 0) {
        const older = await desktop.listChatMessagesPage(conversationId, TRANSCRIPT_PAGE, missing);
        if (ticket !== detailSeq.current) return;
        msgs = [...older.messages, ...msgs];
      }
      if (ticket !== detailSeq.current) return;
      totalRef.current = probe.total;
      setMessages(msgs);
      setHasMore(probe.total > msgs.length);
      loadedCount.current = msgs.length;
      const rs = await desktop.listChatRuns(conversationId);
      const aps = await desktop.listPendingChatApprovals(conversationId);
      if (ticket !== detailSeq.current) return;
      setRuns(rs);
      setApprovals(aps);
      // reconcile the live draft with the persisted transcript: the frozen
      // round text is dropped the moment its exact row exists on disk, so
      // the swap is seamless instead of text-vanishes-then-reappears. A
      // draft whose run died without producing text goes away as well.
      setLiveReply((cur) => {
        if (!cur) return null;
        if (msgs.some((m) => m.role === "assistant" && m.chatRunId === cur.chatRunId && m.content.trim() === cur.text.trim())) {
          return null;
        }
        const run = rs.find((r) => r.id === cur.chatRunId);
        if (run && (run.status === "failed" || run.status === "cancelled" || run.status === "skipped")) return null;
        return cur;
      });
      setPendingUser((cur) =>
        cur && msgs.some((m) => m.role === "user" && m.content === cur.text && Date.parse(m.createdAt) >= cur.at) ? null : cur,
      );
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
      setEvents(Object.fromEntries(eventEntries));
    } catch {
      if (ticket === detailSeq.current) setLoadError(true);
    }
  }, []);

  /** Fetches the previous page and restores the scroll offset afterwards so
      the viewport stays anchored to the message the user was reading.
      Offsets anchor against the remembered conversation total - a message
      appended by a live run shifts the newest-first window, and using the
      row count alone would overlap pages and duplicate rows. */
  const loadOlder = useCallback(async () => {
    const conversationId = selectedId;
    if (!conversationId || loadingOlder || !hasMore) return;
    const container = scrollRef.current;
    const before = container
      ? { height: container.scrollHeight, top: container.scrollTop }
      : null;
    setLoadingOlder(true);
    try {
      const ticket = detailSeq.current;
      const offset = Math.max(0, totalRef.current - loadedCount.current);
      const page = await desktop.listChatMessagesPage(conversationId, offset, TRANSCRIPT_PAGE);
      if (ticket !== detailSeq.current) return;
      totalRef.current = Math.max(totalRef.current, offset + page.messages.length);
      setHasMore(page.hasMore);
      loadedCount.current += page.messages.length;
      if (page.messages.length > 0) {
        const known = new Set(messages.map((m) => m.id));
        const fresh = page.messages.filter((m) => !known.has(m.id));
        if (fresh.length > 0) {
          setMessages((cur) => [...fresh, ...cur]);
        }
      }
      requestAnimationFrame(() => {
        const el = scrollRef.current;
        if (el && before) el.scrollTop = el.scrollHeight - before.height + before.top;
      });
    } catch {
      /* transient; the button stays for a retry */
    } finally {
      setLoadingOlder(false);
    }
  }, [selectedId, loadingOlder, hasMore, messages]);

  const refreshList = useCallback(async () => {
    const [convs, pipes, settings, functions] = await Promise.all([
      desktop.listChatConversations(),
      desktop.listChatPipelines(),
      desktop.getSettings(),
      desktop.listFunctions().catch(() => [] as FunctionSummary[]),
    ]);
    // newest conversation first regardless of backend ordering
    convs.sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
    setConversations(convs);
    setPipelines(pipes);
    setProviders(settings.providers);
    // only published LLM tool functions can be offered to the model
    setToolFunctions(
      functions
        .filter((f) => f.kind === "tool" && f.publishedRevision > 0)
        .map((f) => ({ id: f.id, name: f.name, description: f.description })),
    );
    const provider = settings.providers.find((p) => p.id === settings.defaultProviderId);
    setModelLabel(provider ? (provider.model ? `${provider.name} · ${provider.model}` : provider.name) : null);
    return convs;
  }, []);

  useEffect(() => {
    void (async () => {
      const convs = await refreshList().catch(() => [] as ChatConversation[]);
      if (convs.length === 0) return;
      /* restore the exact thread that was open before leaving the page;
         fall back to the newest within the remembered mode, then overall */
      const restored =
        (lastViewedId ? convs.find((c) => c.id === lastViewedId) : undefined) ??
        (lastViewedMode ? convs.find((c) => c.mode === lastViewedMode) : undefined) ??
        convs[0];
      setSelectedId(restored.id);
      setNewMode(restored.mode);
    })();
  }, [refreshList]);

  /* drop the previous conversation's content immediately so a slow load
     can never show history belonging to another thread */
  useEffect(() => {
    setMessages([]);
    setRuns([]);
    setApprovals([]);
    setEvents({});
    setLiveReply(null);
    setPendingUser(null);
    setHasMore(false);
    loadedCount.current = TRANSCRIPT_PAGE;
    pinnedRef.current = true;
    if (selectedId) void loadDetails(selectedId);
  }, [selectedId, loadDetails]);

  /* live updates pushed from the backend while a run executes */
  useEffect(() => {
    const offs = [
      Events.On("chat.updated", () => {
        /* the reload reconciles the live draft inside loadDetails: clearing
           it here would blank the finished round's text during the IPC
           round-trip and make turns appear to melt into one element */
        void refreshList();
        if (selectedId) void loadDetails(selectedId);
      }),
      Events.On("chat.run.updated", () => {
        if (selectedId) void loadDetails(selectedId);
      }),
      Events.On("chat.approval.requested", () => {
        if (selectedId) void loadDetails(selectedId);
      }),
      /* coalesced token deltas from the model turn in flight; a delta from a
         new run (the next tool round) replaces the finished draft entirely */
      Events.On("chat.token", (e: unknown) => {
        const p = extractPayload(e) as { chatRunId?: string; conversationId?: string; delta?: string } | null;
        if (!p?.delta || p.conversationId !== selectedId) return;
        setLiveReply((cur) =>
          cur && cur.chatRunId === p.chatRunId ? { ...cur, text: cur.text + (p.delta ?? "") } : { chatRunId: p.chatRunId ?? "", text: p.delta ?? "" },
        );
      }),
      /* the round finished streaming: freeze its text in place and pull the
         persisted transcript immediately - the draft hands over to the real
         row the moment the commit lands, with no dots in between */
      Events.On("chat.token.end", (e: unknown) => {
        const p = extractPayload(e) as { chatRunId?: string } | null;
        setLiveReply((cur) => (cur && (!p?.chatRunId || cur.chatRunId === p.chatRunId) ? { ...cur, finished: true } : cur));
        if (selectedId) void loadDetails(selectedId);
      }),
    ];
    return () => offs.forEach((off) => off());
  }, [selectedId, refreshList, loadDetails]);

  /* auto-scroll only while the user is pinned to the bottom; polling during
     a tool round must never yank the view if they scrolled up to read */
  const transcriptKey = `${selectedId}:${messages.length}:${pendingUser?.id ?? ""}:${runs.map((r) => r.status).join(",")}:${liveReply?.text.length ?? 0}`;
  useEffect(() => {
    if (!pinnedRef.current) return;
    bottomRef.current?.scrollIntoView({ behavior: "instant" });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [transcriptKey]);

  /* ---------- derived ---------- */

  const grouped = useMemo(() => {
    const groups: Record<string, ChatConversation[]> = { today: [], yesterday: [], week: [], older: [] };
    for (const c of conversations.filter((x) => x.mode === newMode)) {
      groups[conversationGroup(c.updatedAt)].push(c);
    }
    // newest first inside every group
    for (const list of Object.values(groups)) {
      list.sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
    }
    return groups;
  }, [conversations, newMode]);

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
  /** newest run overall — a failure card only makes sense while this is the
   *  latest outcome; a successful retry supersedes older failures */
  const latestRun = useMemo(
    () =>
      [...runs].sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))[0] ?? null,
    [runs],
  );
  const latestRunFailed =
    !activeRun &&
    latestRun !== null &&
    (latestRun.status === "failed" || latestRun.status === "cancelled") &&
    messages.some((m) => m.chatRunId === latestRun.id);
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
        let conv = await desktop.createChatConversation(
          newMode,
          newMode === "pipeline" ? newPipelineBinding : "",
        );
        if (newMode === "model" && (newModelValue !== "" || newReasoning !== "" || newToolIds.length > 0)) {
          // carry the pre-send picker choices onto the fresh conversation
          const { providerId, model } = splitModelValue(newModelValue);
          conv = await desktop.saveChatConversation({
            ...conv,
            providerId,
            model,
            reasoning: newReasoning,
            toolIds: newToolIds,
          });
        }
        target = conv;
        setSelectedId(conv.id);
        await refreshList();
      }
      if (!target) return;
      setSending(true);
      setDraft("");
      // paint the user's turn immediately - it must be its own bubble from
      // the first frame, not appear together with the reply later
      setPendingUser({ id: `pending-${Date.now()}`, text, at: Date.now() });
      try {
        await desktop.sendChatMessage(target.id, text);
      } catch (error) {
        // nothing was persisted: remove the optimistic row and give the
        // text back so the user can retry without retyping
        setPendingUser(null);
        setDraft(text);
        throw error;
      }
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

  /** Re-runs the last user prompt as a fresh turn after a failed run. */
  const retryLastUserMessage = useCallback(async () => {
    if (!selectedId || sending) return;
    const lastUser = [...messages].reverse().find((m) => m.role === "user");
    if (!lastUser) return;
    setSending(true);
    setPendingUser({ id: `pending-${Date.now()}`, text: lastUser.content, at: Date.now() });
    try {
      await desktop.sendChatMessage(selectedId, lastUser.content);
      await loadDetails(selectedId);
      await refreshList();
    } catch {
      setPendingUser(null);
    } finally {
      setSending(false);
    }
  }, [selectedId, sending, messages, loadDetails, refreshList]);

  const removeConversation = async (conv: ChatConversation) => {
    const ok = await ask({
      title: t("chat.deleteTitle"),
      description: t("chat.deleteDescription", { name: conv.title }),
      confirmLabel: t("chat.deleteConfirm"),
      danger: true,
    });
    if (!ok) return;
    try {
      await desktop.deleteChatConversation(conv.id);
      setSelectedId((cur) => (cur === conv.id ? null : cur));
      await refreshList();
    } catch {
      notify(t("chat.deleteFailed"), "AlertTriangle");
    }
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

  /** Persists one conversation patch (model routing, reasoning, tools). */
  const patchConversation = useCallback(
    async (conv: ChatConversation, patch: Partial<ChatConversation>) => {
      try {
        await desktop.saveChatConversation({ ...conv, ...patch });
        await refreshList();
      } catch {
        notify(t("chat.saveFailed"), "AlertTriangle");
      }
    },
    [refreshList, notify, t],
  );

  const setModelRouting = (value: string) => {
    const { providerId, model } = splitModelValue(value);
    if (selected && selected.mode === "model") void patchConversation(selected, { providerId, model });
    else setNewModelValue(value);
  };

  const setReasoning = (level: string) => {
    if (selected && selected.mode === "model") void patchConversation(selected, { reasoning: level });
    else setNewReasoning(level);
  };

  const setEnabledTools = (ids: string[]) => {
    if (selected && selected.mode === "model") void patchConversation(selected, { toolIds: ids });
    else setNewToolIds(ids);
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

  /* ---------- model / reasoning / tools pickers ---------- */

  const enabledProviders = useMemo(() => providers.filter((p) => p.enabled), [providers]);
  const providerName = useCallback(
    (id: string) => providers.find((p) => p.id === id)?.name ?? id,
    [providers],
  );

  /** Searchable model list across every enabled provider, mirroring the
   *  provider-configuration dropdown; `""` keeps the Settings default. */
  const modelOptions = useMemo<DropdownOption[]>(() => {
    const options: DropdownOption[] = [
      { value: "", label: t("chat.modelAppDefault"), hint: modelLabel ?? undefined },
    ];
    for (const provider of enabledProviders) {
      if (provider.model) {
        options.push({
          value: modelValue(provider.id, ""),
          label: `${provider.name} · ${provider.model}`,
          hint: t("chat.modelProviderDefault"),
        });
      }
      for (const model of provider.models ?? []) {
        if (model.id === provider.model) continue; // already the provider-default entry
        options.push({ value: modelValue(provider.id, model.id), label: `${provider.name} · ${model.name || model.id}` });
      }
    }
    return options;
  }, [enabledProviders, modelLabel, t]);

  const currentModelValue = selected?.mode === "model" ? modelValue(selected.providerId, selected.model) : newModelValue;
  // A saved model that is no longer configured must stay selectable so the
  // conversation never silently loses its routing (same rule as the editor).
  if (currentModelValue && !modelOptions.some((o) => o.value === currentModelValue)) {
    const { providerId: savedProvider, model: savedModel } = splitModelValue(currentModelValue);
    modelOptions.push({
      value: currentModelValue,
      label: `${providerName(savedProvider)}${savedModel ? ` · ${savedModel}` : ""} · ${t("editor.modelSaved")}`,
    });
  }

  const reasoningOptions = useMemo<DropdownOption[]>(
    () => [
      { value: "", label: t("chat.reasoningDefault") },
      ...REASONING_LEVELS.map((level) => ({ value: level, label: t(`chat.reasoning_${level}`) })),
    ],
    [t],
  );
  const currentReasoning = selected?.mode === "model" ? (selected.reasoning ?? "") : newReasoning;
  const currentToolIds = selected?.mode === "model" ? (selected.toolIds ?? []) : newToolIds;

  /** Header chip label: the conversation's resolved routing or the default. */
  const headerModelLabel = useMemo(() => {
    if (!selected || selected.mode !== "model") return modelLabel;
    if (!selected.providerId) return modelLabel;
    const provider = providers.find((p) => p.id === selected.providerId);
    if (!provider) return null;
    const model = selected.model || provider.model;
    return model ? `${provider.name} · ${model}` : provider.name;
  }, [selected, providers, modelLabel]);

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
            className="flex w-full items-center justify-center gap-1.5 rounded-md bg-ink-50 py-1.5 text-[12.5px] font-medium text-fg-onEmphasis transition hover:bg-ink-25"
          >
            <Icon name="Plus" className="h-3.5 w-3.5" />
            {t("chat.new")}
          </button>
          {/* model / pipeline switch — always reachable so either history is
              one click away without creating anything */}
          <div className="flex items-center gap-0.5 rounded-md border border-ink-700 bg-ink-850 p-0.5">
            {(["model", "pipeline"] as const).map((m) => (
              <button
                key={m}
                onClick={() => {
                  setNewMode(m);
                  if (selected && selected.mode !== m) setSelectedId(null);
                }}
                aria-pressed={newMode === m}
                className={cn(
                  "flex h-[26px] flex-1 items-center justify-center gap-1.5 rounded px-2 text-[11.5px] transition",
                  newMode === m ? "bg-ink-700 text-fg" : "text-fg-subtle hover:text-fg",
                )}
              >
                <Icon name={m === "model" ? "Bot" : "Cable"} className="h-3 w-3" />
                {m === "model" ? t("chat.model") : t("chat.pipelines")}
              </button>
            ))}
          </div>
          <SearchInput value={q} onChange={setQ} placeholder={t("chat.searchChats")} />
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-1.5 pb-2">
          {visibleGroups.map((group) => (
            <div key={group.key}>
              <p className="mb-1 px-2 py-1.5 text-[10px] font-medium tracking-[0.09em] text-fg-faint uppercase">
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

          {loadError && conversations.length === 0 && (
            <p className="flex items-center justify-center gap-1.5 px-2 py-4 text-center text-[12px] text-danger-fg">
              <Icon name="AlertTriangle" className="h-3.5 w-3.5 shrink-0" />
              {t("chat.loadFailed")}
            </p>
          )}

          {!loadError && conversations.length === 0 && (
            <p className="px-2 py-4 text-center text-[12px] text-fg-faint">{t("chat.noChats")}</p>
          )}
        </div>
      </aside>

      {/* ── main area ── */}
      <div className="flex min-w-0 flex-1 flex-col">
        {/* header */}
        <div className="flex h-11 shrink-0 items-center gap-2 border-b border-seam px-4">
          {selected ? (
            <>
              <h2 className="truncate text-[13.5px] font-semibold text-fg">{selected.title}</h2>
              <div className="ml-auto flex items-center gap-2">
                <span className="flex items-center gap-1.5 rounded-md border border-ink-700 bg-ink-850 px-2 py-1 text-[11px] text-fg-subtle">
                  <Icon name={selected.mode === "pipeline" ? "Cable" : "Bot"} className="h-3 w-3 shrink-0 text-fg-faint" />
                  {selected.mode === "pipeline"
                    ? headerPipeline ?? t("chat.publishedPipeline")
                    : headerModelLabel ?? t("chat.configureModel")}
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
                  className="grid h-7 w-7 place-items-center rounded-md text-fg-faint transition hover:bg-ink-800 hover:text-fg"
                >
                  <Icon name="Pencil" className="h-[15px] w-[15px]" />
                </button>
                <button
                  onClick={() => void removeConversation(selected)}
                  title={t("chat.delete")}
                  className="grid h-7 w-7 place-items-center rounded-md text-fg-faint transition hover:bg-ink-800 hover:text-danger-fg"
                >
                  <Icon name="Trash2" className="h-[15px] w-[15px]" />
                </button>
              </div>
            </>
          ) : (
            <h2 className="text-[13.5px] font-semibold text-fg">
              {newMode === "pipeline" ? t("chat.chatWithPipeline") : t("chat.new")}
            </h2>
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
              {hasMore && messages.length > 0 && (
                <div className="flex justify-center">
                  <button
                    onClick={() => void loadOlder()}
                    disabled={loadingOlder}
                    className="rounded-full border border-ink-700 bg-ink-850 px-3.5 py-1.5 text-[11.5px] text-fg-subtle transition hover:border-ink-500 hover:text-fg disabled:opacity-50"
                  >
                    {loadingOlder ? t("common.loading") : t("chat.loadEarlier")}
                  </button>
                </div>
              )}

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
                    {item.calls.map((entry, callIndex) => (
                      <ToolCallCard key={`${item.key}:${callIndex}:${entry.call.id || entry.result?.id || "call"}`} entry={entry} />
                    ))}
                  </div>
                ) : (
                  <TranscriptMessage key={item.msg.id} msg={item.msg} pipelineMode={selected.mode === "pipeline"} onCtx={onBubbleCtx} />
                ),
              )}

              {/* a failed run must never look like the assistant simply
                  stopped talking: surface its error inline with retry, but
                  only while it is still the newest outcome - a successful
                  retry supersedes older failures */}
              {latestRunFailed && latestRun && (
                <FailedRunCard
                  key={`err-${latestRun.id}`}
                  error={latestRun.error}
                  onRetry={
                    selected && messages.some((m) => m.role === "user")
                      ? () => void retryLastUserMessage()
                      : undefined
                  }
                />
              )}

              {/* live token stream replaces the idle dots once the first
                  deltas land; a finished round freezes in place (caret off)
                  until the persisted row takes over via the commit reconcile */}
              {liveReply && liveReply.text !== "" ? (
                <LiveReplyBubble text={liveReply.text} pipelineMode={selected.mode === "pipeline"} caret={!liveReply.finished} />
              ) : (
                (activeRun || sending) && (
                  <div className="flex gap-3">
                    <span className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-fg-muted">
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
                )
              )}

              {activeRun && (
                <div className="flex justify-center">
                  <button
                    onClick={() => void cancelRun()}
                    className="flex items-center gap-1.5 rounded-full border border-ink-600 bg-ink-800 px-3 py-1.5 text-[11.5px] text-fg-muted transition hover:border-danger/40 hover:text-danger-fg"
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
                  "grid h-12 w-12 place-items-center rounded-2xl border text-fg-muted",
                  newMode === "pipeline" ? "border-violet-500/40 bg-violet-500/10 text-violet-300" : "border-ink-700 bg-ink-850",
                )}
              >
                <Icon name={newMode === "pipeline" ? "Cable" : "Sparkles"} className="h-5 w-5" />
              </span>
              <h2 className="mt-3 text-[15px] font-semibold text-fg">
                {newMode === "pipeline" ? t("chat.chatWithPipeline") : t("chat.emptyPrompt")}
              </h2>
              <p className="mt-1 max-w-[340px] text-center text-[12px] leading-relaxed text-fg-faint">
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
                      className="group flex w-full items-center gap-2 rounded-xl border border-ink-700/80 bg-ink-850/60 px-4 py-2.5 text-left text-[13px] text-fg-subtle transition hover:border-ink-500 hover:bg-ink-800 hover:text-fg"
                    >
                      <span className="flex-1">{t(key)}</span>
                      <Icon name="ArrowUpRight" className="h-3.5 w-3.5 text-fg-faint group-hover:text-fg-subtle" />
                    </button>
                  ))}
                </div>
              )}

              {newMode === "pipeline" && pipelines.length > 0 && (
                <Dropdown
                  className="mt-6 w-full max-w-[300px]"
                  value={newPipelineBinding}
                  onChange={setPendingBindingId}
                  options={pipelines.map((p) => ({ value: p.bindingId, label: p.pipelineName || p.label }))}
                />
              )}

              {newMode === "pipeline" && (
                <CtaButton onClick={() => void createConversation("pipeline", newPipelineBinding)} disabled={pipelines.length === 0}>
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
                className="w-full resize-none bg-transparent px-4 py-3 text-[13px] text-fg placeholder:text-fg-faint"
              />
              <div className="flex items-center gap-2 px-3 pb-2.5">
                {mode === "pipeline" ? (
                  selected?.mode === "pipeline" || pipelines.length === 0 ? (
                    <span className="flex items-center gap-1.5 text-[11px] text-violet-400/70">
                      <Icon name="Cable" className="h-3 w-3" />
                      {selected?.mode === "pipeline"
                        ? headerPipeline
                        : t("chat.noPipelinesHint")}
                    </span>
                  ) : (
                    <Dropdown
                      compact
                      value={newPipelineBinding}
                      onChange={setPendingBindingId}
                      options={pipelines.map((p) => ({ value: p.bindingId, label: p.pipelineName || p.label }))}
                    />
                  )
                ) : (
                  <>
                    {/* model routing: searchable dropdown over every enabled
                        provider's models, like provider configuration */}
                    <Dropdown
                      compact
                      searchable
                      searchPlaceholder={t("common.searchModels")}
                      className="max-w-[240px]"
                      value={currentModelValue}
                      onChange={setModelRouting}
                      options={modelOptions}
                      placeholder={t("chat.configureModel")}
                    />
                    <Dropdown
                      compact
                      className="max-w-[130px]"
                      value={currentReasoning}
                      onChange={setReasoning}
                      options={reasoningOptions}
                    />
                    <ToolsPicker tools={toolFunctions} enabled={currentToolIds} onChange={setEnabledTools} />
                  </>
                )}

                <span className="ml-auto text-[10.5px] text-fg-faint">{t("chat.enterToSend")}</span>
                <button
                  onClick={() => void send()}
                  disabled={!draft.trim() || sending}
                  aria-label={t("chat.send")}
                  className={cn(
                    "grid h-7 w-7 shrink-0 place-items-center rounded-xl transition",
                    draft.trim() && !sending
                      ? mode === "pipeline"
                        ? "bg-violet-500 text-white hover:bg-violet-400"
                        : "bg-ink-50 text-fg-onEmphasis hover:bg-ink-25"
                      : "cursor-not-allowed bg-ink-800 text-fg-faint",
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
                className="h-7 rounded-md border border-ink-700 bg-ink-850 px-3 text-[11.5px] text-fg-muted transition hover:bg-ink-750"
              >
                {t("chat.deny")}
              </button>
              <button
                onClick={() => void resolveApproval(true)}
                className="h-7 rounded-md bg-ink-50 px-3 text-[11.5px] font-medium text-fg-onEmphasis transition hover:bg-ink-25"
              >
                {t("chat.allow")}
              </button>
            </div>
          }
        >
          <div className="space-y-3">
            <Field label={t("chat.modelWants", { action: approval.toolCall.name.replace(/_/g, " ") })}>
              <pre className="max-h-[220px] overflow-auto rounded-md border border-ink-700 bg-ink-950/60 p-3 font-mono text-[11px] text-fg-muted">
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
                className="h-7 rounded-md border border-ink-700 bg-ink-850 px-3 text-[11.5px] text-fg-muted transition hover:bg-ink-750"
              >
                {t("common.cancel")}
              </button>
              <button
                onClick={() => void commitRename()}
                className="h-7 rounded-md bg-ink-50 px-3 text-[11.5px] font-medium text-fg-onEmphasis transition hover:bg-ink-25"
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
              className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[12.5px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
            />
          </Field>
        </Modal>
      )}
        {toast && <Toaster toast={toast} />}
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
        <span className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-fg-muted">
          <Icon name={pipelineMode ? "Cable" : "Bot"} className="h-3.5 w-3.5" />
        </span>
      )}
      <div
        onContextMenu={(e) => onCtx(e, msg.content)}
        className={cn(
          "max-w-[80%] cursor-default rounded-2xl px-4 py-2.5 text-[13px] leading-relaxed",
          msg.role === "user" ? "bg-ink-50 text-fg-onEmphasis" : "bg-ink-850 text-fg",
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
        <Icon name="ChevronRight" className={cn("h-3 w-3 shrink-0 text-fg-faint transition-transform", open && "rotate-90")} />
        <Icon name="Braces" className="h-3.5 w-3.5 shrink-0 text-violet-300/80" />
        <span className="min-w-0 flex-1 truncate font-mono text-[11.5px] text-fg-muted">
          {entry.call.name.replace(/_/g, " ")}
        </span>
        {running ? (
          <span className="flex shrink-0 items-center gap-1.5 text-[10.5px] text-info-fg">
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-info" />
            {t("chat.toolRunning")}
          </span>
        ) : (
          <Icon name="Check" className="h-3.5 w-3.5 shrink-0 text-success-fg/80" />
        )}
      </button>
      {open && (
        <div className="space-y-2 border-t border-seam px-3 py-2">
          <div>
            <p className="mb-1 text-[10px] uppercase tracking-[0.09em] text-fg-faint">{t("chat.toolArguments")}</p>
            <pre className="max-h-[140px] overflow-auto whitespace-pre-wrap rounded-md border border-ink-700 bg-ink-950/60 px-2 py-1.5 font-mono text-[10.5px] leading-relaxed text-fg-subtle">
              {prettyArguments}
            </pre>
          </div>
          {entry.result && (
            <div>
              <p className="mb-1 text-[10px] uppercase tracking-[0.09em] text-fg-faint">{t("chat.toolResult")}</p>
              <pre className="max-h-[220px] overflow-auto whitespace-pre-wrap rounded-md border border-ink-700 bg-ink-950/60 px-2 py-1.5 font-mono text-[10.5px] leading-relaxed text-fg-subtle">
                {resultText}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

const MarkdownBody = memo(function MarkdownBody({ content, caret }: { content: string; caret?: boolean }) {
  return (
    <div
      className={cn(
        "[&_a]:text-info-fg [&_code]:rounded [&_code]:bg-ink-800 [&_code]:px-1 [&_code]:font-mono [&_code]:text-[12px] [&_li]:ml-4 [&_li]:list-disc [&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:border [&_pre]:border-ink-700 [&_pre]:bg-ink-950/60 [&_pre]:p-3",
        /* streaming mode appends the blinking .live-caret block */
        caret && "live-caret",
      )}
    >
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
    </div>
  );
});

/** Assistant bubble while the model streams: formatted markdown plus a
 *  blinking caret at the end of the text. Once the round finishes the caret
 *  stops and the bubble freezes until the persisted transcript row replaces
 *  it, so consecutive rounds never collapse into one growing element. */
function LiveReplyBubble({ text, pipelineMode, caret = true }: { text: string; pipelineMode: boolean; caret?: boolean }) {
  return (
    <div className="flex gap-3">
      <span className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-fg-muted">
        <Icon name={pipelineMode ? "Cable" : "Bot"} className="h-3.5 w-3.5" />
      </span>
      <div className="max-w-[80%] cursor-default rounded-2xl bg-ink-850 px-4 py-2.5 text-[13px] leading-relaxed text-fg">
        <MarkdownBody content={text} caret={caret} />
      </div>
    </div>
  );
}

/** Call-to-action for starting a pipeline chat from the empty state. */
function CtaButton({ children, onClick, disabled }: { children: React.ReactNode; onClick: () => void; disabled?: boolean }) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={cn(
        "mt-6 h-8 rounded-md px-4 text-[12.5px] font-medium transition",
        disabled ? "cursor-not-allowed bg-ink-800 text-fg-faint" : "bg-ink-50 text-fg-onEmphasis hover:bg-ink-25",
      )}
    >
      {children}
    </button>
  );
}
