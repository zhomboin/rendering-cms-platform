import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { chromium } from 'playwright';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const articleId = 'c6b1f7b8-45d1-4c52-95d0-3cc4a599a640';
const attemptId = 'e40f9f69-38cc-4ddd-8ae7-24fdd4c9f941';
const time = '2030-01-01T00:00:00Z';
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

async function setup(
  t,
  {
    role = 'admin',
    failFirstPrepare = false,
    withCover = true,
    draftSaved = false,
    signingConfigured = true,
  } = {},
) {
  const context = await browser.newContext();
  t.after(() => context.close());
  await context.addInitScript((role) => {
    localStorage.setItem('rendering-cms-token', 'synthetic-token');
    localStorage.setItem(
      'rendering-cms-user',
      JSON.stringify({ userId: 'synthetic-user', name: '测试用户', role }),
    );
  }, role);
  await context.route('https://bridge.example.test/**', (route) =>
    route.fulfill({ body: 'PostBridge' }),
  );
  const calls = [];
  const unexpected = [];
  const article = {
    articleId,
    title: '已发布测试文章',
    articleName: 'synthetic-article',
    slug: 'synthetic-article',
    status: 'published',
    tags: [],
    coverImageUrl: withCover ? 'https://example.test/cover.png' : '',
    publishedAt: time,
    version: 7,
  };
  const target = {
    account_id: 'synthetic-target',
    display_name: '测试账号',
    platform: 'csdn',
    operations: ['save_draft'],
    body_formats: ['markdown'],
  };
  let attempt;
  const prefix = '/api/v1/admin';
  const attemptPath = prefix + '/postbridge/attempts/' + attemptId;
  await context.route('**/api/v1/**', (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname;
    const method = request.method();
    const body = request.postData() ? request.postDataJSON() : undefined;
    calls.push({ path, method, body, search: url.search });
    const reply = (json, status = 200) => route.fulfill({ status, json });
    if (path === prefix + '/articles' && method === 'GET')
      return reply([
        article,
        { ...article, articleId: 'draft-test', status: 'draft', title: '草稿文章' },
      ]);
    if (path === prefix + '/postbridge/connection') {
      return reply({ enabled: true, signingConfigured });
    }
    if (path === prefix + '/postbridge/targets') return reply({ items: [target] });
    if (path === prefix + '/postbridge/registrations' && method === 'POST') {
      return reply(
        { registrationId: 'registration-test', status: 'pending', recorded: false },
        201,
      );
    }
    if (path === prefix + '/postbridge/activate' && method === 'POST') {
      return reply({ status: 'active', targetsVerified: false });
    }
    if (
      path === prefix + '/articles/' + articleId + '/postbridge/preparations' &&
      method === 'POST'
    ) {
      if (failFirstPrepare) {
        failFirstPrepare = false;
        return reply({ error: '准备稿响应暂不可用' }, 502);
      }
      attempt = {
        id: attemptId,
        articleId,
        targetAccountId: target.account_id,
        sourceVersion: 7,
        status: 'prepared',
        revoked: false,
        createdAt: time,
        updatedAt: time,
        authorizationExpiresAt: time,
        canSubmit: false,
        canRefresh: true,
        canCancel: true,
        cancelPending: false,
        previewUrl: 'https://bridge.example.test/prepare/preview',
      };
      return reply(attempt, 201);
    }
    if (path === prefix + '/postbridge/attempts') {
      return reply({
        items: attempt ? [attempt] : [],
        page: 1,
        pageSize: 10,
        total: attempt ? 1 : 0,
      });
    }
    if (path === attemptPath && attempt) return reply(attempt);
    if (path === attemptPath + '/refresh' && attempt) {
      attempt = { ...attempt, status: 'confirmed', canSubmit: true };
      return reply(attempt);
    }
    if (path === attemptPath + '/submit' && attempt) {
      attempt = draftSaved
        ? {
            ...attempt,
            status: 'completed',
            externalStatus: 'draft_saved',
            canSubmit: false,
            canCancel: false,
            remoteUrl: 'https://csdn.example.test/draft',
          }
        : {
            ...attempt,
            status: 'needs_attention',
            externalStatus: 'unknown',
            canSubmit: false,
            detailUrl: 'https://bridge.example.test/jobs/example',
          };
      return reply(attempt);
    }
    if (path === attemptPath + '/cancel' && attempt) {
      attempt = { ...attempt, revoked: true, canCancel: false, cancelResult: 'not_cancellable' };
      return reply(attempt);
    }
    unexpected.push(method + ' ' + path);
    return reply({ error: '不应发送的接口请求' }, 400);
  });
  const page = await context.newPage();
  page.setDefaultTimeout(7000);
  const pageErrors = [];
  page.on('pageerror', (error) => pageErrors.push(error.message));
  t.after(() => {
    assert.deepEqual(unexpected, []);
    assert.deepEqual(pageErrors, []);
  });
  return { page, calls };
}

