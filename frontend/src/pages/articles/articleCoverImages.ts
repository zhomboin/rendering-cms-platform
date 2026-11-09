import { fromMarkdown } from 'mdast-util-from-markdown';
import { isPermanentPublicImageUrl, uniqueArticleImages } from './articleImageUtils';
import type { ArticleImage } from './articleImageUtils';

type MarkdownNode = ReturnType<typeof fromMarkdown>['children'][number];

function* walk(nodes: MarkdownNode[]): Generator<MarkdownNode> {
  for (const node of nodes) {
    yield node;
    if ('children' in node) yield* walk(node.children);
  }
}

function* readStaticImageTags(html: string): Generator<ArticleImage> {
  const tags = /<!--[\s\S]*?(?:-->|$)|<([a-z][\w.-]*)(?=\s|\/?>)((?:"[^"]*"|'[^']*'|[^"'>])*)>/gi;
  for (const [, name, attributes] of html.matchAll(tags)) {
    if (!name || !['img', 'image', 'figure'].includes(name.toLowerCase())) continue;
    const props = new Map<string, string>();
    const attribute = /(?:^|\s)([\w:-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>`{}]+))/g;
    for (const [, key, doubleQuoted, singleQuoted, unquoted] of attributes.matchAll(attribute)) {
      props.set(key.toLowerCase(), doubleQuoted ?? singleQuoted ?? unquoted);
    }
    const url = props.get('src')?.replaceAll('&amp;', '&');
    if (url) yield { url, alt: props.get('alt') ?? '' };
  }
}

export function collectArticleImages(source: string) {
  const { children } = fromMarkdown(source);
  const definitions = new Map<string, string>();
  for (const node of walk(children)) {
    if (node.type === 'definition' && !definitions.has(node.identifier)) {
      definitions.set(node.identifier, node.url);
    }
  }

  const images: ArticleImage[] = [];
  for (const node of walk(children)) {
    if (node.type === 'image') {
      images.push({ url: node.url, alt: node.alt ?? '' });
    } else if (node.type === 'imageReference') {
      const url = definitions.get(node.identifier);
      if (url) images.push({ url, alt: node.alt ?? '' });
    } else if (node.type === 'html') {
      images.push(...readStaticImageTags(node.value));
    }
  }

  return uniqueArticleImages(
    images.filter(({ url }) => /^\/(?![\s\\/])/.test(url) || isPermanentPublicImageUrl(url)),
  );
}
