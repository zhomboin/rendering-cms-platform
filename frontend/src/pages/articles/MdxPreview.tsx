import { Button, Empty, Tooltip, Typography } from 'antd';
import type { ReactNode } from 'react';

const { Text } = Typography;

type MdxPreviewProps = {
  source: string;
  fillHeight?: boolean;
  onEnterFullscreen?: () => void;
};

type PreviewBlock =
  | { type: 'heading'; level: 1 | 2 | 3; text: string }
  | { type: 'paragraph'; text: string }
  | { type: 'quote'; text: string }
  | { type: 'list'; items: string[] }
  | { type: 'code'; code: string };

export function MdxPreview({ source, fillHeight = false, onEnterFullscreen }: MdxPreviewProps) {
  const blocks = parsePreviewBlocks(source);

  return (
    <section className="mdx-preview" aria-label="MDX 预览">
      <div className="mdx-preview__header" style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', gap: 16, alignItems: 'flex-start' }}>
        <div>
          <Text strong style={{ display: 'block', color: '#0F172A', marginBottom: 4 }}>
            MDX 预览
          </Text>
          <Text style={{ fontSize: 12, color: '#64748B' }}>
            用于快速检查正文结构，复杂组件以源码形式展示。
          </Text>
        </div>
        {onEnterFullscreen && (
          <Tooltip title="全屏写作预览">
            <Button
              aria-label="全屏写作预览"
              icon={<FullscreenIcon />}
              onClick={onEnterFullscreen}
            />
          </Tooltip>
        )}
      </div>

      <div
        className={fillHeight ? 'mdx-preview__body mdx-preview__body--fill' : 'mdx-preview__body'}
        style={{
          minHeight: fillHeight ? 'calc(100vh - 145px)' : 420,
          maxHeight: fillHeight ? 'calc(100vh - 145px)' : 720,
          overflow: 'auto',
          padding: 24,
          border: '1px solid #E2E8F0',
          borderRadius: 8,
          background: '#FFFFFF',
        }}
      >
        {blocks.length === 0 ? (
          <Empty description="输入 MDX 正文后在这里预览" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        ) : (
          blocks.map((block, index) => renderBlock(block, index))
        )}
      </div>
    </section>
  );
}

function FullscreenIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M8 3H3v5" />
      <path d="M16 3h5v5" />
      <path d="M8 21H3v-5" />
      <path d="M16 21h5v-5" />
    </svg>
  );
}

function parsePreviewBlocks(source: string): PreviewBlock[] {
  const lines = source.replace(/\r\n/g, '\n').split('\n');
  const blocks: PreviewBlock[] = [];
  let paragraph: string[] = [];
  let listItems: string[] = [];
  let codeLines: string[] = [];
  let inCode = false;

  const flushParagraph = () => {
    if (paragraph.length === 0) return;
    blocks.push({ type: 'paragraph', text: paragraph.join(' ') });
    paragraph = [];
  };

  const flushList = () => {
    if (listItems.length === 0) return;
    blocks.push({ type: 'list', items: listItems });
    listItems = [];
  };

  for (const rawLine of lines) {
    const line = rawLine.trimEnd();
    const trimmed = line.trim();

    if (trimmed.startsWith('```')) {
      if (inCode) {
        blocks.push({ type: 'code', code: codeLines.join('\n') });
        codeLines = [];
        inCode = false;
      } else {
        flushParagraph();
        flushList();
        inCode = true;
      }
      continue;
    }

    if (inCode) {
      codeLines.push(line);
      continue;
    }

    if (!trimmed) {
      flushParagraph();
      flushList();
      continue;
    }

    const heading = /^(#{1,3})\s+(.+)$/.exec(trimmed);
    if (heading) {
      flushParagraph();
      flushList();
      blocks.push({
        type: 'heading',
        level: heading[1].length as 1 | 2 | 3,
        text: heading[2],
      });
      continue;
    }

    const listItem = /^[-*]\s+(.+)$/.exec(trimmed);
    if (listItem) {
      flushParagraph();
      listItems.push(listItem[1]);
      continue;
    }

    if (trimmed.startsWith('>')) {
      flushParagraph();
      flushList();
      blocks.push({ type: 'quote', text: trimmed.replace(/^>\s?/, '') });
      continue;
    }

    paragraph.push(trimmed);
  }

  flushParagraph();
  flushList();
  if (codeLines.length > 0) {
    blocks.push({ type: 'code', code: codeLines.join('\n') });
  }

  return blocks;
}

function renderBlock(block: PreviewBlock, index: number): ReactNode {
  if (block.type === 'heading') {
    const fontSize = block.level === 1 ? 24 : block.level === 2 ? 20 : 17;
    return (
      <h2 key={index} style={{ margin: '20px 0 10px', fontSize, color: '#0F172A', lineHeight: 1.35 }}>
        {renderInlineMarkdown(block.text)}
      </h2>
    );
  }

  if (block.type === 'list') {
    return (
      <ul key={index} style={{ margin: '10px 0 16px', paddingLeft: 22, color: '#334155', lineHeight: 1.8 }}>
        {block.items.map((item, itemIndex) => (
          <li key={`${index}-${itemIndex}`}>{renderInlineMarkdown(item)}</li>
        ))}
      </ul>
    );
  }

  if (block.type === 'quote') {
    return (
      <blockquote
        key={index}
        style={{
          margin: '12px 0 16px',
          padding: '10px 14px',
          borderLeft: '3px solid #4F46E5',
          background: '#F8FAFC',
          color: '#475569',
        }}
      >
        {renderInlineMarkdown(block.text)}
      </blockquote>
    );
  }

  if (block.type === 'code') {
    return (
      <pre
        key={index}
        style={{
          margin: '12px 0 16px',
          padding: 16,
          overflow: 'auto',
          borderRadius: 8,
          background: '#0F172A',
          color: '#E2E8F0',
          fontSize: 13,
          lineHeight: 1.7,
        }}
      >
        <code>{block.code}</code>
      </pre>
    );
  }

  return (
    <p key={index} style={{ margin: '0 0 14px', color: '#334155', fontSize: 14, lineHeight: 1.8 }}>
      {renderInlineMarkdown(block.text)}
    </p>
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
