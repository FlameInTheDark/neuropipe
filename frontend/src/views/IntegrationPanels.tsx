import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Browser } from "@wailsio/runtime";
import { desktop } from "@/lib/bridge";
import type {
  Settings,
  TriggerBinding,
  TwitchDeviceAuthorization,
  TwitchEventDescriptor,
  TwitchIdentity,
  TwitchManualIdentityRequest,
  TwitchStatus,
  DiscordEventDescriptor,
  DiscordIdentity,
  DiscordStatus,
  TelegramIdentity,
  TelegramStatus,
} from "@/lib/types";
import { formatDateTime } from "@/lib/format";
import { ask } from "@/stores/confirmation";
import { Button, Toggle } from "../components/ui";
import { Icon } from "../components/icons";
import { Dropdown } from "../components/Dropdown";
import { Modal, ModalActions } from "../components/primitives/Modal";
import { Field, TextInput } from "../components/primitives/Field";
import { SectionCard } from "./SettingsView";
import { ApplicationCommandsSection } from "../components/discord/ApplicationCommands";

/**
 * The Twitch / Discord / Telegram panels of the Integrations view.
 *
 * The view owns the settings draft, the live service status of every
 * integration and the shared refresh cycle (see IntegrationsView): panels
 * receive `status` and a `refresh` callback as props so switching the sidebar
 * entry never tears down polling, and no two components race for the same
 * status endpoint. Event catalogs are static definitions, so they are fetched
 * once per mount.
 */

/** Persists the draft before a device-code flow so the freshly typed Client
 *  ID is on disk when the backend starts the authorization. */
async function workspace_save(settings: Settings): Promise<void> {
  await desktop.saveSettings(settings);
}

/* ------------------------------------------------------------------ */
/* twitch                                                              */
/* ------------------------------------------------------------------ */

