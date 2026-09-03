/**
 * Desktop-bridge mock for the ChatView transcript live harness. Implements the
 * chat surface ChatView touches and replays the REAL backend event sequence
 * (chat.updated -> chat.token* -> chat.token.end -> chat.run.updated /
 * chat.updated) so the UI runs against faithful timing. State is mutable via
 * window.__chatMock for the Playwright driver.
 *
 * Scenarios via ?case=:
 *   loaded : an existing multi-turn conversation (plain turns + one tool round)
 *   new    : no conversation at all — the first message creates it, exactly
 *            like the user's report ("when I type message in the chat")
 */
import type {
  ChatApproval,
  ChatConversation,
  ChatMessage,
  ChatPipeline,
  ChatRun,
  ChatRunEvent,
  Settings,
} from "../src/lib/types";

type Harness = {
  __wailsEvents: Map<string, Array<(data?: unknown) => void>>;
  __chatMock: {
    emit: (event: string, payload?: unknown) => void;
    conversations: ChatConversation[];
    messages: ChatMessage[];
    runs: ChatRun[];
    events: ChatRunEvent[];
  };
};

const harness = window as unknown as Harness;

const conversations: ChatConversation[] = [];
const messages: ChatMessage[] = [];
const runs: ChatRun[] = [];
const runEvents: ChatRunEvent[] = [];

const scenario = new URLSearchParams(window.location.search).get("case") ?? "loaded";

function nowISO(): string {
  return new Date().toISOString();
}

if (scenario === "loaded") {
  conversations.push({
    id: "conv-1",
    mode: "model",
    title: "Pipelines and reports",
    actionPolicy: "ask",
    providerId: "",
    model: "",
    reasoning: "",
    toolIds: [],
    createdAt: "2026-09-03T08:00:00Z",
    updatedAt: "2026-09-03T08:30:00Z",
  });
  messages.push(
    {
      id: "m-1",
      conversationId: "conv-1",
      chatRunId: "run-1",
      role: "user",
      content: "Hi! What can you help me with?",
      createdAt: "2026-09-03T08:10:00Z",
    },
    {
      id: "m-2",
      conversationId: "conv-1",
      chatRunId: "run-1",
      role: "assistant",
      content:
        "Hello! I can build and run automations: list your pipelines, run them, and manage reports and executions.",
      createdAt: "2026-09-03T08:10:08Z",
    },
    {
      id: "m-3",
      conversationId: "conv-1",
      chatRunId: "run-2",
      role: "user",
      content: "Show me my pipelines please",
      createdAt: "2026-09-03T08:20:00Z",
    },
    {
      id: "m-4",
      conversationId: "conv-1",
      chatRunId: "run-2",
      role: "assistant",
      content: "Let me check your pipelines.",
      toolCalls: [{ id: "call-1", name: "list_pipelines", arguments: {} }],
      createdAt: "2026-09-03T08:20:04Z",
    },
    {
      id: "m-5",
      conversationId: "conv-1",
      chatRunId: "run-2",
      role: "tool",
      toolCallId: "call-1",
      toolName: "list_pipelines",
      content: '[{"name":"Morning report","id":"p-1"},{"name":"Data sync","id":"p-2"}]',
      createdAt: "2026-09-03T08:20:05Z",
    },
    {
      id: "m-6",
      conversationId: "conv-1",
      chatRunId: "run-2",
      role: "assistant",
      content:
        "You have two pipelines: **Morning report** and **Data sync**. Want me to run one of them?",
      createdAt: "2026-09-03T08:20:09Z",
    },
  );
  runs.push(
    {
      id: "run-1",
      conversationId: "conv-1",
      status: "completed",
      statusText: "Completed",
      createdAt: "2026-09-03T08:10:00Z",
      updatedAt: "2026-09-03T08:10:08Z",
    },
    {
      id: "run-2",
      conversationId: "conv-1",
      status: "completed",
      statusText: "Completed",
      createdAt: "2026-09-03T08:20:00Z",
      updatedAt: "2026-09-03T08:20:09Z",
    },
  );
  runEvents.push({
    id: "ev-1",
    chatRunId: "run-2",
    kind: "tool",
    summary: "List pipelines",
    detail: '[{"name":"Morning report","id":"p-1"},{"name":"Data sync","id":"p-2"}]',
    status: "completed",
    createdAt: "2026-09-03T08:20:05Z",
  });
}

function emit(event: string, payload?: unknown): void {
  const handlers = harness.__wailsEvents.get(event) ?? [];
  for (const handler of [...handlers]) handler({ data: payload });
}

