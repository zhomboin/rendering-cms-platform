import { evaluate } from '@mdx-js/mdx';
import remarkGfm from 'remark-gfm';
import * as runtime from 'react/jsx-runtime';
import { collectHeadings, markdownTree, remarkHeadingIds } from './headings.js';
import { remarkMdxSafety } from './safety.js';

export const MARKDOWN_CONTRACT = 'rendering-markdown-1';

// 在 Markdown 语法树定位 JVM 名称，避免改写代码块、行内代码和链接。
function protectJvmNames(source) {
  const edits = [];
  function visit(node) {
    if (node.type === 'html' && /^<br\s*\/?>$/.test(node.value)) {
      edits.push({
        start: node.position.start.offset,
        end: node.position.end.offset,
        value: '<br />',
      });
    } else if (node.type === 'html' && /^<\/?(?:init|clinit)>$/.test(node.value)) {
      edits.push({
        start: node.position.start.offset,
        end: node.position.end.offset,
        value: node.value.replace('<', '&lt;'),
      });
    }
    for (const child of node.children || []) visit(child);
  }
  visit(markdownTree(source));
  for (const edit of edits.sort((a, b) => b.start - a.start)) {
    source = source.slice(0, edit.start) + edit.value + source.slice(edit.end);
  }
  return source;
}

export function parseArticleSource(source = '') {
  const body = String(source).replace(/^\uFEFF?---\r?\n[\s\S]*?\r?\n---(?:\r?\n|$)/, '');
  const content = protectJvmNames(body);
  return { content, headings: collectHeadings(markdownTree(content)) };
}

export async function compileArticle(source) {
  const parsed = parseArticleSource(source);
  const evaluated = await evaluate(parsed.content, {
    ...runtime,
    remarkPlugins: [remarkGfm, remarkMdxSafety, remarkHeadingIds],
  });
  return { ...parsed, Content: evaluated.default };
}
