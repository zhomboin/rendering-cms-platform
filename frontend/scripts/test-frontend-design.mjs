import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';
import { createServer } from 'vite';

const root = fileURLToPath(new URL('../', import.meta.url));
let server, browser, origin;

before(async () => {
  server = await createServer({
    root,
    logLevel: 'error',
    define: { 'import.meta.env.VITE_API_BASE': JSON.stringify('/api/v1') },
    server: { host: '127.0.0.1', port: 0 },
  });
  await server.listen();
  origin = 'http://127.0.0.1:' + server.httpServer.address().port;
  browser = await chromium.launch({ headless: true });
});

after(async () => {
  try {
    await browser?.close();
  } finally {
    await server?.close();
  }
});

async function newPage(t, authenticated = true) {
  const context = await browser.newContext({
    viewport: { width: 320, height: 740 },
    hasTouch: true,
    isMobile: true,
  });
  t.after(() => context.close());
  if (authenticated) {
    await context.addInitScript(() => {
      localStorage.setItem('rendering-cms-token', 'synthetic-token');
    });
  }
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  return page;
}

test('验证码输满后只自动提交一次，失败后可手动重试', async (t) => {
  const page = await newPage(t, false);
  const attempts = [];
  const email = 'operator@example.test';
  await page.route('**/api/v1/auth/login', (route) =>
    route.fulfill({
      json: { mfaRequired: true, email, challengeToken: 'test-challenge', expiresIn: 300 },
    }),
  );
  await page.route('**/api/v1/auth/mfa/verify', (route) => {
    attempts.push(route.request().postDataJSON());
    return attempts.length === 1
      ? route.fulfill({ status: 401, json: { error: '验证码错误或已过期' } })
      : route.fulfill({
          json: {
            token: 'synthetic-token',
            refreshToken: 'synthetic-refresh',
            user: { userId: 'test', email, name: '测试管理员', role: 'admin' },
          },
        });
  });
  await page.route('**/api/v1/admin/assets', (route) => route.fulfill({ json: [] }));

  await page.goto(origin + '/admin/login?redirect=%2Fadmin%2Fassets');
  assert.equal(await page.getByRole('checkbox', { name: '保持登录状态' }).count(), 0);
  await page.getByText('忘记密码？请联系系统管理员。').waitFor();
  await page.getByLabel('工作邮箱').fill(email);
  await page.getByLabel('密码', { exact: true }).fill('example-password');
  await page.getByRole('button', { name: /^登\s*录$/ }).click();

  const cells = page.getByLabel('邮箱验证码').getByRole('textbox');
  await cells.first().focus();
  await page.keyboard.type('01234');
  assert.equal(attempts.length, 0);
  await page.keyboard.type('5');
  await page.getByText('验证码错误或已过期').waitFor();
  assert.deepEqual(attempts, [{ challengeToken: 'test-challenge', code: '012345' }]);
  await page.getByRole('button', { name: '验证并登录' }).click();
  await page.waitForURL('**/admin/assets');
  assert.deepEqual(attempts[1], attempts[0]);
  assert.equal(attempts.length, 2);
});