test('已发布文章须确认省略封面，重试保留幂等请求 ID', async (t) => {
  const { page, calls } = await setup(t, { failFirstPrepare: true });
  await page.goto(origin + '/admin/articles');
  await page.getByRole('link', { name: '保存到 CSDN 草稿箱' }).click();
  const form = page.getByRole('dialog', { name: '创建准备稿' });
  await form.getByRole('combobox', { name: '目标账号' }).click();
  await page.getByRole('option', { name: '测试账号' }).click();
  const submit = form.getByRole('button', { name: '创建准备稿' });
  assert.equal(await submit.isDisabled(), true);
  await form.getByRole('checkbox', { name: '省略封面' }).check();
  await submit.click();
  await form.getByText('准备稿响应暂不可用', { exact: true }).waitFor();
  await submit.click();
  const detail = page.getByRole('dialog', { name: '分发详情' });
  await detail.getByText('待确认', { exact: true }).waitFor();
  const preparations = calls.filter((call) => call.path.endsWith('/preparations'));
  assert.equal(preparations.length, 2);
  assert.match(preparations[0].body.requestId, /^[0-9a-f-]{36}$/i);
  assert.deepEqual(preparations[0].body, {
    requestId: preparations[0].body.requestId,
    expectedVersion: 7,
    targetAccountId: 'synthetic-target',
    textOnly: true,
  });
  assert.deepEqual(preparations[1].body, preparations[0].body);
  assert.equal(await detail.getByRole('button', { name: '提交到 CSDN 草稿箱' }).count(), 0);
  assert.equal(
    await detail.getByRole('link', { name: '打开 PostBridge 确认' }).getAttribute('href'),
    'https://bridge.example.test/prepare/preview',
  );
  await detail.getByRole('button', { name: '刷新状态' }).click();
  await detail.getByRole('button', { name: '提交到 CSDN 草稿箱' }).click();
  await detail.getByText('结果未知', { exact: true }).waitFor();
  assert.equal(await detail.getByRole('button', { name: '提交到 CSDN 草稿箱' }).count(), 0);
  assert.equal(
    await detail.getByRole('link', { name: '在 PostBridge 核对' }).getAttribute('href'),
    'https://bridge.example.test/jobs/example',
  );
  await detail.getByRole('button', { name: '撤销授权' }).click();
  await page.getByRole('button', { name: '确认撤销' }).click();
  await detail.getByText('授权已撤销', { exact: true }).waitFor();
  await detail.getByText('无法取消', { exact: true }).waitFor();
});

