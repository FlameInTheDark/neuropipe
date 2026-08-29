import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "../ui";
import { Dropdown } from "../Dropdown";
import { TextInput } from "../primitives/Field";
import { DrawImageDoc, DrawPin, DrawPinType, isValidPinName, pinUsage } from "@/lib/draw-image";

export interface PinsPanelProps {
  doc: DrawImageDoc;
  onAddPin(): void;
  onPatchPin(name: string, patch: Partial<DrawPin>): void;
  onDeletePin(name: string): void;
  onRenamePin(oldName: string, newName: string): void;
}

export function PinsPanel(props: PinsPanelProps) {
  const { t } = useTranslation();
  const { doc } = props;

  return (
    <div className="muted-scroll h-full space-y-3 overflow-y-auto p-3">
      <div className="flex items-center justify-between">
        <span className="text-[10px] font-semibold uppercase tracking-[0.12em] text-fg-faint">{t("drawImage.inputs")}</span>
        <Button variant="ghost" icon="Plus" className="h-6 px-2 text-[11px]" onClick={props.onAddPin}>
          {t("drawImage.addPin")}
        </Button>
      </div>

      {doc.pins.length === 0 ? (
        <p className="rounded-md border border-ink-700 bg-ink-950/50 p-3 text-[11px] leading-5 text-fg-faint">
          {t("drawImage.inputsEmpty")}
        </p>
      ) : (
        doc.pins.map((pin) => <PinRow key={pin.name} pin={pin} {...props} />)
      )}

      <p className="px-1 text-[10.5px] leading-4 text-fg-faint">{t("drawImage.inputsHint")}</p>
    </div>
  );
}

function PinRow({
  pin,
  doc,
  onPatchPin,
  onDeletePin,
  onRenamePin,
}: PinsPanelProps & { pin: DrawPin; doc: DrawImageDoc }) {
  const { t } = useTranslation();
  const [name, setName] = useState(pin.name);
  const [error, setError] = useState<string | null>(null);
  const usage = pinUsage(doc, pin.name);

  const commitName = () => {
    const trimmed = name.trim();
    if (trimmed === pin.name) {
      setName(pin.name);
      setError(null);
      return;
    }
    if (!isValidPinName(trimmed)) {
      setError(t("drawImage.invalidPinName"));
      setName(pin.name);
      return;
    }
    if (doc.pins.some((candidate) => candidate.name === trimmed)) {
      setError(t("drawImage.duplicatePinName"));
      setName(pin.name);
      return;
    }
    setError(null);
    onRenamePin(pin.name, trimmed);
  };

  return (
    <div className="space-y-1.5 rounded-md border border-ink-700 bg-ink-950/50 p-2.5">
      <div className="flex items-center gap-1.5">
        <input
          value={name}
          onChange={(event) => setName(event.target.value)}
          onBlur={commitName}
          onKeyDown={(event) => {
            if (event.key === "Enter") (event.target as HTMLInputElement).blur();
          }}
          spellCheck={false}
          className={`min-w-0 flex-1 rounded border bg-ink-950 px-2 py-1 font-mono text-[11.5px] text-fg outline-none transition ${
            error ? "border-danger" : "border-ink-700 focus:border-ink-500"
          }`}
        />
        <Dropdown
          compact
          className="w-[86px] shrink-0"
          value={pin.type}
          onChange={(type) => onPatchPin(pin.name, { type: type as DrawPinType })}
          options={[
            { value: "text", label: t("drawImage.pinText") },
            { value: "number", label: t("drawImage.pinNumber") },
            { value: "boolean", label: t("drawImage.pinBoolean") },
            { value: "object", label: t("drawImage.pinObject") },
            { value: "array", label: t("drawImage.pinArray") },
          ]}
        />
        <button
          title={t("drawImage.deletePin")}
          onClick={() => onDeletePin(pin.name)}
          className="grid h-6 w-6 shrink-0 place-items-center rounded text-fg-faint transition hover:bg-ink-750 hover:text-danger-fg"
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" className="h-3.5 w-3.5">
            <path d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </button>
      </div>
      {error ? <p className="px-0.5 text-[10px] text-danger">{error}</p> : null}
      <div className="grid grid-cols-2 gap-1.5">
        <label className="block">
          <span className="mb-0.5 block text-[9.5px] font-medium uppercase tracking-wide text-fg-faint">
            {t("drawImage.sampleValue")}
          </span>
          <TextInput
            value={pin.sample}
            placeholder={pin.type === "array" ? '["a","b"]' : pin.type === "object" ? '{"a":1}' : pin.type === "boolean" ? "true" : pin.type === "number" ? "21" : "Hello"}
            onChange={(sample) => onPatchPin(pin.name, { sample })}
          />
        </label>
        <label className="block">
          <span className="mb-0.5 block text-[9.5px] font-medium uppercase tracking-wide text-fg-faint">
            {t("drawImage.defaultValue")}
          </span>
          <TextInput
            value={pin.default}
            placeholder={t("drawImage.none")}
            onChange={(defaultValue) => onPatchPin(pin.name, { default: defaultValue })}
          />
        </label>
      </div>
      <div className="flex items-center justify-between">
        <span className="text-[10px] text-fg-faint">
          {usage > 0 ? t("drawImage.usedBy", { count: usage }) : t("drawImage.unused")}
        </span>
        <code className="rounded bg-ink-900 px-1.5 py-0.5 font-mono text-[10px] text-fg-subtle">{`{{${pin.name}}}`}</code>
      </div>
    </div>
  );
}
