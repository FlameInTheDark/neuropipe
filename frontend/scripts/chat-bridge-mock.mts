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
  ChatQuestionAnswer,
  ChatQuestions,
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
const pendingQuestions: ChatQuestions[] = [];

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

if (scenario === "questions") {
  conversations.push({
    id: "conv-q",
    mode: "model",
    title: "Database choice",
    actionPolicy: "ask",
    providerId: "",
    model: "",
    reasoning: "",
    toolIds: [],
    createdAt: "2026-09-03T09:00:00Z",
    updatedAt: "2026-09-03T09:30:00Z",
  });
  // A resolved question form from an earlier turn renders as history.
  messages.push(
    {
      id: "qm-1",
      conversationId: "conv-q",
      chatRunId: "qrun-1",
      role: "user",
      content: "Which reporting stack do we use?",
      createdAt: "2026-09-03T09:05:00Z",
    },
    {
      id: "qm-2",
      conversationId: "conv-q",
      chatRunId: "qrun-1",
      role: "assistant",
      content: "",
      toolCalls: [{ id: "ask-0", name: "ask_user_questions", arguments: {} }],
      createdAt: "2026-09-03T09:05:04Z",
    },
    {
      id: "qm-3",
      conversationId: "conv-q",
      chatRunId: "qrun-1",
      role: "tool",
      toolCallId: "ask-0",
      toolName: "ask_user_questions",
      content: JSON.stringify({
        answers: [
          { question: "Charts or tables?", source: "option", answer: "Charts", description: "Visual summaries per metric" },
          { question: "Any export format?", source: "custom", answer: "CSV and XLSX" },
          { question: "Email delivery?", source: "rejected" },
        ],
        note: "source vocabulary",
      }),
      createdAt: "2026-09-03T09:06:00Z",
    },
    {
      id: "qm-4",
      conversationId: "conv-q",
      chatRunId: "qrun-1",
      role: "assistant",
      content: "Noted: charts, CSV/XLSX export, no email.",
      createdAt: "2026-09-03T09:06:05Z",
    },
  );
  runs.push({
    id: "qrun-1",
    conversationId: "conv-q",
    status: "completed",
    statusText: "Completed",
    createdAt: "2026-09-03T09:05:00Z",
    updatedAt: "2026-09-03T09:06:05Z",
  });
  // The live paused turn with the open question form under test.
  messages.push(
    {
      id: "qm-5",
      conversationId: "conv-q",
      chatRunId: "qrun-2",
      role: "user",
      content: "Help me choose a database for the pipeline",
      createdAt: "2026-09-03T09:20:00Z",
    },
    {
      id: "qm-6",
      conversationId: "conv-q",
      chatRunId: "qrun-2",
      role: "assistant",
      content: "A few things first:",
      toolCalls: [
        { id: "ask-1", name: "ask_user_questions", arguments: {} },
        { id: "ask-2", name: "list_pipelines", arguments: {} },
      ],
      createdAt: "2026-09-03T09:20:04Z",
    },
    {
      id: "qm-7",
      conversationId: "conv-q",
      chatRunId: "qrun-2",
      role: "tool",
      toolCallId: "ask-2",
      toolName: "list_pipelines",
      content: "Skipped: the turn paused to ask the user a question before this tool could run.",
      createdAt: "2026-09-03T09:20:05Z",
    },
  );
  runs.push({
    id: "qrun-2",
    conversationId: "conv-q",
    status: "pending",
    statusText: "Waiting for your answers",
    createdAt: "2026-09-03T09:20:00Z",
    updatedAt: "2026-09-03T09:20:05Z",
  });
  runEvents.push(
    {
      id: "qev-1",
      chatRunId: "qrun-2",
      kind: "questions",
      summary: "Asked 3 question(s)",
      detail: "[]",
      status: "pending",
      createdAt: "2026-09-03T09:20:05Z",
    },
  );
  pendingQuestions.push({
    id: "q-form-1",
    conversationId: "conv-q",
    chatRunId: "qrun-2",
    toolCallId: "ask-1",
    status: "pending",
    createdAt: "2026-09-03T09:20:05Z",
    questions: [
      {
        question: "Which database engine should we use?",
        options: [
          { label: "PostgreSQL", description: "Relational, strong consistency" },
          { label: "SQLite", description: "Embedded, zero-ops" },
          { label: "MongoDB", description: "Document store, flexible schema" },
        ],
      },
      {
        question: "How much data do we expect per month?",
        options: [
          { label: "Under 1 GB" },
          { label: "1-100 GB", description: "Typical for local pipelines" },
          { label: "Over 100 GB" },
        ],
      },
      {
        question: "Do you need built-in full-text search?",
        options: [
          { label: "Yes", description: "Indexes every stored payload" },
          { label: "No", description: "Plain storage only" },
        ],
      },
    ],
  });
}

