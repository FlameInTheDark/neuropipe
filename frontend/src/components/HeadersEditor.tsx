import { useState } from "react";
import { Icon } from "./icons";
import { useTranslation } from "react-i18next";

interface HeaderEntry {
  name: string;
  value: string;
}

/** Parses the stored headers config: array of {name|key, value} or a map. */
function headersFromValue(value: unknown): HeaderEntry[] {
  if (Array.isArray(value)) {
    return value.flatMap((item) => {
      if (!item || typeof item !== "object") return [];
      const entry = item as Record<string, unknown>;
      const name = typeof entry.name === "string" ? entry.name : typeof entry.key === "string" ? entry.key : "";
      const val = typeof entry.value === "string" ? entry.value : "";
      // keep blank rows — they are mid-edit
      return [{ name, value: val }];
    });
  }
  if (value && typeof value === "object") {
    return Object.entries(value as Record<string, unknown>).map(([name, v]) => ({
      name,
      value: typeof v === "string" ? v : "",
    }));
  }
  return [];
}

/**
 * Key/value rows editor for the HTTP Request node's request headers.
 * Emits an array of {name, value} entries — the shape the executor accepts
 * natively — instead of hand-edited JSON.
 */
export function HeadersEditor({
  label,
  value,
  onChange,
}: {
  label: string;
  value: unknown;
  onChange: (next: unknown) => void;
}) {
  const { t } = useTranslation();
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const rows = headersFromValue(value);

  const editText = (key: string, v: string) => setDrafts((d) => ({ ...d, [key]: v }));
  const text = (index: number, prop: "name" | "value", fallback: string) =>
    drafts[`${index}:${prop}`] ?? (fallback || "");

  const update = (index: number, patch: Partial<HeaderEntry>) => {
    onChange(rows.map((row, current) => (current === index ? { ...row, ...patch } : row)));
  };

  const inputCls =
    "h-7 min-w-0 flex-1 rounded-md border border-ink-700 bg-ink-900 px-2 font-mono text-[11.5px] text-ink-100 transition focus:border-ink-500 focus:outline-none";

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label>{label}</Label>
        <span className="font-mono text-[10px] text-ink-600">{rows.length}</span>
      </div>

      {rows.length > 0 && (
        <div className="flex items-center gap-1.5 px-0.5 text-[10px] tracking-wide text-ink-600 uppercase">
          <span className="flex-1">{t("editor.headerName")}</span>
          <span className="flex-1">{t("editor.headerValue")}</span>
          <span className="w-7 shrink-0" />
        </div>
      )}

      {rows.map((row, index) => (
        <div key={`${index}-${row.name}`} className="flex items-center gap-1.5">
          <input
            value={text(index, "name", row.name)}
            onChange={(e) => {
              editText(`${index}:name`, e.target.value);
              update(index, { name: e.target.value });
            }}
            placeholder="Content-Type"
            aria-label={t("editor.headerName")}
            className={inputCls}
          />
          <input
            value={text(index, "value", row.value)}
            onChange={(e) => {
              editText(`${index}:value`, e.target.value);
              update(index, { value: e.target.value });
            }}
            placeholder="application/json"
            aria-label={t("editor.headerValue")}
            className={inputCls}
          />
          <button
            onClick={() => onChange(rows.filter((_, current) => current !== index))}
            aria-label={t("common.delete")}
            className="grid h-7 w-7 shrink-0 place-items-center rounded-md text-ink-500 transition hover:bg-rose-500/15 hover:text-rose-300"
          >
            <Icon name="Trash2" className="h-3.5 w-3.5" />
          </button>
        </div>
      ))}

      {rows.length === 0 && (
        <p className="rounded-md border border-dashed border-ink-700 px-2.5 py-2 text-[11px] text-ink-500">
          {t("editor.noHeaders")}
        </p>
      )}

      <button
        onClick={() => onChange([...rows, { name: "", value: "" }])}
        className="flex h-7 w-full items-center justify-center gap-1.5 rounded-md border border-dashed border-ink-700 bg-ink-850/50 px-2 text-[11.5px] text-ink-300 transition hover:border-ink-500 hover:bg-ink-750"
      >
        <Icon name="Plus" className="h-3.5 w-3.5" />
        {t("editor.addHeader")}
      </button>
    </div>
  );
}

function Label({ children }: { children: React.ReactNode }) {
  return <span className="block text-[11px] font-medium text-ink-300">{children}</span>;
}
