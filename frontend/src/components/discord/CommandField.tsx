import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { desktop } from "@/lib/bridge";
import type { DiscordApplicationCommand, DiscordApplicationCommandOption } from "@/lib/types";
import { Dropdown } from "../Dropdown";

/**
 * The Command picker of the Discord Command Trigger: a dropdown over the
 * application commands registered on the selected bot identity. Picking a
 * command stores its id, name, and option schema in the node config so the
 * backend resolver and the canvas twin can grow one typed pin per option.
 */
export function CommandField({
  value,
  identityId,
  onChange,
}: {
  value: unknown;
  identityId: string;
  onChange: (next: unknown) => void;
}) {
  const { t } = useTranslation();
  const [commands, setCommands] = useState<DiscordApplicationCommand[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let cancelled = false;
    if (!identityId) {
      setCommands([]);
      return;
    }
    setLoading(true);
    desktop
      .listDiscordApplicationCommands(identityId, "")
      .then((list) => {
        if (!cancelled) setCommands(list ?? []);
      })
      .catch(() => {
        if (!cancelled) setCommands([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [identityId]);

  const options = useMemo(
    () => [
      { value: "", label: t("discord.commands.pickPlaceholder"), icon: "Slash" },
      ...commands.map((command) => ({
        value: command.id ?? command.name,
        label: `${command.type === 2 || command.type === 3 ? "" : "/"}${command.name}`,
        icon: "Slash",
      })),
    ],
    [commands, t],
  );

  const current = typeof value === "object" && value !== null ? (value as Record<string, unknown>) : {};
  const currentId = String(current.commandId ?? "");
  const currentName = String(current.commandName ?? "");
  const selected = currentId || (currentName ? commands.find((command) => command.name === currentName)?.id ?? currentName : "");

  const pick = (id: string) => {
    const command = commands.find((entry) => entry.id === id);
    if (!command) {
      onChange({});
      return;
    }
    onChange({
      commandId: command.id ?? "",
      commandName: command.name,
      options: flattenOptions(command.options ?? []),
    });
  };

  const optionCount = Array.isArray(current.options) ? current.options.length : 0;

  return (
    <div className="space-y-1">
      <Dropdown value={selected} options={options} onChange={pick} />
      <p className="text-[10.5px] text-fg-faint">
        {loading
          ? t("discord.commands.loading")
          : currentName
            ? `${currentName} · ${optionCount} ${t("discord.commands.optionsCount")}`
            : t("discord.commands.pickHelp")}
      </p>
    </div>
  );
}

/** flattenOptions stores the command's value options (subcommands and
 * groups collapse into their leaf options), carrying name, type, and
 * required — everything the pin resolvers need on both sides of the bridge. */
function flattenOptions(options: DiscordApplicationCommandOption[]): Array<{ name: string; type: number; required: boolean }> {
  const result: Array<{ name: string; type: number; required: boolean }> = [];
  const walk = (entries: DiscordApplicationCommandOption[]) => {
    for (const option of entries) {
      if ((option.options?.length ?? 0) > 0) {
        walk(option.options ?? []);
        continue;
      }
      result.push({ name: option.name, type: option.type, required: Boolean(option.required) });
    }
  };
  walk(options);
  return result;
}