test('评论按状态分页，批量审核部分成功后可重试失败项', async (t) => {
  const page = await newPage(t);
  const now = '2026-01-01T00:00:00Z';
  const comments = Array.from({ length: 12 }, (_, i) => ({
    commentId: String(i + 1),
    articleTitle: '测试文章',
    authorName: '访客 ' + (i + 1),
    body: '测试评论 ' + (i + 1),
    status: 'pending',
    createdAt: now,
  }));
  let rejectOnce = true;
  const reviewed = [];
  await page.route('**/api/v1/admin/comments', (route) => route.fulfill({ json: comments }));
  await page.route('**/api/v1/admin/comments/*', (route) => {
    const id = route.request().url().split('/').at(-1);
    const status = route.request().postDataJSON().status;
    reviewed.push([id, status]);
    if (id === '12' && rejectOnce) {
      rejectOnce = false;
      return route.fulfill({ status: 500, json: { error: '请求暂不可用' } });
    }
    const comment = comments.find((item) => item.commentId === id);
    Object.assign(comment, { status, reviewedAt: now });
    return route.fulfill({ json: { commentId: id, status, reviewedAt: now } });
  });

  await page.goto(origin + '/admin/comments');
  const rows = page.getByRole('listitem').filter({ hasText: /测试评论/ });
  await rows.last().waitFor();
  assert.equal(await rows.count(), 10);
  await page.getByTitle('2', { exact: true }).click();
  await page.getByText('测试评论 12', { exact: true }).waitFor();
  assert.equal(await rows.count(), 2);
  await page.getByRole('checkbox', { name: '全选本页' }).check();
  await page.getByText('已选 2 条').waitFor();
  const reject = page.getByRole('button', { name: '批量拒绝', exact: true });
  await reject.click();
  await page.getByRole('dialog').getByRole('button', { name: '批量拒绝', exact: true }).click();
  await page.getByText('1 条评论已拒绝，1 条失败。请求暂不可用').waitFor();
  assert.equal(await rows.count(), 1);
  assert.equal(await page.getByRole('checkbox', { name: '选择 访客 12 的评论' }).isChecked(), true);
  await reject.click();
  await page.getByRole('dialog').getByRole('button', { name: '批量拒绝', exact: true }).click();
  await page.getByText('1 条评论已拒绝', { exact: true }).waitFor();
  assert.deepEqual(reviewed, [
    ['11', 'rejected'],
    ['12', 'rejected'],
    ['12', 'rejected'],
  ]);
  await page.getByRole('tab', { name: '已拒绝' }).click();
  await page.getByText('测试评论 11', { exact: true }).waitFor();
  assert.equal(await rows.count(), 2);
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), 320);
});

test('资源大小按二进制显示，文件名和类型可组合筛选', async (t) => {
  const page = await newPage(t);
  const assets = [
    {
      assetId: 'doc',
      filename: 'Guide.pdf',
      contentType: 'application/pdf',
      byteSize: 20 * 1024 * 1024,
    },
    { assetId: 'photo', filename: 'Guide.png', contentType: 'image/png', byteSize: 1024 },
    { assetId: 'zip', filename: 'notes.zip', contentType: 'application/zip', byteSize: 1023 },
  ].map((asset) => ({ ...asset, createdAt: '2026-01-01T00:00:00Z' }));
  await page.route('**/api/v1/admin/assets', (route) => route.fulfill({ json: assets }));

  await page.goto(origin + '/admin/assets');
  await page.getByText('20.0 MiB', { exact: true }).waitFor();
  assert.equal(await page.getByText('1.0 KiB', { exact: true }).count(), 1);
  assert.equal(await page.getByText('1023 B', { exact: true }).count(), 1);
  await page.getByRole('textbox', { name: '搜索文件名' }).fill('  GUIDE  ');
  assert.equal(await page.getByText('notes.zip', { exact: true }).count(), 0);
  await page.getByRole('combobox', { name: '文件类型' }).click();
  await page.getByRole('option', { name: '图片', exact: true }).click();
  await page.getByText('Guide.png', { exact: true }).waitFor();
  assert.equal(await page.getByText('Guide.pdf', { exact: true }).count(), 0);
  await page.getByRole('textbox', { name: '搜索文件名' }).fill('无匹配');
  await page.getByText('没有匹配的文件', { exact: true }).waitFor();
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), 320);
});

test('管理员可在手机配置 MCP，编辑不能直接进入管理页', async (t) => {
  const page = await newPage(t);
  await page.addInitScript(() => {
    localStorage.setItem('rendering-cms-user', JSON.stringify({ userId: 'test', role: 'admin' }));
  });
  let settings = {
    revision: 0,
    enabled: false,
    origin: '',
    adminOrigin: '',
    blogOrigin: '',
    issuer: '',
    endpoint: '',
  };
  await page.route('**/api/v1/admin/mcp/settings', async (route) => {
    if (route.request().method() === 'PATCH') {
      const input = route.request().postDataJSON();
      assert.equal(input.revision, 0);
      settings = { ...settings, ...input, revision: 1, endpoint: input.origin + '/api/v1/mcp' };
      return route.fulfill({ status: 204 });
    }
    return route.fulfill({ json: settings });
  });
  await page.route('**/api/v1/admin/mcp/connections', (route) => route.fulfill({ json: [] }));
  await page.goto(origin + '/admin/mcp');
  await page.getByRole('button', { name: '服务设置' }).click();
  await page.getByLabel('CMS API 来源').fill('https://cms.example.invalid');
  await page.getByLabel('博客来源').fill('https://blog.example.invalid');
  await page.getByRole('switch').click();
  await page.getByRole('button', { name: '保存设置' }).click();
  await page.getByText('已开启', { exact: true }).waitFor();
  await page.getByText(settings.endpoint, { exact: true }).waitFor();
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), 320);

  const editor = await newPage(t);
  await editor.addInitScript(() =>
    localStorage.setItem(
      'rendering-cms-user',
      JSON.stringify({ userId: 'editor', role: 'editor' }),
    ),
  );
  await editor.route('**/api/v1/admin/articles**', (route) => route.fulfill({ json: [] }));
  await editor.goto(origin + '/admin/mcp');
  await editor.waitForURL('**/admin/articles');
});

