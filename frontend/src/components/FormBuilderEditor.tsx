import { useMemo, useState } from "react";
import { Button } from "./ui";
import { Modal } from "./primitives/Modal";
import { Dropdown } from "./Dropdown";
import { TextInput } from "./primitives/Field";
import { Icon } from "./icons";
import { formLayoutFromValue } from "@/lib/blueprint-dynamic-pins";
import type { FormItemValue, FormLayoutValue } from "@/lib/types";
import { useTranslation } from "react-i18next";

const GRID_COLUMNS = 4;
const ROW_HEIGHT = 64;
const MIN_ROWS = 6;

function nextItemID(items: readonly FormItemValue[]): string {
  const used = new Set(items.map((item) => item.id));
  for (let index = items.length + 1; ; index += 1) {
    const id = `field_${index}`;
    if (!used.has(id)) return id;
  }
}

function clampSpan(value: number): number {
  return Math.min(GRID_COLUMNS, Math.max(1, value));
}

function clampRowSpan(value: number): number {
  return Math.max(1, value);
}

function findFreeSlot(items: readonly FormItemValue[]): { col: number; row: number } {
  const occupied = new Set<string>();
  for (const item of items) {
    for (let c = 0; c < item.span; c += 1) {
      for (let r = 0; r < item.rowSpan; r += 1) {
        occupied.add(`${item.col + c},${item.row + r}`);
      }
    }
  }
  for (let row = 0; row < 50; row += 1) {
    for (let col = 0; col < GRID_COLUMNS; col += 1) {
      if (!occupied.has(`${col},${row}`)) return { col, row };
    }
  }
  return { col: 0, row: 0 };
}

/** True when placing an item of this footprint would overlap another. */
function collides(items: readonly FormItemValue[], moved: FormItemValue): boolean {
  const span = clampSpan(moved.span);
  const rowSpan = clampRowSpan(moved.rowSpan);
  return items.some((other) => {
    if (other.id === moved.id) return false;
    for (let c = 0; c < span; c += 1) {
      for (let r = 0; r < rowSpan; r += 1) {
        for (let oc = 0; oc < clampSpan(other.span); oc += 1) {
          for (let or = 0; or < clampRowSpan(other.rowSpan); or += 1) {
            if (moved.col + c === other.col + oc && moved.row + r === other.row + or) {
              return true;
            }
          }
        }
      }
    }
    return false;
  });
}

/**
 * FormBuilderEditor is the inspector entry point: a button that opens a
 * full-screen modal containing the visual grid editor. The modal gives the
 * grid enough space to be usable, unlike the narrow inspector panel.
 */
export function FormBuilderEditor({
  value,
  onChange,
}: {
  value: unknown;
  onChange: (value: FormLayoutValue) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button
        variant="ghost"
        icon="LayoutGrid"
        onClick={() => setOpen(true)}
      >
        {t("formBuilder.layout")}
      </Button>
      {open ? (
        <FormBuilderModal
          value={value}
          onChange={onChange}
          onClose={() => setOpen(false)}
        />
      ) : null}
    </>
  );
}

