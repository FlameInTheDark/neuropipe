import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Icon } from "../icons";
import { DrawImageDoc } from "@/lib/draw-image";
import { elementTypeIcon } from "./shared";

export interface LayersPanelProps {
  doc: DrawImageDoc;
  activeLayerId: string;
  selectedElementId: string | null;
  onActiveLayer(id: string): void;
  onSelectElement(id: string | null): void;
  onPatchLayer(id: string, patch: Partial<DrawImageDoc["layers"][number]>): void;
  onAddLayer(): void;
  onDeleteLayer(id: string): void;
  onDuplicateLayer(id: string): void;
  onMoveLayer(id: string, direction: -1 | 1): void;
  onPatchElement(id: string, patch: Record<string, unknown>): void;
  onDeleteElement(id: string): void;
  onDuplicateElement(id: string): void;
  onMoveElement(id: string, direction: -1 | 1): void;
}

export function LayersPanel(props: LayersPanelProps) {
  const { t } = useTranslation();
  const { doc } = props;
  const [renaming, setRenaming] = useState<string | null>(null);

  const layerOf = (id: string) => doc.layers.find((layer) => layer.id === id);

  return (
    <div className="flex h-full min-h-0 flex-col">
      {/* layers */}
      <div className="flex items-center justify-between px-3 pb-1.5 pt-2.5">
        <span className="text-[10px] font-semibold uppercase tracking-[0.12em] text-fg-faint">{t("drawImage.layers")}</span>
        <div className="flex items-center gap-0.5">
          <button
            title={t("drawImage.addLayer")}
            onClick={props.onAddLayer}
            className="grid h-5 w-5 place-items-center rounded text-fg-faint transition hover:bg-ink-750 hover:text-fg"
          >
            <Icon name="Plus" className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
      <div className="muted-scroll max-h-[38%] min-h-0 overflow-y-auto px-2 pb-1">
        {[...doc.layers].reverse().map((layer) => (
          <div
            key={layer.id}
            onClick={() => props.onActiveLayer(layer.id)}
            className={`group relative mb-1 flex cursor-pointer items-center gap-1 rounded-md border px-2 py-1.5 transition ${
              layer.id === props.activeLayerId
                ? "border-ink-500 bg-ink-800/80"
                : "border-transparent hover:border-ink-700 hover:bg-ink-850/60"
            }`}
          >
            <button
              title={layer.visible ? t("drawImage.hideLayer") : t("drawImage.showLayer")}
              onClick={(event) => {
                event.stopPropagation();
                props.onPatchLayer(layer.id, { visible: !layer.visible });
              }}
              className={`grid h-5 w-5 shrink-0 place-items-center rounded transition ${
                layer.visible ? "text-fg-subtle hover:bg-ink-750 hover:text-fg" : "text-fg-faint/60 hover:bg-ink-750"
              }`}
            >
              <Icon name={layer.visible ? "Eye" : "EyeOff"} className="h-3.5 w-3.5" />
            </button>
            <button
              title={layer.locked ? t("drawImage.unlockLayer") : t("drawImage.lockLayer")}
              onClick={(event) => {
                event.stopPropagation();
                props.onPatchLayer(layer.id, { locked: !layer.locked });
              }}
              className={`grid h-5 w-5 shrink-0 place-items-center rounded transition ${
                layer.locked ? "text-warning" : "text-fg-faint/60 hover:bg-ink-750 hover:text-fg-subtle"
              }`}
            >
              <Icon name={layer.locked ? "Lock" : "Unlock"} className="h-3.5 w-3.5" />
            </button>
            {renaming === layer.id ? (
              <input
                autoFocus
                defaultValue={layer.name}
                onClick={(event) => event.stopPropagation()}
                onBlur={(event) => {
                  props.onPatchLayer(layer.id, { name: event.target.value || layer.name });
                  setRenaming(null);
                }}
                onKeyDown={(event) => {
                  if (event.key === "Enter") (event.target as HTMLInputElement).blur();
                  if (event.key === "Escape") setRenaming(null);
                }}
                className="min-w-0 flex-1 rounded border border-ink-500 bg-ink-950 px-1.5 py-0.5 text-[11.5px] text-fg outline-none"
              />
            ) : (
              <span
                className="min-w-0 flex-1 truncate text-[11.5px] text-fg"
                onDoubleClick={(event) => {
                  event.stopPropagation();
                  setRenaming(layer.id);
                }}
                title={layer.name}
              >
                {layer.name}
              </span>
            )}
            {layer.opacity < 1 ? (
              <span className="shrink-0 font-mono text-[9.5px] text-fg-faint">{Math.round(layer.opacity * 100)}%</span>
            ) : null}
            <div className="pointer-events-none absolute right-[30px] top-1/2 flex -translate-y-1/2 items-center gap-0.5 rounded bg-ink-850/95 opacity-0 shadow transition group-hover:pointer-events-auto group-hover:opacity-100">
              <button
                title={t("drawImage.moveUp")}
                onClick={(event) => {
                  event.stopPropagation();
                  props.onMoveLayer(layer.id, 1);
                }}
                className="grid h-5 w-5 place-items-center rounded text-fg-faint transition hover:bg-ink-750 hover:text-fg"
              >
                <Icon name="ChevronUp" className="h-3 w-3" />
              </button>
              <button
                title={t("drawImage.moveDown")}
                onClick={(event) => {
                  event.stopPropagation();
                  props.onMoveLayer(layer.id, -1);
                }}
                className="grid h-5 w-5 place-items-center rounded text-fg-faint transition hover:bg-ink-750 hover:text-fg"
              >
                <Icon name="ChevronDown" className="h-3 w-3" />
              </button>
              <button
                title={t("drawImage.duplicateLayer")}
                onClick={(event) => {
                  event.stopPropagation();
                  props.onDuplicateLayer(layer.id);
                }}
                className="grid h-5 w-5 place-items-center rounded text-fg-faint transition hover:bg-ink-750 hover:text-fg"
              >
                <Icon name="Copy" className="h-3 w-3" />
              </button>
              <button
                title={t("drawImage.deleteLayer")}
                onClick={(event) => {
                  event.stopPropagation();
                  props.onDeleteLayer(layer.id);
                }}
                className="grid h-5 w-5 place-items-center rounded text-fg-faint transition hover:bg-ink-750 hover:text-danger-fg"
              >
                <Icon name="Trash2" className="h-3 w-3" />
              </button>
            </div>
          </div>
        ))}
      </div>

      {/* layer opacity for the active layer */}
      {(() => {
        const active = layerOf(props.activeLayerId);
        if (!active) return null;
        return (
          <div className="flex items-center gap-2 border-t border-seam px-3 py-2">
            <span className="text-[10.5px] text-fg-faint">{t("drawImage.layerOpacity")}</span>
            <input
              type="range"
              min={0}
              max={100}
              value={Math.round(active.opacity * 100)}
              onChange={(event) => props.onPatchLayer(active.id, { opacity: Number(event.target.value) / 100 })}
              className="h-1 flex-1 cursor-pointer appearance-none rounded bg-ink-700 accent-fg"
            />
            <span className="w-8 shrink-0 text-right font-mono text-[10px] text-fg-faint">{Math.round(active.opacity * 100)}%</span>
          </div>
        );
      })()}

      {/* elements */}
      <div className="flex items-center justify-between border-t border-seam px-3 pb-1.5 pt-2">
        <span className="text-[10px] font-semibold uppercase tracking-[0.12em] text-fg-faint">{t("drawImage.elements")}</span>
        <span className="text-[10px] text-fg-faint">{doc.elements.length}</span>
      </div>
      <div className="muted-scroll min-h-0 flex-1 overflow-y-auto px-2 pb-2">
        {doc.elements.length === 0 ? (
          <p className="px-2 py-3 text-[11px] leading-5 text-fg-faint">{t("drawImage.elementsEmpty")}</p>
        ) : (
          [...doc.elements]
            .map((element, index) => ({ element, index }))
            .reverse()
            .map(({ element, index }) => (
              <div
                key={element.id}
                onClick={() => {
                  props.onSelectElement(element.id);
                  props.onActiveLayer(element.layerId);
                }}
                className={`group relative mb-1 flex cursor-pointer items-center gap-1.5 rounded-md border px-2 py-1.5 transition ${
                  element.id === props.selectedElementId
                    ? "border-ink-500 bg-ink-800/80"
                    : "border-transparent hover:border-ink-700 hover:bg-ink-850/60"
                }`}
              >
                <Icon name={elementTypeIcon(element.type)} className="h-3.5 w-3.5 shrink-0 text-fg-subtle" />
                <span className="min-w-0 flex-1 truncate text-[11.5px] text-fg" title={`${element.name} · ${layerOf(element.layerId)?.name ?? ""}`}>
                  {element.name}
                </span>
                {element.visibility.mode === "condition" ? <Icon name="Filter" className="h-3 w-3 shrink-0 text-teal" /> : null}
                {element.repeat ? <Icon name="Repeat" className="h-3 w-3 shrink-0 text-orange" /> : null}
                {element.opacity < 1 ? (
                  <span className="shrink-0 font-mono text-[9.5px] text-fg-faint">{Math.round(element.opacity * 100)}%</span>
                ) : null}
                <button
                  title={element.visible ? t("drawImage.hideElement") : t("drawImage.showElement")}
                  onClick={(event) => {
                    event.stopPropagation();
                    props.onPatchElement(element.id, { visible: !element.visible });
                  }}
                  className={`grid h-5 w-5 shrink-0 place-items-center rounded transition ${
                    element.visible ? "text-fg-subtle hover:bg-ink-750 hover:text-fg" : "text-fg-faint/60 hover:bg-ink-750"
                  }`}
                >
                  <Icon name={element.visible ? "Eye" : "EyeOff"} className="h-3.5 w-3.5" />
                </button>
                <div className="pointer-events-none absolute right-[30px] top-1/2 flex -translate-y-1/2 items-center gap-0.5 rounded bg-ink-850/95 opacity-0 shadow transition group-hover:pointer-events-auto group-hover:opacity-100">
                  <button
                    title={t("drawImage.moveUp")}
                    onClick={(event) => {
                      event.stopPropagation();
                      if (index < doc.elements.length - 1) props.onMoveElement(element.id, 1);
                    }}
                    className="grid h-5 w-5 place-items-center rounded text-fg-faint transition hover:bg-ink-750 hover:text-fg"
                  >
                    <Icon name="ChevronUp" className="h-3 w-3" />
                  </button>
                  <button
                    title={t("drawImage.moveDown")}
                    onClick={(event) => {
                      event.stopPropagation();
                      if (index > 0) props.onMoveElement(element.id, -1);
                    }}
                    className="grid h-5 w-5 place-items-center rounded text-fg-faint transition hover:bg-ink-750 hover:text-fg"
                  >
                    <Icon name="ChevronDown" className="h-3 w-3" />
                  </button>
                  <button
                    title={t("drawImage.duplicateElement")}
                    onClick={(event) => {
                      event.stopPropagation();
                      props.onDuplicateElement(element.id);
                    }}
                    className="grid h-5 w-5 place-items-center rounded text-fg-faint transition hover:bg-ink-750 hover:text-fg"
                  >
                    <Icon name="Copy" className="h-3 w-3" />
                  </button>
                  <button
                    title={t("drawImage.deleteElement")}
                    onClick={(event) => {
                      event.stopPropagation();
                      props.onDeleteElement(element.id);
                    }}
                    className="grid h-5 w-5 place-items-center rounded text-fg-faint transition hover:bg-ink-750 hover:text-danger-fg"
                  >
                    <Icon name="Trash2" className="h-3 w-3" />
                  </button>
                </div>
              </div>
            ))
        )}
      </div>
    </div>
  );
}
