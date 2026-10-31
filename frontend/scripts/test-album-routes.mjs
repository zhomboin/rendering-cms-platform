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

async function setup(t, { existing = false, failCreate = false, failUpload = false } = {}) {
  const context = await browser.newContext();
  t.after(() => context.close());
  await context.addInitScript(() => {
    localStorage.setItem('rendering-cms-token', 'album-route-test');
  });
  await context.route('https://images.example.test/**', route => route.fulfill({ contentType: 'image/png', body: png }));
  let album = existing ? {
    albumId, slug: 'test-album', title: '已有相册', description: '测试相册说明',
    titleEn: null, descriptionEn: null, status: 'draft', tags: [], photoCount: 0,
    coverPhotoId: null, coverUrl: null, coverWidth: null, coverHeight: null,
    publishedAt: null, createdAt: '2026-10-30T00:00:00Z', updatedAt: '2026-10-30T00:00:00Z',
  } : null;
  const photos = [];
  const uploads = [];
  const requests = [];
  const unexpected = [];
  await context.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    const method = request.method();
    requests.push({ path, method, body: request.headers()['content-type']?.includes('application/json') ? request.postDataJSON() : null });
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
    if (path === `/api/v1/admin/albums/${albumId}/photos` && method === 'GET' && album) return reply(photos);
    if (path === `/api/v1/admin/albums/${albumId}/photos` && method === 'POST') {
      const contentType = request.headers()['content-type'] ?? '';
      if (!contentType.startsWith('multipart/form-data; boundary=')) {
        uploads.push({ contentType });
        return reply({ error: '缺少照片文件' }, 400);
      }
      const form = await new Request(request.url(), {
        method: 'POST', headers: { 'Content-Type': contentType }, body: request.postDataBuffer(),
      }).formData();
      const file = form.get('file');
      uploads.push({ contentType, file, alt: form.get('alt') });
      if (failUpload) { failUpload = false; return reply({ error: '照片存储写入失败' }, 500); }
      const photo = { photoId: `photo-${photos.length + 1}`, alt: form.get('alt'), caption: null,
        width: 1, height: 1, position: photos.length + 1, takenAt: null, createdAt: '2026-10-31T00:00:00Z',
        url: 'https://images.example.test/photos/test.png', filename: file.name, contentType: file.type, byteSize: file.size };
      photos.push(photo);
      return reply(photo, 201);
    }
    if (path === `/api/v1/admin/albums/${albumId}/photos/order` && method === 'PUT') {
      const ids = request.postDataJSON().photoIds;
      const sorted = ids.map((id, index) => ({ ...photos.find(p => p.photoId === id), position: index + 1 }));
      photos.splice(0, photos.length, ...sorted);
      return reply(photos);
    }
    if (path === `/api/v1/admin/albums/${albumId}/cover` && method === 'POST') {
      album = { ...album, coverPhotoId: request.postDataJSON().photoId };
      return reply(album);
    }
    const photo = photos.find(p => path === `/api/v1/admin/albums/${albumId}/photos/${p.photoId}`);
    if (photo && method === 'PATCH') { Object.assign(photo, request.postDataJSON()); return reply(photo); }
    if (photo && method === 'DELETE') { photos.splice(photos.indexOf(photo), 1); return route.fulfill({ status: 204 }); }
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
  return { page, requests, uploads, photos, context };
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
  assert.equal(requests.some((r) => r.path === `/api/v1/admin/albums/${albumId}/photos`), false);
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
  assert.equal(requests.some((r) => r.path === `/api/v1/admin/albums/${albumId}/photos`), false);
  await page.getByLabel('标题（中文，必填）', { exact: true }).fill('更新相册');
  await page.getByRole('button', { name: '保存信息' }).click();
  await page.getByRole('heading', { name: '相册信息 · 更新相册' }).waitFor();
  assert.equal(requests.filter((r) => r.method === 'PATCH').length, 1);
  await page.getByRole('button', { name: '返回列表' }).click();
  await page.getByRole('button', { name: '新建相册' }).click();
  await checkCreating(page);
  assert.equal(await page.getByLabel('标题（中文，必填）', { exact: true }).inputValue(), '');
});

