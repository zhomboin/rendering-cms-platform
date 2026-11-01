import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { chromium } from 'playwright';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
import { readFile, mkdir } from 'node:fs/promises';
import ts from 'typescript';

const root = fileURLToPath(new URL('../', import.meta.url));
let server, browser, origin;
before(async () => {
  server = await createServer({ root, logLevel: 'error', define: { 'import.meta.env.VITE_API_BASE': JSON.stringify('/api/v1') }, server: { host: '127.0.0.1', port: 0 } });
  await server.listen();
  origin = 'http://127.0.0.1:' + server.httpServer.address().port;
  browser = await chromium.launch({ headless: true });
});
after(async () => { try { await browser?.close(); } finally { await server?.close(); } });

const imageUrl = 'https://images.example.test/';
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a1FYAAAAASUVORK5CYII=', 'base64');
const asset = (id, extra = {}) => ({ assetId: id, filename: id + '.png', contentType: 'image/png', publicUrl: imageUrl + id + '.png', status: 'active', deletedAt: null, byteSize: 20, ...extra });
const photo = (id, extra = {}) => ({ photoId: id, alt: id, filename: id + '.png', contentType: 'image/png', url: imageUrl + id + '.png', width: 300, height: 200, byteSize: 20, ...extra });
const albums = ['a', 'b', 'empty'].map(id => ({ albumId: id, title: '相册' + id, slug: 'slug-' + id, status: 'draft', photoCount: 2 }));
async function setup(t, options = {}) {
  const context = await browser.newContext({ viewport: { width: 1280, height: 1000 } });
  t.after(() => context.close());
  await context.addInitScript(() => localStorage.setItem('rendering-cms-token', 'article-images-test'));
  const requests = [], unexpected = [], errors = [];
  const state = { assets: [asset('resource')], photos: { a: [photo('shared'), photo('album-only')], b: [photo('b-only')], empty: [] }, fail: [], delays: {}, ...options };
  await context.route('https://images.example.test/**', route => route.fulfill({ contentType: 'image/png', body: png }));
  await context.route('**/api/v1/**', async route => {
    const req = route.request(), path = new URL(req.url()).pathname.replace('/api/v1', '');
    requests.push({ path, method: req.method(), body: req.postDataJSON() });
    if (state.delays[path]) await new Promise(resolve => setTimeout(resolve, state.delays[path]));
    if (state.fail.includes(path)) return route.fulfill({ status: 500, json: { error: '图库暂时不可用' } });
    if (path === '/admin/articles' && req.method() === 'GET') return route.fulfill({ json: ['one', 'two'].map(id => ({ articleId: id, title: id, articleName: id, slug: id, bodyMdx: '前文替换后文', status: 'draft', tags: [], summary: '', coverImageUrl: '', featured: false, featuredRank: 100, featuredAt: null })) });
    if (path === '/admin/assets' && req.method() === 'GET') return route.fulfill({ json: state.assets });
    if (path === '/admin/albums' && req.method() === 'GET') return route.fulfill({ json: albums });
    const match = path.match(/^\/admin\/albums\/([^/]+)\/photos$/);
    if (match && req.method() === 'GET') return route.fulfill({ json: state.photos[match[1]] ?? [] });
    if (path === '/admin/assets/upload-url' && req.method() === 'POST') return route.fulfill({ json: { asset: asset('uploaded', state.uploadAsset ?? {}), uploadUrl: origin + '/upload-test', headers: {}, method: 'PUT' } });
    unexpected.push(req.method() + ' ' + path);
    return route.fulfill({ status: 400, json: { error: '意外请求' } });
  });
  await context.route('**/upload-test', route => route.fulfill({ status: 200 }));
  const page = await context.newPage();
  page.setDefaultTimeout(7000);
  page.on('pageerror', e => errors.push(e.message));
  t.after(() => { assert.deepEqual(unexpected, []); assert.deepEqual(errors, []); });
  await page.goto(origin + '/admin/articles/one/edit', { waitUntil: 'networkidle' });
  const editor = page.locator('[data-article-body-editor]');
  await editor.waitFor();
  return { page, editor, state, requests };
}
async function open(page) {
  await page.getByRole('button', { name: '插入图片', exact: true }).click();
  return page.getByRole('dialog', { name: '插入图片', exact: true });
}
async function chooseAlbum(dialog, title) {
  await dialog.getByRole('combobox', { name: '选择相册' }).click();
  await dialog.page().getByRole('option', { name: title, exact: true }).click();
}
async function waitText(editor, value) {
  await editor.page().waitForFunction(expected => document.querySelector('[data-article-body-editor]').value === expected, value);
}

