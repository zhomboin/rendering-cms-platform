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

async function setup(t, { existing = false, failCreate = false, failUpload = false, beforeUploadReply, withCover = false } = {}) {
  const context = await browser.newContext();
  t.after(() => context.close());
  await context.addInitScript(() => {
    localStorage.setItem('rendering-cms-token', 'album-route-test');
  });
  await context.route('https://images.example.test/**', route => route.fulfill({ contentType: 'image/png', body: png }));
  let album = existing ? {
    albumId, slug: 'test-album', title: '已有相册', description: '测试相册说明',
    titleEn: null, descriptionEn: null, status: 'draft', tags: [], photoCount: 0,
    coverPhotoId: null, coverUrl: withCover ? 'https://images.example.test/cover.png' : null, coverWidth: null, coverHeight: null,
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
      await beforeUploadReply?.(file);
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
  await page.getByLabel('访问标识').fill('new-album');
  await page.getByLabel('标题', { exact: true }).fill('新相册');
  await page.getByLabel('导语', { exact: true }).fill('新相册说明');
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
  await page.getByRole('heading', { name: '相册信息' }).waitFor();
  const creates = requests.filter((r) => r.method === 'POST');
  assert.equal(creates.length, 1);
  assert.equal(creates[0].path, '/api/v1/admin/albums');
  assert.equal(creates[0].body.title, '新相册');
  assert.equal(requests.some((r) => r.path === `/api/v1/admin/albums/${albumId}/photos`), false);
  await page.reload({ waitUntil: 'networkidle' });
  await page.getByRole('heading', { name: '相册信息' }).waitFor();
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
  assert.equal(await page.getByLabel('标题', { exact: true }).inputValue(), '新相册');
  await page.getByRole('button', { name: '创建相册并继续' }).click();
  await page.waitForURL(`**/admin/albums/${albumId}/edit`);
  await page.getByRole('heading', { name: '相册信息' }).waitFor();
  assert.equal(requests.filter((r) => r.method === 'POST').length, 2);
});

