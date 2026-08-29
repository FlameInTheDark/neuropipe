import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { desktop } from "@/lib/bridge";
import type {
  Settings,
  TriggerBinding,
  TwitchStatus,
  DiscordStatus,
  TelegramStatus,
} from "@/lib/types";
import type { Workspace } from "@/features/workspace/useWorkspace";
import { ask } from "@/stores/confirmation";
import { Card, StatusPill, ViewShell } from "../components/ViewShell";
import { Button } from "../components/ui";
import { Icon } from "../components/icons";
import { mergeIdentitySlice, normalizeSettings } from "./SettingsView";
import { DiscordPanel, TelegramPanel, TwitchPanel } from "./IntegrationPanels";
import { cn } from "../utils/cn";

/** Fixed catalog of the built-in integrations — the sidebar entries. */
const INTEGRATIONS = [
  { id: "twitch", icon: "Radio", titleKey: "twitch.title", kind: "EventSub", descKey: "twitch.eventSubDescription" },
  { id: "discord", icon: "Hash", titleKey: "discord.title", kind: "Gateway", descKey: "discord.gatewayDescription" },
  { id: "telegram", icon: "Send", titleKey: "telegram.title", kind: "Bot API", descKey: "telegram.pollingDescription" },
] as const;

type IntegrationId = (typeof INTEGRATIONS)[number]["id"];

/** How often the view re-reads live service state. The gateways/polling loops
 *  connect asynchronously after trust/enable/add-bot actions, and device-code
 *  auth completes server-side, so a one-shot fetch would show stale state. */
const STATUS_POLL_MS = 5000;

interface IntegrationStatuses {
  twitch: TwitchStatus | null;
  discord: DiscordStatus | null;
  telegram: TelegramStatus | null;
}

/**
 * The Integrations section: Twitch, Discord and Telegram side by side in the
 * Databases-style layout — a connection sidebar with live status dots, a
 * header with status and quick stats, and the full management panel for the
 * selected integration (identities, default bot, event triggers).
 */
