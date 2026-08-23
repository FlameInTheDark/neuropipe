import type { ReactNode } from "react";
import type { NavApi } from "./useWorkspaceNav";
import type { Workspace } from "./useWorkspace";
import {
  BoardView,
  ChatView,
  DatastoresView,
  DocsView,
  FunctionsView,
  MetricsView,
  PipelinesView,
  ReportsView,
  RunsView,
  SchedulesView,
  SettingsView,
  TriggersView,
  VariablesView,
} from "../../views";

/** Everything a view might need from the shell. */
export interface ViewContext {
  workspace: Workspace;
  nav: NavApi;
}

/**
 * Maps a rail id to its view.
 * Adding a section means adding one entry here — no switch statement to
 * edit and no changes to `App` (open/closed principle).
 */
export const VIEW_REGISTRY: Record<string, (ctx: ViewContext) => ReactNode> = {
  board: (ctx) => <BoardView workspace={ctx.workspace} nav={ctx.nav} />,
  chat: () => <ChatView />,
  triggers: (ctx) => <TriggersView workspace={ctx.workspace} nav={ctx.nav} />,
  schedules: (ctx) => <SchedulesView workspace={ctx.workspace} />,
  reports: (ctx) => <ReportsView workspace={ctx.workspace} nav={ctx.nav} />,
  runs: (ctx) => <RunsView workspace={ctx.workspace} nav={ctx.nav} />,
  pipelines: (ctx) => <PipelinesView workspace={ctx.workspace} nav={ctx.nav} />,
  functions: (ctx) => <FunctionsView workspace={ctx.workspace} nav={ctx.nav} />,
  variables: (ctx) => <VariablesView workspace={ctx.workspace} />,
  databases: () => <DatastoresView />,
  metrics: () => <MetricsView />,
  docs: () => <DocsView />,
  settings: (ctx) => <SettingsView workspace={ctx.workspace} />,
};

export function renderView(rail: string, ctx: ViewContext): ReactNode {
  const render = VIEW_REGISTRY[rail] ?? VIEW_REGISTRY.pipelines;
  return render(ctx);
}
