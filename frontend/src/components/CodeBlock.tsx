import { isValidElement, useRef, useState, type ReactNode } from 'react'
import { Check, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'

// CodeBlock wraps the `pre` element rendered by MarkdownContent. It adds a
// ChatGPT-style header bar above the code: a language label on the left and a
// copy-to-clipboard button on the right. The code itself keeps the same
// highlighting styles (rehype-highlight tokens) it had before.
function extractLanguage(children: ReactNode): string {
  const child = Array.isArray(children) ? children[0] : children
  if (isValidElement(child)) {
    const className = (child.props as { className?: string } | undefined)?.className ?? ''
    const match = /language-([\w-]+)/.exec(className)
    if (match) return match[1]
  }
  return 'text'
}

export function CodeBlock({ children }: { children?: ReactNode }) {
  const { t } = useTranslation()
  const preRef = useRef<HTMLPreElement>(null)
  const [copied, setCopied] = useState(false)
  const language = extractLanguage(children)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  const handleCopy = async () => {
    const text = preRef.current?.textContent ?? ''
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      if (timer.current) clearTimeout(timer.current)
      timer.current = setTimeout(() => setCopied(false), 1500)
    } catch {
      // Clipboard API may be unavailable in some sandbox contexts; fail silently.
    }
  }

  return (
    <div className="mt-5 max-w-full overflow-hidden rounded-lg border border-zinc-800">
      <div className="flex items-center justify-between border-b border-zinc-800 bg-zinc-900 px-3 py-1.5">
        <span className="text-[10px] text-zinc-500">{language}</span>
        <button
          type="button"
          onClick={handleCopy}
          aria-label={t('chat.copyMessage')}
          className={cn(
            'flex items-center gap-1 text-[10px] text-zinc-500 transition-colors hover:text-zinc-100',
            copied && 'text-zinc-300',
          )}
        >
          {copied ? <Check className="size-3" /> : <Copy className="size-3" />}
          <span>{t('chat.copyMessage')}</span>
        </button>
      </div>
      <pre
        ref={preRef}
        className="muted-scroll max-w-full overflow-x-auto bg-zinc-950 p-4 text-xs leading-6 text-zinc-300 [&_.hljs-attr]:text-sky-300 [&_.hljs-built_in]:text-cyan-300 [&_.hljs-comment]:text-zinc-500 [&_.hljs-keyword]:text-fuchsia-300 [&_.hljs-literal]:text-fuchsia-300 [&_.hljs-meta]:text-zinc-400 [&_.hljs-number]:text-amber-300 [&_.hljs-string]:text-emerald-300 [&_.hljs-title]:text-sky-300 [&_.hljs-type]:text-cyan-300 [&_.hljs-variable]:text-orange-300 [&>code]:block [&>code]:break-normal [&>code]:bg-transparent [&>code]:px-0 [&>code]:py-0 [&>code]:rounded-none"
      >
        {children}
      </pre>
    </div>
  )
}
