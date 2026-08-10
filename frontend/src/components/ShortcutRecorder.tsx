import { useEffect, useId, useRef, useState } from "react";
import { Keyboard, X } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";

const modifierKeys = new Set(["Control", "Alt", "Shift", "Meta", "AltGraph"]);

const namedKeys = new Set([
  "Space",
  "Enter",
  "Tab",
  "Backspace",
  "Delete",
  "Insert",
  "Home",
  "End",
  "PageUp",
  "PageDown",
  "ArrowUp",
  "ArrowDown",
  "ArrowLeft",
  "ArrowRight",
]);

function shortcutKey(event: KeyboardEvent): string | undefined {
  if (modifierKeys.has(event.key) || event.key === "Dead" || event.key === "Unidentified") {
    return undefined;
  }
  if (/^[a-z]$/i.test(event.key)) return event.key.toUpperCase();
  if (/^\d$/.test(event.key)) return event.key;
  if (/^F(?:[1-9]|1\d|2[0-4])$/i.test(event.key)) return event.key.toUpperCase();
  if (event.key === " ") return "Space";
  if (event.key === "Esc") return "Escape";
  return namedKeys.has(event.key) ? event.key : undefined;
}

function shortcutFromEvent(event: KeyboardEvent): string | undefined {
  const key = shortcutKey(event);
  if (!key) return undefined;

  const modifiers: string[] = [];
  if (event.ctrlKey) modifiers.push("Ctrl");
  if (event.altKey) modifiers.push("Alt");
  if (event.shiftKey) modifiers.push("Shift");
  if (event.metaKey) modifiers.push("Meta");

  return modifiers.length > 0 ? [...modifiers, key].join("+") : undefined;
}

function formatShortcut(value: string, translate: (key: string) => string): string {
  const labels: Record<string, string> = {
    Ctrl: translate("shortcutRecorder.keys.ctrl"),
    Alt: translate("shortcutRecorder.keys.alt"),
    Shift: translate("shortcutRecorder.keys.shift"),
    Meta: translate("shortcutRecorder.keys.meta"),
  };
  return value
    .split("+")
    .filter(Boolean)
    .map((part) => labels[part] ?? part)
    .join(" + ");
}

/** Records a keyboard chord and persists the application's canonical shortcut form. */
export function ShortcutRecorder({
  value,
  ariaLabel,
  onValueChange,
}: {
  value: string;
  ariaLabel: string;
  onValueChange: (value: string) => void;
}) {
  const { t } = useTranslation();
  const [recording, setRecording] = useState(false);
  const recordButtonRef = useRef<HTMLButtonElement>(null);
  const helpID = useId();

  useEffect(() => {
    if (!recording) return;

    const onKeyDown = (event: KeyboardEvent) => {
      event.preventDefault();
      event.stopPropagation();
      if (event.key === "Escape") {
        setRecording(false);
        recordButtonRef.current?.focus();
        return;
      }
      if (event.repeat) return;

      const shortcut = shortcutFromEvent(event);
      if (!shortcut) return;
      onValueChange(shortcut);
      setRecording(false);
      recordButtonRef.current?.focus();
    };

    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [onValueChange, recording]);

  const hasShortcut = value.trim().length > 0;
  return (
    <div className="space-y-1.5">
      <div className="flex gap-1.5">
        <Button
          ref={recordButtonRef}
          type="button"
          variant={recording ? "secondary" : "outline"}
          className="min-w-0 flex-1 justify-start"
          aria-label={recording ? t("shortcutRecorder.cancel") : ariaLabel}
          aria-pressed={recording}
          aria-describedby={recording ? helpID : undefined}
          onClick={() => setRecording((current) => !current)}
        >
          <Keyboard aria-hidden className="size-4 shrink-0" />
          <span className="min-w-0 truncate">
            {recording
              ? t("shortcutRecorder.recording")
              : hasShortcut
                ? formatShortcut(value, t)
                : t("shortcutRecorder.record")}
          </span>
        </Button>
        {hasShortcut ? (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            className="shrink-0 px-2"
            aria-label={t("shortcutRecorder.clear")}
            onClick={() => onValueChange("")}
          >
            <X aria-hidden className="size-3.5" />
          </Button>
        ) : null}
      </div>
      {recording ? (
        <p id={helpID} aria-live="polite" className="text-[11px] leading-4 text-zinc-500">
          {t("shortcutRecorder.hint")}
        </p>
      ) : null}
    </div>
  );
}
