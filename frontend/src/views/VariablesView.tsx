import { useEffect, useMemo, useState } from "react";
import { Database, Loader2, MoreHorizontal, Pencil, Plus, RefreshCw, Search, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { ContextMenu, contextMenuPointFromElement, contextMenuPosition, type ContextMenuPoint, type ContextMenuPosition } from "@/components/ContextMenu";
import { EmptyState } from "@/components/EmptyState";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { desktop } from "@/lib/bridge";
import type { DataType, GlobalVariableSummary } from "@/lib/types";
import { formatDate } from "@/lib/utils";
import { useConfirmationStore } from "@/stores/confirmation";
import { useUIStore } from "@/stores/ui";

const NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*$/;

function valuePreview(value: unknown): string {
  if (value === undefined || value === null) return "";
  try {
    const rendered = JSON.stringify(value);
    return rendered.length > 120 ? rendered.slice(0, 117) + "…" : rendered;
  } catch {
    return String(value);
  }
}

function defaultFor(type: DataType): unknown {
  switch (type) {
    case "number":
      return 0;
    case "boolean":
      return false;
    case "object":
      return {};
    case "list":
      return [];
    case "text":
      return "";
    default:
      return null;
  }
}

function parseStructured(type: DataType, raw: string): unknown {
  if (type !== "object" && type !== "list") return raw;
  try {
    const parsed = JSON.parse(raw);
    if (type === "object" && (parsed === null || Array.isArray(parsed) || typeof parsed !== "object")) return {};
    if (type === "list" && !Array.isArray(parsed)) return [];
    return parsed;
  } catch {
    return type === "object" ? {} : [];
  }
}

interface VariableDraft {
  id: string;
  name: string;
  description: string;
  dataType: DataType;
  defaultValue: unknown;
}

function draftFromSummary(item: GlobalVariableSummary): VariableDraft {
  return {
    id: item.id,
    name: item.name,
    description: item.description,
    dataType: item.dataType,
    defaultValue: item.value,
  };
}

function draftBlank(): VariableDraft {
  return { id: "", name: "", description: "", dataType: "text", defaultValue: defaultFor("text") };
}

function VariableDialog({ open, pending, editTarget, onClose, onSave }: {
  open: boolean;
  pending: boolean;
  editTarget: GlobalVariableSummary | undefined;
  onClose: () => void;
  onSave: (draft: VariableDraft) => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState<VariableDraft>(() => (editTarget ? draftFromSummary(editTarget) : draftBlank()));
  const [jsonDraft, setJsonDraft] = useState<string>(() => {
    const initial = draft.defaultValue;
    return typeof initial === "string" ? initial : JSON.stringify(initial ?? defaultFor(draft.dataType), null, 2);
  });
  const [jsonError, setJsonError] = useState(false);
  const isEdit = draft.id !== "";
  const nameValid = NAME_PATTERN.test(draft.name);
  const structuredType = draft.dataType === "object" || draft.dataType === "list";

  const setDataType = (dataType: DataType) => {
    const nextDefault = defaultFor(dataType);
    setDraft((current) => ({ ...current, dataType, defaultValue: nextDefault }));
    setJsonDraft(typeof nextDefault === "string" ? nextDefault : JSON.stringify(nextDefault, null, 2));
    setJsonError(false);
  };

  const onStructuredChange = (raw: string) => {
    setJsonDraft(raw);
    try {
      const parsed = JSON.parse(raw);
      const validShape = draft.dataType === "object"
        ? parsed !== null && !Array.isArray(parsed) && typeof parsed === "object"
        : Array.isArray(parsed);
      setJsonError(!validShape);
      if (validShape) setDraft((current) => ({ ...current, defaultValue: parseStructured(current.dataType, raw) }));
    } catch {
      setJsonError(true);
    }
  };

  const defaultEditor = () => {
    switch (draft.dataType) {
      case "boolean":
        return (
          <Select
            value={String(draft.defaultValue)}
            onValueChange={(next) => setDraft((current) => ({ ...current, defaultValue: next === "true" }))}
            options={[
              { value: "true", label: t("variables.booleanTrue") },
              { value: "false", label: t("variables.booleanFalse") },
            ]}
            ariaLabel={t("variables.defaultLabel")}
          />
        );
      case "number":
        return (
          <Input
            type="number"
            value={String(draft.defaultValue ?? 0)}
            onChange={(event) => setDraft((current) => ({ ...current, defaultValue: Number(event.target.value) }))}
            placeholder={t("variables.defaultPlaceholder")}
          />
        );
      case "object":
      case "list":
        return (
          <div>
            <textarea
              className="min-h-32 w-full rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2 font-mono text-xs text-zinc-100 outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"
              value={jsonDraft}
              onChange={(event) => onStructuredChange(event.target.value)}
              placeholder={draft.dataType === "list" ? "[]" : "{}"}
              aria-label={t("variables.defaultLabel")}
              aria-invalid={jsonError}
            />
            {jsonError && <span className="mt-1 block text-xs text-red-300">{t("variables.defaultInvalidJson")}</span>}
          </div>
        );
      default:
        return (
          <Input
            value={String(draft.defaultValue ?? "")}
            onChange={(event) => setDraft((current) => ({ ...current, defaultValue: event.target.value }))}
            placeholder={t("variables.defaultPlaceholder")}
          />
        );
    }
  };

  const saveDisabled = pending || (!isEdit && (!nameValid || draft.name.trim() === "")) || (structuredType && jsonError);

  return (
    <Dialog open={open} title={isEdit ? t("variables.editTitle") : t("variables.createTitle")} onOpenChange={(next) => { if (!next) onClose(); }} className="max-w-xl">
      <div className="space-y-4 overflow-y-auto px-5 py-4">
        <label className="block">
          <span className="mb-1 block text-xs font-medium text-zinc-400">{t("variables.nameLabel")}</span>
          <Input
            value={draft.name}
            onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))}
            placeholder={t("variables.namePlaceholder")}
            disabled={isEdit}
            aria-invalid={!isEdit && draft.name.trim() !== "" && !nameValid}
          />
          {!isEdit && draft.name.trim() !== "" && !nameValid && (
            <span className="mt-1 block text-xs text-red-300">{t("variables.nameInvalid")}</span>
          )}
        </label>
        <label className="block">
          <span className="mb-1 block text-xs font-medium text-zinc-400">{t("variables.descriptionLabel")}</span>
          <Input
            value={draft.description}
            onChange={(event) => setDraft((current) => ({ ...current, description: event.target.value }))}
            placeholder={t("variables.descriptionPlaceholder")}
          />
        </label>
        <label className="block">
          <span className="mb-1 block text-xs font-medium text-zinc-400">{t("variables.typeLabel")}</span>
          <Select
            value={draft.dataType}
            onValueChange={(next) => setDataType(next as DataType)}
            options={(["text", "number", "boolean", "object", "list", "any"] as DataType[]).map((value) => ({ value, label: t(`variables.type.${value}`) }))}
            ariaLabel={t("variables.typeLabel")}
            disabled={isEdit}
          />
          {isEdit && <span className="mt-1 block text-xs text-zinc-500">{t("variables.typeFrozen")}</span>}
        </label>
        <label className="block">
          <span className="mb-1 block text-xs font-medium text-zinc-400">{t("variables.defaultLabel")}</span>
          {defaultEditor()}
        </label>
      </div>
      <div className="flex justify-end gap-2 border-t border-zinc-800 px-5 py-4">
        <Button variant="ghost" onClick={onClose}>{t("common.cancel")}</Button>
        <Button onClick={() => onSave(draft)} disabled={saveDisabled}>
          {pending ? t("common.saving") : isEdit ? t("common.save") : t("common.create")}
        </Button>
      </div>
    </Dialog>
  );
}