test('图库正常多选在保存的选区插入并恢复光标，读取时没有写请求', async t => {
  const { page, editor, requests } = await setup(t, { assets: [asset('shared'), asset('resource')] });
  await editor.evaluate(el => { el.focus(); el.setSelectionRange(2, 4); });
  const dialog = await open(page);
  assert.equal(requests.some(r => r.path === '/admin/assets' || r.path === '/admin/albums'), false);
  await dialog.getByRole('tab', { name: '相册', exact: true }).click();
  await chooseAlbum(dialog, '相册a');
  await dialog.getByRole('checkbox', { name: '选择 shared', exact: true }).check();
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  await dialog.getByRole('checkbox', { name: '选择 shared.png', exact: true }).waitFor();
  assert.equal(await dialog.getByRole('checkbox', { name: '选择 shared.png', exact: true }).isChecked(), true);
  await dialog.getByRole('checkbox', { name: '选择 resource.png', exact: true }).check();
  await dialog.getByText('已选择 2 张', { exact: true }).waitFor();
  await dialog.getByRole('button', { name: '插入正文', exact: true }).click();
  const expected = '前文\n\n![shared](' + imageUrl + 'shared.png)\n\n![resource.png](' + imageUrl + 'resource.png)\n\n后文';
  await waitText(editor, expected);
  assert.deepEqual(await editor.evaluate(el => [el.selectionStart, el.selectionEnd, document.activeElement === el]), [expected.length - 2, expected.length - 2, true]);
  assert.equal(requests.every(r => r.method === 'GET'), true);
});

test('资源严格筛选公开地址和有效状态，搜索分页与预览独立于选择', async t => {
  const invalid = [
    asset('deleted', { deletedAt: '2026-01-01' }), asset('archived', { status: 'archived' }), asset('pending', { status: 'pending' }), asset('missing-status', { status: undefined }),
    asset('missing-deletedAt', { deletedAt: undefined }), asset('pdf', { contentType: 'application/pdf' }), asset('no-url', { publicUrl: null }),
    asset('signed', { publicUrl: imageUrl + 'signed.png?X-Amz-Signature=secret' }), asset('google', { publicUrl: imageUrl + 'g.png?GoogleAccessId=a&Signature=b&Expires=1' }),
    asset('credential', { publicUrl: 'https://user:pass@images.example.test/a.png' }), asset('js', { publicUrl: 'javascript:alert(1)' }),
  ];
  const { page } = await setup(t, { assets: [...invalid, ...Array.from({ length: 14 }, (_, i) => asset('有效-' + i))] });
  const dialog = await open(page);
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  await dialog.getByRole('checkbox', { name: '选择 有效-0.png', exact: true }).waitFor();
  assert.equal(await dialog.getByRole('checkbox').count(), 12);
  for (const item of invalid) assert.equal(await dialog.getByRole('checkbox', { name: '选择 ' + item.filename, exact: true }).count(), 0);
  await dialog.getByRole('button', { name: '预览 有效-0.png', exact: true }).click();
  await page.getByRole('dialog', { name: '图片预览', exact: true }).waitFor();
  assert.equal(await dialog.getByRole('checkbox', { name: '选择 有效-0.png', exact: true }).isChecked(), false);
  await page.getByRole('dialog', { name: '图片预览', exact: true }).getByRole('button', { name: 'Close', exact: true }).click();
  await dialog.getByRole('button', { name: '下一页', exact: true }).click();
  await dialog.getByRole('checkbox', { name: '选择 有效-12.png', exact: true }).waitFor();
  await dialog.getByRole('textbox', { name: '搜索图片', exact: true }).fill('有效-0');
  await dialog.getByRole('checkbox', { name: '选择 有效-0.png', exact: true }).waitFor();
  assert.equal(await dialog.getByRole('checkbox').count(), 1);
});

