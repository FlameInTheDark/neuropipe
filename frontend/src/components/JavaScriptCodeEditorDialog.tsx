import { useEffect, useMemo, useRef, useState, type Dispatch, type ReactNode, type SetStateAction } from "react";
import { Braces, FileInput, FileOutput, Loader2, Plus, Sparkles, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { JavaScriptTypeSpecEditor } from "@/components/JavaScriptTypeSpecEditor";
import { LLMCodeAssistantDialog } from "@/components/LLMCodeAssistantDialog";
import { desktop } from "@/lib/bridge";
import {
  defaultJavaScriptNodeConfig,
  isJavaScriptIdentifier,
  javascriptDeclarations,
  type JavaScriptCapability,
  type JavaScriptNodeConfig,
  type JavaScriptPinContract,
} from "@/lib/javascript-node";
import { loadMonaco } from "@/lib/monaco";
import type { TypeSpec } from "@/lib/types";

type EditableContract = JavaScriptPinContract;
const capabilityIDs: JavaScriptCapability[] = ["file-read", "file-write", "network"];

function editableContracts(contracts: readonly JavaScriptPinContract[]): EditableContract[] {
  return contracts.map((contract) => ({ ...contract }));
}

function addContract(contracts: readonly EditableContract[], prefix: string): EditableContract[] {
  const occupied = new Set(contracts.map((contract) => contract.id));
  let index = contracts.length + 1;
  while (occupied.has(`${prefix}${index}`)) index += 1;
  return [...contracts, {
    id: `${prefix}${index}`,
    label: `${prefix}${index}`,
    required: false,
    type: { kind: "any" },
  }];
}

function parseContracts(
  contracts: readonly EditableContract[],
  group: "inputs" | "outputs",
  invalidIdentifier: string,
  duplicateIdentifier: string,
  invalidType: string,
): { contracts?: JavaScriptPinContract[]; error?: string } {
  const ids = new Set<string>();
  const parsed: JavaScriptPinContract[] = [];
  for (const contract of contracts) {
    const id = contract.id.trim();
    if (!isJavaScriptIdentifier(id)) return { error: `${group}: ${invalidIdentifier}` };
    if (ids.has(id)) return { error: `${group}: ${duplicateIdentifier}` };
    ids.add(id);
    if (!isValidTypeSpec(contract.type)) {
      return { error: `${group}: ${invalidType}` };
    }
    parsed.push({
      id,
      label: contract.label.trim() || id,
      required: contract.required,
      type: contract.type,
    });
  }
  return { contracts: parsed };
}

function isValidTypeSpec(type: TypeSpec): boolean {
  switch (type.kind) {
    case "any":
    case "bool":
    case "string":
    case "int":
    case "float":
    case "bytes":
      return true;
    case "list":
      return !!type.element && isValidTypeSpec(type.element);
    case "map":
      return type.key?.kind === "string" && !!type.value && isValidTypeSpec(type.value);
    case "record": {
      const seen = new Set<string>();
      return (type.fields ?? []).every((field) => {
        const name = (field.name || field.id).trim();
        if (!name || seen.has(name)) return false;
        seen.add(name);
        return isValidTypeSpec(field.type);
      });
    }
    default:
      return false;
  }
}

export function JavaScriptCodeEditorDialog({
  open,
  config,
  onClose,
  onSave,
}: {
  open: boolean;
  config: Record<string, unknown>;
  onClose: () => void;
  onSave: (config: JavaScriptNodeConfig) => void;
}) {
  const { t } = useTranslation();
  const [code, setCode] = useState("return {};");
  const [inputs, setInputs] = useState<EditableContract[]>([]);
  const [outputs, setOutputs] = useState<EditableContract[]>([]);
  const [capabilities, setCapabilities] = useState<JavaScriptCapability[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [aiOpen, setAIOpen] = useState(false);
  const editorElement = useRef<HTMLDivElement>(null);
  const editor = useRef<import("monaco-editor").editor.IStandaloneCodeEditor>();
  const declaration = useRef<import("monaco-editor").IDisposable>();
  const codeRef = useRef(code);

  useEffect(() => { codeRef.current = code; }, [code]);
  useEffect(() => {
    if (!open) return;
    const current = defaultJavaScriptNodeConfig(config);
    setCode(current.code);
    setInputs(editableContracts(current.inputs));
    setOutputs(editableContracts(current.outputs));
    setCapabilities(current.capabilities);
    setError("");
  }, [config, open]);

  useEffect(() => {
    if (!open || !editorElement.current) return;
    let cancelled = false;
    let currentEditor: import("monaco-editor").editor.IStandaloneCodeEditor | undefined;
    void loadMonaco().then((monaco) => {
      if (cancelled || !editorElement.current) return;
      monaco.editor.defineTheme("neuropipe-dark", {
        base: "vs-dark",
        inherit: true,
        rules: [],
        colors: { "editor.background": "#09090b", "editorGutter.background": "#09090b" },
      });
      currentEditor = monaco.editor.create(editorElement.current, {
        value: codeRef.current,
        language: "javascript",
        theme: "neuropipe-dark",
        automaticLayout: true,
        minimap: { enabled: false },
        fontSize: 13,
        lineHeight: 21,
        padding: { top: 14, bottom: 14 },
        scrollBeyondLastLine: false,
        wordWrap: "on",
        editContext: false,
      });
      currentEditor.onDidChangeModelContent(() => setCode(currentEditor?.getValue() ?? ""));
      editor.current = currentEditor;
      currentEditor.focus();
    }).catch(() => setError(t("javascript.editorUnavailable")));
    return () => {
      cancelled = true;
      declaration.current?.dispose();
      declaration.current = undefined;
      currentEditor?.dispose();
      editor.current = undefined;
    };
  }, [open, t]);

  const draft = useMemo<JavaScriptNodeConfig>(() => ({
    code,
    inputs,
    outputs,
    capabilities,
  }), [capabilities, code, inputs, outputs]);
  useEffect(() => {
    if (!open || !editor.current) return;
    void loadMonaco().then((monaco) => {
      declaration.current?.dispose();
      const javascriptDefaults = (monaco as unknown as {
        languages: { typescript: { javascriptDefaults: { addExtraLib(value: string, path: string): import("monaco-editor").IDisposable } } };
      }).languages.typescript.javascriptDefaults;
      declaration.current = javascriptDefaults.addExtraLib(
        javascriptDeclarations(draft),
        "file:///neuropipe-node-api.d.ts",
      );
    });
  }, [draft, open]);

  const updateContract = (
    setContracts: Dispatch<SetStateAction<EditableContract[]>>,
    index: number,
    change: Partial<EditableContract>,
  ) => setContracts((current) => current.map((contract, contractIndex) =>
    contractIndex === index ? { ...contract, ...change } : contract,
  ));
  const toggleCapability = (capability: JavaScriptCapability, checked: boolean) => {
    setCapabilities((current) => checked
      ? [...new Set([...current, capability])]
      : current.filter((item) => item !== capability));
  };
  const save = async () => {
    const inputResult = parseContracts(inputs, "inputs", t("javascript.invalidIdentifier"), t("javascript.duplicateIdentifier"), t("javascript.invalidType"));
    const outputResult = parseContracts(outputs, "outputs", t("javascript.invalidIdentifier"), t("javascript.duplicateIdentifier"), t("javascript.invalidType"));
    if (inputResult.error || outputResult.error) {
      setError(inputResult.error ?? outputResult.error ?? "");
      return;
    }
    setSaving(true);
    setError("");
    try {
      await desktop.validateJavaScript(code);
      onSave({
        code,
        inputs: inputResult.contracts ?? [],
        outputs: outputResult.contracts ?? [],
        capabilities,
      });
      onClose();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : t("javascript.validationFailed"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog
      open={open}
      title={t("javascript.title")}
      description={t("javascript.description")}
      className="h-[min(900px,calc(100vh-40px))] max-w-[min(1320px,calc(100vw-40px))]"
      onOpenChange={(next) => { if (!next && !saving) onClose(); }}
    >
      <div className="grid min-h-0 flex-1 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="flex min-h-0 flex-col border-b border-zinc-800 lg:border-b-0 lg:border-r">
          <div className="flex items-center gap-2 border-b border-zinc-800 px-4 py-2.5 text-xs text-zinc-500">
            <Braces className="size-4 text-amber-200" />
            <span>{t("javascript.editorHint")}</span>
            <Button
              size="sm"
              variant="ghost"
              className="ml-auto h-7 px-2 text-xs text-violet-300 hover:text-violet-200"
              onClick={() => setAIOpen(true)}
            >
              <Sparkles className="size-3.5" />
              {t("codeAssistant.title", "AI Code Assistant")}
            </Button>
          </div>
          <div ref={editorElement} className="min-h-72 flex-1" />
        </div>
        <div className="muted-scroll min-h-0 overflow-y-auto p-4">
          <ContractList
            icon={<FileInput className="size-3.5" />}
            title={t("javascript.inputs")}
            prefix="input"
            contracts={inputs}
            onAdd={() => setInputs((current) => addContract(current, "input"))}
            onChange={(index, change) => updateContract(setInputs, index, change)}
            onRemove={(index) => setInputs((current) => current.filter((_, currentIndex) => currentIndex !== index))}
          />
          <ContractList
            icon={<FileOutput className="size-3.5" />}
            title={t("javascript.outputs")}
            prefix="output"
            contracts={outputs}
            onAdd={() => setOutputs((current) => addContract(current, "output"))}
            onChange={(index, change) => updateContract(setOutputs, index, change)}
            onRemove={(index) => setOutputs((current) => current.filter((_, currentIndex) => currentIndex !== index))}
          />
          <section className="mt-5 border-t border-zinc-800 pt-4">
            <h3 className="text-xs font-medium text-zinc-200">{t("javascript.access")}</h3>
            <p className="mt-1 text-[11px] leading-4 text-zinc-500">{t("javascript.accessHint")}</p>
            <div className="mt-3 space-y-2">
              {capabilityIDs.map((capability) => (
                <div key={capability} className="flex items-center justify-between gap-3 rounded-md border border-zinc-800 bg-zinc-900/35 px-2.5 py-2">
                  <span className="text-xs text-zinc-300">{t(`javascript.capabilities.${capability}`)}</span>
                  <Switch
                    checked={capabilities.includes(capability)}
                    onCheckedChange={(checked) => toggleCapability(capability, checked)}
                    label={t(`javascript.capabilities.${capability}`)}
                  />
                </div>
              ))}
            </div>
          </section>
        </div>
      </div>
      {error ? <p className="border-t border-red-500/20 bg-red-500/5 px-5 py-2.5 text-xs text-red-200">{error}</p> : null}
      <div className="flex justify-end gap-2 border-t border-zinc-800 px-5 py-3">
        <Button type="button" variant="outline" onClick={onClose} disabled={saving}>{t("common.cancel")}</Button>
        <Button type="button" onClick={() => void save()} disabled={saving}>
          {saving ? <Loader2 className="size-3.5 animate-spin" /> : <Braces className="size-3.5" />}
          {t("javascript.save")}
        </Button>
      </div>
      <LLMCodeAssistantDialog
        open={aiOpen}
        request={{
          editorType: "javascript",
          currentCode: code,
          jsContext: {
            inputs: inputs.map((c) => ({ id: c.id, type: JSON.stringify(c.type) })),
            outputs: outputs.map((c) => ({ id: c.id, type: JSON.stringify(c.type) })),
            capabilities: capabilities,
          },
        }}
        onApply={(generatedCode) => {
          setCode(generatedCode);
          if (editor.current) { editor.current.setValue(generatedCode); }
        }}
        onClose={() => setAIOpen(false)}
      />
    </Dialog>
  );
}

function ContractList({
  icon,
  title,
  prefix,
  contracts,
  onAdd,
  onChange,
  onRemove,
}: {
  icon: ReactNode;
  title: string;
  prefix: string;
  contracts: readonly EditableContract[];
  onAdd: () => void;
  onChange: (index: number, change: Partial<EditableContract>) => void;
  onRemove: (index: number) => void;
}) {
  const { t } = useTranslation();
  return (
    <section className="border-b border-zinc-800 pb-5 last:border-0">
      <div className="flex items-center justify-between gap-2">
        <h3 className="flex items-center gap-1.5 text-xs font-medium text-zinc-200">{icon}{title}</h3>
        <Button type="button" size="sm" variant="ghost" className="h-7 px-2 text-[11px]" onClick={onAdd}>
          <Plus className="size-3" /> {t("javascript.add")}
        </Button>
      </div>
      <p className="mt-1 text-[11px] leading-4 text-zinc-500">{t("javascript.contractHint")}</p>
      <div className="mt-3 space-y-3">
        {contracts.map((contract, index) => (
          <div key={`${prefix}-${index}`} className="rounded-md border border-zinc-800 bg-zinc-900/35 p-2.5">
            <div className="grid grid-cols-[minmax(0,1fr)_auto] gap-2">
              <label className="min-w-0 text-[10px] font-medium uppercase tracking-wide text-zinc-500">
                {t("javascript.identifier")}
                <Input value={contract.id} onChange={(event) => onChange(index, { id: event.target.value })} className="mt-1 h-8 font-mono text-xs" aria-label={`${title} ${index + 1} ${t("javascript.identifier")}`} />
              </label>
              <button type="button" onClick={() => onRemove(index)} className="mt-5 rounded p-1.5 text-zinc-500 hover:bg-zinc-800 hover:text-red-200 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-zinc-400" aria-label={`${t("common.delete")} ${contract.label || index + 1}`}>
                <Trash2 className="size-3.5" />
              </button>
            </div>
            <label className="mt-2 block text-[10px] font-medium uppercase tracking-wide text-zinc-500">
              {t("javascript.label")}
              <Input value={contract.label} onChange={(event) => onChange(index, { label: event.target.value })} className="mt-1 h-8 text-xs" aria-label={`${title} ${index + 1} ${t("javascript.label")}`} />
            </label>
            <div className="mt-2">
              <p className="mb-1 text-[10px] font-medium uppercase tracking-wide text-zinc-500">{t("javascript.type")}</p>
              <JavaScriptTypeSpecEditor
                value={contract.type}
                onChange={(type) => onChange(index, { type })}
                ariaLabel={`${title} ${index + 1} ${t("javascript.type")}`}
              />
            </div>
            <div className="mt-2 flex items-center justify-between gap-2">
              <span className="text-[11px] text-zinc-400">{t("javascript.required")}</span>
              <Switch checked={contract.required} onCheckedChange={(required) => onChange(index, { required })} label={`${title} ${index + 1} ${t("javascript.required")}`} />
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