if (scenario === "rename") {
  conversations.push({
    id: "conv-r",
    mode: "model",
    title: "New chat",
    actionPolicy: "ask",
    providerId: "",
    model: "",
    reasoning: "",
    toolIds: [],
    createdAt: "2026-09-03T10:00:00Z",
    updatedAt: "2026-09-03T10:00:00Z",
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

/** Replays the model-owned rename turn exactly like the chat service does:
 *  the conversation title is patched and pushed via chat.conversation.updated
 *  the moment the rename tool runs (mid-turn), before the final reply. */
async function simulateRenameTurn(conversationId: string, text: string): Promise<void> {
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
  await sleep(120);
  run.status = "running";
  emit("chat.run.updated", { chatRunId: runId });
  await sleep(120);

  // round 1: the model calls rename_conversation
  messages.push({
    id: `m-live-rename-call-${sendCounter}`,
    conversationId,
    chatRunId: runId,
    role: "assistant",
    content: "",
    toolCalls: [{ id: "rename-1", name: "rename_conversation", arguments: { title: "Weather in Munich" } }],
    createdAt: nowISO(),
  });
  messages.push({
    id: `m-live-rename-result-${sendCounter}`,
    conversationId,
    chatRunId: runId,
    role: "tool",
    toolCallId: "rename-1",
    toolName: "rename_conversation",
    content: '{"renamed":true,"title":"Weather in Munich"}',
    createdAt: nowISO(),
  });
  const convIndex = conversations.findIndex((c) => c.id === conversationId);
  if (convIndex >= 0) {
    conversations[convIndex] = { ...conversations[convIndex], title: "Weather in Munich", updatedAt: nowISO() };
    emit("chat.conversation.updated", conversations[convIndex]);
  }
  emit("chat.updated", { chatRunId: runId });
  await sleep(200);

  // round 2: the final streamed reply
  const reply = "Today in Munich: 18 degrees and sunny.";
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
    if (scenario === "rename") {
      void simulateRenameTurn(conversationId, text);
    } else {
      void simulateModelTurn(conversationId, text);
    }
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
  async listPendingChatQuestions(conversationId: string): Promise<ChatQuestions[]> {
    await delay(10);
    return pendingQuestions.filter((q) => q.conversationId === conversationId);
  },
  async resolveChatQuestions(id: string, answers: ChatQuestionAnswer[]): Promise<void> {
    await delay(10);
    const index = pendingQuestions.findIndex((q) => q.id === id);
    if (index < 0) throw new Error("question form is no longer pending");
    const [record] = pendingQuestions.splice(index, 1);
    record.status = "answered";
    record.answers = answers;
    record.resolvedAt = nowISO();
    messages.push({
      id: `qm-tool-${id}`,
      conversationId: record.conversationId,
      chatRunId: record.chatRunId,
      role: "tool",
      toolCallId: record.toolCallId,
      toolName: "ask_user_questions",
      content: JSON.stringify({
        answers: answers.map((answer) => ({
          question: answer.question,
          source: answer.source,
          answer: answer.source === "option" ? answer.chosenLabel : answer.source === "custom" ? answer.custom : undefined,
        })),
        note: "source vocabulary",
      }),
      createdAt: nowISO(),
    });
    const run = runs.find((r) => r.id === record.chatRunId);
    if (run) {
      run.status = "running";
      run.statusText = "Working";
    }
    emit("chat.updated", { chatRunId: record.chatRunId });
    await sleep(120);
    const reply = "Great choice - PostgreSQL with about 50 GB per month. I will set up the pipeline now.";
    for (const chunk of reply.match(/.{1,7}/gs) ?? []) {
      emit("chat.token", { chatRunId: record.chatRunId, conversationId: record.conversationId, delta: chunk });
      await sleep(20);
    }
    emit("chat.token.end", { chatRunId: record.chatRunId });
    await sleep(60);
    messages.push({
      id: `qm-reply-${id}`,
      conversationId: record.conversationId,
      chatRunId: record.chatRunId,
      role: "assistant",
      content: reply,
      createdAt: nowISO(),
    });
    if (run) {
      run.status = "completed";
      run.statusText = "Completed";
    }
    emit("chat.run.updated", { chatRunId: record.chatRunId });
    emit("chat.updated", { chatRunId: record.chatRunId });
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
  pendingQuestions,
};
