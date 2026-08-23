import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { TriggerBinding } from "@/lib/types";
import type { Workspace } from "@/features/workspace/useWorkspace";
import type { NavApi } from "@/features/workspace/useWorkspaceNav";
import { usePersistedValue } from "@/lib/prefs";
import { ViewShell } from "../components/ViewShell";
import { Empty as EmptyHint } from "../components/ui";
import { Icon } from "../components/icons";
import { Tooltip } from "../components/Tooltip";
import { Dropdown } from "../components/Dropdown";
import { useCtxMenu } from "../components/ContextMenu";
import { DeckEditor, KeyEditor } from "../features/board/BoardEditors";
import { useDragReorder } from "../hooks/useDragReorder";
import { cn } from "../utils/cn";

const KEY_MIME = "application/x-neuropipe-board-key";
const KEY_SIZE = { comfortable: 154, compact: 126 } as const;
type Density = keyof typeof KEY_SIZE;

interface LocalDeck {
  id: string;
  name: string;
  icon: string;
}

export type { LocalDeck };

/** board layout state persisted renderer-side; bindings stay backend-owned */
interface BoardLayout {
  decks: LocalDeck[];
  /** binding id -> deck id */
  assignment: Record<string, string>;
  /** binding id -> local display label override */
  labels?: Record<string, string>;
  order: string[];
  hidden: string[];
}

const DEFAULT_LAYOUT: BoardLayout = {
  decks: [{ id: "main", name: "Main", icon: "Grid2x2" }],
  assignment: {},
  order: [],
  hidden: [],
};

