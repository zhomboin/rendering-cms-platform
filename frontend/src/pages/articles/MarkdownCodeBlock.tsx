import { useEffect, useState } from 'react';
import type { HTMLAttributes, ReactElement } from 'react';

export function MarkdownCodeBlock({ children, ...props }: HTMLAttributes<HTMLPreElement>) {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 1500);
    return () => window.clearTimeout(timer);
  }, [copied]);
  const element = children as ReactElement<{ children?: string; className?: string }>;
  const code = String(element?.props?.children ?? '');
  const language = element?.props?.className?.replace(/^language-/, '') ?? '';
  async function copy() {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }
  return (
    <div className="markdown-code">
      <div className="markdown-code__toolbar">
        <span>{language}</span>
        <button type="button" aria-label="复制代码" onClick={copy}>
          {copied ? '已复制' : '复制'}
        </button>
      </div>
      <pre {...props}>{children}</pre>
    </div>
  );
}
