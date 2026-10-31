import assert from 'node:assert/strict';
import { after, before, test } from 'node:test';
import { chromium } from 'playwright';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
import { mkdir } from 'node:fs/promises';

const root = fileURLToPath(new URL('../', import.meta.url));
let server, browser, origin;
before(async () => {
  server = await createServer({ root, logLevel: 'error', define: { 'import.meta.env.VITE_API_BASE': JSON.stringify('/api/v1') }, server: { host: '127.0.0.1', port: 0 } });
  await server.listen();
  origin = 'http://127.0.0.1:' + server.httpServer.address().port;
  browser = await chromium.launch({ headless: true });
});
after(async () => { try { await browser?.close(); } finally { await server?.close(); } });

const article = { articleId: 'article-1', slug: 'real-article', title: '从真实内容理解系统设计', status: 'published', coverImageUrl: 'https://images.example.test/cover.png', tags: ['架构', '实践'], updatedAt: '2026-10-31T08:30:00Z', publishedAt: '2026-10-30T00:00:00Z' };
const draft = { ...article, articleId: 'draft-1', title: '尚未完成的草稿', status: 'draft', coverImageUrl: '' };
const privateArticle = { ...article, articleId: 'private-1', title: '仅后台可见的记录', status: 'private' };
const metrics = { todayViews: 17, periodViews: 132, articleViews: 89, albumViews: 12, newComments: 8, pendingComments: 3 };
function data(days, empty) {
  return {
    overview: { days, generatedAt: '2026-10-31T10:00:00Z', startDate: '2026-10-02', endDate: '2026-10-31', metrics: empty ? Object.fromEntries(Object.keys(metrics).map(k => [k, 0])) : metrics,
      content: { publishedArticles: empty ? 0 : 12, draftArticles: empty ? 0 : 2, privateArticles: empty ? 0 : 1, archivedArticles: 0, publishedAlbums: empty ? 0 : 4, draftAlbums: 0, photoCount: empty ? 0 : 29, missingCoverArticles: empty ? 0 : 2 },
      resources: { assetCount: empty ? 0 : 11, imageCount: empty ? 0 : 8, totalBytes: empty ? 0 : 10485760 },
      recentArticles: empty ? [] : [article, privateArticle], drafts: empty ? [] : [draft],
      pendingComments: empty ? [] : [{ commentId: 'comment-1', articleId: 'article-1', articleTitle: article.title, authorName: '读者', body: '很有帮助的一篇文章', createdAt: article.updatedAt }],
      recentImages: empty ? [] : [{ assetId: 'asset-1', filename: '山间光影.png', publicUrl: article.coverImageUrl, byteSize: 1024, createdAt: article.updatedAt }] },
    articles: { days, articles: empty ? [] : [article.title, 'PostgreSQL 查询优化实践', '为个人博客建立内容工作流', '可靠的对象存储与备份', '构建更清晰的 API 边界'].map((title, i) => ({ ...article, articleId: 'article-' + (i + 1), title, periodViews: 89 - i * 15, totalViews: 320 - i * 50 })) },
    albums: { days, albums: empty ? [] : ['城市漫游', '山间光影', '秋日片段', '旅途札记'].map((title, i) => ({ albumId: 'album-' + (i + 1), slug: 'walk-' + i, title, status: i === 3 ? 'draft' : 'published', coverUrl: article.coverImageUrl, photoCount: 29 - i * 6, periodViews: 12 - i * 3, updatedAt: article.updatedAt })) },
    trend: { days, site: Array.from({ length: days }, (_, i) => { const day = new Date('2026-10-31T00:00:00Z'); day.setUTCDate(day.getUTCDate() - days + i + 1); const date = day.toISOString().slice(0, 10); return { date, views: empty ? 0 : date === '2026-10-29' ? 10 : date === '2026-10-31' ? 17 : 0 }; }), articles: empty ? [] : [{ date: '2026-10-29', slug: article.slug, title: article.title, views: 4 }, { date: '2026-10-31', slug: article.slug, title: article.title, views: 7 }, { date: '2026-11-15', slug: article.slug, title: article.title, views: 99 }] },
  };
}
async function setup(t, options = {}) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1100 } });
  t.after(() => context.close());
  await context.addInitScript(() => localStorage.setItem('rendering-cms-token', 'dashboard-test'));
  const requests = [], unexpected = [], errors = [];
  const state = { empty: false, failed: [], delay: 0, ...options };
  await context.route('https://images.example.test/**', route => state.brokenImages ? route.abort() : route.fulfill({ contentType: 'image/png', body: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a1FYAAAAASUVORK5CYII=', 'base64') }));
  await context.route('**/api/v1/**', async route => {
    const url = new URL(route.request().url());
    const key = url.pathname === '/api/v1/admin/analytics/trend' ? 'trend' : url.pathname.replace('/api/v1/admin/dashboard/', '');
    requests.push({ key, days: url.searchParams.get('days') });
    if (!['overview', 'articles', 'albums', 'trend'].includes(key)) {
      unexpected.push(url.pathname); return route.fulfill({ status: 400, json: { error: '意外请求' } });
    }
    if (state.delay) await new Promise(resolve => setTimeout(resolve, state.delay));
    if (state.failed.includes(key)) return route.fulfill({ status: 500, json: { error: '暂时无法连接数据服务' } });
    const result = data(Number(url.searchParams.get('days')), state.empty);
    if (state.largeViews) result.trend.site[result.trend.site.length - 1].views = 99_999_999;
    if (state.longTitle) result.articles.articles.forEach(a => { a.title = 'LongTitleWithoutBreak'.repeat(30); a.tags = ['LongTagWithoutBreak'.repeat(8)]; });
    return route.fulfill({ json: result[key] });
  });
  const page = await context.newPage();
  page.setDefaultTimeout(10000);
  page.on('pageerror', e => errors.push(e.message));
  t.after(() => { assert.deepEqual(unexpected, []); assert.deepEqual(errors, []); });
  await page.goto(origin + '/admin', { waitUntil: 'networkidle' });
  return { page, state, requests };
}
async function region(page, name) { const element = page.getByRole('region', { name, exact: true }); await element.waitFor(); return element; }

test('运营概览显示真实数据、封面、后台状态和所有管理入口', async t => {
  const { page, requests } = await setup(t);
  await page.getByRole('heading', { name: '运营概览', exact: true }).waitFor();
  assert.equal(await page.getByText('132', { exact: true }).count() > 0, true);
  assert.deepEqual(requests.map(r => [r.key, r.days]).sort(), ['overview', 'articles', 'albums', 'trend'].map(k => [k, '30']).sort());
  assert.equal(await page.locator('main main').count(), 0, '内容区不能嵌套main地标');
  for (const name of ['访问趋势', '内容状态', '文章排行', '运营待办', '相册一览', '近期内容', '资源概况']) await region(page, name);
  assert.equal(await page.getByRole('link', { name: '新建文章', exact: true }).getAttribute('href'), '/admin/articles/new');
  assert.equal(await page.getByRole('link', { name: draft.title, exact: true }).getAttribute('href'), '/admin/articles/draft-1/edit');
  assert.equal(await page.getByRole('link', { name: '城市漫游', exact: true }).getAttribute('href'), '/admin/albums/album-1/photos');
  assert.equal(await page.getByRole('link', { name: privateArticle.title, exact: true }).getAttribute('href'), '/admin/articles/private-1/read');
  await page.getByText('私有 · 仅后台').waitFor();
  await page.getByText('资源登记大小', { exact: true }).waitFor();
  const trend = await region(page, '访问趋势');
  await trend.getByText('查看每日数据').click();
  assert.equal(await trend.getByRole('row').nth(1).getByRole('cell').first().textContent(), '2026-10-02');
  assert.equal(await trend.getByText('2026-11-15', { exact: true }).count(), 0);
  const zeroRow = trend.getByRole('row').filter({ hasText: '2026-10-30' });
  assert.deepEqual(await zeroRow.getByRole('cell').allTextContents(), ['2026-10-30', '0', '0']);
  await mkdir(root + '.tmp/dashboard', { recursive: true });
  await trend.getByText('查看每日数据').click();
  await page.screenshot({ path: root + '.tmp/dashboard/desktop.png', fullPage: true });
});

test('7、30、90 天同步切换四个请求，加载时不展示上一范围结果', async t => {
  const { page, state, requests } = await setup(t);
  for (const days of [7, 90, 30]) {
    state.delay = 250;
    const count = requests.length;
    await page.getByText(days + ' 天', { exact: true }).click();
    await page.getByText('正在加载指标').waitFor();
    await page.getByText('132', { exact: true }).waitFor();
    assert.deepEqual(requests.slice(count).map(r => [r.key, r.days]).sort(), ['overview', 'articles', 'albums', 'trend'].map(k => [k, String(days)]).sort());
  }
});

test('空列表有明确引导，零值保留为零，图表提供空状态', async t => {
  const { page } = await setup(t, { empty: true });
  for (const text of ['所选区间暂无访问记录', '暂无已发布文章', '暂无相册', '暂无近期内容', '暂无图片资源', '暂无待处理事项']) await page.getByText(text, { exact: true }).waitFor();
  assert.equal(await page.getByText('0', { exact: true }).count() >= 6, true);
});

test('各请求失败独立展示错误并支持重试', async t => {
  const { page, state } = await setup(t, { failed: ['articles', 'albums', 'trend'] });
  for (const name of ['文章排行', '相册一览', '访问趋势']) {
    const block = await region(page, name);
    await block.getByText('暂时无法连接数据服务').waitFor();
    state.failed = state.failed.filter(k => k !== ({ '文章排行': 'articles', '相册一览': 'albums', '访问趋势': 'trend' })[name]);
    await block.getByRole('button', { name: '重试', exact: true }).click();
    await block.getByText('暂时无法连接数据服务').waitFor({ state: 'hidden' });
  }
  await page.getByText('132', { exact: true }).waitFor();
});

test('刷新失败保留已有值和失败提示，概览首次失败不会显示伪零值', async t => {
  const { page, state } = await setup(t);
  state.failed = ['overview'];
  await page.getByRole('button', { name: '刷新数据', exact: true }).click();
  await page.getByText('刷新失败，正在显示上次成功数据').first().waitFor();
  await page.getByText('132', { exact: true }).waitFor();
  await page.reload({ waitUntil: 'networkidle' });
  const metrics = await region(page, '关键指标');
  await metrics.getByText('暂时无法连接数据服务').waitFor();
  assert.equal(await metrics.getByText('0', { exact: true }).count(), 0);
  state.failed = [];
  await metrics.getByRole('button', { name: '重试', exact: true }).click();
  await metrics.getByText('132', { exact: true }).waitFor();
});

test('图片失效后标题入口仍可用，长标题与 375px 页面没有横向溢出', async t => {
  const { page } = await setup(t, { brokenImages: true, longTitle: true });
  await page.getByText('图片暂不可用').first().waitFor();
  assert.equal(await page.getByRole('link', { name: '城市漫游', exact: true }).isVisible(), true);
  await page.setViewportSize({ width: 375, height: 812 });
  await page.getByRole('heading', { name: '运营概览', exact: true }).waitFor();
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
  assert.equal(await page.locator('.dashboard-chart text').first().evaluate(el => parseFloat(getComputedStyle(el).fontSize) * el.ownerSVGElement.getBoundingClientRect().width / el.ownerSVGElement.viewBox.baseVal.width >= 11), true, '移动端图表轴文字必须可读');
  await page.screenshot({ path: root + '.tmp/dashboard/mobile.png', fullPage: true });
  await page.setViewportSize({ width: 1100, height: 900 });
  assert.equal(await page.locator('.dashboard-tag').first().evaluate(el => el.getBoundingClientRect().right <= el.parentElement.getBoundingClientRect().right), true, '长标签不能覆盖访问数字');
});


test('概览入口可实际打开新建、草稿编辑、私有阅读和相册图片管理', async t => {
  const { page } = await setup(t);
  await page.route('**/api/v1/admin/articles', route => route.fulfill({ json: [article, draft, privateArticle].map(item => ({
    ...item, articleName: item.slug, bodyMdx: '# 正文\n\n真实后台内容', summary: '', featured: false, featuredRank: 100, featuredAt: null,
  })) }));
  await page.route('**/api/v1/admin/albums/album-1', route => route.fulfill({ json: {
    albumId: 'album-1', slug: 'walk', title: '城市漫游', description: '城市中的光影', status: 'published', tags: [], photoCount: 29, coverPhotoId: null,
  } }));
  await page.route('**/api/v1/admin/albums/album-1/photos', route => route.fulfill({ json: [] }));
  await page.getByRole('link', { name: '新建文章', exact: true }).click();
  await page.waitForURL('**/admin/articles/new');
  await page.getByRole('heading', { name: '新建文章', exact: true }).waitFor();
  await page.goBack({ waitUntil: 'networkidle' });
  await page.getByRole('link', { name: draft.title, exact: true }).click();
  await page.waitForURL('**/admin/articles/draft-1/edit');
  await page.getByRole('heading', { name: '编辑文章', exact: true }).waitFor();
  await page.goBack({ waitUntil: 'networkidle' });
  await page.getByRole('link', { name: privateArticle.title, exact: true }).click();
  await page.waitForURL('**/admin/articles/private-1/read');
  await page.getByRole('heading', { name: privateArticle.title, exact: true }).waitFor();
  await page.goBack({ waitUntil: 'networkidle' });
  await page.getByRole('link', { name: '城市漫游', exact: true }).click();
  await page.waitForURL('**/admin/albums/album-1/photos');
  await page.getByRole('heading', { name: '图片管理', exact: true }).waitFor();
});


test('移动端大访问数使用可读轴标签且数据表保留完整计数', async t => {
  const { page } = await setup(t, { largeViews: true });
  await page.setViewportSize({ width: 375, height: 812 });
  const trend = await region(page, '访问趋势');
  assert.equal((await trend.locator('svg text').allTextContents()).some(label => label.includes('万')), true);
  assert.equal(await trend.locator('svg text').evaluateAll(labels => labels.every(el => {
    const box = el.getBoundingClientRect(), chart = el.ownerSVGElement.getBoundingClientRect();
    return box.left >= chart.left && box.right <= chart.right;
  })), true, '大数轴标签不能超出图表边界');
  await trend.getByText('查看每日数据').click();
  const row = trend.getByRole('row').filter({ hasText: '2026-10-31' });
  assert.equal(await row.getByRole('cell').nth(1).textContent(), '99,999,999');
});
