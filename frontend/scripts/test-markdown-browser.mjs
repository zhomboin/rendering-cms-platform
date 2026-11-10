import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { chromium } from 'playwright';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';

let server, browser, origin;
before(async () => {
  server = await createServer({
    root: fileURLToPath(new URL('../', import.meta.url)),
    logLevel: 'error',
    define: { 'import.meta.env.VITE_API_BASE': JSON.stringify('/api/v1') },
    server: { host: '127.0.0.1', port: 0 },
  });
  await server.listen();
  origin = `http://127.0.0.1:${server.httpServer.address().port}`;
  browser = await chromium.launch({ headless: true });
});
after(async () => {
  try {
    await browser?.close();
  } finally {
    await server?.close();
  }
});

const source = [
  '# 一级',
  '## **二级** `code`',
  '### 三级',
  '#### 四级',
  '##### 五级',
  '###### 六级',
  '## 重复',
  '## 重复',
  '| 名称 | 说明 |\n| --- | --- |\n| <init> | 构造<br />方法 |',
  '1. 有序列表\n   - 嵌套列表',
  '~~~java\n# 不进目录\n~~~',
].join('\n\n');

async function setup(t, route, body = source, viewport = { width: 1440, height: 1000 }) {
  const context = await browser.newContext({ viewport });
  t.after(() => context.close());
  await context.addInitScript(() =>
    localStorage.setItem('rendering-cms-token', 'markdown-fixture'),
  );
  await context.route('**/api/v1/**', (request) =>
    request.fulfill({
      json: [
        {
          articleId: 'markdown-fixture',
          title: '解析回归',
          articleName: 'markdown-fixture',
          bodyMdx: body,
          status: 'draft',
          tags: [],
          summary: '',
          slug: 'abc123',
        },
      ],
    }),
  );
  const page = await context.newPage();
  page.setDefaultTimeout(8000);
  const errors = [];
  page.on('pageerror', (e) => errors.push(e.message));
  t.after(() => assert.deepEqual(errors, []));
  await page.goto(origin + '/admin/articles/markdown-fixture/' + route);
  return page;
}

test('编辑预览使用同一规则并支持完整标题、表格和安全换行', async (t) => {
  const page = await setup(t, 'edit');
  const preview = page.locator('.mdx-preview__body');
  await preview.locator('h6').waitFor();
  assert.equal(await preview.locator('h1').textContent(), '一级');
  assert.equal(await preview.locator('h2').first().textContent(), '二级 code');
  assert.equal(await preview.locator('table').count(), 1);
  assert.equal(await preview.locator('td').first().textContent(), '<init>');
  assert.equal(await preview.locator('br').count(), 1);
  assert.equal(await preview.locator('ol').count(), 1);
});

test('阅读页目录与正文锚点逐项一致且重复标题可跳转', async (t) => {
  const page = await setup(t, 'read');
  await page.locator('.markdown-content h6').waitFor();
  const headings = await page
    .locator('.markdown-content :is(h1,h2,h3,h4,h5,h6)')
    .evaluateAll((els) => els.map((el) => ({ id: el.id, text: el.textContent })));
  const links = await page
    .locator('.article-reading__toc a')
    .evaluateAll((els) =>
      els.map((el) => ({ id: el.getAttribute('href').slice(1), text: el.textContent })),
    );
  assert.deepEqual(links, headings);
  assert.equal(new Set(headings.map((h) => h.id)).size, headings.length);
  await page.locator('.article-reading__toc a').last().click();
  await page.waitForFunction((id) => {
    const top = document.getElementById(id).getBoundingClientRect().top;
    return top >= 0 && top < window.innerHeight;
  }, headings.at(-1).id);
});

test('手机宽表格只在自身区域滚动', async (t) => {
  const table =
    '| 一 | 二 | 三 | 四 | 五 | 六 |\n| --- | --- | --- | --- | --- | --- |\n| a | b | c | d | e | f |';
  const page = await setup(t, 'read', table, { width: 390, height: 844 });
  await page.locator('.markdown-table table').waitFor();
  const size = await page.locator('.markdown-table').evaluate((el) => ({
    own: el.scrollWidth > el.clientWidth,
    document: document.documentElement.scrollWidth <= window.innerWidth,
  }));
  assert.deepEqual(size, { own: true, document: true });
});

test('错误预览不执行表达式，修正后不会保留旧错误', async (t) => {
  const page = await setup(t, 'edit', '{window.__markdownExecuted = true}');
  await page.getByText('正文解析失败', { exact: true }).waitFor();
  assert.equal(await page.evaluate(() => window.__markdownExecuted), undefined);
  await page.locator('[data-article-body-editor]').fill('#### 修正后的标题');
  await page.locator('.mdx-preview__body h4').waitFor();
  assert.equal(await page.getByText('正文解析失败', { exact: true }).count(), 0);
});
