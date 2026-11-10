import { fromMarkdown } from 'mdast-util-from-markdown';
import { gfm } from 'micromark-extension-gfm';
import { gfmFromMarkdown } from 'mdast-util-gfm';

export function markdownTree(source) {
  return fromMarkdown(source, { extensions: [gfm()], mdastExtensions: [gfmFromMarkdown()] });
}

export function headingText(node) {
  if (node.type === 'html') return '';
  if (typeof node.value === 'string') return node.value;
  if (node.type === 'image' || node.type === 'imageReference') return node.alt || '';
  return (node.children || []).map(headingText).join('');
}

export function slugifyHeading(value) {
  const text = String(value ?? '').trim();
  const normalized = text
    .toLowerCase()
    .replace(/[^\w\s-]/g, '')
    .trim()
    .replace(/\s+/g, '-');
  if (normalized) return normalized;
  const points = Array.from(text).map((character) => character.codePointAt(0).toString(16));
  return points.length ? `section-${points.join('-')}` : 'section';
}

export function collectHeadings(tree) {
  const headings = [];
  const used = new Set();
  function visit(node) {
    if (node.type === 'heading') {
      const heading = headingText(node).trim();
      const base = slugifyHeading(heading);
      let id = base;
      let suffix = 2;
      while (used.has(id)) id = `${base}-${suffix++}`;
      used.add(id);
      node.data = { ...node.data, hProperties: { ...node.data?.hProperties, id } };
      headings.push({ id, heading, level: node.depth });
    }
    for (const child of node.children || []) visit(child);
  }
  visit(tree);
  return headings;
}

export function remarkHeadingIds() {
  return (tree) => {
    collectHeadings(tree);
  };
}
