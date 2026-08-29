/**
 * Desktop-bridge mock for the Integrations live harness. Implements the
 * integration surface (statuses, catalogs, identities, triggers, settings)
 * that IntegrationsView + the three panels touch. State is mutable so the
 * harness can flip it at runtime via window.__integrationsMock.
 */
import type {
  DiscordEventDescriptor,
  DiscordIdentity,
  Settings,
  TelegramIdentity,
  TriggerBinding,
  TwitchEventDescriptor,
  TwitchIdentity,
} from "../src/lib/types";

const now = new Date().toISOString();

const state: {
  settings: Settings;
  triggers: TriggerBinding[];
  twitchConnected: boolean;
  discordConnected: boolean;
  telegramConnected: boolean;
  discordError: string;
  savedSettings: Settings[];
  deviceAuthStarted: number;
  removedIdentities: string[];
  trustedTriggers: string[];
  toggledTriggers: Array<{ id: string; enabled: boolean }>;
} = {
  settings: {
    language: "en",
    hideToTrayOnClose: true,
    defaultProviderId: "openai-compatible",
    contentDirectory: "/home/z/.neuropipe",
    retentionDays: 30,
    webhookPort: 7878,
    pluginDirectory: "",
    providers: [{ id: "openai-compatible", name: "OpenAI-compatible", kind: "openai-compatible", baseUrl: "", model: "", enabled: true }],
    maxConcurrentRuns: 4,
    maxConcurrentLLMRuns: 2,
    llamaRuntime: {
      binaryPath: "",
      modelPath: "",
      mode: "auto",
      contextSize: 8192,
      autoStart: false,
    },
    api: {
      enabled: false,
      bindAddress: "127.0.0.1",
      port: 7878,
      authMode: "token",
      token: "",
      exposureAcknowledged: false,
    },
    metrics: {
      detailRetentionDays: 30,
      rollupRetentionDays: 365,
      sampleIntervalSeconds: 30,
      priceRates: [],
    },
    twitch: {
      clientId: "kimne78kx3ncx6brgo4mv6wki5h0ko",
      defaultBotIdentityId: "tw-1",
      identities: [
        {
          id: "tw-1",
          label: "Stream bot",
          userId: "100001",
          login: "stream_bot",
          scopes: ["user:read:chat", "user:write:chat"],
          status: "connected",
          method: "device-code",
        },
        {
          id: "tw-2",
          label: "Mod bot",
          userId: "100002",
          login: "mod_bot",
          scopes: ["user:read:chat"],
          status: "reconnect-required",
          method: "manual",
        },
      ],
    },
    discord: {
      defaultBotIdentityId: "dc-1",
      identities: [
        { id: "dc-1", label: "Helper bot", botUserId: "200001", username: "helper_bot", status: "connected" },
        { id: "dc-2", label: "Old bot", botUserId: "200002", username: "old_bot", status: "invalid" },
      ],
    },
    telegram: {
      defaultBotIdentityId: undefined,
      identities: [],
    },
  },
  triggers: [
    { id: "tr-tw-1", kind: "twitch", label: "!hello command", enabled: true, trusted: true, pipelineId: "p1", published: true, updatedAt: now },
    { id: "tr-tw-2", kind: "twitch", label: "Follower alert", enabled: false, trusted: false, pipelineId: "p2", published: true, updatedAt: now },
    { id: "tr-dc-1", kind: "discord", label: "Mod mail", enabled: false, trusted: false, pipelineId: "p3", published: true, updatedAt: now },
    { id: "tr-tg-1", kind: "telegram", label: "/alert command", enabled: true, trusted: true, pipelineId: "p4", published: true, updatedAt: now },
    { id: "tr-btn-1", kind: "button", label: "Dashboard button", enabled: true, trusted: true, pipelineId: "p5", published: true, updatedAt: now },
  ],
  twitchConnected: true,
  discordConnected: false,
  telegramConnected: true,
  discordError: "gateway: privileged intents not enabled",
  savedSettings: [],
  deviceAuthStarted: 0,
  removedIdentities: [],
  trustedTriggers: [],
  toggledTriggers: [],
};

const delay = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

const twitchCatalog: TwitchEventDescriptor[] = [
  {
    type: "channel.chat.message",
    version: "1",
    label: "Chat message",
    description: "A channel chat message",
    requiredScopes: ["user:read:chat"],
    conditions: [],
    eventType: { kind: "object", properties: [] } as never,
    chatMessage: true,
  },
];

const discordCatalog: DiscordEventDescriptor[] = [
  {
    type: "message.create",
    gatewayEvent: "MESSAGE_CREATE",
    label: "Message created",
    description: "A guild message",
    intents: 15,
    privileged: true,
    chatMessage: true,
    conditions: [],
  },
  {
    type: "guild.member.add",
    gatewayEvent: "GUILD_MEMBER_ADD",
    label: "Member joined",
    description: "A member joined",
    intents: 2,
    privileged: false,
    chatMessage: false,
    conditions: [],
  },
];

const cloneSettings = (): Settings => JSON.parse(JSON.stringify(state.settings));