interface VariableMenu {
  item: GlobalVariableSummary;
  position: ContextMenuPosition;
}

export function VariablesView({ variables, onRefresh }: { variables: GlobalVariableSummary[]; onRefresh: () => Promise<void> }) {
  const { t } = useTranslation();
  const { setError } = useUIStore();
  const requestConfirmation = useConfirmationStore((state) => state.ask);
  const [query, setQuery] = useState("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<GlobalVariableSummary | undefined>(undefined);
  const [workingId, setWorkingId] = useState<string | undefined>(undefined);
  const [refreshing, setRefreshing] = useState(false);
  const [menu, setMenu] = useState<VariableMenu>();

  useEffect(() => {
    void onRefresh();
  }, [onRefresh]);

  const filtered = useMemo(
    () => variables.filter((item) => `${item.name} ${item.description}`.toLowerCase().includes(query.toLowerCase())),
    [variables, query],
  );

  const refresh = async () => {
    try {
      setRefreshing(true);
      await onRefresh();
    } finally {
      setRefreshing(false);
    }
  };

  const save = async (draft: VariableDraft) => {
    try {
      setWorkingId(draft.id || "__new__");
      if (draft.id) {
        const existing = variables.find((item) => item.id === draft.id);
        if (!existing) throw new Error(t("variables.notFound"));
        await desktop.updateGlobalVariable({
          id: draft.id,
          name: existing.name,
          description: draft.description,
          dataType: draft.dataType,
          defaultValue: draft.defaultValue,
        });
      } else {
        await desktop.createGlobalVariable({
          name: draft.name,
          description: draft.description,
          dataType: draft.dataType,
          defaultValue: draft.defaultValue,
        });
      }
      await onRefresh();
      setDialogOpen(false);
      setEditing(undefined);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("variables.saveFailed"));
    } finally {
      setWorkingId(undefined);
    }
  };

  const destroy = async (item: GlobalVariableSummary) => {
    setMenu(undefined);
    const confirmed = await requestConfirmation({
      title: t("variables.deleteTitle"),
      description: t("variables.deleteDescription", { name: item.name }),
      confirmLabel: t("variables.deleteConfirm"),
    });
    if (!confirmed) return;
    try {
      setWorkingId(item.id);
      await desktop.deleteGlobalVariable(item.id);
      await onRefresh();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("variables.deleteFailed"));
    } finally {
      setWorkingId(undefined);
    }
  };

  const openMenu = (point: ContextMenuPoint, item: GlobalVariableSummary) => {
    setMenu({ item, position: contextMenuPosition(point, { width: 160, height: 80 }) });
  };

  return (
    <section className="flex h-full min-h-0 flex-col">
      <PageHeader
        title={t("variables.title")}
        description={t("variables.description")}
        actions={<div className="flex gap-2"><Button variant="outline" onClick={() => void refresh()} disabled={refreshing || workingId !== undefined}>{refreshing ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}{t("common.refresh")}</Button><Button onClick={() => { setEditing(undefined); setDialogOpen(true); }} disabled={refreshing || workingId !== undefined}><Plus className="size-4" />{t("variables.new")}</Button></div>}
      />
      <div className="muted-scroll min-h-0 flex-1 overflow-y-auto p-8">
        <div className="mb-6 max-w-3xl"><div className="relative"><Search className="pointer-events-none absolute left-2.5 top-2 size-4 text-zinc-600" /><Input value={query} onChange={(event) => setQuery(event.target.value)} className="pl-8" placeholder={t("variables.search")} /></div></div>
        {filtered.length === 0 ? (
          <EmptyState icon={Database} title={t("variables.emptyTitle")} description={t("variables.emptyDescription")} action={{ label: t("variables.new"), onClick: () => { setEditing(undefined); setDialogOpen(true); } }} />
        ) : (
          <div className="overflow-hidden rounded-xl border border-zinc-800">
            {filtered.map((item) => (
              <div
                key={item.id}
                className="group grid grid-cols-[minmax(0,1fr)_100px_minmax(0,2fr)_150px_36px] items-center gap-3 border-b border-zinc-800 px-4 py-3.5 last:border-0 hover:bg-zinc-900"
                onContextMenu={(event) => { event.preventDefault(); openMenu(event, item); }}
              >
                <button
                  type="button"
                  onClick={() => { setEditing(item); setDialogOpen(true); }}
                  onKeyDown={(event) => {
                    if (event.key !== "ContextMenu" && !(event.shiftKey && event.key === "F10")) return;
                    event.preventDefault();
                    openMenu(contextMenuPointFromElement(event.currentTarget), item);
                  }}
                  className="flex min-w-0 items-center gap-3 rounded text-left outline-none focus-visible:ring-2 focus-visible:ring-zinc-500"
                >
                  <span className="flex size-8 shrink-0 items-center justify-center rounded-md border border-zinc-800 bg-zinc-900 text-zinc-300"><Database className="size-4" /></span>
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-medium text-zinc-100">{item.name}</span>
                    <span className="mt-1 block truncate text-xs text-zinc-500">{item.description || t("variables.noDescription")}</span>
                  </span>
                </button>
                <span className="text-xs text-zinc-400">{t(`variables.type.${item.dataType}`)}</span>
                <span className="truncate font-mono text-xs text-zinc-500">{valuePreview(item.value)}</span>
                <span className="text-xs text-zinc-500">{formatDate(item.updatedAt)}</span>
                <Button
                  size="sm"
                  variant="ghost"
                  className="size-7 p-0 opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
                  onClick={(event) => openMenu(contextMenuPointFromElement(event.currentTarget), item)}
                  aria-label={t("variables.options", { name: item.name })}
                  aria-haspopup="menu"
                  aria-expanded={menu?.item.id === item.id}
                >
                  <MoreHorizontal className="size-4" />
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>
      {menu ? (
        <ContextMenu position={menu.position} ariaLabel={t("variables.options", { name: menu.item.name })} className="w-44" onClose={() => setMenu(undefined)}>
          <button
            type="button"
            role="menuitem"
            disabled={workingId !== undefined}
            onClick={() => { setEditing(menu.item); setMenu(undefined); setDialogOpen(true); }}
            className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-xs text-zinc-200 outline-none hover:bg-zinc-800 focus-visible:bg-zinc-800 disabled:cursor-not-allowed disabled:opacity-40"
          >
            <Pencil className="size-3.5" />
            {t("common.edit")}
          </button>
          <button
            type="button"
            role="menuitem"
            disabled={workingId !== undefined}
            onClick={() => void destroy(menu.item)}
            className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-xs text-red-300 outline-none hover:bg-red-500/10 focus-visible:bg-red-500/10 disabled:cursor-not-allowed disabled:opacity-40"
          >
            <Trash2 className="size-3.5" />
            {t("common.delete")}
          </button>
        </ContextMenu>
      ) : null}
      {dialogOpen && <VariableDialog open={dialogOpen} pending={workingId !== undefined} editTarget={editing} onClose={() => { setDialogOpen(false); setEditing(undefined); }} onSave={(draft) => void save(draft)} />}
    </section>
  );
}