function FormBuilderModal({
  value,
  onChange,
  onClose,
}: {
  value: unknown;
  onChange: (value: FormLayoutValue) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const layout = useMemo(() => formLayoutFromValue(value), [value]);
  const [selectedID, setSelectedID] = useState<string | undefined>(undefined);
  const [draggedID, setDraggedID] = useState<string | undefined>(undefined);
  /** Local text drafts: config updates round-trip through the graph
   *  asynchronously, so inputs read from here to keep typing responsive. */
  const [edits, setEdits] = useState<Record<string, string>>({});
  const editText = (key: string, v: string) => setEdits((e) => ({ ...e, [key]: v }));
  const dropEdits = (prefix: string) =>
    setEdits((e) => Object.fromEntries(Object.entries(e).filter(([k]) => !k.startsWith(prefix))));

  const rows = useMemo(() => {
    const used = layout.items.reduce((max, item) => Math.max(max, item.row + item.rowSpan), 0);
    return Math.max(MIN_ROWS, used + 1);
  }, [layout.items]);

  const updateItem = (id: string, patch: Partial<FormItemValue>) => {
    const item = layout.items.find((candidate) => candidate.id === id);
    if (!item) return;
    const next: FormItemValue = { ...item, ...patch };
    // Collision check: if the updated item (with new span/rowSpan/col/row)
    // would overlap another item, abort the update.
    const span = clampSpan(next.span);
    const col = Math.min(GRID_COLUMNS - span, Math.max(0, next.col));
    const row = Math.max(0, next.row);
    const candidate = { ...next, span, col, row };
    if (collides(layout.items, candidate)) return;
    onChange({ items: layout.items.map((it) => (it.id === id ? candidate : it)) });
  };

  const addItem = (kind: FormItemValue["kind"]) => {
    const slot = findFreeSlot(layout.items);
    const id = nextItemID(layout.items);
    const item: FormItemValue =
      kind === "text"
        ? { id, kind, label: t("formBuilder.addText"), col: slot.col, row: slot.row, span: 4, rowSpan: 1 }
        : kind === "dropdown"
          ? { id, kind, label: t("formBuilder.addDropdown"), col: slot.col, row: slot.row, span: 2, rowSpan: 1, options: [{ value: "option_1" }] }
          : { id, kind, label: t("formBuilder.addInput"), col: slot.col, row: slot.row, span: 2, rowSpan: 1, inputType: "text" };
    onChange({ items: [...layout.items, item] });
    setSelectedID(id);
  };

  const deleteItem = (id: string) => {
    onChange({ items: layout.items.filter((item) => item.id !== id) });
    if (selectedID === id) setSelectedID(undefined);
    dropEdits(`${id}:`);
  };

  const moveItem = (id: string, col: number, row: number) => {
    const dragged = layout.items.find((item) => item.id === id);
    if (!dragged) return;
    const safeCol = Math.min(GRID_COLUMNS - clampSpan(dragged.span), Math.max(0, col));
    const safeRow = Math.max(0, row);
    // If the dragged item would overlap any other at the target position the
    // move is aborted — items never stack on top of each other.
    const candidate = { ...dragged, col: safeCol, row: safeRow };
    if (collides(layout.items, candidate)) return;
    updateItem(id, { col: safeCol, row: safeRow });
  };

  const addOption = (id: string) => {
    const item = layout.items.find((candidate) => candidate.id === id);
    if (!item) return;
    const options = [...(item.options ?? [])];
    const used = new Set(options.map((option) => option.value));
    let index = options.length + 1;
    while (used.has(`option_${index}`)) index += 1;
    options.push({ value: `option_${index}` });
    updateItem(id, { options });
  };

  const updateOption = (id: string, index: number, patch: Partial<{ value: string; label: string }>) => {
    const item = layout.items.find((candidate) => candidate.id === id);
    if (!item) return;
    const options = (item.options ?? []).map((option, current) =>
      current === index ? { ...option, ...patch } : option,
    );
    updateItem(id, { options });
  };

  const removeOption = (id: string, index: number) => {
    const item = layout.items.find((candidate) => candidate.id === id);
    if (!item) return;
    updateItem(id, { options: (item.options ?? []).filter((_, current) => current !== index) });
  };

  const selected = layout.items.find((item) => item.id === selectedID);
  const kindIcon = (kind: FormItemValue["kind"]) =>
    kind === "text" ? "Type" : kind === "dropdown" ? "ChevronDown" : "TextCursorInput";

  return (
    <Modal
      title={t("formBuilder.layout")}
      icon="LayoutGrid"
      size="full"
      onClose={onClose}
      bodyClassName="min-h-0 flex-1 overflow-hidden p-0"
      footer={
        <div className="ml-auto flex items-center gap-2">
          <Button onClick={onClose}>{t("formBuilder.done")}</Button>
        </div>
      }
    >
      {/* Toolbar */}
      <div className="flex shrink-0 flex-wrap items-center gap-1.5 border-b border-seam px-4 py-2">
        <Button variant="ghost" icon="Type" onClick={() => addItem("text")}>
          {t("formBuilder.addText")}
        </Button>
        <Button variant="ghost" icon="TextCursorInput" onClick={() => addItem("input")}>
          {t("formBuilder.addInput")}
        </Button>
        <Button variant="ghost" icon="ChevronDown" onClick={() => addItem("dropdown")}>
          {t("formBuilder.addDropdown")}
        </Button>
      </div>

      <div className="grid h-full min-h-0 grid-cols-1 lg:grid-cols-[1fr_260px]">
        {/* Grid */}
        <div className="muted-scroll min-h-0 overflow-auto border-b border-seam p-4 lg:border-b-0 lg:border-r">
          <div
            className="grid gap-2"
            style={{
              gridTemplateColumns: `repeat(${GRID_COLUMNS}, minmax(0, 1fr))`,
              gridTemplateRows: `repeat(${rows}, ${ROW_HEIGHT}px)`,
            }}
          >
            {Array.from({ length: rows * GRID_COLUMNS }).map((_, index) => {
              const col = index % GRID_COLUMNS;
              const row = Math.floor(index / GRID_COLUMNS);
              return (
                <div
                  key={`cell-${col}-${row}`}
                  onDragOver={(event) => event.preventDefault()}
                  onDrop={(event) => {
                    event.preventDefault();
                    if (draggedID) moveItem(draggedID, col, row);
                    setDraggedID(undefined);
                  }}
                  className="rounded border border-dashed border-ink-700/60 bg-ink-900/20"
                  style={{ gridColumn: `${col + 1}`, gridRow: `${row + 1}` }}
                />
              );
            })}
            {layout.items.map((item) => {
              const isSelected = item.id === selectedID;
              return (
                <button
                  key={item.id}
                  type="button"
                  draggable
                  onDragStart={() => setDraggedID(item.id)}
                  onDragEnd={() => setDraggedID(undefined)}
                  onDragOver={(event) => event.preventDefault()}
                  onDrop={(event) => {
                    event.preventDefault();
                    if (draggedID && draggedID !== item.id) moveItem(draggedID, item.col, item.row);
                    setDraggedID(undefined);
                  }}
                  onClick={() => setSelectedID(item.id)}
                  className={`group flex flex-col gap-1 rounded-md border p-2 text-left transition ${
                    isSelected
                      ? "border-ink-400 bg-ink-800/80 ring-1 ring-ring/60"
                      : "border-ink-700 bg-ink-850/70 hover:border-ink-600"
                  }`}
                  style={{
                    gridColumn: `${item.col + 1} / span ${clampSpan(item.span)}`,
                    gridRow: `${item.row + 1} / span ${clampRowSpan(item.rowSpan)}`,
                  }}
                  aria-label={item.label || item.id}
                >
                  <span className="flex items-center gap-1 text-[10px] tracking-wide text-fg-faint uppercase">
                    <Icon name={kindIcon(item.kind)} className="h-3 w-3" />
                    {item.kind}
                    <Icon name="GripVertical" className="ml-auto h-3 w-3 text-fg-faint opacity-0 transition-opacity group-hover:opacity-100" />
                  </span>
                  <span className="min-w-0 break-words text-xs font-medium text-fg">
                    {item.label || item.id}
                  </span>
                  {item.kind === "input" ? (
                    <span className="text-[10px] text-fg-faint">
                      {item.inputType === "number" ? t("editor.number") : t("editor.text")}
                    </span>
                  ) : null}
                  {item.kind === "dropdown" ? (
                    <span className="text-[10px] text-fg-faint">
                      {t("formBuilder.options")}: {(item.options ?? []).length}
                    </span>
                  ) : null}
                </button>
              );
            })}
          </div>
        </div>

        {/* Side panel */}
        <aside className="muted-scroll min-h-0 overflow-y-auto bg-ink-950/40 p-4">
          {selected ? (
            <div className="space-y-2.5">
              <div className="flex items-center justify-between gap-2">
                <span className="text-[11px] font-medium text-fg-muted">{t("formBuilder.selected")}</span>
                <Button
                  variant="ghost"
                  icon="Trash2"
                  onClick={() => deleteItem(selected.id)}
                  className="text-fg-faint hover:text-danger-fg"
                >
                  {t("formBuilder.deleteItem")}
                </Button>
              </div>
              <label className="block">
                <span className="mb-1 block text-[10px] font-medium text-fg-faint">{t("formBuilder.itemLabel")}</span>
                <TextInput
                  value={edits[`${selected.id}:label`] ?? selected.label}
                  onChange={(v) => {
                    editText(`${selected.id}:label`, v);
                    updateItem(selected.id, { label: v });
                  }}
                />
              </label>
              <label className="block">
                <span className="mb-1 block text-[10px] font-medium text-fg-faint">{t("formBuilder.span")}</span>
                <Dropdown
                  value={String(selected.span)}
                  onChange={(next) => updateItem(selected.id, { span: clampSpan(Number(next)) })}
                  options={[1, 2, 3, 4].map((n) => ({ value: String(n), label: String(n) }))}
                />
              </label>
              <label className="block">
                <span className="mb-1 block text-[10px] font-medium text-fg-faint">{t("formBuilder.rows")}</span>
                <TextInput
                  value={edits[`${selected.id}:rowSpan`] ?? String(selected.rowSpan)}
                  onChange={(v) => {
                    editText(`${selected.id}:rowSpan`, v);
                    updateItem(selected.id, { rowSpan: clampRowSpan(Number(v) || 1) });
                  }}
                />
              </label>
              {selected.kind === "input" ? (
                <>
                  <label className="block">
                    <span className="mb-1 block text-[10px] font-medium text-fg-faint">{t("formBuilder.inputType")}</span>
                    <Dropdown
                      value={selected.inputType === "number" ? "number" : "text"}
                      onChange={(next) => updateItem(selected.id, { inputType: next === "number" ? "number" : "text" })}
                      options={[
                        { value: "text", label: t("editor.text") },
                        { value: "number", label: t("editor.number") },
                      ]}
                    />
                  </label>
                  <label className="block">
                    <span className="mb-1 block text-[10px] font-medium text-fg-faint">{t("formBuilder.placeholder")}</span>
                    <TextInput
                      value={edits[`${selected.id}:placeholder`] ?? selected.placeholder ?? ""}
                      onChange={(v) => {
                        editText(`${selected.id}:placeholder`, v);
                        updateItem(selected.id, { placeholder: v });
                      }}
                    />
                  </label>
                </>
              ) : null}
              {selected.kind === "dropdown" ? (
                <div className="space-y-1.5">
                  <span className="block text-[10px] font-medium text-fg-faint">{t("formBuilder.options")}</span>
                  {(selected.options ?? []).map((option, index) => (
                    <div key={`${selected.id}-${index}`} className="space-y-1 rounded-md border border-ink-700 bg-ink-850/60 p-1.5">
                      <TextInput
                        value={edits[`${selected.id}:opt${index}:value`] ?? option.value}
                        onChange={(v) => {
                          editText(`${selected.id}:opt${index}:value`, v);
                          updateOption(selected.id, index, { value: v });
                        }}
                        placeholder={t("formBuilder.optionValue")}
                      />
                      <TextInput
                        value={edits[`${selected.id}:opt${index}:label`] ?? option.label ?? ""}
                        onChange={(v) => {
                          editText(`${selected.id}:opt${index}:label`, v);
                          updateOption(selected.id, index, { label: v });
                        }}
                        placeholder={t("formBuilder.optionLabel")}
                      />
                      <Button
                        variant="ghost"
                        icon="Trash2"
                        onClick={() => removeOption(selected.id, index)}
                        className="h-6 px-2 text-[11px] text-fg-faint hover:text-danger-fg"
                      >
                        {t("formBuilder.deleteItem")}
                      </Button>
                    </div>
                  ))}
                  <Button variant="ghost" icon="Plus" onClick={() => addOption(selected.id)}>
                    {t("formBuilder.addOption")}
                  </Button>
                </div>
              ) : null}
            </div>
          ) : (
            <p className="text-[11px] text-fg-faint">{t("formBuilder.noSelection")}</p>
          )}
        </aside>
      </div>
    </Modal>
  );
}
