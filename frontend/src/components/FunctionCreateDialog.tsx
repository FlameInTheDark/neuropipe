import { useEffect, useId, useState } from "react";
import { Bot, Braces, GitBranch, Loader2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import type { CreateFunctionRequest } from "@/lib/types";

type FunctionTemplate = Pick<CreateFunctionRequest, "kind" | "mode"> & {
  id: "workflow" | "pure" | "tool";
};

const templates: readonly FunctionTemplate[] = [
  { id: "workflow", kind: "function", mode: "impure" },
  { id: "pure", kind: "function", mode: "pure" },
  { id: "tool", kind: "tool", mode: "impure" },
];

const templateIcons = { workflow: GitBranch, pure: Braces, tool: Bot };

/** Application-owned modal for choosing a new function's execution contract. */
export function FunctionCreateDialog({
  open,
  pending = false,
  onClose,
  onCreate,
}: {
  open: boolean;
  pending?: boolean;
  onClose: () => void;
  onCreate: (request: CreateFunctionRequest) => void;
}) {
  const { t } = useTranslation();
  const titleID = useId();
  const descriptionID = useId();
  const [selected, setSelected] = useState<FunctionTemplate>(templates[0]);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  useEffect(() => {
    if (!open) return;
    const previousFocus = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : undefined;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !pending) onClose();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      previousFocus?.focus();
    };
  }, [onClose, open, pending]);

  useEffect(() => {
    if (!open) return;
    setSelected(templates[0]);
    setName("");
    setDescription("");
  }, [open]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/70 p-5 backdrop-blur-[2px]"
      onPointerDown={(event) => {
        if (event.target === event.currentTarget && !pending) onClose();
      }}
    >
      <form
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleID}
        aria-describedby={descriptionID}
        onSubmit={(event) => {
          event.preventDefault();
          if (!name.trim() || !description.trim()) return;
          onCreate({ name: name.trim(), description: description.trim(), kind: selected.kind, mode: selected.mode });
        }}
        className="w-full max-w-2xl overflow-hidden rounded-2xl border border-zinc-700 bg-zinc-950 shadow-2xl shadow-black/70"
      >
        <div className="border-b border-zinc-800 bg-gradient-to-br from-fuchsia-500/10 via-zinc-950 to-zinc-950 px-6 py-5">
          <h2 id={titleID} className="text-base font-semibold text-zinc-100">
            {t("functions.createTitle")}
          </h2>
          <p id={descriptionID} className="mt-1.5 max-w-xl text-sm leading-5 text-zinc-400">
            {t("functions.createDescription")}
          </p>
        </div>
        <div className="space-y-5 px-6 py-5">
          <fieldset>
            <legend className="mb-2 text-[10px] font-semibold uppercase tracking-[.14em] text-zinc-500">
              {t("functions.type")}
            </legend>
            <div className="grid gap-3 sm:grid-cols-3">
              {templates.map((template) => {
                const Icon = templateIcons[template.id];
                const active = selected.id === template.id;
                return (
                  <button
                    key={template.id}
                    type="button"
                    aria-pressed={active}
                    onClick={() => setSelected(template)}
                    className={cn(
                      "group rounded-xl border p-4 text-left outline-none transition focus-visible:ring-2 focus-visible:ring-fuchsia-300/70",
                      active
                        ? "border-fuchsia-300/70 bg-fuchsia-500/10 shadow-inner shadow-fuchsia-950/30"
                        : "border-zinc-800 bg-zinc-900/35 hover:border-zinc-600 hover:bg-zinc-900",
                    )}
                  >
                    <span className={cn("flex size-8 items-center justify-center rounded-lg border", active ? "border-fuchsia-300/30 bg-fuchsia-400/15 text-fuchsia-200" : "border-zinc-700 bg-zinc-900 text-zinc-400")}>
                      <Icon className="size-4" />
                    </span>
                    <span className="mt-3 block text-sm font-medium text-zinc-100">
                      {t(`functions.types.${template.id}.title`)}
                    </span>
                    <span className="mt-1 block text-xs leading-5 text-zinc-500">
                      {t(`functions.types.${template.id}.description`)}
                    </span>
                  </button>
                );
              })}
            </div>
          </fieldset>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="block text-xs font-medium text-zinc-300">
              {t("functions.name")}
              <Input
                autoFocus
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder={t("functions.namePlaceholder")}
                className="mt-2"
              />
            </label>
            <label className="block text-xs font-medium text-zinc-300">
              {t("functions.shortDescription")}
              <textarea
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder={t("functions.descriptionPlaceholder")}
                rows={2}
                className="mt-2 min-h-[42px] w-full resize-none rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2 text-sm text-zinc-100 outline-none placeholder:text-zinc-600 focus:border-zinc-500"
              />
            </label>
          </div>
        </div>
        <div className="flex justify-end gap-2 border-t border-zinc-800 bg-zinc-950/80 px-6 py-4">
          <Button type="button" variant="outline" onClick={onClose} disabled={pending}>
            {t("common.cancel")}
          </Button>
          <Button type="submit" disabled={pending || !name.trim() || !description.trim()}>
            {pending ? <Loader2 className="size-4 animate-spin" /> : <Braces className="size-4" />}
            {t("functions.create")}
          </Button>
        </div>
      </form>
    </div>
  );
}
