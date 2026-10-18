import { useCallback, useState } from 'react';
import type { ReactNode } from 'react';
import type { PreviewBlock } from './MdxPreviewParser';

export type MdxRenderVariant = 'preview' | 'reading';

type MdxBlockRendererProps = {
  blocks: PreviewBlock[];
  variant?: MdxRenderVariant;
  headingIdPrefix?: string;
};

export type HeadingAnchor = {
  id: string;
  text: string;
  level: 1 | 2 | 3;
};

export function headingAnchorId(prefix: string, index: number) {
  return `${prefix}heading-${index}`;
}

export function collectHeadingAnchors(blocks: PreviewBlock[], headingIdPrefix: string): HeadingAnchor[] {
  const headings: HeadingAnchor[] = [];
  blocks.forEach((block, index) => {
    if (block.type === 'heading') {
      headings.push({
        id: headingAnchorId(headingIdPrefix, index),
        text: block.text,
        level: block.level,
      });
    }
  });
  return headings;
}

const readingStyles = {
  paragraph: { margin: '0 0 18px', color: '#334155', fontSize: 16, lineHeight: 1.85 },
  listText: '#334155',
  quote: { margin: '20px 0 22px', padding: '14px 18px', borderLeft: '3px solid #7C3AED', background: '#F8FAFC', color: '#475569', fontSize: 15.5, lineHeight: 1.8 },
  code: { margin: '18px 0 22px', padding: 18, fontSize: 13.5, lineHeight: 1.75 },
  tableText: '#334155',
  tableFontSize: 14.5,
  headingFontSizes: { 1: 30, 2: 24, 3: 19 } as Record<1 | 2 | 3, number>,
  headingMargins: { 1: '44px 0 16px', 2: '36px 0 14px', 3: '28px 0 12px' } as Record<1 | 2 | 3, string>,
};

const previewStyles = {
  paragraph: { margin: '0 0 14px', color: '#334155', fontSize: 14, lineHeight: 1.8 },
  listText: '#334155',
  quote: { margin: '12px 0 16px', padding: '10px 14px', borderLeft: '3px solid #4F46E5', background: '#F8FAFC', color: '#475569' },
  code: { margin: '12px 0 16px', padding: 16, fontSize: 13, lineHeight: 1.7 },
  tableText: '#334155',
  tableFontSize: 14,
  headingFontSizes: { 1: 24, 2: 20, 3: 17 } as Record<1 | 2 | 3, number>,
  headingMargins: { 1: '20px 0 10px', 2: '20px 0 10px', 3: '20px 0 10px' } as Record<1 | 2 | 3, string>,
};

export function MdxBlockRenderer({ blocks, variant = 'preview', headingIdPrefix = 'mdx' }: MdxBlockRendererProps) {
  return (
    <>
      {blocks.map((block, index) => renderMdxBlock(block, index, variant, headingIdPrefix))}
    </>
  );
}