test('编辑者看不到分发入口，直接访问也不发 PostBridge 请求', async (t) => {
  const { page, calls } = await setup(t, { role: 'editor' });
  await page.goto(origin + '/admin/articles');
  await page.getByText('已发布测试文章', { exact: true }).waitFor();
  assert.equal(await page.getByRole('link', { name: '保存到 CSDN 草稿箱' }).count(), 0);
  assert.equal(await page.getByRole('menuitem', { name: 'PostBridge' }).count(), 0);
  await page.goto(origin + '/admin/postbridge');
  await page.waitForURL('**/admin/articles');
  assert.equal(
    calls.some((call) => call.path.includes('/postbridge')),
    false,
  );
});

test('连接注册后清除一次性邀请，激活与目标查询独立呈现', async (t) => {
  const { page, calls } = await setup(t);
  await page.goto(origin + '/admin/postbridge');
  await page.getByRole('tab', { name: '连接配置' }).click();
  await page.getByLabel('应用名称').fill('CMS 测试应用');
  await page.getByLabel('一次性邀请').fill('synthetic-invitation');
  await page.getByRole('button', { name: '注册应用' }).click();
  await page.getByText('registration-test', { exact: true }).waitFor();
  await page.getByText('注册记录未保存', { exact: true }).waitFor();
  assert.deepEqual(calls.find((call) => call.path.endsWith('/registrations')).body, {
    name: 'CMS 测试应用',
    invitationToken: 'synthetic-invitation',
  });
  assert.equal(await page.getByLabel('一次性邀请').inputValue(), '');
  const storage = await page.evaluate(() =>
    JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }),
  );
  assert.doesNotMatch(storage, /synthetic-invitation/);
  await page.getByRole('button', { name: '激活密钥' }).click();
  await page.getByText('密钥已激活', { exact: true }).waitFor();
  await page.getByText('目标待核对', { exact: true }).waitFor();
  await page.getByRole('button', { name: '查询目标' }).click();
  await page.getByText('测试账号', { exact: true }).waitFor();
});

test('窄屏下刷新分发详情，确认页返回后查询状态并显示草稿结果', async (t) => {
  const { page, calls } = await setup(t, { withCover: false, draftSaved: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(origin + '/admin/postbridge?articleId=' + articleId + '&prepare=1');
  const form = page.getByRole('dialog', { name: '创建准备稿' });
  await form.getByRole('combobox', { name: '目标账号' }).click();
  await page.getByRole('option', { name: '测试账号' }).click();
  assert.equal(await form.getByRole('checkbox', { name: '省略封面' }).count(), 0);
  await form.getByRole('button', { name: '创建准备稿' }).click();
  const detail = page.getByRole('dialog', { name: '分发详情' });
  await detail.getByText('待确认', { exact: true }).waitFor();
  await page.reload();
  await detail.getByText('待确认', { exact: true }).waitFor();
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
  const popup = page.waitForEvent('popup');
  await detail.getByRole('link', { name: '打开 PostBridge 确认' }).click();
  await (await popup).close();
  await page.bringToFront();
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  await detail.getByRole('button', { name: '提交到 CSDN 草稿箱' }).click();
  assert.equal(calls.filter((call) => call.path.endsWith('/refresh')).length, 1);
  await detail.getByText('已保存到 CSDN 草稿箱', { exact: true }).waitFor();
  assert.equal(
    await detail.getByRole('link', { name: '打开 CSDN 草稿' }).getAttribute('href'),
    'https://csdn.example.test/draft',
  );
  assert.equal(await detail.getByRole('button', { name: '提交到 CSDN 草稿箱' }).count(), 0);
});

test('未配置签名凭据时直接展示连接配置，不请求分发接口', async (t) => {
  const { page, calls } = await setup(t, { signingConfigured: false });
  await page.goto(origin + '/admin/postbridge');
  await page.getByRole('heading', { name: 'PostBridge' }).waitFor();
  await page.getByRole('button', { name: '注册应用' }).waitFor();
  assert.equal(await page.getByRole('button', { name: '激活密钥' }).isDisabled(), true);
  assert.equal(
    calls.some((call) => call.path.endsWith('/postbridge/attempts')),
    false,
  );
});
