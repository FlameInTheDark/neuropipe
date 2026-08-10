import { useMemo, useState } from "react";
import { ArrowRight, Bot, Braces, ChevronDown, MoreHorizontal, Plus, Search, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { ContextMenu, contextMenuPointFromElement, contextMenuPosition, type ContextMenuPoint, type ContextMenuPosition } from "@/components/ContextMenu";
import { EmptyState } from "@/components/EmptyState";
import { FunctionCreateDialog } from "@/components/FunctionCreateDialog";
import { LucideIcon } from "@/components/LucideIconPicker";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { desktop } from "@/lib/bridge";
import type { CreateFunctionRequest, FunctionSummary } from "@/lib/types";
import { formatDate } from "@/lib/utils";
import { useConfirmationStore } from "@/stores/confirmation";
import { useUIStore } from "@/stores/ui";

function FunctionRow({
  item,
  menuOpen,
  onOpen,
  onMenu,
}: {
  item: FunctionSummary;
  menuOpen: boolean;
  onOpen: () => void;
  onMenu: (point: ContextMenuPoint) => void;
}) {
  const { t } = useTranslation();
  const kindLabel = item.kind === "tool"
    ? t("functions.tool")
    : item.mode === "pure"
      ? t("functions.pure")
      : t("functions.impure");
  return (
    <div
      className="group grid grid-cols-[minmax(0,1fr)_100px_150px_36px] items-center gap-3 border-b border-zinc-800 px-4 py-3.5 last:border-0 hover:bg-zinc-900"
      onContextMenu={(event) => {
        event.preventDefault();
        onMenu(event);
      }}
    >
      <button
        type="button"
        onClick={onOpen}
        onKeyDown={(event) => {
          if (event.key !== "ContextMenu" && !(event.shiftKey && event.key === "F10")) return;
          event.preventDefault();
          onMenu(contextMenuPointFromElement(event.currentTarget));
        }}
        className="flex min-w-0 items-center gap-3 rounded text-left outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"
      >
        <span className="flex size-8 shrink-0 items-center justify-center rounded-md border border-zinc-800" style={{ color: item.iconColor, backgroundColor: item.iconBackground }}>
          <LucideIcon name={item.icon} className="size-4" />
        </span>
        <span className="min-w-0">
          <span className="block truncate text-sm font-medium text-zinc-100">{item.name}</span>
          <span className="mt-1 block truncate text-xs text-zinc-500">{item.description || t("functions.noDescription")}</span>
        </span>
      </button>
      <span className={item.kind === "tool" ? "flex items-center gap-1 text-xs text-fuchsia-300" : item.mode === "pure" ? "text-xs text-emerald-300" : "text-xs text-violet-300"}>
        {item.kind === "tool" ? <Bot className="size-3.5" /> : null}
        {kindLabel}
      </span>
      <span className="text-xs text-zinc-500">
        {item.publishedRevision ? t("functions.published", { version: item.publishedRevision }) : t("functions.draft")} · {formatDate(item.updatedAt)}
      </span>
      <Button
        size="sm"
        variant="ghost"
        className="size-7 p-0 opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
        onClick={(event) => onMenu(contextMenuPointFromElement(event.currentTarget))}
        aria-label={t("functions.options", { name: item.name })}
        aria-haspopup="menu"
        aria-expanded={menuOpen}
      >
        <MoreHorizontal className="size-4" />
      </Button>
    </div>
  );
}

interface FunctionMenu {
  item: FunctionSummary;
  position: ContextMenuPosition;
}

export function FunctionsView({ functions, onRefresh }: { functions: FunctionSummary[]; onRefresh: () => Promise<void> }) {
  const { t } = useTranslation();
  const { setError, setScreen } = useUIStore();
  const requestConfirmation = useConfirmationStore((state) => state.ask);
  const [query, setQuery] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<string>();
  const [menu, setMenu] = useState<FunctionMenu>();
  const groups = useMemo(() => functions
    .filter((item) => `${item.name} ${item.description} ${item.category}`.toLowerCase().includes(query.toLowerCase()))
    .reduce<Record<string, FunctionSummary[]>>((all, item) => {
      (all[item.category] ??= []).push(item);
      return all;
    }, {}), [functions, query]);

  const create = async (request: CreateFunctionRequest) => {
    try {
      setCreating(true);
      const created = await desktop.createFunction(request);
      await onRefresh();
      setShowCreate(false);
      setScreen("function-editor", created.id);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("functions.createFailed"));
    } finally {
      setCreating(false);
    }
  };
  const destroy = async (item: FunctionSummary) => {
    setMenu(undefined);
    if (!(await requestConfirmation({
      title: t("functionEditor.deleteTitle"),
      description: t("functionEditor.deleteDescription", { name: item.name }),
      confirmLabel: t("functionEditor.deleteConfirm"),
    }))) return;
    try {
      setDeleting(item.id);
      await desktop.deleteFunction(item.id);
      await onRefresh();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("functionEditor.deleteFailed"));
    } finally {
      setDeleting(undefined);
    }
  };
  const openMenu = (point: ContextMenuPoint, item: FunctionSummary) => {
    setMenu({ item, position: contextMenuPosition(point, { width: 160, height: 48 }) });
  };

  return (
    <section className="flex h-full min-h-0 flex-col">
      <PageHeader
        title={t("functions.title")}
        description={t("functions.description")}
        actions={<Button onClick={() => setShowCreate(true)} disabled={creating || deleting !== undefined}><Plus className="size-4" />{t("functions.new")}</Button>}
      />
      <div className="muted-scroll min-h-0 flex-1 overflow-y-auto p-8">
        <div className="mb-6 max-w-3xl"><div className="relative"><Search className="pointer-events-none absolute left-2.5 top-2 size-4 text-zinc-600" /><Input value={query} onChange={(event) => setQuery(event.target.value)} className="pl-8" placeholder={t("functions.search")} /></div></div>
        {functions.length === 0 ? (
          <EmptyState icon={Braces} title={t("functions.emptyTitle")} description={t("functions.emptyDescription")} action={{ label: t("functions.new"), onClick: () => setShowCreate(true) }} />
        ) : Object.entries(groups).map(([category, items]) => (
          <section key={category} className="mb-7">
            <h2 className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[.14em] text-zinc-500"><ChevronDown className="size-3.5" />{category}</h2>
            <div className="overflow-visible rounded-xl border border-zinc-800">
              {items.map((item) => (
                <FunctionRow
                  key={item.id}
                  item={item}
                  menuOpen={menu?.item.id === item.id}
                  onOpen={() => setScreen("function-editor", item.id)}
                  onMenu={(point) => openMenu(point, item)}
                />
              ))}
            </div>
          </section>
        ))}
      </div>
      {menu ? (
        <ContextMenu position={menu.position} ariaLabel={t("functions.options", { name: menu.item.name })} className="w-40" onClose={() => setMenu(undefined)}>
          <button
            type="button"
            role="menuitem"
            disabled={deleting !== undefined}
            onClick={() => void destroy(menu.item)}
            className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-xs text-red-300 outline-none hover:bg-red-500/10 focus-visible:bg-red-500/10 disabled:cursor-not-allowed disabled:opacity-40"
          >
            <Trash2 className="size-3.5" />
            {t("common.delete")}
          </button>
        </ContextMenu>
      ) : null}
      <FunctionCreateDialog open={showCreate} pending={creating} onClose={() => setShowCreate(false)} onCreate={(request) => void create(request)} />
    </section>
  );
}
