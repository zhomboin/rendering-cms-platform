import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { test } from 'node:test';
import { checkIndex } from './verify-article-index.mjs';

test('以 Pagefind 命中的博客地址判定收录，索引故障保留未知状态', async (t) => {
  const server = createServer((req, res) => {
    if (req.url !== '/pagefind/pagefind.js') {
      res.writeHead(404).end();
      return;
    }
    res.setHeader('Content-Type', 'text/javascript');
    res.end(`export async function search(title) {
      if (title === '异常') throw new Error('索引不可用');
      return { results: [{ data: async () => ({ url: title === '本站文章' ? '/blog/AbC123/' : 'https://other.invalid/blog/AbC123' }) }] };
    }`);
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  t.after(() => new Promise((resolve) => server.close(resolve)));
  const target = 'http://127.0.0.1:' + server.address().port + '/blog/AbC123';
  assert.equal(await checkIndex(target, '本站文章'), 'found');
  assert.equal(await checkIndex(target, '同名文章'), 'not_found');
  await assert.rejects(checkIndex(target, '异常'), /索引不可用/);
});
