import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { desktop } from "@/lib/bridge";
import type {
  DiscordApplicationCommand,
  DiscordApplicationCommandOption,
  DiscordGuildLite,
  DiscordIdentity,
} from "@/lib/types";
import { ask } from "@/stores/confirmation";
import { Badge, Button, IconButton, Toggle } from "../ui";
import { Icon } from "../icons";
import { Dropdown } from "../Dropdown";
import { Modal, ModalActions } from "../primitives/Modal";
import { Field, TextInput } from "../primitives/Field";

/**
 * The Application Commands manager of the Discord integration panel:
 * a scope picker (global or one of the bot's guilds), the live list of
 * commands registered on the bot, and a full CRUD editor for them.
 */

/* Discord option type values. */
const OPTION_TYPES: Array<{ value: number; label: string; group: boolean }> = [
  { value: 3, label: "Text", group: false },
  { value: 4, label: "Integer", group: false },
  { value: 10, label: "Number", group: false },
  { value: 5, label: "Boolean", group: false },
  { value: 6, label: "User", group: false },
  { value: 7, label: "Channel", group: false },
  { value: 8, label: "Role", group: false },
  { value: 9, label: "Mentionable", group: false },
  { value: 11, label: "Attachment", group: false },
  { value: 1, label: "Subcommand", group: true },
  { value: 2, label: "Subcommand group", group: true },
];

const CHANNEL_TYPES: Array<{ value: number; label: string }> = [
  { value: 0, label: "Guild text" },
  { value: 2, label: "Guild voice" },
  { value: 4, label: "Guild category" },
  { value: 5, label: "Guild announcement" },
  { value: 13, label: "Guild stage" },
  { value: 15, label: "Guild forum" },
];

/** Common default member permissions offered in the editor. */
const PERMISSIONS: Array<{ value: number; label: string }> = [
  { value: 1 << 3, label: "Administrator" },
  { value: 1 << 4, label: "Manage server" },
  { value: 1 << 13, label: "Manage channels" },
  { value: 1 << 28, label: "Manage messages" },
  { value: 1 << 1, label: "Kick members" },
  { value: 1 << 2, label: "Ban members" },
  { value: 1 << 11, label: "Mention everyone" },
  { value: 1 << 0, label: "Create invite" },
];

const CONTEXTS: Array<{ value: number; label: string }> = [
  { value: 0, label: "Servers" },
  { value: 1, label: "DMs with the bot" },
  { value: 2, label: "Group DMs" },
];

const emptyCommand = (): DiscordApplicationCommand => ({
  type: 1,
  name: "",
  description: "",
  options: [],
});

