import type { ReactNode } from 'react';

export type MarkdownAction = {
  id: 'bold' | 'italic' | 'underline' | 'strike' | 'inlineCode' | 'codeBlock' | 'link' | 'image';
  icon: ReactNode;
  title: string;
  shortcut?: string;
  prefix?: string;
  suffix?: string;
  placeholder?: string;
};

export const markdownActions: MarkdownAction[] = [
  {
    id: 'bold',
    icon: <strong>B</strong>,
    title: '加粗',
    shortcut: 'Ctrl+B',
    prefix: '**',
    suffix: '**',
    placeholder: '加粗文本',
  },
  {
    id: 'italic',
    icon: <em>I</em>,
    title: '斜体',
    shortcut: 'Ctrl+I',
    prefix: '*',
    suffix: '*',
    placeholder: '斜体文本',
  },
  {
    id: 'underline',
    icon: <span style={{ textDecoration: 'underline' }}>U</span>,
    title: '下划线',
    shortcut: 'Ctrl+U',
    prefix: '<u>',
    suffix: '</u>',
    placeholder: '下划线文本',
  },
  {
    id: 'strike',
    icon: <span style={{ textDecoration: 'line-through' }}>S</span>,
    title: '删除线',
    prefix: '~~',
    suffix: '~~',
    placeholder: '删除线文本',
  },
  {
    id: 'inlineCode',
    icon: <CodeInlineIcon />,
    title: '行内代码',
    prefix: '`',
    suffix: '`',
    placeholder: 'code',
  },
  {
    id: 'codeBlock',
    icon: <CodeBlockIcon />,
    title: '代码块',
    prefix: '```ts\n',
    suffix: '\n```',
    placeholder: 'const example = true;',
  },
  {
    id: 'link',
    icon: <LinkIcon />,
    title: '添加链接',
    shortcut: 'Ctrl+K',
    prefix: '[',
    suffix: '](https://example.com)',
    placeholder: '链接文本',
  },
  { id: 'image', icon: <ImageIcon />, title: '插入图片' },
];

function CodeInlineIcon() {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      width="16"
      height="16"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
    >
      <path d="m8 9-4 3 4 3" />
      <path d="m16 9 4 3-4 3" />
    </svg>
  );
}

function CodeBlockIcon() {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      width="16"
      height="16"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
    >
      <path d="M4 5h16" />
      <path d="M4 19h16" />
      <path d="m9 9-3 3 3 3" />
      <path d="m15 9 3 3-3 3" />
    </svg>
  );
}

function LinkIcon() {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      width="16"
      height="16"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
    >
      <path d="M10 13a5 5 0 0 0 7.1 0l2-2a5 5 0 0 0-7.1-7.1l-1.1 1.1" />
      <path d="M14 11a5 5 0 0 0-7.1 0l-2 2A5 5 0 0 0 12 20.1l1.1-1.1" />
    </svg>
  );
}

function ImageIcon() {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      width="16"
      height="16"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
    >
      <rect x="3" y="5" width="18" height="14" rx="2" />
      <circle cx="8" cy="10" r="1.5" />
      <path d="m21 16-5-5L5 19" />
    </svg>
  );
}

export function FullscreenIcon() {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      width="16"
      height="16"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
    >
      <path d="M8 3H3v5" />
      <path d="M16 3h5v5" />
      <path d="M8 21H3v-5" />
      <path d="M16 21h5v-5" />
    </svg>
  );
}

export function ExitFullscreenIcon() {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      width="16"
      height="16"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
    >
      <path d="M4 14h6v6" />
      <path d="M20 14h-6v6" />
      <path d="M4 10h6V4" />
      <path d="M20 10h-6V4" />
    </svg>
  );
}
