import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button, Toggle } from "../ui";
import { Dropdown } from "../Dropdown";
import { TextInput } from "../primitives/Field";
import { DrawElement, DrawImageDoc, MAX_CANVAS_DIMENSION } from "@/lib/draw-image";
import { desktop } from "@/lib/bridge";
import { ColorInput, NumInput, PaintEditor, PropRow, RepeatEditor, StrokeEditor, VisibilityEditor } from "./shared";

export interface PropertiesPanelProps {
  doc: DrawImageDoc;
  element: DrawElement | null;
  onPatchElement(id: string, patch: Partial<DrawElement>): void;
  onPatchDoc(patch: Partial<Pick<DrawImageDoc, "width" | "height" | "background">>): void;
}

export function PropertiesPanel(props: PropertiesPanelProps) {
  const { t } = useTranslation();
  const { doc, element } = props;

  if (!element) {
    return (
      <div className="space-y-3 p-3">
        <SectionTitle>{t("drawImage.canvasSection")}</SectionTitle>
        <div className="space-y-2 rounded-md border border-ink-700 bg-ink-950/50 p-2.5">
          <PropRow label={t("drawImage.canvasWidth")}>
            <NumInput
              value={doc.width}
              min={1}
              max={MAX_CANVAS_DIMENSION}
              onChange={(width) => props.onPatchDoc({ width: Math.round(width) })}
            />
          </PropRow>
          <PropRow label={t("drawImage.canvasHeight")}>
            <NumInput
              value={doc.height}
              min={1}
              max={MAX_CANVAS_DIMENSION}
              onChange={(height) => props.onPatchDoc({ height: Math.round(height) })}
            />
          </PropRow>
          <PropRow label={t("drawImage.background")}>
            <ColorInput value={doc.background === "transparent" ? "#0b0c0d" : doc.background} onChange={(background) => props.onPatchDoc({ background })} />
          </PropRow>
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-medium text-fg-subtle">{t("drawImage.transparentBg")}</span>
            <Toggle on={doc.background === "transparent"} onChange={(on) => props.onPatchDoc({ background: on ? "transparent" : "#0b0c0d" })} />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-1.5">
          {[
            { w: 1920, h: 1080, label: "1920 × 1080" },
            { w: 1200, h: 630, label: "1200 × 630" },
            { w: 800, h: 450, label: "800 × 450" },
            { w: 512, h: 512, label: "512 × 512" },
          ].map((preset) => (
            <Button
              key={preset.label}
              variant="ghost"
              className="h-6 px-2 text-[11px]"
              onClick={() => props.onPatchDoc({ width: preset.w, height: preset.h })}
            >
              {preset.label}
            </Button>
          ))}
        </div>
        <p className="px-1 text-[10.5px] leading-4 text-fg-faint">{t("drawImage.canvasHint")}</p>
      </div>
    );
  }

  return (
    <div className="muted-scroll h-full space-y-3 overflow-y-auto p-3">
      <SectionTitle>
        {element.name} · {t(`drawImage.types.${element.type}`)}
      </SectionTitle>

      {/* common geometry */}
      <div className="space-y-2 rounded-md border border-ink-700 bg-ink-950/50 p-2.5">
        <ElementNameInput element={element} onPatch={props.onPatchElement} />
        <div className="grid grid-cols-2 gap-x-2 gap-y-1.5">
          <PropRow label="X">
            <NumInput value={element.x} onChange={(x) => props.onPatchElement(element.id, { x })} width="w-full" />
          </PropRow>
          <PropRow label="Y">
            <NumInput value={element.y} onChange={(y) => props.onPatchElement(element.id, { y })} width="w-full" />
          </PropRow>
          <PropRow label="W">
            <NumInput value={element.w} onChange={(w) => props.onPatchElement(element.id, { w })} width="w-full" />
          </PropRow>
          <PropRow label="H">
            <NumInput value={element.h} onChange={(h) => props.onPatchElement(element.id, { h })} width="w-full" />
          </PropRow>
        </div>
        {element.type !== "line" ? (
          <PropRow label={t("drawImage.rotation")}>
            <NumInput value={element.rotation} min={-360} max={360} onChange={(rotation) => props.onPatchElement(element.id, { rotation })} />
          </PropRow>
        ) : null}
        <PropRow label={t("drawImage.opacity")}>
          <input
            type="range"
            min={0}
            max={100}
            value={Math.round(element.opacity * 100)}
            onChange={(event) => props.onPatchElement(element.id, { opacity: Number(event.target.value) / 100 })}
            className="h-1 w-full max-w-[110px] cursor-pointer appearance-none rounded bg-ink-700 accent-fg"
          />
          <span className="w-8 shrink-0 text-right font-mono text-[10px] text-fg-faint">{Math.round(element.opacity * 100)}%</span>
        </PropRow>
      </div>

      {/* type-specific */}
      {(element.type === "rect" || element.type === "ellipse" || element.type === "star") && (
        <>
          <PaintEditor value={element.fill} onChange={(fill) => props.onPatchElement(element.id, { fill })} />
          <StrokeEditor value={element.stroke} onChange={(stroke) => props.onPatchElement(element.id, { stroke })} />
          {element.type === "rect" ? (
            <div className="rounded-md border border-ink-700 bg-ink-950/50 p-2.5">
              <PropRow label={t("drawImage.cornerRadius")}>
                <NumInput value={element.radius} min={0} onChange={(radius) => props.onPatchElement(element.id, { radius })} />
              </PropRow>
            </div>
          ) : null}
          {element.type === "star" ? (
            <div className="space-y-2 rounded-md border border-ink-700 bg-ink-950/50 p-2.5">
              <PropRow label={t("drawImage.starPoints")}>
                <NumInput value={element.starPoints} min={3} max={24} onChange={(starPoints) => props.onPatchElement(element.id, { starPoints: Math.round(starPoints) })} />
              </PropRow>
              <PropRow label={t("drawImage.innerRatio")}>
                <NumInput value={element.innerRatio} min={0.05} max={0.95} step={0.05} onChange={(innerRatio) => props.onPatchElement(element.id, { innerRatio })} />
              </PropRow>
            </div>
          ) : null}
        </>
      )}

      {element.type === "line" && <StrokeEditor value={element.stroke} onChange={(stroke) => props.onPatchElement(element.id, { stroke })} />}

      {element.type === "text" && <TextProperties element={element} onPatch={props.onPatchElement} />}

      {element.type === "image" && <ImageProperties element={element} doc={doc} onPatch={props.onPatchElement} />}

      {/* behavior */}
      <VisibilityEditor value={element.visibility} doc={doc} onChange={(visibility) => props.onPatchElement(element.id, { visibility })} />
      <RepeatEditor value={element.repeat} doc={doc} onChange={(repeat) => props.onPatchElement(element.id, { repeat })} />
    </div>
  );
}