test('已有相册加载和更新正常，返回列表后可以继续新建', async (t) => {
  const { page, requests } = await setup(t, { existing: true });
  await page.goto(`${origin}/admin/albums/${albumId}/edit`, { waitUntil: 'networkidle' });
  await page.getByRole('heading', { name: '相册信息' }).waitFor();
  assert.equal(requests.some((r) => r.path === `/api/v1/admin/albums/${albumId}/photos`), false);
  await page.getByLabel('标题', { exact: true }).fill('更新相册');
  await page.getByRole('button', { name: '保存信息' }).click();
  await page.getByText('相册信息已保存', { exact: true }).waitFor();
  assert.equal(requests.filter((r) => r.method === 'PATCH').length, 1);
  await page.getByRole('button', { name: '返回列表' }).click();
  await page.getByRole('button', { name: '新建相册' }).click();
  await checkCreating(page);
  assert.equal(await page.getByLabel('标题', { exact: true }).inputValue(), '');
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
  await page.getByRole('list', { name: '上传进度', exact: true }).getByText('已上传', { exact: true }).waitFor();
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
  await page.getByRole('heading', { name: '图片管理', exact: true }).waitFor();
  assert.equal(await page.getByLabel('标题', { exact: true }).count(), 0);
  const url = page.getByRole('textbox', { name: '城市夜色的图片 URL', exact: true });
  assert.equal(await url.inputValue(), photos[0].url);
  await page.getByRole('button', { name: '复制城市夜色的图片 URL', exact: true }).click();
  assert.equal(await page.evaluate(() => navigator.clipboard.readText()), photos[0].url);
  await page.reload({ waitUntil: 'networkidle' });
  assert.equal(await url.inputValue(), photos[0].url);
  await page.getByRole('button', { name: '编辑基本信息', exact: true }).click();
  await page.getByRole('heading', { name: '相册信息', exact: true }).waitFor();
  assert.ok(requests.some(r => r.path.endsWith('/photos')));
  await page.getByRole('button', { name: '返回列表' }).click();
  await page.getByRole('button', { name: '图片管理', exact: true }).click();
  await page.getByRole('heading', { name: '图片管理', exact: true }).waitFor();
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
  await page.getByRole('list', { name: '上传进度', exact: true }).getByText('照片存储写入失败', { exact: true }).waitFor();
  await drop('drop.png', 'image/png', png);
  await page.getByRole('list', { name: '上传进度', exact: true }).getByText('已上传', { exact: true }).waitFor();
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
  await page.getByLabel('图片描述').fill('更新后的照片');
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


async function controlUploadProgress(page) {
  // 仅控制上传进度事件，文件请求和 multipart 内容仍经过真实 XHR。
  await page.addInitScript(() => {
    window.albumUploadRequests = [];
    const NativeXHR = window.XMLHttpRequest;
    window.XMLHttpRequest = class extends NativeXHR {
      constructor() {
        super();
        this.upload.addEventListener('progress', event => {
          if (event.isTrusted) event.stopImmediatePropagation();
        });
      }
      send(body) {
        if (body instanceof FormData && body.has('file')) {
          window.albumUploadRequests.push({ xhr: this, name: body.get('file').name });
        }
        return super.send(body);
      }
    };
  });
}

async function emitProgress(page, name, loaded, total) {
  await page.evaluate(({ name, loaded, total }) => {
    const request = window.albumUploadRequests.find(request => request.name === name);
    request.xhr.upload.dispatchEvent(new ProgressEvent('progress', {
      loaded, total: total ?? 0, lengthComputable: total !== undefined,
    }));
  }, { name, loaded, total });
}

test('逐文件进度来自传输事件，100% 等待处理，批量成功与失败独立', async (t) => {
  const pending = new Map();
  t.after(() => { for (const release of pending.values()) release(); });
  const { page, uploads } = await setup(t, { existing: true, failUpload: true,
    beforeUploadReply: file => new Promise(resolve => pending.set(file.name, resolve)),
  });
  await controlUploadProgress(page);
  await page.goto(origin + '/admin/albums/' + albumId + '/photos', { waitUntil: 'networkidle' });
  await page.locator('input[type=file]').setInputFiles(
    ['first.png', 'second.png'].map(name => ({ name, mimeType: 'image/png', buffer: png })),
  );
  await page.waitForFunction(() => window.albumUploadRequests.length === 2);
  const list = page.getByRole('list', { name: '上传进度', exact: true });
  const first = list.getByRole('listitem').filter({ hasText: 'first.png' });
  const second = list.getByRole('listitem').filter({ hasText: 'second.png' });
  await emitProgress(page, 'first.png', 25, 100);
  await first.getByText('25%', { exact: true }).waitFor();
  await emitProgress(page, 'second.png', 70, 100);
  await second.getByText('70%', { exact: true }).waitFor();
  await emitProgress(page, 'first.png', 100, 100);
  await first.getByText('处理中', { exact: true }).waitFor();
  assert.equal(await first.getByText('已上传', { exact: true }).count(), 0);
  await new Promise((resolve, reject) => {
    const deadline = Date.now() + 5000;
    const check = () => {
      if (pending.size === 2) return resolve();
      if (Date.now() >= deadline) return reject(new Error('未收到两项上传请求'));
      setTimeout(check, 10);
    };
    check();
  });
  pending.get('first.png')();
  await first.getByText('照片存储写入失败', { exact: true }).waitFor();
  await page.getByRole('button', { name: '清除记录', exact: true }).click();
  assert.equal(await first.count(), 0);
  await second.waitFor();
  assert.equal(await second.getByText('已上传', { exact: true }).count(), 0);
  pending.get('second.png')();
  await second.getByText('已上传', { exact: true }).waitFor();
  assert.equal(uploads.length, 2);
});

test('未知上传总量不伪造百分比，离开页面取消请求，重新进入无旧进度', async (t) => {
  let release;
  t.after(() => release?.());
  const { page } = await setup(t, { existing: true,
    beforeUploadReply: () => new Promise(resolve => { release = resolve; }),
  });
  await controlUploadProgress(page);
  await page.goto(origin + '/admin/albums/' + albumId + '/photos', { waitUntil: 'networkidle' });
  await page.locator('input[type=file]').setInputFiles({ name: 'unknown.png', mimeType: 'image/png', buffer: png });
  await page.waitForFunction(() => window.albumUploadRequests.length === 1);
  await page.evaluate(() => {
    window.albumUploadRequests[0].xhr.addEventListener('abort', () => { window.uploadAborted = true; });
  });
  await emitProgress(page, 'unknown.png', 10);
  const list = page.getByRole('list', { name: '上传进度', exact: true });
  await list.getByText('上传中', { exact: true }).waitFor();
  assert.doesNotMatch(await list.innerText(), /\d+%/);
  await page.getByRole('button', { name: '编辑基本信息', exact: true }).click();
  await page.waitForFunction(() => window.uploadAborted === true);
  release?.();
  await page.getByRole('button', { name: '管理图片', exact: true }).click();
  await page.getByRole('heading', { name: '图片管理', exact: true }).waitFor();
  assert.equal(await page.getByRole('list', { name: '上传进度', exact: true }).count(), 0);
});

test('照片键盘预览支持切换缩放与关闭，空链接不占预览位置', async (t) => {
  const { page, photos } = await setup(t, { existing: true });
  photos.push(
    { photoId: 'photo-0', alt: '无链接', width: 1, height: 1, url: null },
    { photoId: 'photo-1', alt: '第一张', width: 1, height: 1, url: 'https://images.example.test/first.png' },
    { photoId: 'photo-2', alt: '第二张', width: 1, height: 1, url: 'https://images.example.test/second.png' },
  );
  await page.goto(origin + '/admin/albums/' + albumId + '/photos', { waitUntil: 'networkidle' });
  const preview = page.getByRole('button', { name: '预览第一张', exact: true });
  await preview.focus();
  await page.keyboard.press('Enter');
  const viewer = page.locator('.ant-image-preview');
  await viewer.waitFor({ state: 'visible' });
  assert.match(await viewer.locator('img').getAttribute('src'), /first\.png$/);
  await page.locator('.ant-image-preview-switch-next').click();
  await page.waitForFunction(() => document.querySelector('.ant-image-preview img')?.src.endsWith('second.png'));
  const before = await viewer.locator('img').getAttribute('style');
  await page.locator('[aria-label="zoomIn"]').click();
  await page.waitForFunction(before => document.querySelector('.ant-image-preview img')?.getAttribute('style') !== before, before);
  await page.keyboard.press('Escape');
  await viewer.waitFor({ state: 'hidden' });
  assert.equal(new URL(page.url()).pathname, '/admin/albums/' + albumId + '/photos');
  assert.equal(await page.getByRole('button', { name: '预览无链接', exact: true }).count(), 0);
});

test('相册封面可放大，关闭后保持列表页', async (t) => {
  const { page } = await setup(t, { existing: true, withCover: true });
  await page.goto(origin + '/admin/albums', { waitUntil: 'networkidle' });
  await page.getByRole('button', { name: '预览已有相册封面', exact: true }).click();
  await page.locator('.ant-image-preview').waitFor({ state: 'visible' });
  await page.locator('.ant-image-preview-close').click();
  await page.locator('.ant-image-preview').waitFor({ state: 'hidden' });
  assert.equal(new URL(page.url()).pathname, '/admin/albums');
});

test('图片管理文案简洁，窄屏长文件名不溢出', async (t) => {
  const { page } = await setup(t, { existing: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(origin + '/admin/albums/' + albumId + '/photos', { waitUntil: 'networkidle' });
  await page.getByRole('heading', { name: '图片管理', exact: true }).waitFor();
  await page.getByRole('heading', { name: '照片', exact: true }).waitFor();
  assert.doesNotMatch(await page.locator('main').innerText(), /默认封面|已指定封面|剥离|EXIF|alt 默认|图片管理 ·/);
  await page.locator('input[type=file]').setInputFiles({
    name: '很长的图片文件名'.repeat(16) + '.png', mimeType: 'image/png', buffer: png,
  });
  await page.getByRole('list', { name: '上传进度', exact: true }).getByText('已上传', { exact: true }).waitFor();
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true);
});
