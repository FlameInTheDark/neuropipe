import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { Database, KVCommandResult, KVKey, KVKeyValue, KVServerInfo, TriggerBinding } from "@/lib/types";
import { desktop } from "@/lib/bridge";
import { usePersistedChoice, usePersistedValue } from "@/lib/prefs";
import { buildKeyTree, type KeyTreeNode } from "@/lib/kvKeyTree";
import { ask } from "@/stores/confirmation";
import { Button } from "@/components/ui";
import { Icon } from "@/components/icons";
import { Dropdown } from "@/components/Dropdown";
import { Tooltip } from "@/components/Tooltip";
import { useCtxMenu } from "@/components/ContextMenu";
import { TextInput } from "@/components/primitives/Field";
import { Modal, ModalActions } from "@/components/primitives/Modal";
import { cn } from "@/utils/cn";

const KEY_TYPES = ["", "string", "hash", "list", "set", "zset", "stream"] as const;

const DANGEROUS_COMMANDS = new Set([
  "FLUSHALL", "FLUSHDB", "CONFIG", "SHUTDOWN", "DEBUG", "SCRIPT", "ACL", "CLIENT",
  "CLUSTER", "SLAVEOF", "REPLICAOF", "MODULE", "SAVE", "BGSAVE", "BGREWRITEAOF",
  "FAILOVER", "RESET", "SWAPDB", "MIGRATE", "RESTORE",
]);

/** Splits a console line into command + shell-style quoted arguments. */
function tokenizeConsole(line: string): string[] {
  const result: string[] = [];
  let current = "";
  let quote: string | null = null;
  for (const character of line) {
    if (quote) {
      if (character === quote) quote = null;
      else current += character;
    } else if (character === '"' || character === "'") {
      quote = character;
    } else if (/\s/.test(character)) {
      if (current) result.push(current);
      current = "";
    } else {
      current += character;
    }
  }
  if (current) result.push(current);
  return result;
}

function formatTTL(ttl: number, t: (key: string) => string): string {
  if (ttl === -1) return t("kv.ttlNone");
  if (ttl === -2) return t("kv.ttlGone");
  if (ttl < 60) return `${ttl}s`;
  if (ttl < 3600) return `${Math.floor(ttl / 60)}m ${ttl % 60}s`;
  if (ttl < 86400) return `${Math.floor(ttl / 3600)}h ${Math.floor((ttl % 3600) / 60)}m`;
  return `${Math.floor(ttl / 86400)}d ${Math.floor((ttl % 86400) / 3600)}h`;
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return "";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
}

const TYPE_STYLES: Record<string, string> = {
  string: "bg-fuchsia-400/15 text-fuchsia-300",
  hash: "bg-sky-400/15 text-sky-300",
  list: "bg-amber-400/15 text-amber-300",
  set: "bg-emerald-400/15 text-emerald-300",
  zset: "bg-violet-400/15 text-violet-300",
  stream: "bg-rose-400/15 text-rose-300",
  none: "bg-ink-700/50 text-ink-500",
};

/**
 * The Redis-protocol counterpart of the SQL schema/data/query tabs:
 * a cursor-paged key browser, a command console, and server info.
 */
export function KVBrowser({ database }: { database: Database }) {
  const { t } = useTranslation();
  const [tab, setTab] = useState<"keys" | "console" | "info">("keys");

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="flex items-center gap-2">
        <div className="flex items-center gap-0.5 rounded-lg border border-ink-700 bg-ink-900 p-0.5">
          {([
            { id: "keys", label: t("kv.tabKeys"), icon: "List" },
            { id: "console", label: t("kv.tabConsole"), icon: "Terminal" },
            { id: "info", label: t("kv.tabInfo"), icon: "Info" },
          ] as const).map((entry) => (
            <button
              key={entry.id}
              onClick={() => setTab(entry.id)}
              aria-pressed={tab === entry.id}
              className={cn(
                "flex h-7 items-center gap-1.5 rounded-md px-3 text-[11.5px] transition",
                tab === entry.id ? "bg-ink-700 text-ink-50" : "text-ink-400 hover:text-ink-100",
              )}
            >
              <Icon name={entry.icon} className="h-3 w-3" />
              {entry.label}
            </button>
          ))}
        </div>
        <span className="ml-auto font-mono text-[10.5px] text-ink-500">
          {database.host || database.address}:{database.port || 6379} · db {database.dbIndex ?? 0}
        </span>
      </div>

      {tab === "keys" && <KeysTab database={database} />}
      {tab === "console" && <ConsoleTab database={database} />}
      {tab === "info" && <InfoTab database={database} />}
    </div>
  );
}