test('MCP 连接逐项展示权限，创建与修改支持空授权', async (t) => {
  const page = await newPage(t);
  await page.addInitScript(() =>
    localStorage.setItem('rendering-cms-user', JSON.stringify({ userId: 'test', role: 'admin' })),
  );
  const endpoint = 'https://cms.example.invalid/api/v1/mcp';
  await page.route('**/api/v1/admin/mcp/settings', (route) =>
    route.fulfill({ json: { enabled: true, endpoint, issuer: '' } }),
  );
  const connections = [];
  await page.route('**/api/v1/admin/albums', (route) =>
    route.fulfill({
      json: [
        { albumId: 'gallery', title: '示例相册', status: 'published' },
        { albumId: 'draft', title: '未发布示例', status: 'draft' },
      ],
    }),
  );
  await page.route('**/api/v1/admin/mcp/connections', async (route) => {
    if (route.request().method() === 'POST') {
      const input = route.request().postDataJSON();
      assert.deepEqual(input.permissions, ['get_article', 'upload_album_photo']);
      assert.equal(input.readScope, 'all');
      assert.deepEqual(input.publishedUploadAlbumIds, ['gallery']);
      connections.push({
        id: 'writer',
        label: input.label,
        permissions: input.permissions,
        readScope: input.readScope,
        publishedUploadAlbumIds: input.publishedUploadAlbumIds,
        permissionRevision: 1,
        revoked: false,
      });
      return route.fulfill({
        status: 201,
        json: { id: 'writer', endpoint, token: 'synthetic-token' },
      });
    }
    return route.fulfill({ json: connections });
  });
  await page.route('**/api/v1/admin/mcp/connections/writer', async (route) => {
    assert.deepEqual(route.request().postDataJSON(), {
      permissions: [],
      publishedUploadAlbumIds: [],
      expectedRevision: 1,
      readScope: 'own',
    });
    connections[0].permissions = [];
    connections[0].publishedUploadAlbumIds = [];
    connections[0].readScope = 'own';
    connections[0].permissionRevision++;
    return route.fulfill({ status: 204 });
  });
  await page.goto(origin + '/admin/mcp');
  await page.getByRole('button', { name: '新建连接' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel('连接名称').fill('阅读助手');
  assert.equal(await dialog.getByRole('radio', { name: '仅本连接创建' }).isChecked(), true);
  await dialog.getByRole('radio', { name: '全部可读内容' }).check();
  assert.equal(await dialog.getByRole('checkbox', { name: '发布文章' }).isChecked(), false);
  for (const name of ['创建草稿', '修改草稿', '查询文章状态'])
    await dialog.getByRole('checkbox', { name }).uncheck();
  const photos = dialog.getByRole('group', { name: '照片', exact: true });
  assert.equal(await photos.getByRole('checkbox', { checked: true }).count(), 0);
  const album = dialog.getByRole('group', { name: '相册', exact: true });
  assert.equal(await album.getByRole('checkbox', { checked: true }).count(), 0);
  await photos.getByRole('checkbox', { name: '上传照片' }).check();
  await dialog.getByRole('combobox', { name: '可追加照片的已发布相册' }).click();
  await page.getByText('示例相册', { exact: true }).click();
  await page.keyboard.press('Escape');
  await dialog.getByRole('button', { name: '创建连接' }).click();
  await dialog.getByRole('button', { name: /完\s*成/ }).click();
  const row = page.getByRole('row').filter({ hasText: '阅读助手' });
  await row.getByText('读取文章', { exact: true }).waitFor();
  await row.getByText('上传照片', { exact: true }).waitFor();
  await row.getByText('可追加照片 · 1 个相册').waitFor();
  await row.getByText('全部可读内容', { exact: true }).waitFor();
  assert.equal(await row.getByText('发布文章', { exact: true }).count(), 0);
  await row.getByRole('button', { name: '配置权限' }).click();
  await dialog.getByRole('checkbox', { name: '读取文章' }).uncheck();
  await dialog.getByRole('checkbox', { name: '上传照片' }).uncheck();
  await dialog.getByRole('radio', { name: '仅本连接创建' }).check();
  await dialog.getByRole('button', { name: '保存权限' }).click();
  await row.getByText('无工具权限').waitFor();
  await row.getByText('仅本连接创建', { exact: true }).waitFor();
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), 320);
});

