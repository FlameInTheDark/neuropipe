import type { Edge, Node, Viewport } from "@xyflow/react";

export type PipelineStatus = "draft" | "active" | "archived" | "legacy";
export type TriggerKind = "button" | "cron" | "file" | "hotkey" | "webhook" | "chat" | "twitch";
export type RunStatus =
  "pending" | "running" | "completed" | "failed" | "skipped" | "cancelled";

export interface UpdateAvailability {
  available: boolean;
  version?: string;
  url?: string;
}
export type Capability =
  | "network"
  | "file-read"
  | "file-write"
  | "terminal"
  | "git"
  | "docker"
  | "plugin";
export type PinKind = "exec" | "data" | "tool";
export type PinDirection = "input" | "output";
export type DataType =
  "any" | "text" | "number" | "boolean" | "object" | "list";
export type TypeKind =
  | "any" | "bool" | "string" | "int" | "float" | "bytes" | "list" | "map" | "record";
export interface TypeSpec {
  kind: TypeKind;
  name?: string;
  element?: TypeSpec;
  key?: TypeSpec;
  value?: TypeSpec;
  fields?: TypeFieldSpec[];
}
export interface TypeFieldSpec {
  id: string;
  name: string;
  type: TypeSpec;
  optional?: boolean;
}
export type NodeExecutionMode = "event" | "impure" | "pure" | "visual";
export type FunctionKind = "function" | "tool";

export type FlowNode = Node<{
  type?: string;
  config?: Record<string, unknown>;
  [key: string]: unknown;
}>;
export type FlowEdge = Edge & { kind?: PinKind; waypoints?: { x: number; y: number }[] };

export interface FlowDefinition {
  schemaVersion: number;
  nodes: FlowNode[];
  edges: FlowEdge[];
  viewport: Viewport;
}

export interface Pipeline {
  id: string;
  name: string;
  description: string;
  icon: string;
  iconColor: string;
  iconBackground: string;
  status: PipelineStatus;
  draftDefinition: FlowDefinition;
  publishedRevision: number;
	/** True when the editable draft differs from the revision triggers execute. */
  hasUnpublishedChanges: boolean;
  migrationIssue?: string;
  createdAt: string;
  updatedAt: string;
}

export interface PipelineSummary {
  id: string;
  name: string;
  description: string;
  icon: string;
  iconColor: string;
  iconBackground: string;
  status: PipelineStatus;
  publishedRevision: number;
  triggerCount: number;
  migrationIssue?: string;
  updatedAt: string;
}

export interface TriggerBinding {
  id: string;
  pipelineId: string;
  nodeId: string;
  revision: number;
  kind: TriggerKind;
	  nodeType?: string;
	  config?: Record<string, unknown>;
  label: string;
  icon: string;
  color: string;
  gridPosition: number;
  hotkey?: string;
  cron?: string;
  timezone?: string;
  enabled: boolean;
  trusted: boolean;
  nextRunAt?: string;
  lastRunAt?: string;
  lastRunStatus?: RunStatus;
}

export interface DataField {
  path: string;
  label?: string;
  dataType: DataType;
  description?: string;
  optional?: boolean;
}
export interface NodePort {
  id: string;
  label: string;
  kind: PinKind;
  direction: PinDirection;
  dataType?: DataType;
  type?: TypeSpec;
  fields?: DataField[];
  color?: string;
  required?: boolean;
  default?: unknown;
  maxConnections?: number;
}
export interface ConfigField {
  name: string;
  label: string;
  kind: string;
  placeholder?: string;
  required?: boolean;
  secret?: boolean;
  visibleWhen?: string;
  options?: { value: string; label: string }[];
}
export interface NodeDefinition {
  type: string;
  category: string;
  label: string;
  description: string;
  icon: string;
  color: string;
  mode: NodeExecutionMode;
  triggerKind?: TriggerKind;
	portContractOwned?: boolean;
  inputs: NodePort[];
  outputs: NodePort[];
  fields: ConfigField[];
  capabilities: Capability[];
  defaultConfig: Record<string, unknown>;
  source: string;
}

export interface DocumentationEntry {
  id: string;
  title: string;
  summary?: string;
  category: string[];
  nodeTypes?: string[];
  source: "core" | "plugin";
  pluginId?: string;
}
export interface DocumentationDocument extends DocumentationEntry {
  markdown: string;
}
export interface DocumentationSearchResult {
  document: DocumentationEntry;
  excerpt: string;
}
export interface DocumentationReference {
  documentId: string;
  anchor?: string;
}

export interface FunctionPin {
  id: string;
  name: string;
  description?: string;
  dataType: DataType;
  type?: TypeSpec;
  required?: boolean;
  default?: unknown;
}
export interface CustomFunction {
  id: string;
  name: string;
  description: string;
  category: string;
  icon: string;
  iconColor: string;
  iconBackground: string;
  kind: FunctionKind;
  mode: Extract<NodeExecutionMode, "pure" | "impure">;
  inputs: FunctionPin[];
  outputs: FunctionPin[];
  draftDefinition: FlowDefinition;
  publishedRevision: number;
  createdAt: string;
  updatedAt: string;
}
export interface FunctionSummary {
  id: string;
  name: string;
  description: string;
  category: string;
  icon: string;
  iconColor: string;
  iconBackground: string;
  kind: FunctionKind;
  mode: Extract<NodeExecutionMode, "pure" | "impure">;
  publishedRevision: number;
  updatedAt: string;
}
export interface CreateFunctionRequest {
  name: string;
  description: string;
  kind: FunctionKind;
  mode: Extract<NodeExecutionMode, "pure" | "impure">;
}