test('相册懒加载且快速切换不展示旧照片，照片地址也严格检查', async t => {
  const { page, requests } = await setup(t, { delays: { '/admin/albums/a/photos': 700 }, photos: { a: [photo('old')], b: [photo('b-only'), photo('invalid', { url: null }), photo('temporary', { url: imageUrl + 'tmp?X-Goog-Signature=secret' })] } });
  const dialog = await open(page);
  await dialog.getByRole('tab', { name: '相册', exact: true }).click();
  await chooseAlbum(dialog, '相册a');
  await chooseAlbum(dialog, '相册b');
  await dialog.getByRole('checkbox', { name: '选择 b-only', exact: true }).waitFor();
  await page.waitForTimeout(850);
  assert.equal(await dialog.getByRole('checkbox').count(), 1);
  assert.equal(requests.some(r => r.path.includes('/empty/photos')), false);
});

test('取消重开、跨文章不保留选择，空态和读取失败可以重试', async t => {
  const { page, state, requests } = await setup(t, { fail: ['/admin/assets'] });
  let dialog = await open(page);
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  await dialog.getByText('图库暂时不可用', { exact: true }).waitFor();
  state.fail = [];
  await dialog.getByRole('button', { name: '重试', exact: true }).click();
  await dialog.getByRole('checkbox', { name: '选择 resource.png', exact: true }).check();
  await dialog.getByRole('button', { name: '取消', exact: true }).click();
  dialog = await open(page);
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  assert.equal(await dialog.getByRole('checkbox', { name: '选择 resource.png', exact: true }).isChecked(), false);
  await dialog.getByRole('tab', { name: '相册', exact: true }).click();
  await chooseAlbum(dialog, '相册empty');
  await dialog.getByText('暂无可用图片', { exact: true }).waitFor();
  await dialog.getByRole('button', { name: '取消', exact: true }).click();
  await page.goto(origin + '/admin/articles/two/edit', { waitUntil: 'networkidle' });
  dialog = await open(page);
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  assert.equal(await dialog.getByRole('checkbox', { name: '选择 resource.png', exact: true }).isChecked(), false);
  assert.equal(requests.every(r => r.method === 'GET'), true);
});

test('手动 URL 和上传继续可用，390px 长名称不横向溢出', async t => {
  const long = '很长的图片名称WithoutBreak'.repeat(12);
  const { page, editor } = await setup(t, { assets: [asset('long', { filename: long })] });
  let dialog = await open(page);
  await dialog.getByRole('textbox', { name: '图片 URL', exact: true }).fill('/manual.png');
  await dialog.getByRole('textbox', { name: '替代文本', exact: true }).fill('手动图');
  await dialog.getByRole('button', { name: '插入正文', exact: true }).click();
  assert.equal((await editor.inputValue()).includes('![手动图](/manual.png)'), true);
  dialog = await open(page);
  await dialog.locator('input[type=file]').setInputFiles({ name: '本地图.png', mimeType: 'image/png', buffer: png });
  await page.waitForFunction(url => document.querySelector('input[id$="imageUrl"]').value === url, imageUrl + 'uploaded.png');
  await dialog.getByRole('button', { name: '插入正文', exact: true }).click();
  assert.equal((await editor.inputValue()).includes('![本地图](' + imageUrl + 'uploaded.png)'), true);
  await page.setViewportSize({ width: 390, height: 844 });
  dialog = await open(page);
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  await dialog.getByRole('checkbox', { name: '选择 ' + long, exact: true }).waitFor();
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
  assert.equal(await dialog.evaluate(el => el.scrollWidth <= el.clientWidth), true);
  if (process.env.ARTICLE_IMAGES_SCREENSHOT_DIR) {
    await mkdir(process.env.ARTICLE_IMAGES_SCREENSHOT_DIR, { recursive: true });
    await page.screenshot({ path: process.env.ARTICLE_IMAGES_SCREENSHOT_DIR + '/article-images-mobile.png', fullPage: true });
  }
});