const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a1FYAAAAASUVORK5CYII=', 'base64');

test('图片文件以真实 multipart 上传，保留文件内容和默认 alt', async (t) => {
  const { page, uploads } = await setup(t, { existing: true });
  await page.goto(`${origin}/admin/albums/${albumId}/photos`, { waitUntil: 'networkidle' });
  const uploaded = page.waitForResponse(r => r.request().method() === 'POST');
  await page.locator('input[type=file]').setInputFiles({ name: 'city-night.png', mimeType: 'image/png', buffer: png });
  await uploaded;
  assert.equal(uploads.length, 1);
  assert.match(uploads[0].contentType, /^multipart\/form-data; boundary=/);
  assert.equal(uploads[0].alt, 'city night');
  assert.deepEqual(Buffer.from(await uploads[0].file.arrayBuffer()), png);
  await page.getByText('照片已上传', { exact: true }).waitFor();
});

test('基本信息与图片管理独立，图片 URL 可见并可复制，支持直接刷新', async (t) => {
  const { page, context, photos, requests } = await setup(t, { existing: true });
  photos.push({ photoId: 'photo-1', alt: '城市夜色', caption: null, width: 1, height: 1,
    position: 1, takenAt: null, url: 'https://images.example.test/photos/city.png' });
  await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  await page.goto(`${origin}/admin/albums/${albumId}/edit`, { waitUntil: 'networkidle' });
  assert.equal(await page.locator('input[type=file]').count(), 0);
  await page.getByRole('button', { name: '管理图片', exact: true }).click();
  await page.waitForURL(`**/admin/albums/${albumId}/photos`);
  await page.getByRole('heading', { name: '图片管理 · 已有相册', exact: true }).waitFor();
  assert.equal(await page.getByLabel('标题（中文，必填）', { exact: true }).count(), 0);
  const url = page.getByRole('textbox', { name: '城市夜色的图片 URL', exact: true });
  assert.equal(await url.inputValue(), photos[0].url);
  await page.getByRole('button', { name: '复制城市夜色的图片 URL', exact: true }).click();
  assert.equal(await page.evaluate(() => navigator.clipboard.readText()), photos[0].url);
  await page.reload({ waitUntil: 'networkidle' });
  assert.equal(await url.inputValue(), photos[0].url);
  await page.getByRole('button', { name: '编辑基本信息', exact: true }).click();
  await page.getByRole('heading', { name: '相册信息 · 已有相册', exact: true }).waitFor();
  assert.ok(requests.some(r => r.path.endsWith('/photos')));
  await page.getByRole('button', { name: '返回列表' }).click();
  await page.getByRole('button', { name: '图片管理', exact: true }).click();
  await page.getByRole('heading', { name: '图片管理 · 已有相册', exact: true }).waitFor();
});

test('拖拽上传失败显示后端原因，再次拖入可成功，非法类型不发请求', async (t) => {
  const { page, uploads } = await setup(t, { existing: true, failUpload: true });
  await page.goto(`${origin}/admin/albums/${albumId}/photos`, { waitUntil: 'networkidle' });
  const drop = async (name, type, data) => {
    const transfer = await page.evaluateHandle(({ name, type, data }) => {
      const dt = new DataTransfer(); dt.items.add(new File([new Uint8Array(data)], name, { type })); return dt;
    }, { name, type, data: Array.from(data) });
    await page.locator('.ant-upload-btn').dispatchEvent('drop', { dataTransfer: transfer });
    await transfer.dispose();
  };
  await page.locator('input[type=file]').setInputFiles({ name: 'bad.txt', mimeType: 'text/plain', buffer: Buffer.from('bad') });
  await page.getByText('bad.txt 不是 PNG/JPEG/WebP 图片', { exact: true }).waitFor();
  assert.equal(uploads.length, 0);
  await drop('drop.png', 'image/png', png);
  await page.getByText('照片存储写入失败', { exact: true }).waitFor();
  await drop('drop.png', 'image/png', png);
  await page.getByText('照片已上传', { exact: true }).waitFor();
  assert.equal(uploads.length, 2);
  await page.getByRole('textbox', { name: 'drop的图片 URL', exact: true }).waitFor();
});

