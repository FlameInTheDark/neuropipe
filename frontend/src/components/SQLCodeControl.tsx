import { lazy, Suspense, useEffect, useState } from "react";
import { Database } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { desktop } from "@/lib/bridge";

const SQLCodeEditorDialog = lazy(() => import("@/components/SQLCodeEditorDialog").then((module) => ({ default: module.SQLCodeEditorDialog })));

export function SQLCodeControl({ config, onChange }: { config: Record<string, unknown>; onChange: (config: Record<string, unknown>) => void }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return <><Button type="button" variant="outline" className="w-full justify-start" onClick={() => setOpen(true)}><Database className="size-3.5 text-emerald-300" />{t("sql.edit")}</Button>{open ? <Suspense fallback={null}><SQLCodeEditorDialog open config={config} onClose={() => setOpen(false)} onSave={onChange} /></Suspense> : null}</>;
}

export function DatabaseSelectControl({ value, onChange, ariaLabel }: { value: string; onChange: (value: string) => void; ariaLabel: string }) {
  const { t } = useTranslation();
  const [options, setOptions] = useState<{ value: string; label: string }[]>([]);
  useEffect(() => {
    let cancelled = false;
    void desktop.listDatabases()
      .then((items) => { if (!cancelled) setOptions(items.map((item) => ({ value: item.id, label: item.name }))); })
      .catch(() => { if (!cancelled) setOptions([]); });
    return () => { cancelled = true; };
  }, []);
  return <Select value={value} onValueChange={onChange} options={options} placeholder={t("sql.selectDatabase")} ariaLabel={ariaLabel} />;
}