test('Markdown 转义特殊字符且保留正文、替换选区和边界位置', async () => {
  const source = await readFile(root + 'src/pages/articles/articleImageUtils.ts', 'utf8');
  const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } }).outputText;
  const { insertArticleImages, isPermanentPublicImageUrl, assetToImage } = await import('data:text/javascript;base64,' + Buffer.from(compiled).toString('base64'));
  const result = insertArticleImages('前文替换后文', 2, 4, [{ url: 'https://example.test/a (b).png?q=x y', alt: '方[括]\\号\n换行' }]);
  assert.equal(result.value, '前文\n\n![方\\[括\\]\\\\号 换行](https://example.test/a%20%28b%29.png?q=x%20y)\n\n后文');
  assert.equal(result.cursor, result.value.length - 2);
  assert.equal(insertArticleImages('尾文', 0, 0, [{ url: '/a.png', alt: '' }]).value, '![](/a.png)\n\n尾文');
  assert.equal(insertArticleImages('前文\n\n', 4, 4, [{ url: '/a.png', alt: '图' }]).value, '前文\n\n![图](/a.png)');
  for (const url of ['data:image/png,a', 'file:///a', '//example.test/a', 'https://a:b@example.test/a', 'https://e.test/a?x-amz-signature=a', 'https://e.test/a?X-Goog-Credential=a', 'https://e.test/a?AWSAccessKeyId=a']) assert.equal(isPermanentPublicImageUrl(url), false, url);
  assert.equal(isPermanentPublicImageUrl('https://e.test/a?width=200'), true);
  assert.equal(assetToImage(asset('missing-deleted', { deletedAt: undefined })), null);
});

test('特殊字符替代文本插入后在真实预览中仍显示图片并还原描述', async t => {
  const { page } = await setup(t, { assets: [asset('special', { filename: '图[一]\\\\反斜线_星*{括}<号>' })] });
  const dialog = await open(page);
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  await dialog.getByRole('checkbox', { name: '选择 图[一]\\\\反斜线_星*{括}<号>', exact: true }).check();
  await dialog.getByRole('button', { name: '插入正文', exact: true }).click();
  await page.locator('.article-editor__preview').getByRole('img', { name: '图[一]\\\\反斜线_星*{括}<号>', exact: true }).waitFor();
});

test('跨相册多选保持、取消单张与清空选择生效，来源切换不会串图', async t => {
  const { page, editor, requests } = await setup(t, { delays: { '/admin/albums/a/photos': 500 } });
  const dialog = await open(page);
  await dialog.getByRole('tab', { name: '相册', exact: true }).click();
  await chooseAlbum(dialog, '相册a');
  await dialog.getByRole('checkbox', { name: '选择 shared', exact: true }).check();
  await chooseAlbum(dialog, '相册b');
  await dialog.getByRole('checkbox', { name: '选择 b-only', exact: true }).check();
  await chooseAlbum(dialog, '相册a');
  assert.equal(await dialog.getByRole('checkbox', { name: '选择 shared', exact: true }).isChecked(), true);
  await dialog.getByRole('checkbox', { name: '选择 shared', exact: true }).uncheck();
  await dialog.getByText('已选择 1 张', { exact: true }).waitFor();
  await dialog.getByRole('button', { name: '清空选择', exact: true }).click();
  assert.equal(await dialog.getByRole('button', { name: '插入正文', exact: true }).isDisabled(), true);
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  await dialog.getByRole('checkbox', { name: '选择 resource.png', exact: true }).check();
  assert.equal(await dialog.getByRole('checkbox', { name: '选择 shared', exact: true }).count(), 0);
  await dialog.getByRole('tab', { name: '上传 / 链接', exact: true }).click();
  const count = requests.length;
  await page.waitForTimeout(150);
  assert.equal(requests.length, count);
  await dialog.getByRole('button', { name: '取消', exact: true }).click();
  assert.equal(await editor.inputValue(), '前文替换后文');
});

test('相册列表和照片请求错误分别可重试，零资源及无匹配结果有空态', async t => {
  const { page, state } = await setup(t, { fail: ['/admin/albums'], assets: [] });
  const dialog = await open(page);
  await dialog.getByRole('tab', { name: '相册', exact: true }).click();
  await dialog.getByText('图库暂时不可用', { exact: true }).waitFor();
  state.fail = ['/admin/albums/a/photos'];
  await dialog.getByRole('button', { name: '重试', exact: true }).click();
  await chooseAlbum(dialog, '相册a');
  await dialog.getByText('图库暂时不可用', { exact: true }).waitFor();
  state.fail = [];
  await dialog.getByRole('button', { name: '重试', exact: true }).click();
  await dialog.getByRole('checkbox', { name: '选择 shared', exact: true }).waitFor();
  await dialog.getByRole('textbox', { name: '搜索图片', exact: true }).fill('没有这个名字');
  await dialog.getByText('没有匹配的图片', { exact: true }).waitFor();
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  await dialog.getByText('暂无可用图片', { exact: true }).waitFor();
});

