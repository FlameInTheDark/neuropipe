import { useState } from "react";
import { Loader2, Sparkles, X } from "lucide-react";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { desktop } from "@/lib/bridge";
import type { CodeGenerationRequest, CodeGenerationResponse } from "@/lib/types";
import { useTranslation } from "react-i18next";

interface LLMCodeAssistantDialogProps {
  open: boolean;
  request: Omit<CodeGenerationRequest, "prompt">;
  onApply: (code: string) => void;
  onClose: () => void;
}

export function LLMCodeAssistantDialog({ open, request, onApply, onClose }: LLMCodeAssistantDialogProps) {
  const { t } = useTranslation();
  const [prompt, setPrompt] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const generate = async () => {
    if (!prompt.trim()) return;
    setLoading(true);
    setError("");
    try {
      const response: CodeGenerationResponse = await desktop.generateCode({
        ...request,
        prompt,
      } as CodeGenerationRequest);
      if (response.code) {
        onApply(response.code);
        onClose();
        setPrompt("");
      } else {
        setError(t("codeAssistant.empty", "The model returned no code. Try a different prompt."));
      }
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("codeAssistant.failed", "Code generation failed."));
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      void generate();
    }
  };

  return (
    <Dialog
      open={open}
      title={t("codeAssistant.title", "AI Code Assistant")}
      description={t("codeAssistant.description", "Describe what you want the code to do. The model will generate or edit the code based on the current editor state.")}
      onOpenChange={(next) => { if (!next && !loading) { onClose(); } }}
      className="max-w-2xl"
    >
      <div className="space-y-4 p-5">
        <textarea
          value={prompt}
          onChange={(event) => setPrompt(event.target.value)}
          onKeyDown={handleKeyDown}
          disabled={loading}
          placeholder={t("codeAssistant.placeholder", "e.g. Write a query that counts orders per customer in the last 30 days, sorted by count descending")}
          className="min-h-32 w-full resize-y rounded-md border border-zinc-700 bg-zinc-950 px-3 py-2.5 text-sm text-zinc-100 outline-none placeholder:text-zinc-600 focus:border-zinc-500"
          autoFocus
        />
        {error ? <p className="text-xs text-red-400">{error}</p> : null}
        {loading ? (
          <div className="flex items-center gap-2 text-sm text-zinc-400">
            <Loader2 className="size-4 animate-spin" />
            {t("codeAssistant.thinking", "Thinking…")}
          </div>
        ) : null}
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={() => { onClose(); setPrompt(""); }} disabled={loading}>
            <X className="size-3.5" />
            {t("common.cancel", "Cancel")}
          </Button>
          <Button onClick={() => void generate()} disabled={loading || !prompt.trim()}>
            {loading ? <Loader2 className="size-3.5 animate-spin" /> : <Sparkles className="size-3.5" />}
            {t("codeAssistant.generate", "Generate")}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