/* ---------------- Keys tab ---------------- */

function KeysTab({ database }: { database: Database }) {
  const { t } = useTranslation();
  const [pattern, setPattern] = useState("*");
  const [typeFilter, setTypeFilter] = useState("");
  const [keys, setKeys] = useState<KVKey[]>([]);
  const [cursor, setCursor] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [reloadToken, setReloadToken] = useState(0);
  const [view, setView] = usePersistedChoice<"list" | "grouped">(
    "neuropipe.kv.keysView.v1",
    ["list", "grouped"],
    "list",
  );
  const [separator, setSeparator] = usePersistedValue("neuropipe.kv.keysSeparator.v1", ":");
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const tree = useMemo(() => buildKeyTree(keys, separator), [keys, separator]);
  const ctx = useCtxMenu();

  const loadFirst = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const page = await desktop.kvScanKeys(database.id, { cursor: 0, match: pattern || "*", type: typeFilter || undefined, count: 100 });
      setKeys(page.keys ?? []);
      setCursor(page.nextCursor);
      setHasMore(page.nextCursor !== 0);
    } catch (e) {
      setKeys([]);
      setHasMore(false);
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [database.id, pattern, typeFilter]);

  useEffect(() => {
    setSelected(null);
    void loadFirst();
  }, [loadFirst, reloadToken]);

  const loadMore = async () => {
    if (loading || !hasMore) return;
    setLoading(true);
    try {
      const page = await desktop.kvScanKeys(database.id, { cursor, match: pattern || "*", type: typeFilter || undefined, count: 100 });
      setKeys((prev) => [...prev, ...(page.keys ?? [])]);
      setCursor(page.nextCursor);
      setHasMore(page.nextCursor !== 0);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  const deleteKey = async (key: KVKey) => {
    const ok = await ask({
      title: t("kv.deleteKeyTitle"),
      description: t("kv.deleteKeyDescription", { name: key.name }),
      confirmLabel: t("common.delete"),
      danger: true,
    });
    if (!ok) return;
    await desktop.kvDeleteKeys(database.id, [key.name]).catch(() => undefined);
    setSelected(null);
    setReloadToken((token) => token + 1);
  };

  const onKeyContext = (e: React.MouseEvent, key: KVKey) =>
    ctx(e, [
      { label: t("kv.copyKey"), icon: "Copy", onSelect: () => navigator.clipboard?.writeText(key.name) },
      { type: "sep" },
      {
        label: t("common.delete"),
        icon: "Trash2",
        danger: true,
        onSelect: () => void deleteKey(key),
      },
    ]);

  return (
    <div className="flex min-h-0 flex-1 gap-3">
      <div className="flex min-w-0 flex-1 flex-col gap-2 overflow-hidden rounded-xl border border-ink-700/80 bg-ink-900/60">
        <div className="flex items-center gap-2 border-b border-seam px-3 py-2">
          <SearchPrefixInput value={pattern} onChange={setPattern} placeholder={t("kv.patternPlaceholder")} />
          <Dropdown
            value={typeFilter}
            onChange={setTypeFilter}
            className="w-[130px]"
            placeholder={t("kv.typeAny")}
            options={KEY_TYPES.map((type) => ({
              value: type,
              label: type === "" ? t("kv.typeAny") : t(`kv.type.${type}`),
            }))}
          />
          <div className="flex items-center gap-0.5 rounded-lg border border-ink-700 bg-ink-900 p-0.5">
            {([
              { id: "list", icon: "List", label: t("kv.viewList") },
              { id: "grouped", icon: "ListTree", label: t("kv.viewGrouped") },
            ] as const).map((entry) => (
              <Tooltip key={entry.id} content={entry.label} side="bottom" delay={200}>
                <button
                  onClick={() => setView(entry.id)}
                  aria-pressed={view === entry.id}
                  aria-label={entry.label}
                  className={cn(
                    "flex h-6 w-7 items-center justify-center rounded-md transition",
                    view === entry.id ? "bg-ink-700 text-ink-50" : "text-ink-400 hover:text-ink-100",
                  )}
                >
                  <Icon name={entry.icon} className="h-3.5 w-3.5" />
                </button>
              </Tooltip>
            ))}
          </div>
          {view === "grouped" && (
            <input
              value={separator}
              onChange={(event) => setSeparator(event.target.value)}
              placeholder=":"
              maxLength={8}
              aria-label={t("kv.separatorLabel")}
              className="h-8 w-14 shrink-0 rounded-md border border-ink-700 bg-ink-850 px-2 text-center font-mono text-[12px] text-ink-100 focus:border-ink-400 focus:outline-none"
            />
          )}
          <Button variant="ghost" icon="RefreshCw" disabled={loading} onClick={() => setReloadToken((token) => token + 1)}>
            {t("common.refresh")}
          </Button>
          <span className="ml-auto font-mono text-[10.5px] text-ink-500">
            {loading ? t("common.loading") : t("kv.keyCount", { count: keys.length })}
          </span>
        </div>
        {error && (
          <p className="mx-3 rounded-md border border-rose-500/20 bg-rose-500/5 px-2.5 py-2 font-mono text-[10.5px] text-rose-200">
            {error}
          </p>
        )}
        <div className="min-h-0 flex-1 overflow-y-auto">
          {keys.length === 0 && !loading && !error ? (
            <p className="px-3 py-3 text-[12px] text-ink-500">{t("kv.noKeys")}</p>
          ) : view === "grouped" ? (
            <KeyTreeRows
              nodes={tree}
              depth={0}
              expanded={expanded}
              onToggle={(path) => setExpanded((prev) => ({ ...prev, [path]: !prev[path] }))}
              selected={selected}
              onSelect={setSelected}
              onContext={onKeyContext}
            />
          ) : (
            keys.map((key) => (
              <button
                key={key.name}
                onClick={() => setSelected(key.name)}
                onContextMenu={(e) => onKeyContext(e, key)}
                className={cn(
                  "flex w-full items-center gap-2.5 border-b border-seam/60 px-3 py-[7px] text-left transition hover:bg-ink-850",
                  selected === key.name && "bg-ink-800/70",
                )}
              >
                <span className={cn("shrink-0 rounded px-1.5 py-px font-mono text-[9.5px] uppercase", TYPE_STYLES[key.type] ?? TYPE_STYLES.none)}>
                  {key.type}
                </span>
                <span className="min-w-0 flex-1 truncate font-mono text-[11.5px] text-ink-100">{key.name}</span>
                <span className="shrink-0 font-mono text-[10px] text-ink-500">{formatTTL(key.ttl, t)}</span>
                {(key.size ?? 0) > 0 && <span className="shrink-0 font-mono text-[10px] text-ink-600">{formatBytes(key.size ?? 0)}</span>}
              </button>
            ))
          )}
          {hasMore && (
            <button
              onClick={() => void loadMore()}
              disabled={loading}
              className="w-full border-t border-seam px-3 py-2 text-[11.5px] text-ink-400 transition hover:bg-ink-850 hover:text-ink-100 disabled:opacity-50"
            >
              {loading ? t("common.loading") : t("kv.loadMore")}
            </button>
          )}
        </div>
      </div>
      {selected && <ValuePanel database={database} keyName={selected} onDeleted={() => setReloadToken((token) => token + 1)} />}
    </div>
  );
}

function KeyTreeRows({
  nodes,
  depth,
  expanded,
  onToggle,
  selected,
  onSelect,
  onContext,
}: {
  nodes: KeyTreeNode[];
  depth: number;
  expanded: Record<string, boolean>;
  onToggle: (path: string) => void;
  selected: string | null;
  onSelect: (name: string) => void;
  onContext: (e: React.MouseEvent, key: KVKey) => void;
}) {
  const { t } = useTranslation();
  return (
    <>
      {nodes.map((node) =>
        node.kind === "folder" ? (
          <div key={`folder:${node.path}`}>
            <button
              onClick={() => onToggle(node.path)}
              aria-expanded={expanded[node.path] ?? false}
              className="flex w-full items-center gap-1.5 border-b border-seam/60 py-[7px] pr-3 text-left transition hover:bg-ink-850"
              style={{ paddingLeft: 12 + depth * 14 }}
            >
              <Icon
                name={expanded[node.path] ? "ChevronDown" : "ChevronRight"}
                className="h-3 w-3 shrink-0 text-ink-500"
              />
              <Icon name="FolderOpen" className="h-3.5 w-3.5 shrink-0 text-amber-300/70" />
              <span className="min-w-0 flex-1 truncate font-mono text-[11.5px] text-ink-200">
                {node.name || t("kv.emptySegment")}
              </span>
              <span className="shrink-0 font-mono text-[10px] text-ink-500">{node.count}</span>
            </button>
            {(expanded[node.path] ?? false) && (
              <KeyTreeRows
                nodes={node.children}
                depth={depth + 1}
                expanded={expanded}
                onToggle={onToggle}
                selected={selected}
                onSelect={onSelect}
                onContext={onContext}
              />
            )}
          </div>
        ) : (
          <button
            key={`key:${node.key.name}`}
            onClick={() => onSelect(node.key.name)}
            onContextMenu={(e) => onContext(e, node.key)}
            className={cn(
              "flex w-full items-center gap-2.5 border-b border-seam/60 py-[7px] pr-3 text-left transition hover:bg-ink-850",
              selected === node.key.name && "bg-ink-800/70",
            )}
            style={{ paddingLeft: 12 + depth * 14 }}
          >
            <span
              className={cn(
                "shrink-0 rounded px-1.5 py-px font-mono text-[9.5px] uppercase",
                TYPE_STYLES[node.key.type] ?? TYPE_STYLES.none,
              )}
            >
              {node.key.type}
            </span>
            <span className="min-w-0 flex-1 truncate font-mono text-[11.5px] text-ink-100">
              {node.name || t("kv.emptySegment")}
            </span>
            <span className="shrink-0 font-mono text-[10px] text-ink-500">{formatTTL(node.key.ttl, t)}</span>
            {(node.key.size ?? 0) > 0 && (
              <span className="shrink-0 font-mono text-[10px] text-ink-600">{formatBytes(node.key.size ?? 0)}</span>
            )}
          </button>
        ),
      )}
    </>
  );
}

function SearchPrefixInput({ value, onChange, placeholder }: { value: string; onChange: (value: string) => void; placeholder: string }) {
  return (
    <div className="relative min-w-0 flex-1">
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 font-mono text-[12px] text-ink-100 focus:border-ink-400 focus:outline-none"
      />
    </div>
  );
}

/* ---------------- Value panel ---------------- */

function ValuePanel({ database, keyName, onDeleted }: { database: Database; keyName: string; onDeleted: () => void }) {
  const { t } = useTranslation();
  const [value, setValue] = useState<KVKeyValue | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [ttlDraft, setTtlDraft] = useState("");
  const [ttlEditing, setTtlEditing] = useState(false);
  const ctx = useCtxMenu();

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setValue(await desktop.kvKeyValue(database.id, keyName));
    } catch (e) {
      setValue(null);
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [database.id, keyName]);

  useEffect(() => {
    setTtlEditing(false);
    void load();
  }, [load]);

  const applyTTL = async (seconds: number) => {
    await desktop.kvSetTTL(database.id, keyName, seconds).catch(() => undefined);
    setTtlEditing(false);
    void load();
  };

  const deleteKey = async () => {
    const ok = await ask({
      title: t("kv.deleteKeyTitle"),
      description: t("kv.deleteKeyDescription", { name: keyName }),
      confirmLabel: t("common.delete"),
      danger: true,
    });
    if (!ok) return;
    await desktop.kvDeleteKeys(database.id, [keyName]).catch(() => undefined);
    onDeleted();
  };

  const onContext = (e: React.MouseEvent) =>
    ctx(e, [
      { label: t("kv.copyKey"), icon: "Copy", onSelect: () => navigator.clipboard?.writeText(keyName) },
      { label: t("kv.copyValue"), icon: "Copy", onSelect: () => navigator.clipboard?.writeText(valuePreview(value?.value)) },
      { type: "sep" },
      { label: t("common.delete"), icon: "Trash2", danger: true, onSelect: () => void deleteKey() },
    ]);

  return (
    <div onContextMenu={onContext} className="flex w-[420px] shrink-0 flex-col gap-2 overflow-hidden rounded-xl border border-ink-700/80 bg-ink-900/60">
      <div className="flex items-center gap-2 border-b border-seam px-3 py-2">
        <span className={cn("shrink-0 rounded px-1.5 py-px font-mono text-[9.5px] uppercase", TYPE_STYLES[value?.type ?? "none"] ?? TYPE_STYLES.none)}>
          {value?.type ?? "…"}
        </span>
        <span className="min-w-0 flex-1 truncate font-mono text-[11.5px] text-ink-100" title={keyName}>
          {keyName}
        </span>
        <Button variant="ghost" icon="RefreshCw" disabled={loading} onClick={() => { void load(); }}><span /></Button>
        <Button variant="ghost" icon="Trash2" className="hover:text-rose-300" onClick={() => { void deleteKey(); }}><span /></Button>
      </div>

      {error ? (
        <p className="mx-3 rounded-md border border-rose-500/20 bg-rose-500/5 px-2.5 py-2 font-mono text-[10.5px] text-rose-200">{error}</p>
      ) : loading || !value ? (
        <p className="px-3 py-3 text-[12px] text-ink-500">{t("common.loading")}</p>
      ) : value.type === "none" ? (
        <p className="px-3 py-3 text-[12px] text-ink-500">{t("kv.keyGone")}</p>
      ) : (
        <>
          <div className="min-h-0 flex-1 overflow-y-auto px-3 py-2">
            <TypedValue value={value} />
          </div>
          <div className="border-t border-seam px-3 py-2">
            {ttlEditing ? (
              <div className="flex items-center gap-1.5">
                <TextInput value={ttlDraft} onChange={setTtlDraft} type="number" placeholder="3600" className="h-7" />
                <Button
                  variant="solid"
                  onClick={() => {
                    const parsed = Number(ttlDraft);
                    if (Number.isFinite(parsed)) void applyTTL(Math.trunc(parsed));
                  }}
                >
                  {t("common.save")}
                </Button>
                <Button variant="ghost" onClick={() => setTtlEditing(false)}>
                  {t("common.cancel")}
                </Button>
              </div>
            ) : (
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="font-mono text-[10.5px] text-ink-500">
                  {t("kv.ttlLabel")}: {formatTTL(value.ttl, t)}
                </span>
                <Button variant="ghost" className="h-6 px-1.5 text-[10.5px]" onClick={() => { setTtlDraft(value.ttl > 0 ? String(value.ttl) : ""); setTtlEditing(true); }}>
                  {t("kv.editTtl")}
                </Button>
                <span className="text-ink-700">·</span>
                <Button variant="ghost" className="h-6 px-1.5 text-[10.5px]" onClick={() => void applyTTL(60)}>
                  60s
                </Button>
                <Button variant="ghost" className="h-6 px-1.5 text-[10.5px]" onClick={() => void applyTTL(3600)}>
                  1h
                </Button>
                <Button variant="ghost" className="h-6 px-1.5 text-[10.5px]" onClick={() => void applyTTL(86400)}>
                  24h
                </Button>
                <Button variant="ghost" className="h-6 px-1.5 text-[10.5px]" onClick={() => void applyTTL(-1)}>
                  {t("kv.removeTtl")}
                </Button>
                {value.truncated && <span className="ml-auto text-[10px] text-amber-300/80">{t("kv.truncated")}</span>}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}

function valuePreview(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function TypedValue({ value }: { value: KVKeyValue }) {
  const { t } = useTranslation();
  if (value.type === "string") {
    const text = typeof value.value === "string" ? value.value : valuePreview(value.value);
    const isJson = (() => {
      try {
        const parsed = JSON.parse(text);
        return typeof parsed === "object" && parsed !== null;
      } catch {
        return false;
      }
    })();
    return (
      <pre className="whitespace-pre-wrap break-all font-mono text-[11px] leading-[1.6] text-ink-200">
        {isJson ? JSON.stringify(JSON.parse(text), null, 2) : text}
      </pre>
    );
  }
  if (value.type === "hash" && value.value && typeof value.value === "object" && !Array.isArray(value.value)) {
    const entries = Object.entries(value.value as Record<string, unknown>);
    return (
      <div className="divide-y divide-seam/60">
        {entries.map(([field, item]) => (
          <div key={field} className="grid grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)] gap-2 py-1">
            <span className="truncate font-mono text-[11px] text-ink-400" title={field}>{field}</span>
            <span className="break-all font-mono text-[11px] text-ink-100">{String(item)}</span>
          </div>
        ))}
        {entries.length === 0 && <p className="py-1 text-[11px] text-ink-500">{t("kv.emptyValue")}</p>}
      </div>
    );
  }
  if ((value.type === "list" || value.type === "set") && Array.isArray(value.value)) {
    return (
      <div className="divide-y divide-seam/60">
        {value.value.map((item, index) => (
          <div key={index} className="flex gap-2 py-1">
            <span className="w-10 shrink-0 text-right font-mono text-[10px] text-ink-600">{value.type === "list" ? index : "·"}</span>
            <span className="break-all font-mono text-[11px] text-ink-100">{String(item)}</span>
          </div>
        ))}
        {value.value.length === 0 && <p className="py-1 text-[11px] text-ink-500">{t("kv.emptyValue")}</p>}
      </div>
    );
  }
  if (value.type === "zset" && Array.isArray(value.value)) {
    return (
      <div className="divide-y divide-seam/60">
        {value.value.map((entry, index) => {
          const record = entry as { member?: string; score?: number };
          return (
            <div key={index} className="flex items-baseline gap-2 py-1">
              <span className="w-16 shrink-0 text-right font-mono text-[10.5px] text-amber-300/80">{String(record.score ?? 0)}</span>
              <span className="break-all font-mono text-[11px] text-ink-100">{String(record.member ?? "")}</span>
            </div>
          );
        })}
      </div>
    );
  }
  if (value.type === "stream" && Array.isArray(value.value)) {
    return (
      <div className="space-y-1.5">
        {value.value.map((entry, index) => {
          const record = entry as { id?: string; fields?: Record<string, unknown> };
          return (
            <div key={index} className="rounded-md border border-ink-700/60 bg-ink-850/60 px-2.5 py-1.5">
              <p className="font-mono text-[10px] text-rose-300/80">{record.id}</p>
              <div className="mt-1 space-y-0.5">
                {Object.entries(record.fields ?? {}).map(([field, item]) => (
                  <p key={field} className="break-all font-mono text-[10.5px] text-ink-300">
                    <span className="text-ink-500">{field}: </span>
                    {String(item)}
                  </p>
                ))}
              </div>
            </div>
          );
        })}
      </div>
    );
  }
  return <pre className="whitespace-pre-wrap break-all font-mono text-[11px] text-ink-200">{valuePreview(value.value)}</pre>;
}

/* ---------------- Console tab ---------------- */

function ConsoleTab({ database }: { database: Database }) {
  const { t } = useTranslation();
  const [command, setCommand] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [running, setRunning] = useState(false);
  const [result, setResult] = useState<KVCommandResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [confirmDanger, setConfirmDanger] = useState(false);
  const pendingDanger = useRef<string[] | null>(null);
  const outputRef = useRef<HTMLDivElement>(null);

  const parts = useMemo(() => tokenizeConsole(command), [command]);
  const commandWord = (parts[0] ?? "").toUpperCase();
  const isDangerous = DANGEROUS_COMMANDS.has(commandWord);

  const run = async (allowDangerous: boolean) => {
    if (!command.trim() || running) return;
    setRunning(true);
    setError(null);
    setResult(null);
    try {
      const reply = await desktop.kvDebug(database.id, commandWord, parts.slice(1), allowDangerous);
      setResult(reply);
      setHistory((prev) => [command, ...prev.filter((entry) => entry !== command)].slice(0, 50));
      setHistoryIndex(-1);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setRunning(false);
    }
  };

  const submit = () => {
    if (!command.trim()) return;
    if (isDangerous) {
      pendingDanger.current = parts;
      setConfirmDanger(true);
      return;
    }
    void run(false);
  };

  useEffect(() => {
    outputRef.current?.scrollTo({ top: outputRef.current.scrollHeight });
  }, [result, error]);

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-hidden rounded-xl border border-ink-700/80 bg-ink-900/60">
      <div className="flex items-center gap-2 border-b border-seam px-3 py-2">
        <Icon name="Terminal" className="h-3.5 w-3.5 text-ink-500" />
        <input
          value={command}
          onChange={(event) => setCommand(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") submit();
            if (event.key === "ArrowUp" && history.length > 0) {
              event.preventDefault();
              const next = historyIndex < 0 ? 0 : Math.min(historyIndex + 1, history.length - 1);
              setHistoryIndex(next);
              setCommand(history[next]);
            }
            if (event.key === "ArrowDown" && historyIndex >= 0) {
              event.preventDefault();
              const next = historyIndex - 1;
              setHistoryIndex(next);
              setCommand(next < 0 ? "" : history[next]);
            }
          }}
          placeholder={t("kv.consolePlaceholder")}
          spellCheck={false}
          className="h-8 min-w-0 flex-1 rounded-md border border-ink-700 bg-ink-850 px-2.5 font-mono text-[12px] text-ink-100 focus:border-ink-400 focus:outline-none"
        />
        {isDangerous && (
          <span className="shrink-0 rounded bg-rose-500/15 px-1.5 py-px font-mono text-[9.5px] text-rose-300">{t("kv.dangerous")}</span>
        )}
        <Button variant="solid" icon="Play" disabled={running || !command.trim()} onClick={submit}>
          {t("kv.run")}
        </Button>
      </div>
      <div ref={outputRef} className="min-h-0 flex-1 overflow-y-auto px-3 py-2">
        {error ? (
          <pre className="whitespace-pre-wrap break-all font-mono text-[11px] leading-[1.6] text-rose-200">{error}</pre>
        ) : result ? (
          <div className="space-y-1">
            {result.isNil ? (
              <p className="font-mono text-[11px] text-ink-500">{t("kv.nilReply")}</p>
            ) : (
              <pre className="whitespace-pre-wrap break-all font-mono text-[11px] leading-[1.6] text-ink-200">{valuePreview(result.value)}</pre>
            )}
            {result.truncated && <p className="text-[10px] text-amber-300/80">{t("kv.truncated")}</p>}
          </div>
        ) : (
          <p className="text-[11.5px] text-ink-500">{t("kv.consoleHint")}</p>
        )}
      </div>
      {confirmDanger && (
        <Modal
          title={t("kv.dangerTitle")}
          icon="AlertTriangle"
          onClose={() => setConfirmDanger(false)}
          footer={
            <ModalActions
              onCancel={() => setConfirmDanger(false)}
              onConfirm={() => {
                setConfirmDanger(false);
                void run(true);
              }}
              confirmLabel={t("kv.dangerConfirm")}
            />
          }
        >
          <p className="px-4 text-[12.5px] leading-relaxed text-ink-300">{t("kv.dangerDescription", { command: commandWord })}</p>
        </Modal>
      )}
    </div>
  );
}

/* ---------------- Info tab ---------------- */

function InfoTab({ database }: { database: Database }) {
  const { t } = useTranslation();
  const [info, setInfo] = useState<KVServerInfo | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [triggers, setTriggers] = useState<TriggerBinding[]>([]);
  const [reloadToken, setReloadToken] = useState(0);

  const load = useCallback(async () => {
    setError(null);
    try {
      setInfo(await desktop.kvInfo(database.id));
    } catch (e) {
      setInfo(null);
      setError(e instanceof Error ? e.message : String(e));
    }
    try {
      const all = await desktop.listKVTriggers();
      setTriggers(all.filter((binding) => binding.config?.databaseId === database.id));
    } catch {
      setTriggers([]);
    }
  }, [database.id]);

  useEffect(() => {
    void load();
  }, [load, reloadToken]);

  const trust = async (binding: TriggerBinding) => {
    await desktop.trustKVTrigger(binding.id).catch(() => undefined);
    setReloadToken((token) => token + 1);
  };

  const toggle = async (binding: TriggerBinding, enabled: boolean) => {
    await desktop.setKVTriggerEnabled(binding.id, enabled).catch(() => undefined);
    setReloadToken((token) => token + 1);
  };

  const cards = info
    ? [
        [t("kv.infoFlavor"), info.flavor],
        [t("kv.infoVersion"), info.version || "—"],
        [t("kv.infoUptime"), formatUptime(info.uptimeSeconds)],
        [t("kv.infoClients"), String(info.connectedClients)],
        [t("kv.infoMemory"), info.usedMemoryHuman || formatBytes(info.usedMemory)],
        [t("kv.infoTotalKeys"), String(info.totalKeys)],
      ]
    : [];

  return (
    <div className="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
      <div className="flex items-center gap-2">
        <Button icon="RefreshCw" variant="ghost" onClick={() => setReloadToken((token) => token + 1)}>
          {t("common.refresh")}
        </Button>
        {info && info.databases.length > 0 && (
          <span className="ml-auto font-mono text-[10.5px] text-ink-500">
            {info.databases.map((db) => `db${db.index}: ${db.keys}`).join(" · ")}
          </span>
        )}
      </div>
      {error ? (
        <p className="rounded-md border border-rose-500/20 bg-rose-500/5 px-3 py-2 font-mono text-[11px] text-rose-200">{error}</p>
      ) : (
        <div className="grid grid-cols-3 gap-2.5">
          {cards.map(([label, value]) => (
            <div key={label} className="rounded-xl border border-ink-700/80 bg-ink-900/60 p-3">
              <span className="text-[10px] tracking-wide text-ink-500 uppercase">{label}</span>
              <p className="mt-1 truncate text-[13px] font-semibold text-ink-50">{value}</p>
            </div>
          ))}
        </div>
      )}

      <div className="rounded-xl border border-ink-700/80 bg-ink-900/60">
        <p className="border-b border-seam px-3 py-2 text-[10.5px] font-medium tracking-[0.08em] text-ink-400 uppercase">
          {t("kv.triggersTitle")}
        </p>
        {triggers.length === 0 ? (
          <p className="px-3 py-3 text-[12px] text-ink-500">{t("kv.triggersEmpty")}</p>
        ) : (
          triggers.map((binding) => (
            <div key={binding.id} className="flex items-center gap-2.5 border-b border-seam/60 px-3 py-2 last:border-b-0">
              <div className="min-w-0 flex-1">
                <p className="truncate text-[12.5px] text-ink-100">{binding.label}</p>
                <p className="truncate font-mono text-[10px] text-ink-500">
                  {[binding.config?.channels, binding.config?.patterns].filter(Boolean).join(" · ") || "—"}
                </p>
              </div>
              <span className="font-mono text-[10px] text-ink-500">
                {binding.enabled ? t("kv.triggerEnabled") : t("kv.triggerDisabled")}
              </span>
              {!binding.trusted && (
                <Button variant="solid" icon="ShieldCheck" onClick={() => void trust(binding)}>
                  {t("kv.trust")}
                </Button>
              )}
              <Button
                variant="solid"
                disabled={!binding.trusted && !binding.enabled}
                onClick={() => void toggle(binding, !binding.enabled)}
              >
                {binding.enabled ? t("kv.disable") : t("kv.enable")}
              </Button>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

function formatUptime(seconds: number): string {
  if (seconds <= 0) return "—";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}