export interface ProviderConfig {
  id: string;
  name: string;
  kind: "ollama" | "llamacpp" | "openai-compatible";
  baseUrl: string;
  model: string;
  apiKeyRef?: string;
  enabled: boolean;
}
export type RuntimeMode = "auto" | "cpu" | "cuda" | "vulkan" | "hip";
export interface LlamaRuntimeSettings {
  binaryPath: string;
  modelPath: string;
  runtimeVersion?: string;
  mode: RuntimeMode;
  contextSize: number;
  autoStart: boolean;
}
export interface LlamaRuntimeStatus {
  running: boolean;
  endpoint: string;
  mode: RuntimeMode;
  model: string;
  message: string;
}
export interface RuntimeArtifact {
  url?: string;
  size: number;
  sha256?: string;
}
export interface InstallProgress {
  kind: "runtime" | "model";
  stage:
    | "preparing"
    | "downloading"
    | "installing"
    | "installed"
    | "saving"
    | "complete"
    | "failed";
  label: string;
  downloadedBytes: number;
  totalBytes: number;
  bytesPerSecond: number;
  percentage: number;
}
export interface LlamaRuntimeRelease {
  version: string;
  publishedAt?: string;
  cpu: RuntimeArtifact;
  cuda: RuntimeArtifact;
  vulkan: RuntimeArtifact;
  hip: RuntimeArtifact;
}
export interface InstalledLlamaRuntime {
  version: string;
  cpuInstalled: boolean;
  cudaInstalled: boolean;
  vulkanInstalled: boolean;
  hipInstalled: boolean;
}
export interface LlamaRuntimeCatalogStatus {
  root: string;
  selectedVersion?: string;
  installed: InstalledLlamaRuntime[];
}
export interface LlamaRuntimeInstallRequest {
  version: string;
  mode: Exclude<RuntimeMode, "auto">;
}
export interface ModelSearchResult {
  id: string;
  author?: string;
  avatarUrl?: string;
  downloads: number;
  likes: number;
  lastModified?: string;
  tags?: string[];
}
export interface ModelSearchRequest {
  query: string;
  sort: "recommended" | "downloads" | "recent";
}
export interface ModelFile {
  name: string;
  size: number;
  sha256?: string;
  quantization?: string;
  recommended?: boolean;
}
export interface ModelDetail {
  id: string;
  author?: string;
  avatarUrl?: string;
  downloads: number;
  likes: number;
  lastModified?: string;
  tags?: string[];
  readme?: string;
  files: ModelFile[];
}
export interface ModelInstallRequest {
  repository: string;
  file: string;
}
export interface LocalModel {
  id: string;
  name: string;
  path: string;
  size: number;
  repository?: string;
  author?: string;
  avatarUrl?: string;
  downloads: number;
  likes: number;
  lastModified?: string;
  tags?: string[];
  quantization?: string;
  sha256?: string;
  installedAt?: string;
}
export type APIAuthMode = "token" | "none";
export interface APISettings {
  enabled: boolean;
  bindAddress: string;
  port: number;
  authMode: APIAuthMode;
  allowedOrigins: string[];
  adminEnabled: boolean;
  exposureAcknowledged: boolean;
}
export interface APIStatus {
  running: boolean;
  endpoint?: string;
  tokenConfigured: boolean;
  message?: string;
}
export interface Settings {