test('正文首尾和中间光标插图保留前后文字，插图同步本地草稿', async t => {
  const { page, editor } = await setup(t);
  for (const position of [0, 2, 6]) {
    await editor.fill('前文替换后文');
    await editor.evaluate((el, pos) => { el.focus(); el.setSelectionRange(pos, pos); }, position);
    const dialog = await open(page);
    await dialog.getByRole('tab', { name: '资源', exact: true }).click();
    await dialog.getByRole('checkbox', { name: '选择 resource.png', exact: true }).check();
    await dialog.getByRole('button', { name: '插入正文', exact: true }).click();
    const before = '前文替换后文'.slice(0, position), after = '前文替换后文'.slice(position);
    const expected = before + (before ? '\n\n' : '') + '![resource.png](' + imageUrl + 'resource.png)' + (after ? '\n\n' : '') + after;
    await waitText(editor, expected);
    assert.equal(await editor.evaluate(el => el.selectionStart), expected.length - after.length);
    assert.equal(await page.evaluate(() => JSON.parse(localStorage.getItem('rendering-cms-article-editor-draft:one')).values.bodyMdx), expected);
  }
  await page.reload({ waitUntil: 'networkidle' });
  await page.getByText('已恢复本地未保存草稿', { exact: true }).waitFor();
  assert.equal((await editor.inputValue()).includes('![resource.png]'), true);
});

test('手动链接验证、上传失败和缺少公开地址均保留正文且允许重试', async t => {
  const { page, editor, state } = await setup(t, { fail: ['/admin/assets/upload-url'] });
  const dialog = await open(page);
  await dialog.getByRole('textbox', { name: '图片 URL', exact: true }).fill('javascript:alert(1)');
  await dialog.getByRole('button', { name: '插入正文', exact: true }).click();
  await dialog.getByText('图片 URL 必须以 http(s):// 或 / 开头', { exact: true }).waitFor();
  await dialog.getByRole('textbox', { name: '图片 URL', exact: true }).fill('');
  await dialog.locator('input[type=file]').setInputFiles({ name: '失败图.png', mimeType: 'image/png', buffer: png });
  await page.getByText('图库暂时不可用', { exact: true }).waitFor();
  assert.equal(await editor.inputValue(), '前文替换后文');
  state.fail = [];
  state.uploadAsset = { publicUrl: null };
  await dialog.locator('input[type=file]').setInputFiles({ name: '无公开地址.png', mimeType: 'image/png', buffer: png });
  await page.getByText('图片已上传，但当前存储未返回可公开访问的 URL', { exact: true }).waitFor();
  assert.equal(await dialog.getByRole('textbox', { name: '图片 URL', exact: true }).inputValue(), '');
  state.uploadAsset = {};
  await dialog.locator('input[type=file]').setInputFiles({ name: '成功图.png', mimeType: 'image/png', buffer: png });
  await page.waitForFunction(url => document.querySelector('input[id$="imageUrl"]').value === url, imageUrl + 'uploaded.png');
  await dialog.getByRole('button', { name: '插入正文', exact: true }).click();
  assert.equal((await editor.inputValue()).includes('![成功图]('), true);
});

test('弹窗打开时切换文章关闭旧选择，上传晚返回不会污染重开弹窗', async t => {
  const { page, state } = await setup(t, { delays: { '/admin/assets/upload-url': 600 } });
  let dialog = await open(page);
  await dialog.locator('input[type=file]').setInputFiles({ name: '旧上传.png', mimeType: 'image/png', buffer: png });
  await dialog.getByRole('button', { name: '取消', exact: true }).click();
  dialog = await open(page);
  await page.waitForTimeout(800);
  assert.equal(await dialog.getByRole('textbox', { name: '图片 URL', exact: true }).inputValue(), '');
  state.delays = {};
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  await dialog.getByRole('checkbox', { name: '选择 resource.png', exact: true }).check();
  await page.evaluate(() => { history.pushState({}, '', '/admin/articles/two/edit'); dispatchEvent(new PopStateEvent('popstate')); });
  await dialog.waitFor({ state: 'hidden' });
  dialog = await open(page);
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  assert.equal(await dialog.getByRole('checkbox', { name: '选择 resource.png', exact: true }).isChecked(), false);
});

