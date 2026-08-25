import { useEffect, useRef, useState } from "react";
import { Icon } from "./icons";
import { Button } from "./ui";
import { Modal } from "./primitives/Modal";
import { Field } from "./primitives/Field";
import { Dropdown } from "./Dropdown";
import { useConfirmation } from "@/stores/confirmation";
import { useInputDialog, useFormDialog } from "@/stores/dialogs";
import type { FormDialogField } from "@/lib/types";
import { useTranslation } from "react-i18next";

/**
 * Hosts every backend-initiated dialog: confirmations from view actions,
 * plus the Input Dialog and Form nodes executed by pipelines.
 * Mounted once in `App`.
 */
export function DialogHosts() {
  return (
    <>
      <ConfirmHost />
      <InputHost />
      <FormHost />
    </>
  );
}

function ConfirmHost() {
  const { t } = useTranslation();
  const request = useConfirmation((s) => s.request);
  const respond = useConfirmation((s) => s.respond);
  const confirmRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (request) confirmRef.current?.focus();
  }, [request]);

  if (!request) return null;
  return (
    <Modal
      title={request.title}
      icon={request.danger ? "AlertTriangle" : "HelpCircle"}
      size="sm"
      onClose={() => respond(false)}
      footer={
        <div className="ml-auto flex items-center gap-2">
          <Button onClick={() => respond(false)}>{t("common.cancel")}</Button>
          <button
            ref={confirmRef}
            onClick={() => respond(true)}
            className={
              request.danger
                ? "h-7 rounded-md bg-rose-500/90 px-3 text-[11.5px] font-medium text-white transition hover:bg-rose-500"
                : "h-7 rounded-md bg-ink-50 px-3 text-[11.5px] font-medium text-ink-950 transition hover:bg-white"
            }
          >
            {request.confirmLabel}
          </button>
        </div>
      }
    >
      <p className="text-[12.5px] leading-relaxed text-ink-300">{request.description}</p>
    </Modal>
  );
}

function InputHost() {
  const { t } = useTranslation();
  const request = useInputDialog((s) => s.request);
  const respond = useInputDialog((s) => s.respond);
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setValue("");
    setError(null);
    if (request) inputRef.current?.focus();
  }, [request]);

  if (!request) return null;

  const submit = () => {
    const trimmed = value.trim();
    if (!trimmed) {
      setError(t("inputDialog.errorEmpty"));
      return;
    }
    if (request.inputType === "number" && Number.isNaN(Number(trimmed))) {
      setError(t("inputDialog.errorNumber"));
      return;
    }
    respond({ canceled: false, value });
  };

  return (
    <Modal
      title={request.title}
      icon="TextCursorInput"
      size="sm"
      onClose={() => respond({ canceled: true, value: "" })}
      footer={
        <div className="ml-auto flex items-center gap-2">
          <Button onClick={() => respond({ canceled: true, value: "" })}>
            {request.cancelLabel || t("inputDialog.cancel")}
          </Button>
          <Button variant="primary" onClick={submit}>
            {request.continueLabel || t("inputDialog.continue")}
          </Button>
        </div>
      }
    >
      <div className="space-y-3">
        {request.message && (
          <p className="text-[12.5px] leading-relaxed text-ink-300">{request.message}</p>
        )}
        <Field label={request.label || t("inputDialog.value")} hint={error ?? undefined}>
          <input
            ref={inputRef}
            autoFocus
            type={request.inputType === "number" ? "number" : "text"}
            value={value}
            onChange={(e) => {
              setValue(e.target.value);
              setError(null);
            }}
            placeholder={request.placeholder}
            onKeyDown={(e) => {
              if (e.key === "Enter") submit();
            }}
            className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[12.5px] text-ink-100 placeholder:text-ink-500 transition focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
          />
        </Field>
      </div>
    </Modal>
  );
}

const GRID_COLUMNS = "repeat(4, minmax(0, 1fr))";

function FormHost() {
  const { t } = useTranslation();
  const request = useFormDialog((s) => s.request);
  const respond = useFormDialog((s) => s.respond);
  const [values, setValues] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setValues({});
    setError(null);
  }, [request]);

  if (!request) return null;

  const submit = () => {
    const out: Record<string, string | number> = {};
    for (const item of request.items) {
      if (item.kind !== "input") continue;
      const raw = values[item.id]?.trim() ?? "";
      if (item.inputType === "number") {
        if (!raw) {
          setError(t("inputDialog.errorEmpty"));
          return;
        }
        const n = Number(raw);
        if (Number.isNaN(n)) {
          setError(t("formBuilder.errorNumber", { name: item.label }));
          return;
        }
        out[item.id] = n;
      } else {
        out[item.id] = values[item.id] ?? "";
      }
    }
    respond({ canceled: false, values: out });
  };

  const renderItem = (item: FormDialogField) => {
    // col/row are 0-indexed in the layout; grid lines are 1-indexed.
    const col = Math.min(3, Math.max(0, item.col));
    const span = Math.min(4 - col, Math.max(1, item.span || 1));
    const style: React.CSSProperties = {
      gridColumn: `${col + 1} / span ${span}`,
      gridRow: `${item.row + 1} / span ${Math.max(1, item.rowSpan || 1)}`,
    };
    if (item.kind === "text") {
      return (
        <p key={item.id} style={style} className="self-end pb-1 text-[12px] leading-snug text-ink-300">
          {item.label}
        </p>
      );
    }
    if (item.kind === "dropdown") {
      return (
        <label key={item.id} style={style} className="block">
          <span className="mb-1 block text-[11px] text-ink-400">{item.label}</span>
          <Dropdown
            value={values[item.id] ?? ""}
            onChange={(v) => setValues((prev) => ({ ...prev, [item.id]: v }))}
            placeholder="…"
            options={(item.options ?? []).map((o) => ({ value: o.value, label: o.label || o.value }))}
          />
        </label>
      );
    }
    return (
      <label key={item.id} style={style} className="block">
        <span className="mb-1 block text-[11px] text-ink-400">{item.label}</span>
        <input
          type={item.inputType === "number" ? "number" : "text"}
          value={values[item.id] ?? ""}
          placeholder={item.placeholder}
          onChange={(e) => setValues((v) => ({ ...v, [item.id]: e.target.value }))}
          onKeyDown={(e) => {
            if (e.key === "Enter") submit();
          }}
          className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 text-[12.5px] text-ink-100 placeholder:text-ink-500 focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
        />
      </label>
    );
  };

  return (
    <Modal
      title={request.title}
      icon="LayoutList"
      size="wide"
      onClose={() => respond({ canceled: true, values: {} })}
      footer={
        <div className="ml-auto flex items-center gap-2">
          {error && (
            <span className="flex items-center gap-1.5 text-[11.5px] text-rose-300">
              <Icon name="AlertTriangle" className="h-3.5 w-3.5" />
              {error}
            </span>
          )}
          <Button onClick={() => respond({ canceled: true, values: {} })}>
            {request.cancelLabel || t("common.cancel")}
          </Button>
          <Button variant="primary" onClick={submit}>
            {request.continueLabel || t("common.save")}
          </Button>
        </div>
      }
    >
      <div className="space-y-3">
        {request.message && (
          <p className="text-[12.5px] leading-relaxed text-ink-300">{request.message}</p>
        )}
        <div
          style={{
            gridTemplateColumns: GRID_COLUMNS,
            gridAutoRows: "minmax(56px, auto)",
          }}
          className="grid items-start gap-2.5"
        >
          {request.items.map(renderItem)}
        </div>
      </div>
    </Modal>
  );
}