/** Stream-deck style launcher for published Button Triggers. */
export function BoardView({ workspace, nav }: { workspace: Workspace; nav: NavApi }) {
  const { t } = useTranslation();
  const [layout, setLayout] = usePersistedValue<BoardLayout>("neuropipe.board.layout.v1", DEFAULT_LAYOUT);
  const [activeDeck, setActiveDeck] = useState(layout.decks[0]?.id ?? "main");
  const [fired, setFired] = useState<string | null>(null);
  const [overDeck, setOverDeck] = useState<string | null>(null);
  const [density, setDensity] = usePersistedValue<Density>("neuropipe.board.density.v1", "comfortable" as Density);
  const [editKey, setEditKey] = useState<TriggerBinding | null>(null);
  const [editDeck, setEditDeck] = useState<LocalDeck | null>(null);
  const ctx = useCtxMenu();

  /* keep active deck valid when decks disappear */
  useEffect(() => {
    if (!layout.decks.some((d) => d.id === activeDeck)) {
      setActiveDeck(layout.decks[0]?.id ?? "main");
    }
  }, [layout.decks, activeDeck]);

  const drag = useDragReorder(KEY_MIME, (source, target) =>
    setLayout((l) => ({ ...l, order: reorderStrings(orderOf(l), source, target) })),
  );

  const buttons = useMemo(() => {
    const visible = workspace.buttonBindings.filter((b) => !layout.hidden.includes(b.id));
    const order = orderOf(layout);
    return [...visible].sort((a, b) => {
      const ia = order.indexOf(a.id);
      const ib = order.indexOf(b.id);
      return (ia < 0 ? Number.MAX_SAFE_INTEGER : ia) - (ib < 0 ? Number.MAX_SAFE_INTEGER : ib);
    });
  }, [workspace.buttonBindings, layout]);

  const deckKeys = buttons.filter((b) => (layout.assignment[b.id] ?? layout.decks[0]?.id) === activeDeck);
  const deck = layout.decks.find((d) => d.id === activeDeck) ?? layout.decks[0];
  const keySize = KEY_SIZE[density];

  /* ---------- key actions ---------- */

  const run = async (binding: TriggerBinding) => {
    if (drag.consumeClickAfterDrag()) return;
    if (workspace.running[binding.pipelineId]) {
      await workspace.stopPipeline(binding.pipelineId);
      return;
    }
    setFired(binding.id);
    window.setTimeout(() => setFired((f) => (f === binding.id ? null : f)), 1200);
    await workspace.runTrigger(binding.id);
  };

  const saveKeyLabel = (binding: TriggerBinding, customName: string) => {
    setLayout((l) => ({
      ...l,
      labels: { ...(l.labels ?? {}), [binding.id]: customName },
    }));
    setEditKey(null);
  };

  const blankKeyLabel = (b: TriggerBinding): string =>
    (layout.labels?.[b.id]) || b.label || pipelineName(b);

  function pipelineName(b: TriggerBinding): string {
    return workspace.pipelines.find((p) => p.id === b.pipelineId)?.name ?? b.pipelineId;
  }

  /* ---------- deck actions ---------- */

  const addDeck = () => {
    const id = `deck_${Date.now()}`;
    setLayout((l) => ({ ...l, decks: [...l.decks, { id, name: `${t("board.deck")} ${l.decks.length + 1}`, icon: "Grid2x2" }] }));
    setActiveDeck(id);
  };

  const saveDeck = (next: LocalDeck) => {
    setLayout((l) => ({ ...l, decks: l.decks.map((d) => (d.id === next.id ? next : d)) }));
    setEditDeck(null);
  };

  const deleteDeck = (id: string) => {
    if (layout.decks.length <= 1) return;
    setLayout((l) => ({
      ...l,
      decks: l.decks.filter((d) => d.id !== id),
      // keys of a deleted deck fall back to the first deck
    }));
  };

  const moveDeck = (id: string, dir: -1 | 1) =>
    setLayout((l) => {
      const at = l.decks.findIndex((d) => d.id === id);
      const to = at + dir;
      if (at < 0 || to < 0 || to >= l.decks.length) return l;
      const decks = [...l.decks];
      const [item] = decks.splice(at, 1);
      decks.splice(to, 0, item);
      return { ...l, decks };
    });

  const moveToDeck = (keyId: string, deckId: string) =>
    setLayout((l) => ({ ...l, assignment: { ...l.assignment, [keyId]: deckId } }));

  /* ---------- context menus ---------- */

  const deckMenu = (e: React.MouseEvent, d: LocalDeck) =>
    ctx(e, [
      { label: t("board.renameDeck"), icon: "Pencil", onSelect: () => setEditDeck(d) },
      { type: "sep" },
      { label: t("board.moveLeft"), icon: "ArrowLeft", disabled: layout.decks[0].id === d.id, onSelect: () => moveDeck(d.id, -1) },
      {
        label: t("board.moveRight"),
        icon: "ArrowUpRight",
        disabled: layout.decks[layout.decks.length - 1].id === d.id,
        onSelect: () => moveDeck(d.id, 1),
      },
      { type: "sep" },
      { label: t("board.deleteDeck"), icon: "Trash2", danger: true, disabled: layout.decks.length <= 1, onSelect: () => deleteDeck(d.id) },
    ]);

  const keyMenu = (e: React.MouseEvent, b: TriggerBinding) =>
    ctx(e, [
      { label: t("board.run", { name: blankKeyLabel(b) }), icon: "Play", onSelect: () => void run(b) },
      { label: t("board.editKey"), icon: "Pencil", onSelect: () => setEditKey(b) },
      { type: "sep" },
      { label: t("board.openPipeline"), icon: "Cable", onSelect: () => nav.goto("pipelines") },
      ...(layout.decks.length > 1
        ? [
            { type: "sep" as const },
            ...layout.decks
              .filter((d) => d.id !== (layout.assignment[b.id] ?? layout.decks[0].id))
              .map((d) => ({
                label: `${t("board.moveTo")} ${d.name}`,
                icon: d.icon,
                onSelect: () => moveToDeck(b.id, d.id),
              })),
          ]
        : []),
      { type: "sep" },
      {
        label: t("board.removeFromBoard"),
        icon: "Trash2",
        danger: true,
        onSelect: () => setLayout((l) => ({ ...l, hidden: [...l.hidden, b.id] })),
      },
    ]);

  return (
    <ViewShell
      title={t("board.title")}
      subtitle={t("board.description")}
      actions={
        <>
          <Dropdown
            compact
            value={density}
            onChange={(v) => setDensity(v as Density)}
            options={[
              { value: "comfortable", label: t("board.largeKeys"), icon: "Grid2x2" },
              { value: "compact", label: t("board.compactKeys"), icon: "LayoutGrid" },
            ]}
          />
        </>
      }
    >
      {/* deck tabs */}
      <div className="mb-3 flex items-center gap-1.5">
        <div className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto rounded-xl border border-ink-700/80 bg-ink-900/60 p-1">
          {layout.decks.map((d) => {
            const active = d.id === activeDeck;
            const count = buttons.filter((b) => (layout.assignment[b.id] ?? layout.decks[0]?.id) === d.id).length;
            return (
              <button
                key={d.id}
                onClick={() => setActiveDeck(d.id)}
                onDoubleClick={() => setEditDeck(d)}
                onContextMenu={(e) => deckMenu(e, d)}
                onDragOver={(e) => {
                  if (!drag.dragging) return;
                  e.preventDefault();
                  setOverDeck(d.id);
                }}
                onDragLeave={() => setOverDeck((v) => (v === d.id ? null : v))}
                onDrop={(e) => {
                  e.preventDefault();
                  const source = drag.readSource(e);
                  if (source) moveToDeck(source, d.id);
                  setOverDeck(null);
                  drag.end();
                }}
                className={cn(
                  "flex h-8 shrink-0 items-center gap-2 rounded-lg px-2.5 text-[12px] transition",
                  active ? "bg-ink-750 text-ink-50" : "text-ink-400 hover:bg-ink-850 hover:text-ink-100",
                  overDeck === d.id && !active && "ring-1 ring-ink-300",
                )}
              >
                <Icon name={d.icon} className="h-3.5 w-3.5 shrink-0" />
                <span className="max-w-[130px] truncate font-medium">{d.name}</span>
                <span
                  className={cn(
                    "rounded px-1 font-mono text-[9.5px]",
                    active ? "bg-ink-900 text-ink-400" : "bg-ink-800 text-ink-600",
                  )}
                >
                  {count}
                </span>
              </button>
            );
          })}
          <Tooltip content={t("board.newDeck")} side="bottom">
            <button
              onClick={addDeck}
              aria-label={t("board.newDeck")}
              className="grid h-8 w-8 shrink-0 place-items-center rounded-lg text-ink-500 transition hover:bg-ink-850 hover:text-ink-100"
            >
              <Icon name="Plus" className="h-4 w-4" />
            </button>
          </Tooltip>
        </div>

        <Tooltip content={t("board.deckSettings")} side="bottom">
          <button
            onClick={() => deck && setEditDeck(deck)}
            className="grid h-9 w-9 shrink-0 place-items-center rounded-xl border border-ink-700 bg-ink-850 text-ink-400 transition hover:border-ink-500 hover:text-ink-50"
          >
            <Icon name="Settings2" className="h-4 w-4" />
          </button>
        </Tooltip>
      </div>

      {/* recessed deck surface */}
      <div className="rounded-2xl border border-ink-700/80 bg-ink-950/70 p-3.5 shadow-[0_1px_0_rgba(255,255,255,0.025)_inset,0_16px_40px_-28px_rgba(0,0,0,0.95)]">
        <div className="mb-3 flex items-center gap-2 px-1">
          <span className="flex items-center gap-1.5 text-[10.5px] font-medium tracking-[0.09em] text-ink-400 uppercase">
            <Icon name={deck?.icon ?? "Grid2x2"} className="h-3.5 w-3.5" />
            {deck?.name}
          </span>
          <span className="rounded bg-ink-800 px-1.5 py-px font-mono text-[10px] text-ink-500">
            {t("status.keys", { count: deckKeys.length })}
          </span>
          <span className="ml-auto flex items-center gap-1.5 text-[10.5px] text-ink-500">
            <Icon name="GripVertical" className="h-3.5 w-3.5" />
            {t("board.dragHint")}
          </span>
        </div>

        {deckKeys.length === 0 ? (
          <EmptyHint icon="Grid2x2" text={t("board.emptyDescription")} />
        ) : (
          <div
            className="grid justify-start gap-3"
            style={{ gridTemplateColumns: `repeat(auto-fill, ${keySize}px)` }}
            onDragOver={(e) => e.preventDefault()}
          >
            {deckKeys.map((b, index) => (
              <DeckKey
                key={b.id}
                binding={b}
                index={index}
                label={blankKeyLabel(b)}
                pipelineName={pipelineName(b)}
                running={Boolean(workspace.running[b.pipelineId]) || fired === b.id}
                dragging={drag.dragging === b.id}
                dropTarget={drag.over === b.id && drag.dragging !== b.id}
                dragProps={drag.itemProps(b.id)}
                onRun={() => void run(b)}
                onEdit={() => setEditKey(b)}
                onContextMenu={(e) => keyMenu(e, b)}
              />
            ))}
          </div>
        )}
      </div>

      {editKey && (
        <KeyEditor
          value={{ ...editKey, displayLabel: blankKeyLabel(editKey) }}
          onCancel={() => setEditKey(null)}
          onSave={(label) => saveKeyLabel(editKey, label)}
        />
      )}

      {editDeck && (
        <DeckEditor
          value={editDeck}
          canDelete={layout.decks.length > 1}
          onCancel={() => setEditDeck(null)}
          onDelete={() => {
            deleteDeck(editDeck.id);
            setEditDeck(null);
          }}
          onSave={saveDeck}
        />
      )}
    </ViewShell>
  );
}

