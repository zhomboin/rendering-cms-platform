import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { compileArticle, parseArticleSource } from './engine.js';

async function render(source) {
  const article = await compileArticle(source);
  return { ...article, html: renderToStaticMarkup(createElement(article.Content)) };
}

test('同一解析契约包含相同实现摘要', async () => {
  const manifest = JSON.parse(await readFile(new URL('./contract.json', import.meta.url), 'utf8'));
  const lock = JSON.parse(
    await readFile(new URL('../../package-lock.json', import.meta.url), 'utf8'),
  );
  for (const [name, expected] of Object.entries(manifest.dependencies)) {
    assert.equal(lock.packages['node_modules/' + name].version, expected, name);
  }
  for (const [name, expected] of Object.entries(manifest.files)) {
    const actual = createHash('sha256')
      .update(await readFile(new URL(name, import.meta.url)))
      .digest('hex');
    assert.equal(actual, expected, name);
  }
});

test('表格中的安全换行与 JVM 名称可以直接渲染', async () => {
  const { html } = await render(
    '| 名称 | 说明 |\n| --- | --- |\n| <init> | 构造<br />方法 |\n| <clinit> | 静态<br>初始化 |',
  );
  assert.match(html, /<table>/);
  assert.match(html, /&lt;init&gt;/);
  assert.match(html, /&lt;clinit&gt;/);
  assert.equal((html.match(/<br\/>/g) || []).length, 2);
});

test('表格保留转义管道、对齐方式与行内格式', async () => {
  const { html } = await render('| **符号** | 说明 |\n| :--- | ---: |\n| `a\\|b` | ~~删除~~ |');
  assert.match(html, /<strong>符号<\/strong>/);
  assert.match(html, /a\|b/);
  assert.match(html, /text-align:right/);
  assert.match(html, /<del>删除<\/del>/);
});

test('目录覆盖六级、Setext、中文与含格式的标题', async () => {
  const source =
    '# 一级\n\n## **二级** `code`\n\n### 三级\n\n#### 四级\n\n##### 五级\n\n###### 六级\n\nSetext\n======';
  const { headings, html } = await render(source);
  assert.deepEqual(
    headings.map((h) => h.level),
    [1, 2, 3, 4, 5, 6, 1],
  );
  assert.equal(headings[1].heading, '二级 code');
  for (const heading of headings) assert.ok(html.includes(`id="${heading.id}"`), heading.id);
  assert.doesNotMatch(html, /object-object/);
});

test('重复标题及自身带编号的标题没有锚点冲突', async () => {
  const { headings, html } = await render('## Title\n\n## Title\n\n## Title-2\n\n## Title');
  assert.deepEqual(
    headings.map((h) => h.id),
    ['title', 'title-2', 'title-2-2', 'title-3'],
  );
  assert.equal(new Set(headings.map((h) => h.id)).size, 4);
  for (const heading of headings) assert.ok(html.includes(`id="${heading.id}"`));
});

test('围栏代码、行内代码与代码内的标题保持字面内容', async () => {
  const source = '~~~java\n# 假标题\n<init>\n{sample}\n~~~\n\n`<init>`\n\n    ## 缩进代码';
  const { headings, html, content } = await render(source);
  assert.deepEqual(headings, []);
  assert.match(content, /<init>/);
  assert.match(html, /language-java/);
  assert.match(html, /# 假标题/);
  assert.match(html, /\{sample\}/);
});

test('有序列表、嵌套列表、任务清单、引用和下划线保留结构', async () => {
  const { html } = await render(
    '1. 第一项\n   - 嵌套\n\n- [x] 完成\n\n> **引用**\n\n<u>下划线</u>',
  );
  assert.match(html, /<ol>/);
  assert.match(html, /<ul>/);
  assert.match(html, /type="checkbox"/);
  assert.match(html, /checked/);
  assert.match(html, /<blockquote>/);
  assert.match(html, /<u>下划线<\/u>/);
});

test('前置元数据不进入正文与目录，支持 CRLF', async () => {
  const parsed = parseArticleSource('---\r\ntitle: 标题\r\n---\r\n## 正文');
  assert.equal(parsed.headings.length, 1);
  assert.doesNotMatch(parsed.content, /title:/);
});

test('允许的图片组件与 Gallery 字面量仍然可用', async () => {
  const { Content } = await compileArticle(
    '<Figure src="/image.png" alt="图" />\n\n<Gallery items={[{src:"/one.png",alt:"一"}]} />',
  );
  const html = renderToStaticMarkup(
    createElement(Content, {
      components: {
        Figure: (props) => createElement('img', props),
        Gallery: ({ items }) =>
          createElement(
            'div',
            null,
            items.map((item) => createElement('img', { ...item, key: item.src })),
          ),
      },
    }),
  );
  assert.match(html, /src="\/image.png"/);
  assert.match(html, /src="\/one.png"/);
});

for (const source of [
  '{globalThis.__markdownExecuted = true}',
  'export const value = 1',
  '<script>bad</script>',
  '<br onClick="bad" />',
  '<u style="color:red">bad</u>',
  '<Figure {...globalThis.process} />',
  '<Gallery items={[{src: getImage()}]} />',
  '[危险](javascript:alert%281%29)',
  '[引用][x]\n\n[x]: javascript:alert%281%29',
  '![危险](data:image/svg+xml,test)',
]) {
  test(`拒绝不受支持的执行结构：${source.slice(0, 32)}`, async () => {
    await assert.rejects(compileArticle(source));
    assert.equal(globalThis.__markdownExecuted, undefined);
  });
}
