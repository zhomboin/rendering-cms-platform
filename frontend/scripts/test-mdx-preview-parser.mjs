import assert from 'node:assert/strict';
import { test } from 'node:test';
import { parsePreviewBlocks } from '../.tmp/mdx-preview-parser/MdxPreviewParser.js';

test('parses markdown pipe tables into preview table blocks', () => {
  const blocks = parsePreviewBlocks(`
| table | a2 |
| --- | --- |
| b1 | b2 |
| c1 | c2 |
`);

  assert.deepEqual(blocks, [
    {
      type: 'table',
      headers: ['table', 'a2'],
      rows: [
        ['b1', 'b2'],
        ['c1', 'c2'],
      ],
    },
  ]);
});

test('keeps non-table pipe text as a paragraph', () => {
  const blocks = parsePreviewBlocks('这不是表格 | 只是普通文本');

  assert.deepEqual(blocks, [
    {
      type: 'paragraph',
      text: '这不是表格 | 只是普通文本',
    },
  ]);
});