export function IntegrationsView({ workspace }: { workspace: Workspace }) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState<IntegrationId>("twitch");
  const [statuses, setStatuses] = useState<IntegrationStatuses>({
    twitch: null,
    discord: null,
    telegram: null,
  });
  const [draft, setDraft] = useState<Settings | null>(
    workspace.settings ? normalizeSettings(workspace.settings) : null,
  );
  const [saving, setSaving] = useState(false);
  /* local edits must survive background workspace refreshes; the ref clears
     after a successful save so external changes flow in again. */
  const dirtyRef = useRef(false);

  const refresh = useCallback(async () => {
    const [twitch, discord, telegram] = await Promise.all([
      desktop.getTwitchStatus().catch(() => null),
      desktop.getDiscordStatus().catch(() => null),
      desktop.getTelegramStatus().catch(() => null),
      workspace.refreshTriggers(),
      workspace.refreshSettings(),
    ]);
    setStatuses({ twitch, discord, telegram });
  }, [workspace.refreshTriggers, workspace.refreshSettings]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  /* keep the sidebar dots and the header live while the view is open */
  useEffect(() => {
    const timer = window.setInterval(() => void refresh(), STATUS_POLL_MS);
    return () => window.clearInterval(timer);
  }, [refresh]);

  /* re-sync the draft when the workspace loads/changes settings externally */
  useEffect(() => {
    if (!workspace.settings || saving) return;
    const next = normalizeSettings(workspace.settings);
    setDraft((d) => {
      if (!d || !dirtyRef.current) return next; // clean draft: adopt wholesale
      // Dirty draft: unsaved edits survive, but backend-managed identity
      // state (added/removed bots, default rotation) always flows in so the
      // panels never show stale identities.
      return {
        ...d,
        twitch: mergeIdentitySlice(d.twitch, next.twitch),
        discord: mergeIdentitySlice(d.discord, next.discord),
        telegram: mergeIdentitySlice(d.telegram, next.telegram),
      };
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace.settings, saving]);

  if (!draft) {
    return (
      <ViewShell title={t("integrations.title")} subtitle={t("integrations.description")}>
        <div className="grid h-full place-items-center">
          <p className="text-[12.5px] text-fg-faint">{t("common.unavailable")}</p>
        </div>
      </ViewShell>
    );
  }

  const patch = (p: Partial<Settings>) => {
    dirtyRef.current = true;
    setDraft((d) => (d ? { ...d, ...p } : d));
  };

  const save = async () => {
    if (!draft || saving) return;
    // exposure handshake: non-loopback or unauthenticated API listeners need
    // explicit consent before they are persisted (the draft carries the whole
    // settings object, even though this view only edits integration slices).
    const api = draft.api;
    const isLoopback = api.bindAddress === "127.0.0.1" || api.bindAddress === "::1";
    const risky = api.enabled && (!isLoopback || api.authMode === "none");
    let next = draft;
    if (risky && !api.exposureAcknowledged) {
      const ok = await ask({
        title: t("api.exposureTitle"),
        description: t("api.exposureDescription"),
        confirmLabel: t("api.exposureConfirm"),
        danger: true,
      });
      if (!ok) return;
      next = { ...draft, api: { ...draft.api, exposureAcknowledged: true } };
      setDraft(next);
    }
    setSaving(true);
    try {
      await workspace.saveSettings(next);
      dirtyRef.current = false; // draft now matches persisted state
      workspace.notify(t("settings.saved"), "Check");
    } catch {
      workspace.notify(t("settings.saveFailed"), "AlertTriangle");
    } finally {
      setSaving(false);
    }
  };

  const meta = INTEGRATIONS.find((i) => i.id === selected)!;
  const status = statuses[selected];
  const identities =
    selected === "twitch" ? draft.twitch.identities : selected === "discord" ? draft.discord.identities : draft.telegram.identities;
  const defaultIdentityId =
    selected === "twitch" ? draft.twitch.defaultBotIdentityId : selected === "discord" ? draft.discord.defaultBotIdentityId : draft.telegram.defaultBotIdentityId;
  const kindTriggers = workspace.triggers.filter((tr: TriggerBinding) => tr.kind === selected);
  const connectedIdentities = identities.filter((i) => i.status === "connected");
  const defaultIdentity = identities.find((i) => i.id === defaultIdentityId);

  return (
    <ViewShell
      title={t("integrations.title")}
      subtitle={t("integrations.description")}
      padded={false}
      actions={
        <Button icon="Save" variant="primary" onClick={() => void save()} disabled={saving}>
          {saving ? t("common.saving") : t("settings.save")}
        </Button>
      }
    >
      <div className="flex h-full min-h-0">
        {/* integration list */}
        <div className="w-[280px] shrink-0 overflow-y-auto border-r border-seam p-2.5">
          {INTEGRATIONS.map((i) => {
            const st = statuses[i.id];
            return (
              <button
                key={i.id}
                onClick={() => setSelected(i.id)}
                className={cn(
                  "mb-1.5 flex w-full items-center gap-2.5 rounded-lg border px-2.5 py-2 text-left transition",
                  selected === i.id
                    ? "border-ink-500 bg-ink-800/70"
                    : "border-transparent hover:border-ink-700 hover:bg-ink-850",
                )}
              >
                <span className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-ink-700 bg-ink-850 text-fg-subtle">
                  <Icon name={i.icon} className="h-4 w-4" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[12.5px] font-medium text-fg">{t(i.titleKey)}</span>
                  <span className="block truncate text-[11px] uppercase text-fg-faint">{i.kind}</span>
                </span>
                <span
                  className={cn(
                    "h-1.5 w-1.5 shrink-0 rounded-full",
                    st?.connected ? "bg-success" : st?.lastError ? "bg-danger" : "bg-ink-500",
                  )}
                />
              </button>
            );
          })}
        </div>

        <div className="fade-in flex min-h-0 flex-1 flex-col gap-4 overflow-hidden p-4">
          {/* header */}
          <div className="flex items-start gap-3">
            <span className="grid h-10 w-10 shrink-0 place-items-center rounded-xl border border-ink-700 bg-ink-850 text-fg">
              <Icon name={meta.icon} className="h-[18px] w-[18px]" />
            </span>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <h2 className="truncate text-[15px] font-semibold text-fg">{t(meta.titleKey)}</h2>
                <StatusPill status={status?.connected ? "connected" : status?.lastError ? "error" : "idle"} />
              </div>
              <p className="mt-0.5 truncate text-[11px] text-fg-faint">
                {status?.lastError || t(meta.descKey, { count: status?.activeSubscriptions ?? 0 })}
              </p>
            </div>
            <Button icon="RefreshCw" onClick={() => void refresh()}>
              {t("common.refresh")}
            </Button>
          </div>

          {/* stat cards */}
          <div className="grid grid-cols-4 gap-2.5">
            {[
              [t("integrations.identitiesStat"), t("integrations.ofConnected", { connected: connectedIdentities.length, total: identities.length })],
              [t("integrations.defaultBotStat"), defaultIdentity?.label ?? "—"],
              [t("integrations.subscriptionsStat"), status ? String(status.activeSubscriptions) : "—"],
              [
                t("integrations.triggersStat"),
                t("integrations.ofEnabled", {
                  enabled: kindTriggers.filter((b) => b.enabled).length,
                  total: kindTriggers.length,
                }),
              ],
            ].map(([k, v]) => (
              <Card key={k} className="p-3">
                <span className="text-[10px] tracking-wide text-fg-faint uppercase">{k}</span>
                <p className="mt-1 truncate text-[13px] font-semibold text-fg" title={v}>{v}</p>
              </Card>
            ))}
          </div>

          {/* management panel */}
          <div className="min-h-0 flex-1 overflow-y-auto">
            {selected === "twitch" && (
              <TwitchPanel draft={draft} patch={patch} triggers={workspace.triggers} status={statuses.twitch} refresh={refresh} />
            )}
            {selected === "discord" && (
              <DiscordPanel draft={draft} patch={patch} triggers={workspace.triggers} status={statuses.discord} refresh={refresh} />
            )}
            {selected === "telegram" && (
              <TelegramPanel draft={draft} patch={patch} triggers={workspace.triggers} status={statuses.telegram} refresh={refresh} />
            )}
          </div>
        </div>
      </div>
    </ViewShell>
  );
}
