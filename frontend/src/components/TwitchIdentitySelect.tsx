import { useEffect, useState } from "react";
import { Select } from "@/components/ui/select";
import { desktop } from "@/lib/bridge";
import type { TwitchIdentity } from "@/lib/types";
import { useTranslation } from "react-i18next";

// TwitchIdentitySelect is shared between settings and node inspectors so they
// present the same safe identity labels and never expose token material.
export function TwitchIdentitySelect({
  value,
  onValueChange,
  ariaLabel,
}: {
  value: string;
  onValueChange: (value: string) => void;
  ariaLabel: string;
}) {
  const { t } = useTranslation();
  const [identities, setIdentities] = useState<TwitchIdentity[]>([]);
  useEffect(() => {
    let current = true;
    void desktop.getSettings().then((settings) => {
      if (current) setIdentities(settings.twitch?.identities ?? []);
    });
    return () => { current = false; };
  }, []);
  return (
    <Select
      value={value}
      onValueChange={onValueChange}
      ariaLabel={ariaLabel}
      placeholder={t("twitch.identityPlaceholder")}
      options={identities.map((identity) => ({
        value: identity.id,
        label: identity.status === "connected"
          ? identity.label
          : `${identity.label} — ${t("twitch.reconnectRequired")}`,
      }))}
    />
  );
}