test('无 URL 说明原因，复制被拒绝时提示手动复制', async (t) => {
  const { page, photos } = await setup(t, { existing: true });
  photos.push({ photoId: 'photo-1', alt: '无链接图片', width: 1, height: 1, url: null },
    { photoId: 'photo-2', alt: '有链接图片', width: 1, height: 1, url: 'https://images.example.test/test.png' });
  await page.addInitScript(() => Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: () => Promise.reject(new Error('denied')) }, configurable: true,
  }));
  await page.goto(`${origin}/admin/albums/${albumId}/photos`, { waitUntil: 'networkidle' });
  await page.getByText('暂无图片 URL，请检查公开访问地址配置', { exact: true }).waitFor();
  assert.equal(await page.getByRole('button', { name: '复制无链接图片的图片 URL', exact: true }).count(), 0);
  await page.getByRole('button', { name: '复制有链接图片的图片 URL', exact: true }).click();
  await page.getByText('复制失败，请选中图片 URL 手动复制', { exact: true }).waitFor();
});

test('图片管理保留编辑、排序、封面与删除操作', async (t) => {
  const { page, photos, requests } = await setup(t, { existing: true });
  photos.push(...['第一张', '第二张'].map((alt, index) => ({ photoId: `photo-${index + 1}`,
    alt, width: 1, height: 1, position: index + 1, url: null })));
  await page.goto(`${origin}/admin/albums/${albumId}/photos`, { waitUntil: 'networkidle' });
  await page.getByRole('button', { name: '编辑照片', exact: true }).first().click();
  await page.getByLabel('alt（无障碍必填）').fill('更新后的照片');
  await page.getByRole('button', { name: '保存照片', exact: true }).click();
  await page.getByText('照片信息已更新', { exact: true }).waitFor();
  assert.equal(photos[0].alt, '更新后的照片');
  const reordered = page.waitForResponse(r => r.request().method() === 'PUT' && r.url().endsWith('/order'));
  await page.getByRole('button', { name: '下移照片', exact: true }).first().click();
  await reordered;
  assert.equal(photos[0].photoId, 'photo-2');
  await page.locator('.ant-card-meta-title').first().getByText('第二张', { exact: true }).waitFor();
  await page.getByRole('button', { name: '设置照片封面', exact: true }).nth(1).click();
  await page.getByText('封面已更新', { exact: true }).waitFor();
  assert.equal(requests.find(r => r.path.endsWith('/cover')).body.photoId, 'photo-1');
  await page.getByRole('button', { name: '删除照片', exact: true }).first().click();
  await page.getByRole('button', { name: /^删\s*除$/ }).click();
  await page.getByText('照片已删除', { exact: true }).waitFor();
  assert.equal(photos.length, 1);
});

test('超出 20MB 的图片在本地拒绝，不发上传请求', async (t) => {
  const { page, uploads } = await setup(t, { existing: true });
  await page.goto(`${origin}/admin/albums/${albumId}/photos`, { waitUntil: 'networkidle' });
  await page.locator('input[type=file]').setInputFiles({ name: 'large.png', mimeType: 'image/png', buffer: Buffer.alloc(20 * 1024 * 1024 + 1) });
  await page.getByText('large.png 超过 20MB 上限', { exact: true }).waitFor();
  assert.equal(uploads.length, 0);
});
