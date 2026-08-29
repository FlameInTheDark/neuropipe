import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import type { Storage, StorageEntry } from "@/lib/types";
import { desktop } from "@/lib/bridge";
import { formatDateTime } from "@/lib/format";
import { ask } from "@/stores/confirmation";
import { Button } from "@/components/ui";
import { Icon } from "@/components/icons";
import { Tooltip } from "@/components/Tooltip";
import { Modal, ModalActions } from "@/components/primitives/Modal";
import { Field, TextInput } from "@/components/primitives/Field";
import { cn } from "@/utils/cn";

/** Joins a directory path and a name the way remote storages expect. */
function joinPath(dir: string, name: string): string {
  if (!dir) return name.replace(/^\/+/, "");
  return `${dir.replace(/\/+$/, "")}/${name.replace(/^\/+/, "")}`;
}

function parentPath(path: string): string {
  const index = path.lastIndexOf("/");
  return index <= 0 ? "" : path.slice(0, index);
}

function formatSize(bytes: number): string {
  if (bytes <= 0) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GiB`;
}

type Busy = "" | "list" | "upload" | "download" | "delete" | "mkdir" | "move";

/** One pending prompt: create a folder or rename/move an entry. */
type Prompt =
  | { kind: "mkdir" }
  | { kind: "move"; entry: StorageEntry };

/**
 * Remote file browser for one storage connection: navigate folders, list
 * entries, upload local files, download remote ones, create folders,
 * rename/move, and delete — all through the Desktop bridge which resolves
 * credentials server-side.
 */
export function StorageBrowser({ storage }: { storage: Storage }) {
  const { t } = useTranslation();
  const [path, setPath] = useState("");
  const [entries, setEntries] = useState<StorageEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<Busy>("");
  const [prompt, setPrompt] = useState<Prompt | null>(null);
  const [promptValue, setPromptValue] = useState("");
  const requestSeq = useRef(0);

  /** Loads one directory listing; stale responses are dropped via seq. */
  const load = useCallback(
    async (dir: string) => {
      const seq = ++requestSeq.current;
      setBusy((b) => (b === "" ? "list" : b));
      setError(null);
      try {
        const result = await desktop.storageListFiles(storage.id, dir);
        if (seq !== requestSeq.current) return;
        setEntries(result.entries ?? []);
      } catch (e) {
        if (seq !== requestSeq.current) return;
        setEntries([]);
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        if (seq === requestSeq.current) setBusy("");
      }
    },
    [storage.id],
  );

  useEffect(() => {
    setPath("");
    setEntries(null);
    setError(null);
    void load("");
  }, [storage.id, load]);

  const navigate = (dir: string) => {
    setPath(dir);
    void load(dir);
  };

  const refresh = () => void load(path);

  /* ---------------- upload ---------------- */

  const upload = async () => {
    if (busy) return;
    try {
      const localPath = await desktop.chooseStorageUploadFile();
      if (!localPath) return;
      setBusy("upload");
      setError(null);
      const fileName = localPath.split(/[\\/]/).pop() ?? "upload";
      await desktop.storageUploadFile(storage.id, localPath, joinPath(path, fileName));
      await load(path);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy("");
    }
  };

  /* ---------------- download ---------------- */

  const download = async (entry: StorageEntry) => {
    if (busy || entry.isDir) return;
    try {
      const suggested = entry.name;
      const localPath = await desktop.chooseStorageSaveFile(suggested);
      if (!localPath) return;
      setBusy("download");
      setError(null);
      await desktop.storageDownloadFile(storage.id, entry.path, localPath);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy("");
    }
  };

  /* ---------------- delete ---------------- */

  const remove = async (entry: StorageEntry) => {
    if (busy) return;
    const ok = await ask({
      title: entry.isDir ? t("storages.deleteFolderTitle") : t("storages.deleteFileTitle"),
      description: entry.isDir
        ? t("storages.deleteFolderDescription", { name: entry.name })
        : t("storages.deleteFileDescription", { name: entry.name }),
      confirmLabel: t("common.delete"),
      danger: true,
    });
    if (!ok) return;
    setBusy("delete");
    setError(null);
    try {
      await desktop.storageDeleteEntry(storage.id, entry.path, true);
      await load(path);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setBusy("");
    }
  };

  /* ---------------- folder / rename prompts ---------------- */

  const openMkdir = () => {
    setPromptValue("");
    setPrompt({ kind: "mkdir" });
  };

  const openMove = (entry: StorageEntry) => {
    setPromptValue(entry.name);
    setPrompt({ kind: "move", entry });
  };

  const submitPrompt = async () => {
    if (!prompt || busy) return;
    const value = promptValue.trim();
    if (!value) return;
    setBusy(prompt.kind === "mkdir" ? "mkdir" : "move");
    setError(null);
    try {
      if (prompt.kind === "mkdir") {
        await desktop.storageMakeDir(storage.id, joinPath(path, value));
      } else {
        // The prompt accepts either a bare name (rename in place) or a path
        // containing slashes (move) — both join onto the current directory.
        const to = value.includes("/") ? value : joinPath(path, value);
        await desktop.storageMoveEntry(storage.id, prompt.entry.path, to);
      }
      setPrompt(null);
      await load(path);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setBusy("");
    }
  };

  /* ---------------- breadcrumbs ---------------- */

  const crumbs = path ? path.split("/") : [];

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-hidden">
      {/* toolbar */}
      <div className="flex flex-wrap items-center gap-2">
        <div className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto rounded-lg border border-ink-700 bg-ink-900/60 px-2 py-1.5">
          <button
            onClick={() => navigate("")}
            className="flex shrink-0 items-center gap-1.5 text-[11.5px] text-fg-subtle transition hover:text-fg"
            title={t("storages.rootFolder")}
          >
            <Icon name={storage.driver === "s3" ? "Cloud" : "Globe"} className="h-3.5 w-3.5" />
            {t("storages.rootFolder")}
          </button>
          {crumbs.map((crumb, i) => (
            <span key={i} className="flex shrink-0 items-center gap-1">
              <span className="text-fg-faint">/</span>
              <button
                onClick={() => navigate(crumbs.slice(0, i + 1).join("/"))}
                className={cn(
                  "max-w-[180px] truncate font-mono text-[11px] transition",
                  i === crumbs.length - 1 ? "text-fg" : "text-fg-subtle hover:text-fg",
                )}
              >
                {crumb}
              </button>
            </span>
          ))}
        </div>
        <Button variant="ghost" icon="RefreshCw" disabled={busy !== ""} onClick={refresh}>
          {t("common.refresh")}
        </Button>
        <Button variant="solid" icon="UploadCloud" disabled={busy !== ""} onClick={() => void upload()}>
          {busy === "upload" ? t("common.loading") : t("storages.upload")}
        </Button>
        <Button variant="solid" icon="FolderPlus" disabled={busy !== ""} onClick={openMkdir}>
          {t("storages.newFolder")}
        </Button>
      </div>

      {error && (
        <p className="flex items-start gap-2 rounded-lg bg-danger/10 px-3 py-2 text-[11.5px] leading-relaxed text-danger-fg">
          <Icon name="AlertTriangle" className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span className="min-w-0 break-all">{error}</span>
        </p>
      )}

      {/* entry table */}
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-ink-700/80 bg-ink-900/60">
        <div className="flex h-8 shrink-0 items-center gap-3 border-b border-seam px-3 text-[10px] uppercase tracking-[0.09em] text-fg-faint">
          <span className="flex-1">{t("storages.colName")}</span>
          <span className="w-[80px] shrink-0 text-right">{t("storages.colSize")}</span>
          <span className="w-[140px] shrink-0 text-right">{t("storages.colModified")}</span>
          <span className="w-[92px] shrink-0" />
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {entries === null ? (
            <p className="px-3 py-3 text-[11.5px] text-fg-faint">{t("common.loading")}</p>
          ) : entries.length === 0 ? (
            <p className="px-3 py-3 text-[11.5px] text-fg-faint">{t("storages.folderEmpty")}</p>
          ) : (
            entries.map((entry) => (
              <div
                key={entry.path}
                className="group flex items-center gap-3 border-b border-seam/50 px-3 py-1.5 transition last:border-b-0 hover:bg-ink-850/60"
              >
                <button
                  onClick={() => entry.isDir && navigate(entry.path)}
                  className="flex min-w-0 flex-1 items-center gap-2 text-left"
                >
                  <Icon
                    name={entry.isDir ? "FolderOpen" : "FileText"}
                    className={cn("h-3.5 w-3.5 shrink-0", entry.isDir ? "text-warning-fg/80" : "text-fg-faint")}
                  />
                  <span className="truncate font-mono text-[11.5px] text-fg">{entry.name}</span>
                  {entry.isDir && (
                    <span className="shrink-0 rounded bg-ink-800 px-1 text-[9.5px] text-fg-faint">
                      {t("storages.folderTag")}
                    </span>
                  )}
                </button>
                <span className="w-[80px] shrink-0 text-right font-mono text-[10.5px] text-fg-faint">
                  {entry.isDir ? "—" : formatSize(entry.size)}
                </span>
                <span className="w-[140px] shrink-0 text-right font-mono text-[10.5px] text-fg-faint">
                  {entry.modTime ? formatDateTime(entry.modTime) : "—"}
                </span>
                <span className="flex w-[92px] shrink-0 items-center justify-end gap-0.5">
                  {!entry.isDir && (
                    <Tooltip content={t("storages.download")} side="top">
                      <button
                        onClick={() => void download(entry)}
                        disabled={busy !== ""}
                        aria-label={t("storages.download")}
                        className="grid h-6 w-6 place-items-center rounded-md text-fg-faint transition hover:bg-ink-750 hover:text-fg disabled:opacity-40"
                      >
                        <Icon name="Download" className="h-3.5 w-3.5" />
                      </button>
                    </Tooltip>
                  )}
                  <Tooltip content={t("storages.rename")} side="top">
                    <button
                      onClick={() => openMove(entry)}
                      disabled={busy !== ""}
                      aria-label={t("storages.rename")}
                      className="grid h-6 w-6 place-items-center rounded-md text-fg-faint transition hover:bg-ink-750 hover:text-fg disabled:opacity-40"
                    >
                      <Icon name="Pencil" className="h-3.5 w-3.5" />
                    </button>
                  </Tooltip>
                  <Tooltip content={t("common.delete")} side="top">
                    <button
                      onClick={() => void remove(entry)}
                      disabled={busy !== ""}
                      aria-label={t("common.delete")}
                      className="grid h-6 w-6 place-items-center rounded-md text-fg-faint transition hover:bg-danger/20 hover:text-danger-fg disabled:opacity-40"
                    >
                      <Icon name="Trash2" className="h-3.5 w-3.5" />
                    </button>
                  </Tooltip>
                </span>
              </div>
            ))
          )}
        </div>
        <div className="flex h-8 shrink-0 items-center gap-2 border-t border-seam px-3 text-[10.5px] text-fg-faint">
          <Icon name="Files" className="h-3 w-3" />
          {entries === null
            ? ""
            : t("storages.entryCount", { count: entries.length })}
          {busy === "list" && <Icon name="Loader2" className="h-3 w-3 animate-spin" />}
        </div>
      </div>

      {/* folder / rename prompt */}
      {prompt && (
        <Modal
          title={prompt.kind === "mkdir" ? t("storages.newFolderTitle") : t("storages.renameTitle")}
          icon={prompt.kind === "mkdir" ? "FolderPlus" : "Pencil"}
          size="sm"
          onClose={() => setPrompt(null)}
          footer={
            <ModalActions
              onCancel={() => setPrompt(null)}
              onConfirm={() => void submitPrompt()}
              confirmLabel={prompt.kind === "mkdir" ? t("storages.createFolder") : t("storages.renameApply")}
              disabled={!promptValue.trim() || busy !== ""}
            />
          }
        >
          <div className="space-y-3 p-4">
            <Field
              label={prompt.kind === "mkdir" ? t("storages.folderName") : t("storages.newName")}
              hint={prompt.kind === "move" ? t("storages.renameHint") : undefined}
              required
            >
              <div
                onKeyDown={(e: React.KeyboardEvent) => {
                  if (e.key === "Enter" && promptValue.trim()) void submitPrompt();
                }}
              >
                <TextInput
                  value={promptValue}
                  onChange={setPromptValue}
                  autoFocus
                  mono
                  placeholder={prompt.kind === "mkdir" ? "reports/2026" : "final.csv"}
                />
              </div>
            </Field>
            {prompt.kind === "move" && (
              <p className="font-mono text-[10.5px] text-fg-faint">
                {parentPath(prompt.entry.path) ? `${parentPath(prompt.entry.path)}/` : ""}
                <span className="text-fg-subtle">{prompt.entry.name}</span>
              </p>
            )}
          </div>
        </Modal>
      )}
    </div>
  );
}
