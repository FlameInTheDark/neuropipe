import { useTranslation } from "react-i18next";
import { Button, Toggle } from "../ui";
import { Dropdown } from "../Dropdown";
import { TextInput } from "../primitives/Field";
import {
  CONDITION_OPS,
  DrawElement,
  DrawImageDoc,
  DrawPaint,
  DrawPinType,
  DrawRepeat,
  DrawStroke,
  DrawVisibility,
  isPseudoPin,
  OPS_WITHOUT_VALUE,
} from "@/lib/draw-image";

/* ------------------------------------------------------------------ */
/* small controls                                                      */
/* ------------------------------------------------------------------ */

export function PropRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex items-center justify-between gap-2">
      <span className="shrink-0 text-[11px] font-medium text-fg-subtle">{label}</span>
      <span className="flex min-w-0 flex-1 justify-end gap-1.5">{children}</span>
    </label>
  );
}

export function NumInput({
  value,
  onChange,
  placeholder,
  width = "w-[64px]",
  step = 1,
  min,
  max,
}: {
  value: number;
  onChange: (value: number) => void;
  placeholder?: string;
  width?: string;
  step?: number;
  min?: number;
  max?: number;
}) {
  return (
    <input
      type="number"
      value={Number.isFinite(value) ? Math.round(value * 100) / 100 : 0}
      step={step}
      min={min}
      max={max}
      placeholder={placeholder}
      onChange={(event) => {
        const parsed = Number(event.target.value);
        if (Number.isFinite(parsed)) onChange(parsed);
      }}
      className={`${width} rounded-md border border-ink-700 bg-ink-850 px-1.5 py-1 text-right text-[11.5px] text-fg outline-none transition focus:border-ink-500`}
    />
  );
}