export function TwitchPanel({
  draft,
  patch,
  triggers,
  status,
  refresh,
}: {
  draft: Settings;
  patch: (p: Partial<Settings>) => void;
  triggers: TriggerBinding[];
  status: TwitchStatus | null;
  refresh: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const [catalog, setCatalog] = useState<TwitchEventDescriptor[]>([]);
  const [auth, setAuth] = useState<TwitchDeviceAuthorization | null>(null);
  const [authLabel, setAuthLabel] = useState("");
  const [manualOpen, setManualOpen] = useState(false);
  const [manualLabel, setManualLabel] = useState("");
  const [manualToken, setManualToken] = useState("");

  const twitchTriggers = triggers.filter((tr) => tr.kind === "twitch");

  useEffect(() => {
    desktop
      .listTwitchEventCatalog()
      .then(setCatalog)
      .catch(() => setCatalog([]));
  }, []);

  const connectScopes = useMemo(
    () => [...new Set(["user:read:chat", "user:write:chat", ...catalog.flatMap((e) => e.requiredScopes)])],
    [catalog],
  );

  const startAuth = async (identity?: TwitchIdentity) => {
    try {
      await workspace_save(draft);
      const authorization = await desktop.startTwitchDeviceAuthorization({
        identityId: identity?.id,
        label: authLabel || identity?.label || t("twitch.identityLabelPlaceholder"),
        scopes: connectScopes,
      });
      setAuth(authorization);
    } catch {
      /* error surfaces via status */
    }
  };

  const cancelAuth = async () => {
    if (!auth) return;
    await desktop.cancelTwitchDeviceAuthorization(auth.id).catch(() => undefined);
    setAuth(null);
  };

  const addManual = async () => {
    if (!manualLabel.trim() || !manualToken.trim()) return;
    const request: TwitchManualIdentityRequest = { label: manualLabel.trim(), accessToken: manualToken.trim() };
    setManualOpen(false);
    setManualLabel("");
    setManualToken("");
    try {
      await desktop.addTwitchManualIdentity(request);
      await refresh();
    } catch {
      /* keep dialog state cleared; status shows failure */
    }
  };

  const removeIdentity = async (identity: TwitchIdentity) => {
    const ok = await ask({
      title: t("twitch.removeTitle"),
      description: t("twitch.removeDescription", { name: identity.label }),
      confirmLabel: t("twitch.remove"),
      danger: true,
    });
    if (!ok) return;
    await desktop.removeTwitchIdentity(identity.id).catch(() => undefined);
    await refresh();
  };

  const trustTrigger = async (binding: TriggerBinding) => {
    await desktop.trustTwitchTrigger(binding.id).catch(() => undefined);
    await refresh();
  };

  const toggleTrigger = async (binding: TriggerBinding, enabled: boolean) => {
    await desktop.setTwitchTriggerEnabled(binding.id, enabled).catch(() => undefined);
    await refresh();
  };

  const openVerification = async () => {
    if (!auth) return;
    try {
      await Browser.OpenURL(auth.verificationUri);
    } catch {
      /* outside Wails */
    }
  };

  return (
    <div className="mx-auto max-w-[720px] space-y-3">
      <SectionCard title={t("twitch.connection")}>
        <div className="space-y-3">
          <Field label={t("twitch.clientId")}>
            <TextInput mono value={draft.twitch.clientId} onChange={(v) => patch({ twitch: { ...draft.twitch, clientId: v } })} placeholder={t("twitch.clientIdPlaceholder")} />
          </Field>
          <div className="flex items-center justify-between rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
            <div className="min-w-0">
              <p className="text-[12.5px] font-medium text-fg">
                {status?.connected ? t("twitch.connected") : t("twitch.disconnected")}
              </p>
              <p className="truncate text-[11px] text-fg-faint">
                {status?.lastError || t("twitch.eventSubDescription", { count: status?.activeSubscriptions ?? 0 })}
              </p>
            </div>
            {status?.connected && <Icon name="Check" className="h-4 w-4 shrink-0 text-success-fg" />}
          </div>
        </div>
      </SectionCard>

      <SectionCard title={t("twitch.identities")}>
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            <Button
              icon="Cable"
              variant="primary"
              disabled={!draft.twitch.clientId.trim()}
              onClick={() => {
                setAuthLabel("");
                void startAuth();
              }}
            >
              {t("twitch.connect")}
            </Button>
            <Button icon="KeyRound" variant="solid" onClick={() => setManualOpen(true)}>
              {t("twitch.manualToken")}
            </Button>
          </div>

          <Field label={t("twitch.defaultBotIdentity")}>
            <Dropdown
              value={draft.twitch.defaultBotIdentityId ?? ""}
              onChange={(v) => patch({ twitch: { ...draft.twitch, defaultBotIdentityId: v || undefined } })}
              placeholder={t("twitch.defaultBotIdentityPlaceholder")}
              options={[
                { value: "", label: t("twitch.defaultBotIdentityPlaceholder") },
                ...draft.twitch.identities
                  .filter((identity) => identity.status === "connected")
                  .map((identity) => ({ value: identity.id, label: identity.label, icon: "Bot" })),
              ]}
            />
          </Field>

          {draft.twitch.identities.length === 0 ? (
            <p className="rounded-lg border border-dashed border-ink-700 px-3 py-3 text-[12px] text-fg-faint">
              {t("twitch.noIdentities")}
            </p>
          ) : (
            draft.twitch.identities.map((identity) => (
              <div key={identity.id} className="flex items-center gap-2 rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[12.5px] font-medium text-fg">{identity.label}</p>
                  <p className="truncate text-[11px] text-fg-faint">
                    {identity.login} · {identity.scopes.length > 0 ? identity.scopes.join(", ") : t("twitch.noScopes")}
                  </p>
                </div>
                {identity.status !== "connected" && (
                  <span className="shrink-0 rounded bg-warning/15 px-2 py-1 text-[10.5px] text-warning-fg">
                    {t("twitch.reconnectRequired")}
                  </span>
                )}
                {identity.status !== "connected" && (
                  <Button variant="solid" onClick={() => void startAuth(identity)}>
                    {t("twitch.reconnect")}
                  </Button>
                )}
                <Button icon="Trash2" variant="solid" onClick={() => void removeIdentity(identity)}>
                  {t("common.delete")}
                </Button>
              </div>
            ))
          )}
        </div>
      </SectionCard>

      <SectionCard title={t("twitch.triggers")}>
        <p className="mb-3 text-[11.5px] leading-relaxed text-fg-faint">{t("twitch.triggersHelp")}</p>
        {twitchTriggers.length === 0 ? (
          <p className="rounded-lg border border-dashed border-ink-700 px-3 py-3 text-[12px] text-fg-faint">
            {t("twitch.noTriggers")}
          </p>
        ) : (
          twitchTriggers.map((binding) => (
            <div key={binding.id} className="flex items-center gap-3 border-b border-seam/70 py-2 last:border-b-0">
              <span className="min-w-0 flex-1 truncate text-[12.5px] font-medium text-fg">{binding.label}</span>
              {!binding.trusted ? (
                <Button icon="ShieldCheck" variant="solid" onClick={() => void trustTrigger(binding)}>
                  {t("schedules.trust")}
                </Button>
              ) : (
                <Toggle on={binding.enabled} onChange={(v) => void toggleTrigger(binding, v)} />
              )}
            </div>
          ))
        )}
      </SectionCard>

      {/* device-code flow */}
      {auth && (
        <Modal
          title={t("twitch.connectTitle")}
          icon="ShieldCheck"
          onClose={() => void cancelAuth()}
          footer={
            <div className="ml-auto flex items-center gap-2">
              <Button variant="ghost" onClick={() => void cancelAuth()}>
                {t("common.cancel")}
              </Button>
              <Button icon="ExternalLink" variant="primary" onClick={() => void openVerification()}>
                {t("twitch.openTwitch")}
              </Button>
            </div>
          }
        >
          <div className="space-y-3 text-center">
            <p className="text-[12.5px] text-fg-subtle">{t("twitch.openVerification", { url: auth.verificationUri })}</p>
            <code className="block rounded-xl border border-ink-650 bg-ink-950/70 px-4 py-4 font-mono text-[26px] tracking-[0.35em] text-fg select-all">
              {auth.userCode}
            </code>
            <p className="text-[11px] text-fg-faint">{t("twitch.expiresAt", { time: formatDateTime(auth.expiresAt) })}</p>
          </div>
        </Modal>
      )}

      {/* manual token */}
      {manualOpen && (
        <Modal
          title={t("twitch.manualTitle")}
          icon="KeyRound"
          onClose={() => setManualOpen(false)}
          footer={
            <ModalActions
              onCancel={() => setManualOpen(false)}
              onConfirm={() => void addManual()}
              confirmLabel={t("common.save")}
              disabled={!manualLabel.trim() || !manualToken.trim()}
            />
          }
        >
          <div className="space-y-3">
            <p className="text-[12px] leading-relaxed text-fg-subtle">{t("twitch.manualDescription")}</p>
            <Field label={t("twitch.identityLabel")}>
              <TextInput autoFocus value={manualLabel} onChange={setManualLabel} placeholder={t("twitch.identityLabelPlaceholder")} />
            </Field>
            <Field label={t("twitch.accessToken")}>
              <input
                type="password"
                autoComplete="off"
                value={manualToken}
                onChange={(e) => setManualToken(e.target.value)}
                className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 font-mono text-[12px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
              />
            </Field>
          </div>
        </Modal>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* discord                                                             */
/* ------------------------------------------------------------------ */

export function DiscordPanel({
  draft,
  patch,
  triggers,
  status,
  refresh,
}: {
  draft: Settings;
  patch: (p: Partial<Settings>) => void;
  triggers: TriggerBinding[];
  status: DiscordStatus | null;
  refresh: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const [catalog, setCatalog] = useState<DiscordEventDescriptor[]>([]);
  const [addOpen, setAddOpen] = useState(false);
  const [addLabel, setAddLabel] = useState("");
  const [addToken, setAddToken] = useState("");
  const [addError, setAddError] = useState("");

  const discordTriggers = triggers.filter((tr) => tr.kind === "discord");

  useEffect(() => {
    desktop
      .listDiscordEventCatalog()
      .then(setCatalog)
      .catch(() => setCatalog([] as DiscordEventDescriptor[]));
  }, []);

  const needsPrivilegedIntents = useMemo(
    () => catalog.some((descriptor) => descriptor.privileged),
    [catalog],
  );

  const addBot = async () => {
    if (!addToken.trim()) return;
    try {
      await desktop.addDiscordManualIdentity({ label: addLabel.trim(), token: addToken.trim() });
      setAddOpen(false);
      setAddLabel("");
      setAddToken("");
      setAddError("");
      await refresh();
    } catch (error) {
      setAddError(String((error as { message?: string })?.message ?? error));
    }
  };

  const removeIdentity = async (identity: DiscordIdentity) => {
    const ok = await ask({
      title: t("discord.removeTitle"),
      description: t("discord.removeDescription", { name: identity.label }),
      confirmLabel: t("common.delete"),
      danger: true,
    });
    if (!ok) return;
    await desktop.removeDiscordIdentity(identity.id).catch(() => undefined);
    await refresh();
  };

  const trustTrigger = async (binding: TriggerBinding) => {
    await desktop.trustDiscordTrigger(binding.id).catch(() => undefined);
    await refresh();
  };

  const toggleTrigger = async (binding: TriggerBinding, enabled: boolean) => {
    await desktop.setDiscordTriggerEnabled(binding.id, enabled).catch(() => undefined);
    await refresh();
  };

  return (
    <div className="mx-auto max-w-[720px] space-y-3">
      <SectionCard title={t("discord.connection")}>
        <div className="space-y-3">
          <div className="flex items-center justify-between rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
            <div className="min-w-0">
              <p className="text-[12.5px] font-medium text-fg">
                {status?.connected ? t("discord.connected") : t("discord.disconnected")}
              </p>
              <p className="truncate text-[11px] text-fg-faint">
                {status?.lastError || t("discord.gatewayDescription", { count: status?.activeSubscriptions ?? 0 })}
              </p>
            </div>
            {status?.connected && <Icon name="Check" className="h-4 w-4 shrink-0 text-success-fg" />}
          </div>
          {needsPrivilegedIntents && (
            <p className="rounded-lg border border-warning/30 bg-warning/10 px-3 py-2.5 text-[11.5px] leading-relaxed text-warning-fg">
              {t("discord.intentsWarning")}
            </p>
          )}
        </div>
      </SectionCard>

      <SectionCard title={t("discord.identities")}>
        <div className="space-y-3">
          <Button icon="Cable" variant="primary" onClick={() => setAddOpen(true)}>
            {t("discord.addBot")}
          </Button>

          <Field label={t("discord.defaultBotIdentity")}>
            <Dropdown
              value={draft.discord.defaultBotIdentityId ?? ""}
              onChange={(v) => patch({ discord: { ...draft.discord, defaultBotIdentityId: v || undefined } })}
              placeholder={t("discord.defaultBotIdentityPlaceholder")}
              options={[
                { value: "", label: t("discord.defaultBotIdentityPlaceholder") },
                ...draft.discord.identities
                  .filter((identity) => identity.status === "connected")
                  .map((identity) => ({ value: identity.id, label: identity.label, icon: "Bot" })),
              ]}
            />
          </Field>

          {draft.discord.identities.length === 0 ? (
            <p className="rounded-lg border border-dashed border-ink-700 px-3 py-3 text-[12px] text-fg-faint">
              {t("discord.noIdentities")}
            </p>
          ) : (
            draft.discord.identities.map((identity) => (
              <div key={identity.id} className="flex items-center gap-2 rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[12.5px] font-medium text-fg">{identity.label}</p>
                  <p className="truncate text-[11px] text-fg-faint">@{identity.username}</p>
                </div>
                {identity.status !== "connected" && (
                  <span className="shrink-0 rounded bg-warning/15 px-2 py-1 text-[10.5px] text-warning-fg">
                    {t("discord.invalidIdentity")}
                  </span>
                )}
                <Button icon="Trash2" variant="solid" onClick={() => void removeIdentity(identity)}>
                  {t("common.delete")}
                </Button>
              </div>
            ))
          )}
        </div>
      </SectionCard>

      <SectionCard title={t("discord.commands.section")}>
        <p className="mb-3 text-[11.5px] leading-relaxed text-fg-faint">{t("discord.commands.sectionHelp")}</p>
        <ApplicationCommandsSection identities={draft.discord.identities} />
      </SectionCard>

      <SectionCard title={t("discord.triggers")}>
        <p className="mb-3 text-[11.5px] leading-relaxed text-fg-faint">{t("discord.triggersHelp")}</p>
        {discordTriggers.length === 0 ? (
          <p className="rounded-lg border border-dashed border-ink-700 px-3 py-3 text-[12px] text-fg-faint">
            {t("discord.noTriggers")}
          </p>
        ) : (
          discordTriggers.map((binding) => (
            <div key={binding.id} className="flex items-center gap-3 border-b border-seam/70 py-2 last:border-b-0">
              <span className="min-w-0 flex-1 truncate text-[12.5px] font-medium text-fg">{binding.label}</span>
              {!binding.trusted ? (
                <Button icon="ShieldCheck" variant="solid" onClick={() => void trustTrigger(binding)}>
                  {t("schedules.trust")}
                </Button>
              ) : (
                <Toggle on={binding.enabled} onChange={(v) => void toggleTrigger(binding, v)} />
              )}
            </div>
          ))
        )}
      </SectionCard>

      {addOpen && (
        <Modal
          title={t("discord.addTitle")}
          icon="KeyRound"
          onClose={() => setAddOpen(false)}
          footer={
            <ModalActions
              onCancel={() => setAddOpen(false)}
              onConfirm={() => void addBot()}
              confirmLabel={t("common.save")}
              disabled={!addToken.trim()}
            />
          }
        >
          <div className="space-y-3">
            <p className="text-[12px] leading-relaxed text-fg-subtle">{t("discord.tokenDescription")}</p>
            <Field label={t("discord.identityLabel")}>
              <TextInput autoFocus value={addLabel} onChange={setAddLabel} placeholder={t("discord.identityLabelPlaceholder")} />
            </Field>
            <Field label={t("discord.botToken")}>
              <input
                type="password"
                autoComplete="off"
                value={addToken}
                onChange={(e) => setAddToken(e.target.value)}
                className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 font-mono text-[12px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
              />
            </Field>
            {addError && <p className="text-[11.5px] text-danger-fg">{addError}</p>}
          </div>
        </Modal>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* telegram                                                            */
/* ------------------------------------------------------------------ */

export function TelegramPanel({
  draft,
  patch,
  triggers,
  status,
  refresh,
}: {
  draft: Settings;
  patch: (p: Partial<Settings>) => void;
  triggers: TriggerBinding[];
  status: TelegramStatus | null;
  refresh: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const [addOpen, setAddOpen] = useState(false);
  const [addLabel, setAddLabel] = useState("");
  const [addToken, setAddToken] = useState("");
  const [addError, setAddError] = useState("");

  const telegramTriggers = triggers.filter((tr) => tr.kind === "telegram");

  const addBot = async () => {
    if (!addToken.trim()) return;
    try {
      await desktop.addTelegramManualIdentity({ label: addLabel.trim(), token: addToken.trim() });
      setAddOpen(false);
      setAddLabel("");
      setAddToken("");
      setAddError("");
      await refresh();
    } catch (error) {
      setAddError(String((error as { message?: string })?.message ?? error));
    }
  };

  const removeIdentity = async (identity: TelegramIdentity) => {
    const ok = await ask({
      title: t("telegram.removeTitle"),
      description: t("telegram.removeDescription", { name: identity.label }),
      confirmLabel: t("common.delete"),
      danger: true,
    });
    if (!ok) return;
    await desktop.removeTelegramIdentity(identity.id).catch(() => undefined);
    await refresh();
  };

  const trustTrigger = async (binding: TriggerBinding) => {
    await desktop.trustTelegramTrigger(binding.id).catch(() => undefined);
    await refresh();
  };

  const toggleTrigger = async (binding: TriggerBinding, enabled: boolean) => {
    await desktop.setTelegramTriggerEnabled(binding.id, enabled).catch(() => undefined);
    await refresh();
  };

  return (
    <div className="mx-auto max-w-[720px] space-y-3">
      <SectionCard title={t("telegram.connection")}>
        <div className="space-y-3">
          <div className="flex items-center justify-between rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
            <div className="min-w-0">
              <p className="text-[12.5px] font-medium text-fg">
                {status?.connected ? t("telegram.connected") : t("telegram.disconnected")}
              </p>
              <p className="truncate text-[11px] text-fg-faint">
                {status?.lastError || t("telegram.pollingDescription", { count: status?.activeSubscriptions ?? 0 })}
              </p>
            </div>
            {status?.connected && <Icon name="Check" className="h-4 w-4 shrink-0 text-success-fg" />}
          </div>
          <p className="rounded-lg border border-ink-700 bg-ink-900/40 px-3 py-2.5 text-[11.5px] leading-relaxed text-fg-faint">
            {t("telegram.privacyModeHint")}
          </p>
        </div>
      </SectionCard>

      <SectionCard title={t("telegram.identities")}>
        <div className="space-y-3">
          <Button icon="Cable" variant="primary" onClick={() => setAddOpen(true)}>
            {t("telegram.addBot")}
          </Button>

          <Field label={t("telegram.defaultBotIdentity")}>
            <Dropdown
              value={draft.telegram.defaultBotIdentityId ?? ""}
              onChange={(v) => patch({ telegram: { ...draft.telegram, defaultBotIdentityId: v || undefined } })}
              placeholder={t("telegram.defaultBotIdentityPlaceholder")}
              options={[
                { value: "", label: t("telegram.defaultBotIdentityPlaceholder") },
                ...draft.telegram.identities
                  .filter((identity) => identity.status === "connected")
                  .map((identity) => ({ value: identity.id, label: identity.label, icon: "Bot" })),
              ]}
            />
          </Field>

          {draft.telegram.identities.length === 0 ? (
            <p className="rounded-lg border border-dashed border-ink-700 px-3 py-3 text-[12px] text-fg-faint">
              {t("telegram.noIdentities")}
            </p>
          ) : (
            draft.telegram.identities.map((identity) => (
              <div key={identity.id} className="flex items-center gap-2 rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[12.5px] font-medium text-fg">{identity.label}</p>
                  <p className="truncate text-[11px] text-fg-faint">@{identity.username}</p>
                </div>
                {identity.status !== "connected" && (
                  <span className="shrink-0 rounded bg-warning/15 px-2 py-1 text-[10.5px] text-warning-fg">
                    {t("telegram.invalidIdentity")}
                  </span>
                )}
                <Button icon="Trash2" variant="solid" onClick={() => void removeIdentity(identity)}>
                  {t("common.delete")}
                </Button>
              </div>
            ))
          )}
        </div>
      </SectionCard>

      <SectionCard title={t("telegram.triggers")}>
        <p className="mb-3 text-[11.5px] leading-relaxed text-fg-faint">{t("telegram.triggersHelp")}</p>
        {telegramTriggers.length === 0 ? (
          <p className="rounded-lg border border-dashed border-ink-700 px-3 py-3 text-[12px] text-fg-faint">
            {t("telegram.noTriggers")}
          </p>
        ) : (
          telegramTriggers.map((binding) => (
            <div key={binding.id} className="flex items-center gap-3 border-b border-seam/70 py-2 last:border-b-0">
              <span className="min-w-0 flex-1 truncate text-[12.5px] font-medium text-fg">{binding.label}</span>
              {!binding.trusted ? (
                <Button icon="ShieldCheck" variant="solid" onClick={() => void trustTrigger(binding)}>
                  {t("schedules.trust")}
                </Button>
              ) : (
                <Toggle on={binding.enabled} onChange={(v) => void toggleTrigger(binding, v)} />
              )}
            </div>
          ))
        )}
      </SectionCard>

      {addOpen && (
        <Modal
          title={t("telegram.addTitle")}
          icon="KeyRound"
          onClose={() => setAddOpen(false)}
          footer={
            <ModalActions
              onCancel={() => setAddOpen(false)}
              onConfirm={() => void addBot()}
              confirmLabel={t("common.save")}
              disabled={!addToken.trim()}
            />
          }
        >
          <div className="space-y-3">
            <p className="text-[12px] leading-relaxed text-fg-subtle">{t("telegram.tokenDescription")}</p>
            <Field label={t("telegram.identityLabel")}>
              <TextInput autoFocus value={addLabel} onChange={setAddLabel} placeholder={t("telegram.identityLabelPlaceholder")} />
            </Field>
            <Field label={t("telegram.botToken")}>
              <input
                type="password"
                autoComplete="off"
                value={addToken}
                onChange={(e) => setAddToken(e.target.value)}
                className="h-8 w-full rounded-md border border-ink-700 bg-ink-850 px-2.5 font-mono text-[12px] text-fg focus:border-ink-400 focus:bg-ink-800 focus:outline-none"
              />
            </Field>
            {addError && <p className="text-[11.5px] text-danger-fg">{addError}</p>}
          </div>
        </Modal>
      )}
    </div>
  );
}
