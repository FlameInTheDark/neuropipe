import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import { cn } from "../utils/cn";

/**
 * Shared rich-markdown renderer (react-markdown + GFM + highlight.js).
 * Single source of truth for report/article typography - the text editor's
 * preview and the Reports page must render identical output.
 */
export function MarkdownRenderer({ text, className }: { text: string; className?: string }) {
  return (
    <div className={cn("mx-auto max-w-[740px] text-[13.5px] leading-[1.7] text-ink-200", className)}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[[rehypeHighlight, { detect: true, ignoreMissing: true }]]}
        components={{
          h1: (p) => <h1 className="mt-6 mb-3 text-[22px] font-bold tracking-tight text-ink-50" {...p} />,
          h2: (p) => <h2 className="mt-5 mb-2.5 text-[18px] font-semibold text-ink-50" {...p} />,
          h3: (p) => <h3 className="mt-4 mb-2 text-[15px] font-semibold text-ink-50" {...p} />,
          h4: (p) => <h4 className="mt-3 mb-1.5 text-[13.5px] font-semibold text-ink-100" {...p} />,
          p: (p) => <p className="my-2.5" {...p} />,
          a: ({ href, children, ...rest }) => (
            <a
              href={href}
              target="_blank"
              rel="noreferrer"
              className="text-sky-300 underline decoration-sky-300/40 underline-offset-2 hover:decoration-sky-300"
              {...rest}
            >
              {children}
            </a>
          ),
          strong: (p) => <strong className="font-semibold text-ink-50" {...p} />,
          em: (p) => <em className="italic text-ink-100" {...p} />,
          del: (p) => <del className="text-ink-500" {...p} />,
          ul: (p) => <ul className="my-2.5 ml-5 list-disc space-y-1 marker:text-ink-500" {...p} />,
          ol: (p) => <ol className="my-2.5 ml-5 list-decimal space-y-1 marker:text-ink-500" {...p} />,
          li: (p) => <li className="pl-1" {...p} />,
          hr: (p) => <hr className="my-5 border-ink-700" {...p} />,
          blockquote: (p) => (
            <blockquote
              className="my-3 border-l-2 border-ink-500 pl-3.5 text-ink-300 italic"
              {...p}
            />
          ),
          table: (p) => (
            <div className="my-3 overflow-x-auto rounded-lg border border-ink-700">
              <table className="w-full border-collapse text-[12.5px]" {...p} />
            </div>
          ),
          thead: (p) => <thead className="bg-ink-850/60" {...p} />,
          th: (p) => (
            <th
              className="border-b border-ink-700 px-3 py-1.5 text-left font-semibold text-ink-100"
              {...p}
            />
          ),
          td: (p) => (
            <td className="border-b border-seam px-3 py-1.5 text-ink-300 last:border-b-0" {...p} />
          ),
          code: ({ className, children, ...rest }) => {
            const inline = !className;
            if (inline) {
              return (
                <code
                  className="rounded bg-ink-800 px-1 py-px font-mono text-[12px] text-ink-100"
                  {...rest}
                >
                  {children}
                </code>
              );
            }
            return (
              <code className={cn("font-mono text-[12px]", className)} {...rest}>
                {children}
              </code>
            );
          },
          pre: ({ children, ...rest }) => (
            <pre
              className="my-3 overflow-x-auto rounded-lg border border-ink-700 bg-ink-950 p-3.5 text-[12px] leading-relaxed"
              {...rest}
            >
              {children}
            </pre>
          ),
        }}
      >
        {text}
      </ReactMarkdown>
    </div>
  );
}
