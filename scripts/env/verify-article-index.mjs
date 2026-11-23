import { createRequire } from 'node:module';
import { pathToFileURL } from 'node:url';

const requireFrontend = createRequire(new URL('../../frontend/package.json', import.meta.url));
const { chromium } = requireFrontend('playwright');

export async function checkIndex(publicUrl, title) {
  const target = new URL(publicUrl);
  const browser = await chromium.launch();
  const timer = setTimeout(() => {
    void browser.close();
  }, 30_000);
  try {
    const page = await browser.newPage({ serviceWorkers: 'block' });
    // 使用无凭据的空白文档，仅加载博客自身发布的 Pagefind 文件。
    await page.route('**/*', async (route) => {
      const url = new URL(route.request().url());
      if (url.href === target.href && route.request().isNavigationRequest()) {
        return route.fulfill({ contentType: 'text/html', body: '<html lang="zh-CN"></html>' });
      }
      if (url.origin === target.origin && url.pathname.startsWith('/pagefind/')) {
        return route.continue();
      }
      return route.abort();
    });
    await page.goto(target.href);
    return await page.evaluate(
      async ({ href, title }) => {
        const { search } = await import('/pagefind/pagefind.js');
        const { results } = await search(title);
        for (let start = 0; start < results.length; start += 8) {
          const batch = await Promise.all(
            results.slice(start, start + 8).map((item) => item.data()),
          );
          if (
            batch.some((item) => {
              const result = new URL(item.url, href);
              const target = new URL(href);
              return (
                result.origin === target.origin &&
                result.pathname.replace(/\/$/, '') === target.pathname
              );
            })
          )
            return 'found';
        }
        return 'not_found';
      },
      { href: target.href, title },
    );
  } finally {
    clearTimeout(timer);
    await browser.close();
  }
}

async function main() {
  const [articleId] = process.argv.slice(2);
  const origin = process.env.MCP_ORIGIN;
  const token = process.env.CMS_ADMIN_TOKEN;
  delete process.env.CMS_ADMIN_TOKEN;
  if (!origin || !token || !/^[a-f\d-]{36}$/i.test(articleId ?? '')) {
    throw new Error('请设置 MCP_ORIGIN、CMS_ADMIN_TOKEN，并传入文章 UUID');
  }
  const api = new URL('/api/v1/admin/mcp/articles/' + articleId + '/search-check', origin);
  if (
    api.protocol !== 'https:' &&
    !(api.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(api.hostname))
  ) {
    throw new Error('需要 HTTPS 或开发回环地址');
  }
  const headers = { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json' };
  const get = await fetch(api, { headers, redirect: 'error', signal: AbortSignal.timeout(5000) });
  if (!get.ok) throw new Error('读取核验目标失败（HTTP ' + get.status + '）');
  const { publicUrl, title, version } = await get.json();
  const result = await checkIndex(publicUrl, title);
  const save = await fetch(api, {
    method: 'POST',
    headers,
    redirect: 'error',
    signal: AbortSignal.timeout(5000),
    body: JSON.stringify({ version, result }),
  });
  if (!save.ok) throw new Error('记录核验结果失败（HTTP ' + save.status + '）');
  console.log(JSON.stringify({ articleId, version, search: result }));
}

if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url) {
  main().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
