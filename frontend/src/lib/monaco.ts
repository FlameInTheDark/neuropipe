import EditorWorker from "monaco-editor/editor/editor.worker?worker";
import TypeScriptWorker from "monaco-editor/language/typescript/ts.worker?worker";
import type * as Monaco from "monaco-editor";

declare global {
  interface Window {
    MonacoEnvironment?: {
      getWorker(moduleId: string, label: string): Worker;
    };
  }
}

/** Loads Monaco only for the JavaScript editor instead of the regular app shell. */
export async function loadMonaco(): Promise<typeof Monaco> {
  window.MonacoEnvironment ??= {
    getWorker(_moduleId, label) {
      return label === "javascript" || label === "typescript"
        ? new TypeScriptWorker()
        : new EditorWorker();
    },
  };
  return import("monaco-editor");
}