function orderOf(l: BoardLayout): string[] {
  return l.order ?? [];
}

/** Moves a string id within a plain id list (board order is not an object list). */
function reorderStrings(list: string[], sourceId: string, targetId: string): string[] {
  if (sourceId === targetId) return list;
  const from = list.indexOf(sourceId);
  const to = list.indexOf(targetId);
  const next = list.filter((id) => id !== sourceId);
  if (from < 0 || to < 0) {
    next.push(sourceId);
    return next;
  }
  const at = next.indexOf(targetId);
  next.splice(at, 0, sourceId);
  return next;
}

/** A single physical-looking key on the deck. */
function DeckKey({
  binding,
  index,
  label,
  pipelineName,
  running,
  dragging,
  dropTarget,
  dragProps,
  onRun,
  onEdit,
  onContextMenu,
}: {
  binding: TriggerBinding;
  index: number;
  label: string;
  pipelineName: string;
  running: boolean;
  dragging: boolean;
  dropTarget: boolean;
  dragProps: Record<string, unknown>;
  onRun: () => void;
  onEdit: () => void;
  onContextMenu: (e: React.MouseEvent) => void;
}) {
  const { t } = useTranslation();

  return (
    <div
      {...dragProps}
      onContextMenu={onContextMenu}
      className={cn(
        "relative aspect-square rounded-[18px] border border-ink-800/80 p-[5px] transition-all duration-150",
        "bg-gradient-to-b from-ink-850 to-ink-950 shadow-[0_10px_20px_-16px_rgba(0,0,0,0.98),0_1px_0_rgba(255,255,255,0.025)_inset]",
        dragging && "scale-95 opacity-35",
        dropTarget && "translate-x-1 border-ink-500/80 shadow-[0_0_0_2px_rgba(124,124,136,0.18)]",
      )}
    >
      <button
        onClick={onRun}
        className={cn(
          "group relative flex h-full w-full flex-col overflow-hidden rounded-[13px] border px-3 py-3 text-left outline-none transition",
          "active:translate-y-[1px] active:shadow-none focus-visible:ring-2 focus-visible:ring-ink-600/80",
          running
            ? "border-emerald-400/35 bg-emerald-400/10 shadow-[0_0_26px_-12px_rgba(52,211,153,0.55),0_1px_0_rgba(255,255,255,0.05)_inset]"
            : "border-ink-700/80 bg-[radial-gradient(circle_at_30%_20%,rgba(255,255,255,0.045),transparent_34%),linear-gradient(145deg,#17171b,#0c0c0e)] shadow-[0_3px_0_#050506,0_8px_18px_-12px_rgba(0,0,0,0.98),0_1px_0_rgba(255,255,255,0.035)_inset] hover:border-ink-600",
        )}
      >
        <span className="flex w-full items-center">
          <span
            className={cn(
              "h-1.5 w-1.5 rounded-full transition",
              running ? "bg-emerald-400 pulse-ring" : "bg-emerald-400/65",
            )}
          />
          <span className="ml-auto font-mono text-[9px] text-ink-600">
            {String(index + 1).padStart(2, "0")}
          </span>
        </span>

        <span
          className={cn(
            "mt-auto grid h-9 w-9 place-items-center rounded-[10px] border transition",
            running
              ? "border-emerald-400/35 bg-emerald-400/10 text-emerald-300"
              : "border-ink-700/80 bg-ink-850/80 text-ink-300 shadow-[0_1px_0_rgba(255,255,255,0.025)_inset] group-hover:border-ink-600 group-hover:text-ink-50",
          )}
        >
          <Icon
            name={running ? "Loader2" : binding.icon || "Play"}
            className={cn("h-[17px] w-[17px]", running && "animate-spin")}
          />
        </span>

        <span className="mt-2 block w-full truncate text-[12px] font-semibold text-ink-50">{label}</span>
        <span
          className={cn(
            "mt-0.5 block w-full truncate text-[10px]",
            running ? "text-emerald-300/80" : "text-ink-500",
          )}
        >
          {running ? t("board.running") : pipelineName}
        </span>

        <span className="absolute top-1.5 right-1.5 flex items-center gap-0.5 opacity-0 transition group-hover:opacity-100">
          <Tooltip content={t("board.editKey")} side="top">
            <span
              role="button"
              tabIndex={-1}
              aria-label={t("board.editKey")}
              onClick={(e) => {
                e.stopPropagation();
                onEdit();
              }}
              className="grid h-5 w-5 place-items-center rounded text-ink-500 transition hover:bg-ink-700 hover:text-ink-50"
            >
              <Icon name="Pencil" className="h-3 w-3" />
            </span>
          </Tooltip>
          <span className="grid h-5 w-5 place-items-center rounded text-ink-600">
            <Icon name="GripVertical" className="h-3.5 w-3.5" />
          </span>
        </span>
      </button>
    </div>
  );
}

