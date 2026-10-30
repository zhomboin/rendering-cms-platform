import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { chromium } from 'playwright';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const albumId = 'c6b1f7b8-45d1-4c52-95d0-3cc4a599a640';
let server;
let browser;
let origin;

before(async () => {
  server = await createServer({
    root,
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

async function setup(t, { existing = false, failCreate = false } = {}) {
  const context = await browser.newContext();
  t.after(() => context.close());
  await context.addInitScript(() => {
    localStorage.setItem('rendering-cms-token', 'album-route-test');
  });
  let album = existing ? {
    albumId, slug: 'test-album', title: '已有相册', description: '测试相册说明',
    titleEn: null, descriptionEn: null, status: 'draft', tags: [], photoCount: 0,
    coverPhotoId: null, coverUrl: null, coverWidth: null, coverHeight: null,
    publishedAt: null, createdAt: '2026-10-30T00:00:00Z', updatedAt: '2026-10-30T00:00:00Z',
  } : null;
  const requests = [];
  const unexpected = [];
  await context.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    requests.push({ path, method, body: request.postDataJSON() });
    const reply = (json, status = 200) => route.fulfill({ status, json });
    if (path === '/api/v1/admin/albums' && method === 'GET') return reply(album ? [album] : []);
    if (path === '/api/v1/admin/albums' && method === 'POST') {
      if (failCreate) {
        failCreate = false;
        return reply({ error: '创建失败，请重试' }, 500);
      }
      album = { albumId, status: 'draft', photoCount: 0, tags: [], ...request.postDataJSON() };
      return reply(album, 201);
    }
    if (path === `/api/v1/admin/albums/${albumId}` && method === 'GET' && album) return reply(album);
    if (path === `/api/v1/admin/albums/${albumId}/photos` && method === 'GET' && album) return reply([]);
    if (path === `/api/v1/admin/albums/${albumId}` && method === 'PATCH' && album) {
      album = { ...album, ...request.postDataJSON() };
      return reply(album);
    }
    unexpected.push(`${method} ${path}`);
    return reply({ error: '不应发送的接口请求' }, 400);
  });
  const page = await context.newPage();
  const errors = [];
  page.setDefaultTimeout(10_000);
  page.on('pageerror', (error) => errors.push(error.message));
  t.after(() => {
    assert.deepEqual(unexpected, [], '不应发送空 ID 或其它错误路径');
    assert.deepEqual(errors, [], '页面不应发生未处理异常');
  });
  return { page, requests };
}

async function checkCreating(page) {
  await page.getByRole('heading', { name: '新建相册', exact: true }).waitFor();
  assert.equal(await page.getByRole('button', { name: '发布相册' }).count(), 0);
  assert.equal(await page.getByText('点击或拖拽照片到这里上传').count(), 0);
}

async function fillAlbum(page) {
  await page.getByLabel('slug（公开路由 /photos/{slug}）').fill('new-album');
  await page.getByLabel('标题（中文，必填）', { exact: true }).fill('新相册');
  await page.getByLabel('导语（中文，必填）', { exact: true }).fill('新相册说明');
}

test('从列表新建、保存并刷新编辑页，只使用真实相册 ID 查询', async (t) => {
  const { page, requests } = await setup(t);
  await page.goto(`${origin}/admin/albums`, { waitUntil: 'networkidle' });
  await page.getByRole('button', { name: '新建相册' }).click();
  await checkCreating(page);
  assert.equal(requests.filter((r) => r.path !== '/api/v1/admin/albums').length, 0);
  await fillAlbum(page);
  await page.getByRole('button', { name: '创建相册并继续' }).click();
  await page.waitForURL(`**/admin/albums/${albumId}/edit`);
  await page.getByRole('heading', { name: '相册信息 · 新相册' }).waitFor();
  const creates = requests.filter((r) => r.method === 'POST');
  assert.equal(creates.length, 1);
  assert.equal(creates[0].path, '/api/v1/admin/albums');
  assert.equal(creates[0].body.title, '新相册');
  assert.ok(requests.some((r) => r.path === `/api/v1/admin/albums/${albumId}/photos`));
  await page.reload({ waitUntil: 'networkidle' });
  await page.getByRole('heading', { name: '相册信息 · 新相册' }).waitFor();
});

test('直接打开新建页，创建失败保留表单并允许重试', async (t) => {
  const { page, requests } = await setup(t, { failCreate: true });
  await page.goto(`${origin}/admin/albums/new`, { waitUntil: 'networkidle' });
  await checkCreating(page);
  assert.deepEqual(requests, []);
  await fillAlbum(page);
  await page.getByRole('button', { name: '创建相册并继续' }).click();
  await page.getByText('创建失败，请重试', { exact: true }).waitFor();
  assert.equal(new URL(page.url()).pathname, '/admin/albums/new');
  assert.equal(await page.getByLabel('标题（中文，必填）', { exact: true }).inputValue(), '新相册');
  await page.getByRole('button', { name: '创建相册并继续' }).click();
  await page.waitForURL(`**/admin/albums/${albumId}/edit`);
  await page.getByRole('heading', { name: '相册信息 · 新相册' }).waitFor();
  assert.equal(requests.filter((r) => r.method === 'POST').length, 2);
});

test('已有相册加载和更新正常，返回列表后可以继续新建', async (t) => {
  const { page, requests } = await setup(t, { existing: true });
  await page.goto(`${origin}/admin/albums/${albumId}/edit`, { waitUntil: 'networkidle' });
  await page.getByRole('heading', { name: '相册信息 · 已有相册' }).waitFor();
  assert.ok(requests.some((r) => r.path === `/api/v1/admin/albums/${albumId}/photos`));
  await page.getByLabel('标题（中文，必填）', { exact: true }).fill('更新相册');
  await page.getByRole('button', { name: '保存信息' }).click();
  await page.getByRole('heading', { name: '相册信息 · 更新相册' }).waitFor();
  assert.equal(requests.filter((r) => r.method === 'PATCH').length, 1);
  await page.getByRole('button', { name: '返回列表' }).click();
  await page.getByRole('button', { name: '新建相册' }).click();
  await checkCreating(page);
  assert.equal(await page.getByLabel('标题（中文，必填）', { exact: true }).inputValue(), '');
});