  language: "en" | "de" | "fr" | "ru";
  hideToTrayOnClose: boolean;
  defaultProviderId: string;
  contentDirectory: string;
  retentionDays: number;
  webhookPort: number;
  pluginDirectory: string;
  providers: ProviderConfig[];
  maxConcurrentRuns: number;
  maxConcurrentLLMRuns: number;
  llamaRuntime: LlamaRuntimeSettings;
  api: APISettings;
  metrics: MetricsSettings;
	  twitch: TwitchSettings;
}
export type TwitchIdentityStatus = "connected" | "expired" | "reconnect-required" | "revoked";
export type TwitchConnectionMethod = "device-code" | "manual";
export interface TwitchIdentity { id: string; label: string; userId: string; login: string; scopes: string[]; expiresAt?: string; status: TwitchIdentityStatus; method: TwitchConnectionMethod; }
export interface TwitchSettings { clientId: string; defaultBotIdentityId?: string; identities: TwitchIdentity[]; }
export interface TwitchEventConditionField { id: string; label: string; description: string; required: boolean; }
export interface TwitchEventDescriptor { type: string; version: string; label: string; description: string; requiredScopes: string[]; conditions: TwitchEventConditionField[]; eventType: TypeSpec; chatMessage: boolean; }
export interface TwitchStatus { connected: boolean; connectionState: string; activeSubscriptions: number; lastError?: string; }
export interface TwitchDeviceAuthorizationRequest { identityId?: string; label: string; scopes: string[]; }
export interface TwitchDeviceAuthorization { id: string; userCode: string; verificationUri: string; expiresAt: string; intervalSeconds: number; }
export interface TwitchManualIdentityRequest { label: string; accessToken: string; }
export interface TrayMenuLabels {
  show: string;
  settings: string;
  hide: string;
  close: string;
}
export interface ModelPriceRate {
  providerId: string;
  model: string;
  inputUsdPerMillion: number;
  outputUsdPerMillion: number;
}
export interface MetricsSettings {
  detailRetentionDays: number;
  rollupRetentionDays: number;
  sampleIntervalSeconds: number;
  priceRates: ModelPriceRate[];
}
export interface MetricsFilter {
  from: string;
  to: string;
  pipelineIds?: string[];
  providerIds?: string[];
  models?: string[];
  triggerKinds?: TriggerKind[];
  statuses?: RunStatus[];
}
export interface MetricsKPI { value: number; previousValue: number; available: boolean; }
export interface MetricsPoint { at: string; value: number; value2?: number; value3?: number; }
export interface MetricsRunPoint { at: string; completed: number; failed: number; skipped: number; cancelled: number; }
export interface MetricsBreakdown { id: string; label: string; value: number; secondary?: number; }
export interface MetricsPipelineHealth { pipelineId: string; name: string; at: string; completed: number; failed: number; }
export interface MetricsResourcePoint { at: string; process: string; cpuPercent: number; workingSet: number; }
export interface MetricsOverview {
  filter: MetricsFilter;
  granularity: "hour" | "day" | "month";
  runs: MetricsKPI;
  successRate: MetricsKPI;
  averageDurationMs: MetricsKPI;
  p95DurationMs: MetricsKPI;
  llmCalls: MetricsKPI;
  promptTokens: MetricsKPI;
  completionTokens: MetricsKPI;
  estimatedCostUsd: MetricsKPI;
  runSeries: MetricsRunPoint[];
  durationSeries: MetricsPoint[];
  llmSeries: MetricsPoint[];
  queueSeries: MetricsPoint[];
  pipelines: MetricsPipelineHealth[];
  failures: MetricsBreakdown[];
  slowNodes: MetricsBreakdown[];
  models: MetricsBreakdown[];
  triggers: MetricsBreakdown[];
  activity: MetricsBreakdown[];
  resources: MetricsResourcePoint[];
  tokensUnavailable: number;
  unpricedCalls: number;
  localCalls: number;
}
export interface SecretMetadata {
  name: string;
  createdAt: string;
  updatedAt: string;
}
export interface PluginStatus {
  id: string;
  name: string;
  version: string;
  path: string;
  enabled: boolean;
  healthy: boolean;
  nodeCount: number;
  description: string;
  error?: string;
}
export interface NodeRun {
  nodeId: string;
  nodeType: string;
  status: RunStatus;
  input?: unknown;
  output?: unknown;
  error?: string;
  startedAt: string;
  finishedAt: string;
}
export interface Execution {
  id: string;
  pipelineId: string;
  triggerId?: string;
  status: RunStatus;
  startedAt: string;
  finishedAt?: string;
  error?: string;
  nodeRuns: NodeRun[];
}
export interface Report {
  id: string;
  pipelineId: string;
  pipelineName: string;
  executionId: string;
  nodeId: string;
  title: string;
  tags: string[];
  markdown: string;
  createdAt: string;
  executionStartedAt: string;
}

export type ChatMode = "model" | "pipeline";
export type ChatActionPolicy = "ask" | "always";
export type ChatMessageRole = "user" | "assistant" | "tool" | "system";

export interface ChatToolCall {
  id: string;
  name: string;
  arguments: Record<string, unknown>;
}
export interface ChatConversation {
  id: string;
  mode: ChatMode;
  title: string;
  pipelineId?: string;
  triggerBindingId?: string;
  actionPolicy: ChatActionPolicy;
  createdAt: string;
  updatedAt: string;
}
export interface ChatMessage {
  id: string;
  conversationId: string;
  chatRunId?: string;
  role: ChatMessageRole;
  content: string;
  toolCallId?: string;
  toolName?: string;
  toolCalls?: ChatToolCall[];
  createdAt: string;
}
export interface ChatRun {
  id: string;
  conversationId: string;
  executionId?: string;
  status: RunStatus;
  statusText: string;
  error?: string;
  createdAt: string;
  updatedAt: string;
}
export interface ChatRunEvent {
  id: string;
  chatRunId: string;
  kind: string;
  summary: string;
  detail?: string;
  status?: RunStatus;
  createdAt: string;
}
export interface ChatApproval {
  id: string;
  conversationId: string;
  chatRunId: string;
  toolCall: ChatToolCall;
  status: string;
  createdAt: string;
  resolvedAt?: string;
}
export interface ChatPipeline {
  bindingId: string;
  pipelineId: string;
  pipelineName: string;
  label: string;
  icon: string;
  color: string;
  revision: number;
}
