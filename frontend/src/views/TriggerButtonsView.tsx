import { useEffect, useRef, useState } from "react";
import { Loader2, Play, Square, Workflow } from "lucide-react";
import { EmptyState } from "@/components/EmptyState";
import { LucideIcon } from "@/components/LucideIconPicker";
import { PageHeader } from "@/components/PageHeader";
import { desktop } from "@/lib/bridge";
import { cn } from "@/lib/utils";
import type { PipelineSummary, TriggerBinding } from "@/lib/types";
import { useUIStore } from "@/stores/ui";
import { useTranslation } from "react-i18next";

type ButtonFeedback = "idle" | "running" | "success" | "failure";

const feedbackDurationMs = 900;

function pipelineFor(
  pipelines: readonly PipelineSummary[],
  binding: TriggerBinding,
) {
  return pipelines.find((pipeline) => pipeline.id === binding.pipelineId);
}

/** Stream Deck-style surface for persistent Button Trigger bindings. */
export function TriggerButtonsView({
  buttons,
  pipelines,
  onRefresh,
}: {
  buttons: TriggerBinding[];
  pipelines: PipelineSummary[];
  onRefresh: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const { setError, setScreen } = useUIStore();
  const [feedback, setFeedback] = useState<Record<string, ButtonFeedback>>({});
  const [runningPipelines, setRunningPipelines] = useState<Record<string, boolean>>({});
  const timers = useRef(new Map<string, number>());

  useEffect(
    () => () => {
      timers.current.forEach((timer) => window.clearTimeout(timer));
      timers.current.clear();
    },
    [],
  );

  // Poll running state so the button toggles to Stop when the pipeline is
  // active, regardless of whether the run was started from this board, the
  // editor, or a schedule.
  useEffect(() => {
    if (buttons.length === 0) return
    let cancelled = false
    const poll = async () => {
      try {
        const results = await Promise.all(
          buttons.map(async (binding) => ({
            id: binding.pipelineId,
            running: await desktop.isPipelineRunning(binding.pipelineId),
          })),
        )
        if (cancelled) return
        const next: Record<string, boolean> = {}
        for (const result of results) {
          if (result.running) next[result.id] = true
        }
        setRunningPipelines(next)
      } catch {
        // ignore polling errors
      }
    }
    void poll()
    const interval = window.setInterval(poll, 2000)
    return () => { cancelled = true; window.clearInterval(interval) }
  }, [buttons])

  const finishFeedback = (id: string, status: "success" | "failure") => {
    const previous = timers.current.get(id);
    if (previous !== undefined) window.clearTimeout(previous);
    setFeedback((current) => ({ ...current, [id]: status }));
    timers.current.set(
      id,
      window.setTimeout(() => {
        timers.current.delete(id);
        setFeedback((current) => ({ ...current, [id]: "idle" }));
      }, feedbackDurationMs),
    );
  };

  const run = async (binding: TriggerBinding) => {
    if (feedback[binding.id] && feedback[binding.id] !== "idle") return;
    setFeedback((current) => ({ ...current, [binding.id]: "running" }));
    try {
      const execution = await desktop.runTrigger(binding.id);
      if (execution.status !== "completed") {
        throw new Error(execution.error || t("board.runIncomplete"));
      }
      await onRefresh();
      finishFeedback(binding.id, "success");
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : t("board.runFailed");
      setError(message);
      finishFeedback(binding.id, "failure");
    }
  };

  const stop = async (binding: TriggerBinding) => {
    try {
      await desktop.cancelPipelineExecution(binding.pipelineId);
      setRunningPipelines((current) => { const next = { ...current }; delete next[binding.pipelineId]; return next })
      await onRefresh();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("board.stopFailed"));
    }
  };

  return (
    <section className="flex h-full min-h-0 flex-col">
      <PageHeader
        title={t("board.title")}
        description={t("board.description")}
      />
      <div className="muted-scroll min-h-0 flex-1 overflow-y-auto p-7">
        {buttons.length === 0 ? (
          <EmptyState
            icon={Workflow}
            title={t("board.emptyTitle")}
            description={t("board.emptyDescription")}
            action={{
              label: t("board.openPipelines"),
              onClick: () => setScreen("pipelines"),
            }}
          />
        ) : (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(108px,1fr))] gap-x-5 gap-y-7 sm:grid-cols-[repeat(auto-fill,minmax(120px,1fr))]">
            {buttons.map((binding) => {
              const state = feedback[binding.id] ?? "idle";
              const pipeline = pipelineFor(pipelines, binding);
              const name = pipeline?.name ?? binding.label;
              const detail =
                name === binding.label
                  ? binding.hotkey || t("board.buttonTrigger")
                  : binding.label;
              const isRunning = Boolean(runningPipelines[binding.pipelineId]);
              const handleClick = () => {
                if (isRunning) void stop(binding);
                else void run(binding);
              };
              return (
                <button
                  key={binding.id}
                  type="button"
                  disabled={state !== "idle" && !isRunning}
                  aria-label={isRunning ? t("board.stop", { name }) : t("board.run", { name })}
                  onClick={handleClick}
                  className="group min-w-0 text-center outline-none focus-visible:ring-2 focus-visible:ring-white/60 focus-visible:ring-offset-4 focus-visible:ring-offset-zinc-950 disabled:cursor-wait"
                >
                  <span
                    className={cn(
                      "relative mx-auto flex size-20 items-center justify-center overflow-hidden rounded-xl border border-white/15 shadow-lg shadow-black/35 transition-transform duration-150 group-hover:-translate-y-0.5 group-active:translate-y-0 sm:size-24",
                      state === "success" && "trigger-button-success",
                      state === "failure" && "trigger-button-failure",
                      isRunning && "trigger-button-running",
                    )}
                    style={{
                      backgroundColor: pipeline?.iconBackground || binding.color || "#fafafa",
                      color: pipeline?.iconColor || "#18181b",
                    }}
                  >
                    <LucideIcon name={pipeline?.icon || binding.icon} className="size-9 stroke-[1.9]" />
                    {(state === "running" || isRunning) ? (
                      <span className="absolute inset-0 flex items-center justify-center bg-black/55">
                        {isRunning ? <Square className="size-7 text-white" /> : <Loader2 className="size-7 animate-spin text-white" />}
                      </span>
                    ) : null}
                  </span>
                  <span className="mt-2 block truncate text-xs font-medium text-zinc-100">
                    {name}
                  </span>
                  <span className="mt-1 block truncate text-[11px] text-zinc-600">
                    {isRunning ? t("board.running") : detail}
                  </span>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </section>
  );
}