function renderMdxBlock(block: PreviewBlock, index: number, variant: MdxRenderVariant, headingIdPrefix: string): ReactNode {
  const styles = variant === 'reading' ? readingStyles : previewStyles;

  if (block.type === 'heading') {
    const fontSize = styles.headingFontSizes[block.level];
    return (
      <h2
        key={index}
        id={variant === 'reading' ? headingAnchorId(headingIdPrefix, index) : undefined}
        style={{
          margin: styles.headingMargins[block.level],
          fontSize,
          fontWeight: 700,
          color: '#0F172A',
          lineHeight: 1.4,
        }}
      >
        {renderInlineMarkdown(block.text)}
      </h2>
    );
  }

  if (block.type === 'list') {
    return (
      <ul
        key={index}
        style={{
          margin: variant === 'reading' ? '10px 0 20px' : '10px 0 16px',
          paddingLeft: 22,
          color: styles.listText,
          fontSize: variant === 'reading' ? 16 : 14,
          lineHeight: variant === 'reading' ? 1.9 : 1.8,
        }}
      >
        {block.items.map((item, itemIndex) => (
          <li key={`${index}-${itemIndex}`} style={{ marginBottom: variant === 'reading' ? 6 : 0 }}>
            {renderInlineMarkdown(item)}
          </li>
        ))}
      </ul>
    );
  }

  if (block.type === 'quote') {
    return (
      <blockquote key={index} style={styles.quote}>
        {renderInlineMarkdown(block.text)}
      </blockquote>
    );
  }

  if (block.type === 'table') {
    return (
      <div
        key={index}
        style={{
          margin: variant === 'reading' ? '16px 0 22px' : '12px 0 18px',
          overflowX: 'auto',
          border: '1px solid #CBD5E1',
          borderRadius: 8,
        }}
      >
        <table
          style={{
            width: '100%',
            minWidth: 420,
            borderCollapse: 'collapse',
            color: styles.tableText,
            fontSize: styles.tableFontSize,
          }}
        >
          <thead>
            <tr>
              {block.headers.map((header, headerIndex) => (
                <th
                  key={`${index}-header-${headerIndex}`}
                  style={{
                    padding: variant === 'reading' ? '12px 14px' : '10px 12px',
                    borderBottom: '1px solid #CBD5E1',
                    background: '#F8FAFC',
                    color: '#0F172A',
                    fontWeight: 700,
                    textAlign: 'left',
                    verticalAlign: 'top',
                  }}
                >
                  {renderInlineMarkdown(header)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {block.rows.map((row, rowIndex) => (
              <tr key={`${index}-row-${rowIndex}`}>
                {row.map((cell, cellIndex) => (
                  <td
                    key={`${index}-cell-${rowIndex}-${cellIndex}`}
                    style={{
                      padding: variant === 'reading' ? '12px 14px' : '10px 12px',
                      borderTop: rowIndex === 0 ? 0 : '1px solid #E2E8F0',
                      color: styles.tableText,
                      verticalAlign: 'top',
                      overflowWrap: 'anywhere',
                    }}
                  >
                    {renderInlineMarkdown(cell)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  }

  if (block.type === 'code') {
    return (
      <CodeBlock
        key={index}
        code={block.code}
        styles={styles.code}
        copyable={variant === 'reading'}
      />
    );
  }

  return (
    <p key={index} style={styles.paragraph}>
      {renderInlineMarkdown(block.text)}
    </p>
  );
}

function CodeBlock({
  code,
  styles,
  copyable,
}: {
  code: string;
  styles: { margin: string; padding: number; fontSize: number; lineHeight: number };
  copyable: boolean;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // 剪贴板权限受限时静默失败，用户仍可手动选中文本复制。
    }
  }, [code]);

  return (
    <div style={{ position: 'relative', margin: styles.margin }}>
      {copyable && (
        <button
          type="button"
          aria-label={copied ? '已复制' : '复制代码'}
          onClick={handleCopy}
          style={{
            position: 'absolute',
            top: 8,
            right: 8,
            display: 'inline-flex',
            alignItems: 'center',
            gap: 4,
            padding: '3px 8px',
            border: '1px solid #334155',
            borderRadius: 6,
            background: copied ? '#334155' : 'rgba(15, 23, 42, 0.65)',
            color: '#E2E8F0',
            fontSize: 12,
            lineHeight: 1.6,
            cursor: 'pointer',
          }}
        >
          {copied ? '已复制' : '复制'}
        </button>
      )}
      <pre
        style={{
          margin: 0,
          padding: styles.padding,
          paddingTop: copyable ? styles.padding + 4 : styles.padding,
          overflowX: 'auto',
          whiteSpace: 'pre-wrap',
          overflowWrap: 'anywhere',
          borderRadius: 8,
          background: '#0F172A',
          color: '#E2E8F0',
          fontSize: styles.fontSize,
          lineHeight: styles.lineHeight,
        }}
      >
        <code>{code}</code>
      </pre>
    </div>
  );
}

function renderInlineMarkdown(text: string): ReactNode[] {
  const pattern = /(!\[([^\]]*)\]\((https?:\/\/[^)\s]+|\/[^)\s]+)\)|\*\*([^*]+)\*\*|~~([^~]+)~~|<u>(.*?)<\/u>|`([^`]+)`|\[([^\]]+)\]\((https?:\/\/[^)\s]+|\/[^)\s]+|mailto:[^)\s]+)\)|\*([^*\n]+)\*)/g;
  const nodes: ReactNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(text))) {
    if (match.index > lastIndex) {
      nodes.push(text.slice(lastIndex, match.index));
    }

    const key = `${match.index}-${pattern.lastIndex}`;
    if (match[2] !== undefined && match[3]) {
      nodes.push(
        <img
          key={key}
          src={match[3]}
          alt={match[2]}
          style={{
            display: 'block',
            maxWidth: '100%',
            height: 'auto',
            margin: '12px 0',
            borderRadius: 8,
            border: '1px solid #E2E8F0',
          }}
        />,
      );
    } else if (match[4]) {
      nodes.push(<strong key={key}>{match[4]}</strong>);
    } else if (match[5]) {
      nodes.push(<del key={key}>{match[5]}</del>);
    } else if (match[6]) {
      nodes.push(<u key={key}>{match[6]}</u>);
    } else if (match[7]) {
      nodes.push(
        <code key={key} style={{ padding: '2px 5px', borderRadius: 4, background: '#F1F5F9', color: '#0F172A' }}>
          {match[7]}
        </code>,
      );
    } else if (match[8] && match[9]) {
      nodes.push(
        <a key={key} href={match[9]} target="_blank" rel="noreferrer" style={{ color: '#4F46E5' }}>
          {match[8]}
        </a>,
      );
    } else if (match[10]) {
      nodes.push(<em key={key}>{match[10]}</em>);
    }

    lastIndex = pattern.lastIndex;
  }

  if (lastIndex < text.length) {
    nodes.push(text.slice(lastIndex));
  }

  return nodes;
}