test('现有 Markdown 工具与预览格式保持可用', async t => {
  const { page, editor } = await setup(t);
  await editor.fill('文本');
  await editor.evaluate(el => { el.focus(); el.setSelectionRange(0, 2); });
  await page.getByRole('button', { name: '加粗 Ctrl+B', exact: true }).click();
  await waitText(editor, '**文本**');
  await editor.fill('**加粗** ~~删除~~ <u>下划线</u> \x60代码\x60 [链接](https://example.test/) *斜体*\n\n![](https://images.example.test/empty-alt.png)');
  const preview = page.locator('.article-editor__preview');
  assert.equal(await preview.locator('strong').filter({ hasText: '加粗' }).textContent(), '加粗');
  assert.equal(await preview.locator('del').textContent(), '删除');
  assert.equal(await preview.locator('u').textContent(), '下划线');
  assert.equal(await preview.locator('code').textContent(), '代码');
  assert.equal(await preview.getByRole('link', { name: '链接', exact: true }).getAttribute('href'), 'https://example.test/');
  assert.equal(await preview.locator('em').textContent(), '斜体');
  assert.equal(await preview.locator('img').getAttribute('alt'), '');
});

test('上传后切回资源立即读到新图片，重新打开图库刷新资源状态', async t => {
  const { page, state, requests } = await setup(t);
  let dialog = await open(page);
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  await dialog.getByRole('checkbox', { name: '选择 resource.png', exact: true }).waitFor();
  await dialog.getByRole('tab', { name: '上传 / 链接', exact: true }).click();
  await dialog.locator('input[type=file]').setInputFiles({ name: '新图.png', mimeType: 'image/png', buffer: png });
  await page.waitForFunction(url => document.querySelector('input[id$="imageUrl"]').value === url, imageUrl + 'uploaded.png');
  state.assets.push(asset('uploaded'));
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  await dialog.getByRole('checkbox', { name: '选择 uploaded.png', exact: true }).waitFor();
  await dialog.getByRole('button', { name: '取消', exact: true }).click();
  state.assets = [asset('fresh')];
  dialog = await open(page);
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  await dialog.getByRole('checkbox', { name: '选择 fresh.png', exact: true }).waitFor();
  assert.equal(await dialog.getByRole('checkbox', { name: '选择 resource.png', exact: true }).count(), 0);
  assert.equal(requests.filter(r => r.path === '/admin/assets').length, 3);
});

test('缓存刷新期间隐藏旧候选，资源状态和相册照片以最新响应为准', async t => {
  const { page, state } = await setup(t, { assets: [asset('archived-later'), asset('deleted-later')] });
  let dialog = await open(page);
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  await dialog.getByRole('checkbox', { name: '选择 archived-later.png', exact: true }).waitFor();
  await dialog.getByRole('checkbox', { name: '选择 deleted-later.png', exact: true }).waitFor();
  await dialog.getByRole('tab', { name: '相册', exact: true }).click();
  await chooseAlbum(dialog, '相册a');
  await dialog.getByRole('checkbox', { name: '选择 shared', exact: true }).waitFor();
  await dialog.getByRole('button', { name: '取消', exact: true }).click();

  state.assets = [asset('archived-later', { status: 'archived' }), asset('deleted-later', { status: 'deleted', deletedAt: '2026-11-01T00:00:00Z' })];
  state.photos.a = [photo('shared', { url: null })];
  state.delays = { '/admin/assets': 700, '/admin/albums/a/photos': 700 };
  dialog = await open(page);
  const resourceRequest = page.waitForRequest('**/api/v1/admin/assets');
  await dialog.getByRole('tab', { name: '资源', exact: true }).click();
  await resourceRequest;
  assert.equal(await dialog.getByRole('checkbox').count(), 0, '新响应返回前不能选择缓存里的旧资源');
  await dialog.getByRole('status').getByText('正在加载图片', { exact: false }).waitFor();
  await dialog.getByText('暂无可用图片', { exact: true }).waitFor();

  await dialog.getByRole('tab', { name: '相册', exact: true }).click();
  const photoRequest = page.waitForRequest('**/api/v1/admin/albums/a/photos');
  await chooseAlbum(dialog, '相册a');
  await photoRequest;
  assert.equal(await dialog.getByRole('checkbox').count(), 0, '相册刷新完成前不能选择缓存里的旧照片');
  await dialog.getByRole('status').getByText('正在加载图片', { exact: false }).waitFor();
  await dialog.getByText('暂无可用图片', { exact: true }).waitFor();
});