test('删除文章须重新确认变更的版本，失败可重试', async (t) => {
  const page = await newPage(t);
  const article = {
    articleId: 'delete-example',
    articleName: 'delete-example',
    title: '待删除的文章',
    slug: 'Abcd12',
    status: 'published',
    tags: [],
    version: 2,
  };
  await page.addInitScript(() => {
    localStorage.setItem('rendering-cms-user', JSON.stringify({ role: 'admin' }));
    localStorage.setItem('rendering-cms-article-editor-draft:delete-example', '本地草稿');
  });
  let deleted = false;
  const versions = [];
  await page.route('**/api/v1/admin/articles', (route) => route.fulfill({ json: deleted ? [] : [article] }));
  await page.route('**/api/v1/admin/articles/delete-example?*', (route) => {
    assert.equal(route.request().method(), 'DELETE');
    versions.push(new URL(route.request().url()).searchParams.get('expectedVersion'));
    if (versions.length === 1) {
      article.version++;
      article.title = '已修改的文章';
      return route.fulfill({ status: 409, json: { error: '版本冲突' } });
    }
    if (versions.length === 2) {
      return route.fulfill({ status: 503, json: { error: '暂时无法删除' } });
    }
    deleted = true;
    return route.fulfill({ status: 204 });
  });

  await page.goto(origin + '/admin/articles');
  const dialog = page.getByRole('dialog');
  async function openDelete() {
    await page.getByRole('button', { name: '更多操作：' + article.title }).click();
    await page.getByRole('menuitem', { name: '删除文章' }).click();
    await dialog.getByText('《' + article.title + '》', { exact: false }).waitFor();
  }
  const confirm = dialog.getByRole('button', { name: '删除文章' });
  await openDelete();
  await page.keyboard.press('Escape');
  await dialog.waitFor({ state: 'hidden' });
  await page.waitForFunction(
    () => document.activeElement?.getAttribute('aria-label') === '更多操作：待删除的文章',
  );
  assert.deepEqual(versions, []);
  await openDelete();
  await confirm.click();
  await page.getByText('文章已变更，请核对最新内容后再删除').waitFor();
  await dialog.waitFor({ state: 'hidden' });
  await openDelete();
  await confirm.click();
  await dialog.getByText('暂时无法删除').waitFor();
  await confirm.click();
  await page.getByText('文章已删除', { exact: true }).waitFor();
  await dialog.waitFor({ state: 'hidden' });
  await page.waitForFunction(() => document.activeElement?.textContent?.replace(/\s/g, '') === '刷新');
  assert.deepEqual(versions, ['2', '3', '3']);
  assert.equal(await page.getByRole('link', { name: article.title }).count(), 0);
  assert.equal(
    await page.evaluate(() => localStorage.getItem('rendering-cms-article-editor-draft:delete-example')),
    null,
  );
});

test('编辑角色的文章菜单隐藏删除入口', async (t) => {
  const page = await newPage(t);
  await page.addInitScript(() => {
    localStorage.setItem('rendering-cms-user', JSON.stringify({ role: 'editor' }));
  });
  await page.route('**/api/v1/admin/articles', (route) =>
    route.fulfill({
      json: [
        {
          articleId: 'sample',
          articleName: 'sample',
          title: '草稿',
          slug: 'Sample',
          status: 'draft',
          tags: [],
          version: 1,
        },
      ],
    }),
  );
  await page.goto(origin + '/admin/articles');
  await page.getByRole('button', { name: '更多操作：草稿' }).click();
  await page.getByRole('menuitem', { name: '设为私有' }).waitFor();
  assert.equal(await page.getByRole('menuitem', { name: '删除文章' }).count(), 0);
});