function SectionTitle({ children }: { children: React.ReactNode }) {
  return <span className="block px-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-fg-faint">{children}</span>;
}

function ElementNameInput({ element, onPatch }: { element: DrawElement; onPatch: PropertiesPanelProps["onPatchElement"] }) {
  const { t } = useTranslation();
  return (
    <TextInput value={element.name} placeholder={t("drawImage.elementName")} onChange={(name) => onPatch(element.id, { name })} />
  );
}

function TextProperties({ element, onPatch }: { element: DrawElement; onPatch: PropertiesPanelProps["onPatchElement"] }) {
  const { t } = useTranslation();
  return (
    <>
      <div className="space-y-2 rounded-md border border-ink-700 bg-ink-950/50 p-2.5">
        <textarea
          value={element.content}
          onChange={(event) => onPatch(element.id, { content: event.target.value })}
          placeholder={t("drawImage.textContent")}
          rows={3}
          className="w-full resize-y rounded-md border border-ink-700 bg-ink-950 px-2 py-1.5 text-[12px] leading-5 text-fg outline-none transition focus:border-ink-500"
        />
        <p className="text-[10px] leading-4 text-fg-faint">{t("drawImage.textPlaceholderHint")}</p>
        <div className="grid grid-cols-2 gap-1.5">
          <Dropdown
            compact
            value={element.fontFamily}
            onChange={(fontFamily) => onPatch(element.id, { fontFamily: fontFamily as DrawElement["fontFamily"] })}
            options={[
              { value: "inter", label: "Inter" },
              { value: "jetbrains-mono", label: "JetBrains Mono" },
            ]}
          />
          <Dropdown
            compact
            value={String(element.weight)}
            onChange={(weight) => onPatch(element.id, { weight: Number(weight) })}
            options={[
              { value: "400", label: "400" },
              { value: "500", label: "500" },
              { value: "600", label: "600" },
              { value: "700", label: "700" },
              { value: "800", label: "800" },
            ]}
          />
        </div>
        <div className="grid grid-cols-2 gap-x-2 gap-y-1.5">
          <PropRow label={t("drawImage.fontSize")}>
            <NumInput value={element.fontSize} min={1} max={512} onChange={(fontSize) => onPatch(element.id, { fontSize })} width="w-full" />
          </PropRow>
          <PropRow label={t("drawImage.lineHeight")}>
            <NumInput value={element.lineHeight} min={0.5} max={3} step={0.1} onChange={(lineHeight) => onPatch(element.id, { lineHeight })} width="w-full" />
          </PropRow>
        </div>
        <PropRow label={t("drawImage.color")}>
          <ColorInput value={element.color} onChange={(color) => onPatch(element.id, { color })} />
        </PropRow>
        <div className="grid grid-cols-2 gap-1.5">
          <Dropdown
            compact
            value={element.align}
            onChange={(align) => onPatch(element.id, { align: align as DrawElement["align"] })}
            options={[
              { value: "left", label: t("drawImage.alignLeft") },
              { value: "center", label: t("drawImage.alignCenter") },
              { value: "right", label: t("drawImage.alignRight") },
            ]}
          />
          <Dropdown
            compact
            value={element.valign}
            onChange={(valign) => onPatch(element.id, { valign: valign as DrawElement["valign"] })}
            options={[
              { value: "top", label: t("drawImage.valignTop") },
              { value: "middle", label: t("drawImage.valignMiddle") },
              { value: "bottom", label: t("drawImage.valignBottom") },
            ]}
          />
        </div>
        <div className="flex items-center justify-between">
          <span className="text-[11px] font-medium text-fg-subtle">{t("drawImage.italic")}</span>
          <Toggle on={element.italic} onChange={(italic) => onPatch(element.id, { italic })} />
        </div>
        <PropRow label={t("drawImage.wrapWidth")}>
          <NumInput
            value={element.wrapWidth}
            min={-1}
            onChange={(wrapWidth) => onPatch(element.id, { wrapWidth })}
          />
        </PropRow>
        <p className="text-[10px] leading-4 text-fg-faint">{t("drawImage.wrapHint")}</p>
      </div>
    </>
  );
}