export const desktop = {
  getSettings: async () => {
    await delay(20);
    return cloneSettings();
  },
  saveSettings: async (next: Settings) => {
    state.savedSettings.push(JSON.parse(JSON.stringify(next)));
    state.settings = JSON.parse(JSON.stringify(next));
    await delay(30);
  },
  listTriggers: async () => {
    await delay(20);
    return state.triggers;
  },
  listSchedules: async () => [] as TriggerBinding[],

  getTwitchStatus: async () => {
    await delay(20);
    return {
      connected: state.twitchConnected,
      connectionState: state.twitchConnected ? "connected" : "disconnected",
      activeSubscriptions: state.twitchConnected ? 3 : 0,
    };
  },
  getDiscordStatus: async () => {
    await delay(20);
    return {
      connected: state.discordConnected,
      connectionState: state.discordConnected ? "connected" : "disconnected",
      activeSubscriptions: state.discordConnected ? 2 : 0,
      ...(state.discordConnected ? {} : { lastError: state.discordError }),
    };
  },
  getTelegramStatus: async () => {
    await delay(20);
    return {
      connected: state.telegramConnected,
      connectionState: state.telegramConnected ? "connected" : "disconnected",
      activeSubscriptions: 1,
    };
  },

  listTwitchEventCatalog: async () => {
    await delay(20);
    return twitchCatalog;
  },
  listDiscordEventCatalog: async () => {
    await delay(20);
    return discordCatalog;
  },

  startTwitchDeviceAuthorization: async (request: { label: string }) => {
    state.deviceAuthStarted++;
    await delay(40);
    return {
      id: `auth-${state.deviceAuthStarted}`,
      userCode: "ABCD-EFGH",
      verificationUri: "https://www.twitch.tv/activate",
      expiresAt: new Date(Date.now() + 600_000).toISOString(),
      intervalSeconds: 5,
      label: request.label,
    };
  },
  cancelTwitchDeviceAuthorization: async () => undefined,
  addTwitchManualIdentity: async (request: { label: string; accessToken: string }) => {
    const identity: TwitchIdentity = {
      id: `tw-${state.settings.twitch.identities.length + 1}`,
      label: request.label,
      userId: "100999",
      login: "manual_bot",
      scopes: ["user:read:chat"],
      status: "connected",
      method: "manual",
    };
    state.settings.twitch.identities.push(identity);
    await delay(40);
  },
  removeTwitchIdentity: async (id: string) => {
    state.removedIdentities.push(id);
    state.settings.twitch.identities = state.settings.twitch.identities.filter((i) => i.id !== id);
    if (state.settings.twitch.defaultBotIdentityId === id) state.settings.twitch.defaultBotIdentityId = undefined;
    await delay(30);
  },
  trustTwitchTrigger: async (id: string) => {
    state.trustedTriggers.push(id);
    const t = state.triggers.find((x) => x.id === id);
    if (t) t.trusted = true;
    await delay(20);
  },
  setTwitchTriggerEnabled: async (id: string, enabled: boolean) => {
    state.toggledTriggers.push({ id, enabled });
    const t = state.triggers.find((x) => x.id === id);
    if (t) t.enabled = enabled;
    await delay(20);
  },

  addDiscordManualIdentity: async (request: { label: string; token: string }) => {
    if (request.token === "bad-token") throw new Error("invalid token");
    const identity: DiscordIdentity = {
      id: `dc-${state.settings.discord.identities.length + 1}`,
      label: request.label,
      botUserId: "200999",
      username: "new_bot",
      status: "connected",
    };
    state.settings.discord.identities.push(identity);
    if (!state.settings.discord.defaultBotIdentityId) state.settings.discord.defaultBotIdentityId = identity.id;
    await delay(40);
  },
  removeDiscordIdentity: async (id: string) => {
    state.removedIdentities.push(id);
    state.settings.discord.identities = state.settings.discord.identities.filter((i) => i.id !== id);
    if (state.settings.discord.defaultBotIdentityId === id) state.settings.discord.defaultBotIdentityId = undefined;
    await delay(30);
  },
  trustDiscordTrigger: async (id: string) => {
    state.trustedTriggers.push(id);
    const t = state.triggers.find((x) => x.id === id);
    if (t) t.trusted = true;
    await delay(20);
  },
  setDiscordTriggerEnabled: async (id: string, enabled: boolean) => {
    state.toggledTriggers.push({ id, enabled });
    const t = state.triggers.find((x) => x.id === id);
    if (t) t.enabled = enabled;
    await delay(20);
  },

  addTelegramManualIdentity: async (request: { label: string; token: string }) => {
    const identity: TelegramIdentity = {
      id: `tg-${state.settings.telegram.identities.length + 1}`,
      label: request.label,
      botUserId: "300999",
      username: "alert_bot",
      status: "connected",
    };
    state.settings.telegram.identities.push(identity);
    if (!state.settings.telegram.defaultBotIdentityId) state.settings.telegram.defaultBotIdentityId = identity.id;
    await delay(40);
  },
  removeTelegramIdentity: async (id: string) => {
    state.removedIdentities.push(id);
    state.settings.telegram.identities = state.settings.telegram.identities.filter((i) => i.id !== id);
    if (state.settings.telegram.defaultBotIdentityId === id) state.settings.telegram.defaultBotIdentityId = undefined;
    await delay(30);
  },
  trustTelegramTrigger: async (id: string) => {
    state.trustedTriggers.push(id);
    const t = state.triggers.find((x) => x.id === id);
    if (t) t.trusted = true;
    await delay(20);
  },
  setTelegramTriggerEnabled: async (id: string, enabled: boolean) => {
    state.toggledTriggers.push({ id, enabled });
    const t = state.triggers.find((x) => x.id === id);
    if (t) t.enabled = enabled;
    await delay(20);
  },
};

export function wailsUnavailable(): Error {
  return new Error("wails unavailable");
}

declare global {
  interface Window {
    __integrationsMock: typeof state;
  }
}
window.__integrationsMock = state;
