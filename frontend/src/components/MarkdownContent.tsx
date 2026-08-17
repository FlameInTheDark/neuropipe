import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import rehypeRaw from "rehype-raw";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import { Browser } from "@wailsio/runtime";
import type { ReactNode } from "react";
import { CodeBlock } from "@/components/CodeBlock";

const permittedSchemes = new Set(["http:", "https:"]);
const markdownSchema = {
  ...defaultSchema,
  protocols: {
    ...defaultSchema.protocols,
    href: [...(defaultSchema.protocols?.href ?? []), "docs"],
  },
};
const encodedSafeHTMLTag =
  /&lt;(\/?(?:a|abbr|b|blockquote|br|code|del|details|div|em|h[1-6]|hr|i|img|kbd|li|ol|p|pre|s|small|span|strong|sub|summary|sup|table|tbody|td|th|thead|tr|u|ul)\b[^<>]*?)&gt;/gi;

// A few model cards escape otherwise-valid layout tags. Decode only a known
// presentation subset before rehype parses and sanitizes it; scripts, styles,
// event handlers, and unsafe URLs still never reach the rendered DOM.
function normalizeSafeEmbeddedHTML(markdown: string) {
  return markdown.replace(encodedSafeHTMLTag, "<$1>");
}

function externalURL(
  href: string | undefined,
  baseURL?: string,
): string | undefined {
  if (!href) return undefined;
  try {
    const target = new URL(href, baseURL);
    return permittedSchemes.has(target.protocol)
      ? target.toString()
      : undefined;
  } catch {
    return undefined;
  }
}

// markdownComponents is shared by reports and remote model cards so both use
// the same safe, readable app typography.
function headingID(children: ReactNode): string {
  const text = collectText(children)
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]+/gu, "-")
    .replace(/(^-|-$)/g, "");
  return text || "section";
}

function collectText(value: ReactNode): string {
  if (typeof value === "string" || typeof value === "number") return String(value);
  if (Array.isArray(value)) return value.map(collectText).join("");
  if (value && typeof value === "object" && "props" in value) {
    return collectText((value as { props?: { children?: ReactNode } }).props?.children ?? "");
  }
  return "";
}

function documentLink(href: string | undefined): { id: string; anchor?: string } | undefined {
  if (!href?.startsWith("docs:")) return undefined;
  const [id, anchor] = href.slice("docs:".length).split("#", 2);
  return id ? { id, anchor } : undefined;
}

function components(baseURL?: string, onDocumentLink?: (id: string, anchor?: string) => void): Components {
  return {
    h1: ({ children }) => (
      <h1 id={headingID(children)} className="mt-8 scroll-mt-6 break-words text-2xl font-semibold tracking-tight text-zinc-100 first:mt-0">
        {children}
      </h1>
    ),
    h2: ({ children }) => (
      <h2 id={headingID(children)} className="mt-7 scroll-mt-6 break-words text-xl font-semibold tracking-tight text-zinc-100">
        {children}
      </h2>
    ),
    h3: ({ children }) => (
      <h3 id={headingID(children)} className="mt-6 scroll-mt-6 break-words text-base font-semibold text-zinc-100">
        {children}
      </h3>
    ),
    p: ({ children }) => (
      <p className="mt-4 break-words [overflow-wrap:anywhere] text-sm leading-7 text-zinc-300 first:mt-0">
        {children}
      </p>
    ),
    ul: ({ children }) => (
      <ul className="mt-4 list-disc space-y-1 break-words pl-5 text-sm leading-7 text-zinc-300">
        {children}
      </ul>
    ),
    ol: ({ children }) => (
      <ol className="mt-4 list-decimal space-y-1 break-words pl-5 text-sm leading-7 text-zinc-300">
        {children}
      </ol>
    ),
    li: ({ children }) => <li>{children}</li>,
    blockquote: ({ children }) => (
      <blockquote className="mt-5 break-words border-l-2 border-zinc-700 pl-4 text-sm italic leading-7 text-zinc-400">
        {children}
      </blockquote>
    ),
    hr: () => <hr className="my-7 border-zinc-800" />,
    a: ({ children, href }) => {
      const internal = documentLink(href);
      if (internal && onDocumentLink) {
        return <button type="button" onClick={() => onDocumentLink(internal.id, internal.anchor)} className="break-all text-left text-zinc-100 underline decoration-zinc-500 underline-offset-4 hover:decoration-zinc-200">{children}</button>;
      }
      const target = externalURL(href, baseURL);
      return target ? (
        <a
          href={target}
          onClick={(event) => {
            event.preventDefault();
            Browser.OpenURL(target);
          }}
          className="break-all text-zinc-100 underline decoration-zinc-500 underline-offset-4 hover:decoration-zinc-200"
        >
          {children}
        </a>
      ) : (
        <span className="text-zinc-400">{children}</span>
      );
    },
    pre: ({ children }) => <CodeBlock>{children}</CodeBlock>,
    code: ({ children, className }) =>
      className ? (
        <code className={`font-mono text-[0.85em] ${className}`}>
          {children}
        </code>
      ) : (
        <code className="break-all rounded bg-zinc-800 px-1.5 py-0.5 font-mono text-[0.85em] text-zinc-200">
          {children}
        </code>
      ),
    table: ({ children }) => (
      <div className="mt-5 max-w-full overflow-x-auto">
        <table className="w-full min-w-max border-collapse text-left text-sm">
          {children}
        </table>
      </div>
    ),
    th: ({ children }) => (
      <th className="break-words border-b border-zinc-700 px-3 py-2 font-medium text-zinc-100">
        {children}
      </th>
    ),
    td: ({ children }) => (
      <td className="break-words border-b border-zinc-800 px-3 py-2 align-top leading-6 text-zinc-300">
        {children}
      </td>
    ),
    img: ({ alt, src }) => (
      <img
        src={externalURL(src, baseURL)}
        alt={alt ?? ""}
        className="mt-5 h-auto max-w-full rounded-lg border border-zinc-800"
      />
    ),
    input: ({ type, checked }) =>
      type === "checkbox" ? (
        <input
          type="checkbox"
          checked={checked}
          readOnly
          className="mr-2 accent-zinc-100"
        />
      ) : null,
  };
}

export function MarkdownContent({
  markdown,
  baseURL,
  onDocumentLink,
}: {
  markdown: string;
  baseURL?: string;
  onDocumentLink?: (id: string, anchor?: string) => void;
}) {
  return (
    <div className="report-markdown min-w-0 max-w-full overflow-hidden">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[
          rehypeRaw,
          [rehypeSanitize, markdownSchema],
          [rehypeHighlight, { detect: true }],
        ]}
        components={components(baseURL, onDocumentLink)}
      >
        {normalizeSafeEmbeddedHTML(markdown)}
      </ReactMarkdown>
    </div>
  );
}