function ImageProperties({
  element,
  doc,
  onPatch,
}: {
  element: DrawElement;
  doc: DrawImageDoc;
  onPatch: PropertiesPanelProps["onPatchElement"];
}) {
  const { t } = useTranslation();
  const [picking, setPicking] = useState(false);
  const pinOptions = doc.pins.map((pin) => ({ value: pin.name, label: pin.name }));

  const chooseFile = async () => {
    setPicking(true);
    try {
      const path = await desktop.chooseImageFile();
      if (path) onPatch(element.id, { source: { kind: "path", value: path } });
    } catch {
      /* dialog cancelled */
    } finally {
      setPicking(false);
    }
  };

  return (
    <div className="space-y-2 rounded-md border border-ink-700 bg-ink-950/50 p-2.5">
      <Dropdown
        compact
        value={element.source.kind}
        onChange={(kind) => onPatch(element.id, { source: { kind: kind as DrawElement["source"]["kind"], value: "" } })}
        options={[
          { value: "url", label: t("drawImage.sourceUrl") },
          { value: "path", label: t("drawImage.sourcePath") },
          { value: "pin", label: t("drawImage.sourcePin") },
        ]}
      />
      {element.source.kind === "pin" ? (
        <Dropdown
          compact
          value={element.source.value}
          placeholder={t("drawImage.pickPin")}
          onChange={(value) => onPatch(element.id, { source: { kind: "pin", value } })}
          options={pinOptions}
        />
      ) : (
        <>
          <TextInput
            value={element.source.value}
            placeholder={element.source.kind === "url" ? "https://example.com/icon.png" : t("drawImage.pathPlaceholder")}
            onChange={(value) => onPatch(element.id, { source: { ...element.source, value } })}
          />
          {element.source.kind === "path" ? (
            <Button variant="ghost" icon="FolderOpen" className="h-6 px-2 text-[11px]" onClick={chooseFile} disabled={picking}>
              {t("drawImage.browse")}
            </Button>
          ) : null}
        </>
      )}
      <Dropdown
        compact
        value={element.fit}
        onChange={(fit) => onPatch(element.id, { fit: fit as DrawElement["fit"] })}
        options={[
          { value: "fill", label: t("drawImage.fitFill") },
          { value: "contain", label: t("drawImage.fitContain") },
          { value: "cover", label: t("drawImage.fitCover") },
        ]}
      />
      <PropRow label={t("drawImage.cornerRadius")}>
        <NumInput value={element.radius} min={0} onChange={(radius) => onPatch(element.id, { radius })} />
      </PropRow>
      <PropRow label={t("drawImage.onMissing")}>
        <Dropdown
          compact
          className="w-[110px]"
          value={element.onMissing}
          onChange={(onMissing) => onPatch(element.id, { onMissing: onMissing as DrawElement["onMissing"] })}
          options={[
            { value: "skip", label: t("drawImage.onMissingSkip") },
            { value: "error", label: t("drawImage.onMissingError") },
          ]}
        />
      </PropRow>
      <p className="text-[10px] leading-4 text-fg-faint">{t("drawImage.imageHint")}</p>
    </div>
  );
}