export function ApplicationCommandsSection({ identities }: { identities: DiscordIdentity[] }) {
  const { t } = useTranslation();
  const connected = identities.filter((identity) => identity.status === "connected");
  const [identityId, setIdentityId] = useState("");
  const [guilds, setGuilds] = useState<DiscordGuildLite[]>([]);
  const [guildId, setGuildId] = useState("");
  const [commands, setCommands] = useState<DiscordApplicationCommand[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [editing, setEditing] = useState<DiscordApplicationCommand | null>(null);

  // Application commands belong to one bot application, so the panel always
  // works on an explicitly selected identity. A stale selection (bot removed
  // or disconnected while the panel was open) falls back to the first
  // connected bot instead of silently querying a dead session.
  const activeIdentity =
    identityId && connected.some((identity) => identity.id === identityId)
      ? identityId
      : (connected[0]?.id ?? "");

  useEffect(() => {
    if (!activeIdentity) return;
    let cancelled = false;
    setGuilds([]);
    setGuildId("");
    desktop
      .listDiscordGuilds(activeIdentity)
      .then((list) => {
        if (!cancelled) setGuilds(list ?? []);
      })
      .catch(() => {
        if (!cancelled) setGuilds([]);
      });
    return () => {
      cancelled = true;
    };
  }, [activeIdentity]);

  const reload = useCallback(async () => {
    if (!activeIdentity) return;
    setLoading(true);
    setError("");
    try {
      setCommands((await desktop.listDiscordApplicationCommands(activeIdentity, guildId)) ?? []);
    } catch (err) {
      setCommands([]);
      setError(String((err as { message?: string })?.message ?? err));
    } finally {
      setLoading(false);
    }
  }, [activeIdentity, guildId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const remove = async (command: DiscordApplicationCommand) => {
    const ok = await ask({
      title: t("discord.commands.deleteTitle"),
      description: t("discord.commands.deleteDescription", { name: command.name }),
      confirmLabel: t("common.delete"),
      danger: true,
    });
    if (!ok) return;
    try {
      await desktop.deleteDiscordApplicationCommand(activeIdentity, guildId, command.id ?? "");
      await reload();
    } catch (err) {
      setError(String((err as { message?: string })?.message ?? err));
    }
  };

  if (connected.length === 0) return null;

  const typeLabel = (type?: number) =>
    type === 2 ? t("discord.commands.typeUser") : type === 3 ? t("discord.commands.typeMessage") : t("discord.commands.typeSlash");
  const countOptions = (command: DiscordApplicationCommand): number =>
    (command.options ?? []).reduce(
      (total, option) =>
        total + 1 + (option.options?.length ?? 0) + (option.options ?? []).reduce((inner, child) => inner + (child.options?.length ?? 0), 0),
      0,
    );

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <label className="flex items-center gap-1.5">
          <span className="text-[11px] font-medium uppercase tracking-wide text-fg-faint">
            {t("discord.commands.botLabel")}
          </span>
          <Dropdown
            value={activeIdentity}
            onChange={(v) => setIdentityId(v)}
            options={connected.map((identity) => ({
              value: identity.id,
              label: identity.label,
              icon: "Bot",
              hint: identity.username ? `@${identity.username}` : undefined,
            }))}
          />
        </label>
        <Dropdown
          value={guildId}
          onChange={(v) => setGuildId(v)}
          searchable
          searchPlaceholder={t("discord.commands.searchServer")}
          options={[
            { value: "", label: t("discord.commands.scopeGlobal"), icon: "Globe" },
            ...guilds.map((guild) => ({ value: guild.id, label: guild.name, icon: "Building" })),
          ]}
        />
        <div className="flex-1" />
        <Button icon="RefreshCw" variant="solid" onClick={() => void reload()} disabled={loading}>
          {t("common.refresh")}
        </Button>
        <Button icon="Plus" variant="primary" onClick={() => setEditing(emptyCommand())}>
          {t("discord.commands.new")}
        </Button>
      </div>

      {error ? (
        <p className="rounded-lg border border-danger/30 bg-danger/10 px-3 py-2.5 text-[11.5px] leading-relaxed text-danger-fg">{error}</p>
      ) : null}

      {commands.length === 0 ? (
        <p className="rounded-lg border border-dashed border-ink-700 px-3 py-3 text-[12px] text-fg-faint">
          {loading ? t("discord.commands.loading") : t("discord.commands.empty")}
        </p>
      ) : (
        commands.map((command) => (
          <div key={command.id} className="flex items-center gap-3 rounded-lg border border-ink-700 bg-ink-900/60 px-3 py-2.5">
            <Icon name="Slash" className="h-4 w-4 shrink-0 text-fg-faint" />
            <div className="min-w-0 flex-1">
              <p className="flex items-center gap-2 truncate text-[12.5px] font-medium text-fg">
                /{command.name}
                <Badge>{typeLabel(command.type)}</Badge>
                {command.nsfw ? <Badge>{t("discord.commands.nsfw")}</Badge> : null}
              </p>
              <p className="truncate text-[11px] text-fg-faint">
                {command.description || "—"} · {countOptions(command)} {t("discord.commands.optionsCount")}
                {guildId ? "" : ` · ${t("discord.commands.globalNote")}`}
              </p>
            </div>
            <IconButton icon="Pencil" label={t("common.edit")} onClick={() => setEditing(command)} />
            <IconButton icon="Trash2" label={t("common.delete")} onClick={() => void remove(command)} />
          </div>
        ))
      )}

      {editing ? (
        <CommandEditorModal
          identityId={activeIdentity}
          guildId={guildId}
          command={editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            void reload();
          }}
        />
      ) : null}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* command editor modal                                                */
/* ------------------------------------------------------------------ */

function CommandEditorModal({
  identityId,
  guildId,
  command,
  onClose,
  onSaved,
}: {
  identityId: string;
  guildId: string;
  command: DiscordApplicationCommand;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState<DiscordApplicationCommand>(() => structuredClone(command));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const isSlash = (draft.type ?? 1) === 1;
  const isNew = !command.id;

  const patch = (partial: Partial<DiscordApplicationCommand>) => setDraft((prev) => ({ ...prev, ...partial }));

  const save = async () => {
    setSaving(true);
    setError("");
    try {
      const payload: DiscordApplicationCommand = { ...draft, type: draft.type ?? 1 };
      if (!isSlash) {
        delete payload.description;
        delete payload.options;
      }
      if (isNew) {
        const create = { ...payload } as Partial<DiscordApplicationCommand>;
        delete create.id;
        await desktop.createDiscordApplicationCommand({ identityId, guildId, command: create as DiscordApplicationCommand });
      } else {
        await desktop.updateDiscordApplicationCommand({ identityId, guildId, command: payload });
      }
      onSaved();
    } catch (err) {
      setError(String((err as { message?: string })?.message ?? err));
    } finally {
      setSaving(false);
    }
  };

  const nameValid = draft.name.length >= 1 && draft.name.length <= 32 && (!isSlash || /^[a-z0-9_-]+$/.test(draft.name));
  const descriptionValid = !isSlash || ((draft.description ?? "").length >= 1 && (draft.description ?? "").length <= 100);

  return (
    <Modal
      title={isNew ? t("discord.commands.editorNew") : t("discord.commands.editorEdit", { name: command.name })}
      icon="Slash"
      size="lg"
      onClose={onClose}
      footer={
        <ModalActions
          onCancel={onClose}
          onConfirm={() => void save()}
          confirmLabel={isNew ? t("discord.commands.create") : t("common.save")}
          disabled={saving || !nameValid || !descriptionValid}
        />
      }
    >
      <div className="space-y-4">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-[180px_1fr]">
          <Field label={t("discord.commands.type")}>
            <Dropdown
              value={String(draft.type ?? 1)}
              onChange={(v) => patch({ type: Number(v) })}
              options={[
                { value: "1", label: t("discord.commands.typeSlash") },
                { value: "2", label: t("discord.commands.typeUser") },
                { value: "3", label: t("discord.commands.typeMessage") },
              ]}
            />
          </Field>
          <Field label={`${t("discord.commands.name")} (${draft.name.length}/32)`}>
            <TextInput
              autoFocus
              value={draft.name}
              onChange={(v) => patch({ name: isSlash ? v.toLowerCase() : v })}
              placeholder={isSlash ? "weather" : "Ask AI"}
            />
            {!nameValid && draft.name.length > 0 ? (
              <p className="mt-1 text-[11px] text-danger-fg">{t("discord.commands.nameRule")}</p>
            ) : null}
          </Field>
        </div>

        {isSlash ? (
          <Field label={`${t("discord.commands.description")} (${(draft.description ?? "").length}/100)`}>
            <TextInput value={draft.description ?? ""} onChange={(v) => patch({ description: v })} placeholder="Get the current weather for a city" />
          </Field>
        ) : null}

        {isSlash ? (
          <div className="space-y-2 rounded-lg border border-ink-700 bg-ink-900/40 p-3">
            <div className="flex items-center justify-between">
              <p className="text-[12.5px] font-medium text-fg">
                {t("discord.commands.options")} ({draft.options?.length ?? 0}/25)
              </p>
              <Button
                icon="Plus"
                variant="solid"
                onClick={() => patch({ options: [...(draft.options ?? []), { type: 3, name: "", description: "" }] })}
              >
                {t("discord.commands.addOption")}
              </Button>
            </div>
            {(draft.options ?? []).length === 0 ? (
              <p className="px-1 py-1 text-[11.5px] text-fg-faint">{t("discord.commands.optionsHelp")}</p>
            ) : (
              <div className="space-y-2">
                {(draft.options ?? []).map((option, index) => (
                  <OptionEditor
                    key={index}
                    option={option}
                    depth={0}
                    onChange={(next) => patch({ options: (draft.options ?? []).map((item, i) => (i === index ? next : item)) })}
                    onRemove={() => patch({ options: (draft.options ?? []).filter((_, i) => i !== index) })}
                    onMove={(delta) => {
                      const options = [...(draft.options ?? [])];
                      const target = index + delta;
                      if (target < 0 || target >= options.length) return;
                      [options[index], options[target]] = [options[target], options[index]];
                      patch({ options });
                    }}
                  />
                ))}
              </div>
            )}
          </div>
        ) : null}

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div className="space-y-2">
            <p className="text-[12.5px] font-medium text-fg">{t("discord.commands.permissions")}</p>
            <div className="space-y-1.5 rounded-lg border border-ink-700 bg-ink-900/40 p-3">
              {PERMISSIONS.map((permission) => (
                <label key={permission.value} className="flex items-center gap-2 text-[12px] text-fg-subtle">
                  <input
                    type="checkbox"
                    className="h-3.5 w-3.5 accent-[var(--fg)]"
                    checked={((draft.defaultMemberPermission ?? 0) & permission.value) !== 0}
                    onChange={(event) => {
                      const current = draft.defaultMemberPermission ?? 0;
                      patch({
                        defaultMemberPermission: event.target.checked ? current | permission.value : current & ~permission.value,
                      });
                    }}
                  />
                  {permission.label}
                </label>
              ))}
              <p className="pt-1 text-[11px] text-fg-faint">{t("discord.commands.permissionsHelp")}</p>
            </div>
          </div>
          <div className="space-y-2">
            <p className="text-[12.5px] font-medium text-fg">{t("discord.commands.availability")}</p>
            <div className="space-y-2 rounded-lg border border-ink-700 bg-ink-900/40 p-3">
              <label className="flex items-center gap-2 text-[12px] text-fg-subtle">
                <Toggle on={draft.nsfw ?? false} onChange={(v) => patch({ nsfw: v })} />
                {t("discord.commands.nsfw")}
              </label>
              {!guildId ? (
                <>
                  {CONTEXTS.map((context) => (
                    <label key={context.value} className="flex items-center gap-2 text-[12px] text-fg-subtle">
                      <input
                        type="checkbox"
                        className="h-3.5 w-3.5 accent-[var(--fg)]"
                        checked={(draft.contexts ?? []).includes(context.value)}
                        onChange={(event) => {
                          const current = draft.contexts ?? [];
                          patch({
                            contexts: event.target.checked ? [...current, context.value] : current.filter((value) => value !== context.value),
                          });
                        }}
                      />
                      {context.label}
                    </label>
                  ))}
                  <p className="text-[11px] text-fg-faint">{t("discord.commands.contextsHelp")}</p>
                </>
              ) : (
                <p className="text-[11px] text-fg-faint">{t("discord.commands.guildScopeNote")}</p>
              )}
            </div>
          </div>
        </div>

        {error ? (
          <p className="rounded-lg border border-danger/30 bg-danger/10 px-3 py-2.5 text-[11.5px] leading-relaxed text-danger-fg">{error}</p>
        ) : null}
        <p className="text-[11px] text-fg-faint">
          {guildId ? t("discord.commands.instantUpdateNote") : t("discord.commands.globalPropagationNote")}
        </p>
      </div>
    </Modal>
  );
}

/* ------------------------------------------------------------------ */
/* option editor                                                       */
/* ------------------------------------------------------------------ */

function OptionEditor({
  option,
  depth,
  onChange,
  onRemove,
  onMove,
}: {
  option: DiscordApplicationCommandOption;
  depth: number;
  onChange: (next: DiscordApplicationCommandOption) => void;
  onRemove: () => void;
  onMove: (delta: number) => void;
}) {
  const { t } = useTranslation();
  const patch = (partial: Partial<DiscordApplicationCommandOption>) => onChange({ ...option, ...partial });

  const isGroup = option.type === 2;
  const isSubcommand = option.type === 1;
  const isValue = !isGroup && !isSubcommand;
  const allowsChoices = option.type === 3 || option.type === 4 || option.type === 10;
  const numeric = option.type === 4 || option.type === 10;
  const textual = option.type === 3;

  const availableTypes = useMemo(() => {
    // Depth 0 allows groups, subcommands, and values; deeper levels only
    // allow subcommands (inside groups) or value options.
    if (depth === 0) return OPTION_TYPES;
    return OPTION_TYPES.filter((entry) => entry.value === 1 || !entry.group);
  }, [depth]);

  return (
    <div className="rounded-lg border border-ink-700 bg-ink-850/70 p-2.5" style={{ marginLeft: depth * 16 }}>
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-[150px_1fr_auto]">
        <Dropdown
          value={String(option.type)}
          onChange={(v) =>
            patch({
              type: Number(v),
              choices: undefined,
              options: undefined,
              minValue: undefined,
              maxValue: undefined,
              minLength: undefined,
              maxLength: undefined,
            })
          }
          options={availableTypes.map((entry) => ({ value: String(entry.value), label: entry.label }))}
          compact
        />
        <TextInput value={option.name} onChange={(v) => patch({ name: v.toLowerCase() })} placeholder={t("discord.commands.optionName")} />
        <div className="flex items-center gap-1">
          <IconButton icon="ChevronUp" label={t("discord.commands.moveUp")} onClick={() => onMove(-1)} />
          <IconButton icon="ChevronDown" label={t("discord.commands.moveDown")} onClick={() => onMove(1)} />
          <IconButton icon="Trash2" label={t("common.delete")} onClick={onRemove} />
        </div>
      </div>
      {isValue ? (
        <div className="mt-2 grid grid-cols-1 gap-2 sm:grid-cols-[1fr_130px]">
          <TextInput value={option.description ?? ""} onChange={(v) => patch({ description: v })} placeholder={`${t("discord.commands.description")} (1-100)`} />
          <label className="flex items-center gap-2 text-[11.5px] text-fg-subtle">
            <Toggle on={option.required ?? false} onChange={(v) => patch({ required: v })} />
            {t("discord.commands.required")}
          </label>
        </div>
      ) : null}
      {isValue ? (
        <div className="mt-2 flex flex-wrap gap-3">
          {numeric ? (
            <>
              <label className="flex items-center gap-1.5 text-[11.5px] text-fg-subtle">
                {t("discord.commands.minValue")}
                <input
                  type="number"
                  className="h-7 w-24 rounded-md border border-ink-700 bg-ink-850 px-2 text-[12px] text-fg focus:border-ink-400 focus:outline-none"
                  value={option.minValue ?? ""}
                  onChange={(event) => patch({ minValue: event.target.value === "" ? undefined : Number(event.target.value) })}
                />
              </label>
              <label className="flex items-center gap-1.5 text-[11.5px] text-fg-subtle">
                {t("discord.commands.maxValue")}
                <input
                  type="number"
                  className="h-7 w-24 rounded-md border border-ink-700 bg-ink-850 px-2 text-[12px] text-fg focus:border-ink-400 focus:outline-none"
                  value={option.maxValue ?? ""}
                  onChange={(event) => patch({ maxValue: event.target.value === "" ? undefined : Number(event.target.value) })}
                />
              </label>
            </>
          ) : null}
          {textual ? (
            <>
              <label className="flex items-center gap-1.5 text-[11.5px] text-fg-subtle">
                {t("discord.commands.minLength")}
                <input
                  type="number"
                  className="h-7 w-20 rounded-md border border-ink-700 bg-ink-850 px-2 text-[12px] text-fg focus:border-ink-400 focus:outline-none"
                  value={option.minLength ?? ""}
                  onChange={(event) => patch({ minLength: event.target.value === "" ? undefined : Number(event.target.value) })}
                />
              </label>
              <label className="flex items-center gap-1.5 text-[11.5px] text-fg-subtle">
                {t("discord.commands.maxLength")}
                <input
                  type="number"
                  className="h-7 w-20 rounded-md border border-ink-700 bg-ink-850 px-2 text-[12px] text-fg focus:border-ink-400 focus:outline-none"
                  value={option.maxLength ?? ""}
                  onChange={(event) => patch({ maxLength: event.target.value === "" ? undefined : Number(event.target.value) })}
                />
              </label>
            </>
          ) : null}
          {option.type === 7 ? (
            <div className="flex w-full flex-wrap gap-2">
              {CHANNEL_TYPES.map((channelType) => (
                <label key={channelType.value} className="flex items-center gap-1.5 rounded-md border border-ink-700 bg-ink-900/60 px-2 py-1 text-[11px] text-fg-subtle">
                  <input
                    type="checkbox"
                    className="h-3 w-3 accent-[var(--fg)]"
                    checked={(option.channelTypes ?? []).includes(channelType.value)}
                    onChange={(event) => {
                      const current = option.channelTypes ?? [];
                      patch({
                        channelTypes: event.target.checked ? [...current, channelType.value] : current.filter((value) => value !== channelType.value),
                      });
                    }}
                  />
                  {channelType.label}
                </label>
              ))}
            </div>
          ) : null}
        </div>
      ) : null}

      {allowsChoices ? (
        <div className="mt-2 space-y-1.5">
          <div className="flex items-center justify-between">
            <p className="text-[11.5px] font-medium text-fg-subtle">
              {t("discord.commands.choices")} ({option.choices?.length ?? 0}/25)
            </p>
            <Button icon="Plus" variant="solid" onClick={() => patch({ choices: [...(option.choices ?? []), { name: "", value: "" }] })}>
              {t("discord.commands.addChoice")}
            </Button>
          </div>
          {(option.choices ?? []).map((choice, index) => (
            <div key={index} className="grid grid-cols-[1fr_1fr_auto] items-center gap-2">
              <TextInput
                value={choice.name}
                onChange={(v) => patch({ choices: (option.choices ?? []).map((item, i) => (i === index ? { ...item, name: v } : item)) })}
                placeholder={t("discord.commands.choiceName")}
              />
              <TextInput
                value={String(choice.value)}
                onChange={(v) =>
                  patch({
                    choices: (option.choices ?? []).map((item, i) =>
                      i === index ? { ...item, value: numeric && v !== "" && !Number.isNaN(Number(v)) ? Number(v) : v } : item,
                    ),
                  })
                }
                placeholder={t("discord.commands.choiceValue")}
              />
              <IconButton
                icon="Trash2"
                label={t("common.delete")}
                onClick={() => patch({ choices: (option.choices ?? []).filter((_, i) => i !== index) })}
              />
            </div>
          ))}
        </div>
      ) : null}

      {isValue ? null : depth < 2 ? (
        <div className="mt-2 space-y-1.5">
          <div className="flex items-center justify-between">
            <p className="text-[11.5px] font-medium text-fg-subtle">
              {isGroup ? t("discord.commands.subcommands") : t("discord.commands.options")} ({option.options?.length ?? 0}/25)
            </p>
            <Button
              icon="Plus"
              variant="solid"
              onClick={() => patch({ options: [...(option.options ?? []), { type: isGroup ? 1 : 3, name: "", description: "" }] })}
            >
              {isGroup ? t("discord.commands.addSubcommand") : t("discord.commands.addOption")}
            </Button>
          </div>
          {(option.options ?? []).map((child, index) => (
            <OptionEditor
              key={index}
              option={child}
              depth={depth + 1}
              onChange={(next) => patch({ options: (option.options ?? []).map((item, i) => (i === index ? next : item)) })}
              onRemove={() => patch({ options: (option.options ?? []).filter((_, i) => i !== index) })}
              onMove={(delta) => {
                const options = [...(option.options ?? [])];
                const target = index + delta;
                if (target < 0 || target >= options.length) return;
                [options[index], options[target]] = [options[target], options[index]];
                patch({ options });
              }}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
