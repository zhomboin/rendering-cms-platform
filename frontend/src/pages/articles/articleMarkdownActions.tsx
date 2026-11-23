import type { ReactNode } from 'react';
import { CodeInlineIcon, CodeBlockIcon, LinkIcon, ImageIcon } from './ArticleEditorIcons';

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