const settings: Settings = {
  defaultProviderId: "prov-1",
  providers: [
    {
      id: "prov-1",
      name: "OpenAI",
      kind: "openai-compatible",
      baseURL: "https://api.openai.com",
      model: "gpt-4o",
      enabled: true,
      models: [
        { id: "gpt-4o", name: "GPT-4o" },
        { id: "gpt-4o-mini", name: "GPT-4o mini" },
      ],
    },
  ],
} as unknown as Settings;

let sendCounter = 0;

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** Replays one plain model turn exactly like the chat service does:
 *  chat.updated (user row) -> chat.run.updated (running) -> chat.token* ->
 *  chat.token.end -> chat.run.updated (completed) -> chat.updated. */
async function simulateModelTurn(conversationId: string, text: string): Promise<void> {
  sendCounter += 1;
  const runId = `run-live-${sendCounter}`;
  const run: ChatRun = {
    id: runId,
    conversationId,
    status: "pending",
    statusText: "Working",
    createdAt: nowISO(),
    updatedAt: nowISO(),
  };
  runs.push(run);
  messages.push({
    id: `m-live-user-${sendCounter}`,
    conversationId,
    chatRunId: runId,
    role: "user",
    content: text,
    createdAt: nowISO(),
  });
  emit("chat.updated", { chatRunId: runId });
  await sleep(150);
  run.status = "running";
  emit("chat.run.updated", { chatRunId: runId });

  const reply = "Simulated assistant reply for: " + text;
  for (const chunk of reply.match(/.{1,7}/gs) ?? []) {
    emit("chat.token", { chatRunId: runId, conversationId, delta: chunk });
    await sleep(25);
  }
  emit("chat.token.end", { chatRunId: runId });
  await sleep(80);
  run.status = "completed";
  run.statusText = "Completed";
  messages.push({
    id: `m-live-assistant-${sendCounter}`,
    conversationId,
    chatRunId: runId,
    role: "assistant",
    content: reply,
    createdAt: nowISO(),
  });
  emit("chat.run.updated", { chatRunId: runId });
  emit("chat.updated", { chatRunId: runId });
}

const delay = (ms: number) => sleep(ms);

export const desktop = {
  async listChatConversations(): Promise<ChatConversation[]> {
    await delay(10);
    return [...conversations];
  },
  async listChatPipelines(): Promise<ChatPipeline[]> {
    await delay(10);
    return [];
  },
  async getSettings(): Promise<Settings> {
    await delay(10);
    return settings;
  },
  async listFunctions(): Promise<unknown[]> {
    await delay(10);
    return [];
  },
  async createChatConversation(mode: string): Promise<ChatConversation> {
    await delay(10);
    const conv: ChatConversation = {
      id: `conv-new-${Date.now()}`,
      mode: mode as "model" | "pipeline",
      title: "New chat",
      actionPolicy: "ask",
      createdAt: nowISO(),
      updatedAt: nowISO(),
    };
    conversations.push(conv);
    return conv;
  },
  async saveChatConversation(conv: ChatConversation): Promise<ChatConversation> {
    await delay(10);
    const index = conversations.findIndex((c) => c.id === conv.id);
    if (index >= 0) conversations[index] = { ...conv };
    return conv;
  },
  async sendChatMessage(conversationId: string, text: string): Promise<ChatRun> {
    await delay(10);
    void simulateModelTurn(conversationId, text);
    return {
      id: `run-live-${sendCounter + 1}`,
      conversationId,
      status: "pending",
      statusText: "Working",
      createdAt: nowISO(),
      updatedAt: nowISO(),
    };
  },
  async cancelChatRun(): Promise<void> {},
  async deleteChatConversation(id: string): Promise<void> {
    const index = conversations.findIndex((c) => c.id === id);
    if (index >= 0) conversations.splice(index, 1);
  },
  async listChatMessagesPage(
    conversationId: string,
    offset: number,
    limit: number,
  ): Promise<{ messages: ChatMessage[]; hasMore: boolean; total: number }> {
    await delay(10);
    const all = messages
      .filter((m) => m.conversationId === conversationId)
      .sort((a, b) => a.createdAt.localeCompare(b.createdAt));
    const end = Math.max(0, all.length - offset);
    const start = Math.max(0, end - limit);
    return { messages: all.slice(start, end), hasMore: start > 0, total: all.length };
  },
  async listChatRuns(conversationId: string): Promise<ChatRun[]> {
    await delay(10);
    return runs.filter((r) => r.conversationId === conversationId);
  },
  async listPendingChatApprovals(): Promise<ChatApproval[]> {
    await delay(10);
    return [];
  },
  async listChatRunEvents(runId: string): Promise<ChatRunEvent[]> {
    await delay(10);
    return runEvents.filter((e) => e.chatRunId === runId);
  },
  async resolveChatApproval(): Promise<void> {},
};

harness.__chatMock = {
  emit,
  conversations,
  messages,
  runs,
  events: runEvents,
};
