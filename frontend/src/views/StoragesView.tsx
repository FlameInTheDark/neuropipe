import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import type { Storage } from "@/lib/types";
import { desktop } from "@/lib/bridge";
import { formatDateTime } from "@/lib/format";
import { ask } from "@/stores/confirmation";
import { Card, EmptyState, StatusPill, ViewShell } from "../components/ViewShell";
import { Button } from "../components/ui";
import { Icon } from "../components/icons";
import { StorageConnectionModal } from "../features/storage/StorageConnectionModal";
import { StorageBrowser } from "../features/storage/StorageBrowser";
import { cn } from "../utils/cn";

/** One-line summary of where this storage points. */
function detailLine(item: Storage): string {
  if (item.driver === "s3") {
    const scheme = item.secure === false ? "http" : "https";
    const region = item.region ? ` · ${item.region}` : "";
    return `${scheme}://${item.endpoint || "s3.amazonaws.com"}/${item.bucket || ""}${region}`;
  }
  const tls = item.tlsMode === "implicit" ? "ftps" : item.tlsMode === "explicit" ? "ftp+tls" : "ftp";
  const dir = item.baseDir ? `/${item.baseDir.replace(/^\/+|\/+$/g, "")}` : "";
  return `${tls}://${item.host || "host"}:${item.port || 21}${dir}`;
}

/**
 * The Storages section: register S3 and FTP connections (sidebar), inspect
 * their status, and browse / manage their remote files — the storage
 * counterpart of the Databases view.
 */
export function StoragesView() {
  const { t } = useTranslation();
  const [storages, setStorages] = useState<Storage[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Storage | null>(null);

  const load = useCallback(async () => {
    const list = await desktop.listStorages();
    setStorages(list);
    setSelectedId((prev) => prev ?? list[0]?.id ?? null);
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const selected = storages.find((s) => s.id === selectedId) ?? storages[0];

  const ping = async () => {
    if (!selected) return;
    try {
      await desktop.pingStorage(selected.id);
      await load();
    } catch {
      /* status pill reflects the failure */
    }
  };

  const removeStorage = async (item: Storage) => {
    const ok = await ask({
      title: t("storages.unregisterTitle"),
      description: t("storages.unregisterDescription", { name: item.name }),
      confirmLabel: t("storages.unregister"),
      danger: true,
    });
    if (!ok) return;
    await desktop.deleteStorage(item.id);
    setSelectedId(null);
    await load();
  };

  return (
    <ViewShell
      title={t("storages.title")}
      subtitle={t("storages.description")}
      padded={false}
      actions={
        <Button
          icon="Plus"
          variant="primary"
          onClick={() => {
            setEditing(null);
            setDialogOpen(true);
          }}
        >
          {t("storages.newConnection")}
        </Button>
      }
    >
      <div className="flex h-full min-h-0">
        {/* connection list */}
        <div className="w-[280px] shrink-0 overflow-y-auto border-r border-seam p-2.5">
          {storages.length === 0 ? (
            <EmptyState icon="Cloud" title={t("storages.emptyTitle")} hint={t("storages.emptyDescription")} />
          ) : (
            storages.map((s) => (
              <button
                key={s.id}
                onClick={() => setSelectedId(s.id)}
                className={cn(
                  "mb-1.5 flex w-full items-center gap-2.5 rounded-lg border px-2.5 py-2 text-left transition",
                  selected?.id === s.id
                    ? "border-ink-500 bg-ink-800/70"
                    : "border-transparent hover:border-ink-700 hover:bg-ink-850",
                )}
              >
                <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-fg-subtle">
                  <Icon name={s.driver === "s3" ? "Cloud" : "Globe"} className="h-4 w-4" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[12.5px] font-medium text-fg">{s.name}</span>
                  <span className="block truncate text-[11px] uppercase text-fg-faint">{s.driver}</span>
                </span>
                <span
                  className={cn(
                    "h-1.5 w-1.5 shrink-0 rounded-full",
                    s.status === "connected" ? "bg-success" : s.status === "error" ? "bg-danger" : "bg-ink-500",
                  )}
                />
              </button>
            ))
          )}

          <button
            onClick={() => {
              setEditing(null);
              setDialogOpen(true);
            }}
            className="mt-1 flex w-full items-center gap-2 rounded-lg border border-dashed border-ink-700 px-2.5 py-2 text-[11.5px] text-fg-faint transition hover:border-ink-500 hover:text-fg-muted"
          >
            <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850">
              <Icon name="Plus" className="h-4 w-4" />
            </span>
            {t("storages.newConnection")}
          </button>
        </div>

        {!selected ? (
          <div className="flex min-w-0 flex-1 items-center justify-center">
            <EmptyState icon="Cloud" title={t("storages.noSelectionTitle")} hint={t("storages.noSelectionHint")} />
          </div>
        ) : (
          <div className="fade-in flex min-h-0 flex-1 flex-col gap-4 overflow-hidden p-4">
            {/* header */}
            <div className="flex items-start gap-3">
              <span className="grid h-10 w-10 shrink-0 place-items-center rounded-xl border border-ink-700 bg-ink-850 text-fg">
                <Icon name={selected.driver === "s3" ? "Cloud" : "Globe"} className="h-[18px] w-[18px]" />
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <h2 className="truncate text-[15px] font-semibold text-fg">{selected.name}</h2>
                  <StatusPill status={selected.status === "connected" ? "connected" : selected.status === "error" ? "error" : "idle"} />
                </div>
                <p className="mt-0.5 truncate font-mono text-[11px] text-fg-faint">{detailLine(selected)}</p>
              </div>
              <Button icon="RefreshCw" onClick={() => void ping()}>
                {t("storages.ping")}
              </Button>
              <Button
                variant="solid"
                icon="Pencil"
                onClick={() => {
                  setEditing(selected);
                  setDialogOpen(true);
                }}
              >
                {t("common.edit")}
              </Button>
              <Button
                variant="solid"
                icon="Trash2"
                className="hover:bg-danger/20 hover:text-danger-fg"
                onClick={() => void removeStorage(selected)}
              >
                {t("storages.unregister")}
              </Button>
            </div>

            {/* stat cards */}
            <div className="grid grid-cols-4 gap-2.5">
              {[
                [t("storages.driver"), selected.driver.toUpperCase()],
                [
                  selected.driver === "s3" ? t("storages.bucket") : t("storages.baseDirLabel"),
                  selected.driver === "s3" ? (selected.bucket || "—") : (selected.baseDir || "—"),
                ],
                [t("storages.updated"), formatDateTime(selected.updatedAt)],
                [t("storages.lastPing"), formatDateTime(selected.lastPingAt)],
              ].map(([k, v]) => (
                <Card key={k} className="p-3">
                  <span className="text-[10px] tracking-wide text-fg-faint uppercase">{k}</span>
                  <p className="mt-1 truncate text-[13px] font-semibold text-fg" title={v}>{v}</p>
                </Card>
              ))}
            </div>

            {/* file browser */}
            <StorageBrowser storage={selected} />
          </div>
        )}
      </div>

      {dialogOpen && (
        <StorageConnectionModal
          existing={editing}
          onClose={() => setDialogOpen(false)}
          onSaved={() => {
            setDialogOpen(false);
            setSelectedId(null);
            void load();
          }}
        />
      )}
    </ViewShell>
  );
}
