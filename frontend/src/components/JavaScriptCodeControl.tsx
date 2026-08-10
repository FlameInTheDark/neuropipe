import { lazy, Suspense, useState } from "react";
import { Braces } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import type { JavaScriptNodeConfig } from "@/lib/javascript-node";

const JavaScriptCodeEditorDialog = lazy(() =>
  import("@/components/JavaScriptCodeEditorDialog").then((module) => ({
    default: module.JavaScriptCodeEditorDialog,
  })),
);

/** Small inspector control; Monaco remains out of the normal editor bundle. */
export function JavaScriptCodeControl({
  config,
  onChange,
}: {
  config: Record<string, unknown>;
  onChange: (config: JavaScriptNodeConfig) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button type="button" variant="outline" className="w-full justify-start" onClick={() => setOpen(true)}>
        <Braces className="size-3.5 text-amber-200" />
        {t("javascript.edit")}
      </Button>
      {open ? (
        <Suspense fallback={null}>
          <JavaScriptCodeEditorDialog
            open={open}
            config={config}
            onClose={() => setOpen(false)}
            onSave={onChange}
          />
        </Suspense>
      ) : null}
    </>
  );
}