export function ColorInput({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return (
    <span className="flex items-center gap-1.5">
      <input
        type="color"
        value={normalizeColorPicker(value)}
        onChange={(event) => onChange(event.target.value)}
        className="h-6 w-7 cursor-pointer rounded border border-ink-700 bg-ink-850 p-0.5"
      />
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        spellCheck={false}
        className="w-[76px] rounded-md border border-ink-700 bg-ink-850 px-1.5 py-1 font-mono text-[11px] uppercase text-fg outline-none transition focus:border-ink-500"
      />
    </span>
  );
}

function normalizeColorPicker(hex: string): string {
  const value = hex.startsWith("#") ? hex.slice(1) : hex;
  if (value.length === 3 || value.length === 6) return `#${value.slice(0, 6)}`;
  if (value.length === 8) return `#${value.slice(0, 6)}`;
  return "#000000";
}

/* ------------------------------------------------------------------ */
/* paint editor (solid / linear / radial)                              */
/* ------------------------------------------------------------------ */

export function PaintEditor({ value, onChange }: { value: DrawPaint; onChange: (value: DrawPaint) => void }) {
  const { t } = useTranslation();
  return (
    <div className="space-y-2 rounded-md border border-ink-700 bg-ink-950/50 p-2">
      <div className="flex items-center justify-between">
        <span className="text-[11px] font-semibold text-fg-muted">{t("drawImage.fill")}</span>
        <Dropdown
          compact
          className="w-[110px]"
          value={value.type}
          onChange={(next) => {
            if (next === "solid") {
              onChange({ type: "solid", color: value.type === "solid" ? value.color : "#4ea7fc" });
            } else if (next === "linear") {
              onChange({
                type: "linear",
                x0: 0,
                y0: 0,
                x1: 200,
                y1: 0,
                stops: value.type === "radial" ? value.stops : [
                  { offset: 0, color: "#4ea7fc" },
                  { offset: 1, color: "#00b8cc" },
                ],
              });
            } else {
              onChange({
                type: "radial",
                cx: 100,
                cy: 100,
                r: 100,
                stops: value.type === "linear" ? value.stops : [
                  { offset: 0, color: "#f0bf00" },
                  { offset: 1, color: "#eb5757" },
                ],
              });
            }
          }}
          options={[
            { value: "solid", label: t("drawImage.paintSolid") },
            { value: "linear", label: t("drawImage.paintLinear") },
            { value: "radial", label: t("drawImage.paintRadial") },
          ]}
        />
      </div>
      {value.type === "solid" ? (
        <PropRow label={t("drawImage.color")}>
          <ColorInput value={value.color} onChange={(color) => onChange({ type: "solid", color })} />
        </PropRow>
      ) : (
        <GradientEditor value={value} onChange={onChange} />
      )}
    </div>
  );
}

function GradientEditor({ value, onChange }: { value: Extract<DrawPaint, { type: "linear" | "radial" }>; onChange: (value: DrawPaint) => void }) {
  const { t } = useTranslation();
  const stops = value.stops;
  const patchStop = (index: number, patch: Partial<{ offset: number; color: string }>) => {
    const next = stops.map((stop, current) => (current === index ? { ...stop, ...patch } : stop));
    onChange({ ...value, stops: next } as DrawPaint);
  };
  const addStop = () => {
    const last = stops[stops.length - 1]?.offset ?? 1;
    onChange({ ...value, stops: [...stops, { offset: Math.max(0, last - 0.25), color: "#ffffff" }] } as DrawPaint);
  };
  const removeStop = (index: number) => {
    if (stops.length <= 2) return;
    onChange({ ...value, stops: stops.filter((_, current) => current !== index) } as DrawPaint);
  };
  return (
    <div className="space-y-1.5">
      <div
        className="h-4 rounded border border-ink-700"
        style={{
          background:
            value.type === "linear"
              ? `linear-gradient(to right, ${stops.map((s) => `${s.color} ${s.offset * 100}%`).join(", ")})`
              : `radial-gradient(circle, ${stops.map((s) => `${s.color} ${s.offset * 100}%`).join(", ")})`,
        }}
      />
      {stops.map((stop, index) => (
        <div key={index} className="flex items-center gap-1.5">
          <NumInput value={stop.offset} min={0} max={1} step={0.05} width="w-[52px]" onChange={(offset) => patchStop(index, { offset })} />
          <ColorInput value={stop.color} onChange={(color) => patchStop(index, { color })} />
          <Button variant="ghost" icon="Trash2" className="h-6 px-1.5 text-fg-faint hover:text-danger-fg" onClick={() => removeStop(index)}>
            {""}
          </Button>
        </div>
      ))}
      <Button variant="ghost" icon="Plus" className="h-6 px-2 text-[11px]" onClick={addStop}>
        {t("drawImage.addStop")}
      </Button>
      {value.type === "linear" ? (
        <div className="grid grid-cols-4 gap-1">
          {(["x0", "y0", "x1", "y1"] as const).map((key) => (
            <input
              key={key}
              type="number"
              value={value[key]}
              onChange={(event) => {
                const parsed = Number(event.target.value);
                if (Number.isFinite(parsed)) onChange({ ...value, [key]: parsed } as DrawPaint);
              }}
              placeholder={key}
              className="rounded-md border border-ink-700 bg-ink-850 px-1.5 py-1 text-right text-[11px] text-fg outline-none focus:border-ink-500"
            />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-3 gap-1">
          {(["cx", "cy", "r"] as const).map((key) => (
            <input
              key={key}
              type="number"
              value={value[key]}
              onChange={(event) => {
                const parsed = Number(event.target.value);
                if (Number.isFinite(parsed)) onChange({ ...value, [key]: parsed } as DrawPaint);
              }}
              placeholder={key}
              className="rounded-md border border-ink-700 bg-ink-850 px-1.5 py-1 text-right text-[11px] text-fg outline-none focus:border-ink-500"
            />
          ))}
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* stroke editor                                                       */
/* ------------------------------------------------------------------ */

export function StrokeEditor({
  value,
  onChange,
}: {
  value: DrawStroke | null;
  onChange: (value: DrawStroke | null) => void;
}) {
  const { t } = useTranslation();
  const stroke = value ?? { color: "#f7f8f8", width: 2, dash: [], cap: "round" as const, join: "round" as const };
  return (
    <div className="space-y-2 rounded-md border border-ink-700 bg-ink-950/50 p-2">
      <div className="flex items-center justify-between">
        <span className="text-[11px] font-semibold text-fg-muted">{t("drawImage.stroke")}</span>
        <Toggle on={value !== null} onChange={(on) => onChange(on ? stroke : null)} />
      </div>
      {value !== null && (
        <>
          <PropRow label={t("drawImage.color")}>
            <ColorInput value={value.color} onChange={(color) => onChange({ ...value, color })} />
          </PropRow>
          <PropRow label={t("drawImage.width")}>
            <NumInput value={value.width} min={0} step={0.5} onChange={(width) => onChange({ ...value, width })} />
          </PropRow>
          <PropRow label={t("drawImage.dash")}>
            <TextInput
              value={value.dash.join(", ")}
              placeholder="8, 4"
              onChange={(dash) =>
                onChange({
                  ...value,
                  dash: dash
                    .split(",")
                    .map((part) => Number(part.trim()))
                    .filter((n) => Number.isFinite(n) && n > 0),
                })
              }
            />
          </PropRow>
          <div className="grid grid-cols-2 gap-1.5">
            <Dropdown
              compact
              value={value.cap}
              onChange={(cap) => onChange({ ...value, cap: cap as DrawStroke["cap"] })}
              options={[
                { value: "butt", label: t("drawImage.capButt") },
                { value: "round", label: t("drawImage.capRound") },
                { value: "square", label: t("drawImage.capSquare") },
              ]}
            />
            <Dropdown
              compact
              value={value.join}
              onChange={(join) => onChange({ ...value, join: join as DrawStroke["join"] })}
              options={[
                { value: "miter", label: t("drawImage.joinMiter") },
                { value: "round", label: t("drawImage.joinRound") },
                { value: "bevel", label: t("drawImage.joinBevel") },
              ]}
            />
          </div>
        </>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* visibility editor                                                   */
/* ------------------------------------------------------------------ */

export function VisibilityEditor({
  value,
  doc,
  onChange,
}: {
  value: DrawVisibility;
  doc: DrawImageDoc;
  onChange: (value: DrawVisibility) => void;
}) {
  const { t } = useTranslation();
  const pinOptions = [
    ...doc.pins.map((pin) => ({ value: pin.name, label: pin.name, hint: pin.type })),
    ...(value.pin === "item" || value.pin === "index" || value.pin.startsWith("item.")
      ? [{ value: value.pin, label: value.pin, hint: t("drawImage.pseudoPin") }]
      : []),
  ];
  const pinType: DrawPinType | "pseudo" = isPseudoPin(value.pin)
    ? "pseudo"
    : (doc.pins.find((pin) => pin.name === value.pin)?.type ?? "text");
  const ops = CONDITION_OPS[pinType] ?? CONDITION_OPS.text;
  const needsValue = !OPS_WITHOUT_VALUE.has(value.op);
  return (
    <div className="space-y-2 rounded-md border border-ink-700 bg-ink-950/50 p-2">
      <div className="flex items-center justify-between">
        <span className="text-[11px] font-semibold text-fg-muted">{t("drawImage.visibility")}</span>
        <Dropdown
          compact
          className="w-[110px]"
          value={value.mode}
          onChange={(mode) =>
            onChange(mode === "always" ? { mode: "always", pin: "", op: "", value: "" } : { mode: "condition", pin: doc.pins[0]?.name ?? "", op: "isTrue", value: "" })
          }
          options={[
            { value: "always", label: t("drawImage.visibilityAlways") },
            { value: "condition", label: t("drawImage.visibilityCondition") },
          ]}
        />
      </div>
      {value.mode === "condition" && (
        <>
          <Dropdown
            compact
            value={value.pin}
            placeholder={t("drawImage.pickPin")}
            onChange={(pin) => {
              const type = isPseudoPin(pin) ? "pseudo" : doc.pins.find((p) => p.name === pin)?.type ?? "text";
              const firstOp = CONDITION_OPS[type]?.[0]?.value ?? "eq";
              onChange({ ...value, pin, op: firstOp, value: "" });
            }}
            options={pinOptions}
          />
          <Dropdown
            compact
            value={value.op}
            onChange={(op) => onChange({ ...value, op })}
            options={ops.map((op) => ({ value: op.value, label: t(op.labelKey) }))}
          />
          {needsValue && (
            <TextInput value={value.value} placeholder={t("drawImage.compareValue")} onChange={(next) => onChange({ ...value, value: next })} />
          )}
        </>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* repeat editor                                                       */
/* ------------------------------------------------------------------ */

export function RepeatEditor({
  value,
  doc,
  onChange,
}: {
  value: DrawRepeat | null;
  doc: DrawImageDoc;
  onChange: (value: DrawRepeat | null) => void;
}) {
  const { t } = useTranslation();
  const repeat = value ?? { pin: doc.pins.find((p) => p.type === "array")?.name ?? "", offsetX: 100, offsetY: 0, limit: 0 };
  return (
    <div className="space-y-2 rounded-md border border-ink-700 bg-ink-950/50 p-2">
      <div className="flex items-center justify-between">
        <span className="text-[11px] font-semibold text-fg-muted">{t("drawImage.repeat")}</span>
        <Toggle on={value !== null} onChange={(on) => onChange(on ? repeat : null)} />
      </div>
      {value !== null && (
        <>
          <Dropdown
            compact
            value={value.pin}
            placeholder={t("drawImage.pickArrayPin")}
            onChange={(pin) => onChange({ ...value, pin })}
            options={doc.pins.filter((pin) => pin.type === "array").map((pin) => ({ value: pin.name, label: pin.name }))}
          />
          <div className="grid grid-cols-2 gap-1.5">
            <PropRow label="ΔX">
              <NumInput value={value.offsetX} onChange={(offsetX) => onChange({ ...value, offsetX })} width="w-full" />
            </PropRow>
            <PropRow label="ΔY">
              <NumInput value={value.offsetY} onChange={(offsetY) => onChange({ ...value, offsetY })} width="w-full" />
            </PropRow>
          </div>
          <PropRow label={t("drawImage.repeatLimit")}>
            <NumInput
              value={value.limit}
              min={0}
              max={100}
              onChange={(limit) => onChange({ ...value, limit: Math.round(limit) })}
            />
          </PropRow>
          <p className="text-[10px] leading-4 text-fg-faint">{t("drawImage.repeatHint")}</p>
        </>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* element type icon                                                   */
/* ------------------------------------------------------------------ */

export function elementTypeIcon(type: DrawElement["type"]): string {
  switch (type) {
    case "rect":
      return "Square";
    case "ellipse":
      return "Circle";
    case "line":
      return "Minus";
    case "star":
      return "Star";
    case "text":
      return "Type";
    case "image":
      return "Image";
  }
}
