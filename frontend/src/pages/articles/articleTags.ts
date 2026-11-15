// 与数据库的 normalize_article_tag 使用相同的转换规则。
export function normalizeArticleTag(value: string) {
  const tag = value
    .normalize('NFKD')
    .replace(/([A-Za-z])[\u0300-\u036f]+/g, '$1')
    .normalize('NFC')
    .replace(/([A-Z])([A-Z][a-z])/g, '$1-$2')
    .replace(/([a-z0-9])([A-Z])/g, '$1-$2')
    .toLowerCase()
    .replace(
      /[\p{P}\p{White_Space}\u0000-\u002f\u003a-\u0040\u005b-\u0060\u007b-\u007f]+/gu,
      '-',
    )
    .replace(/^-|-$/g, '');
  return /[\p{L}\p{Nd}]/u.test(tag) ? tag : '';
}

export function normalizeArticleTags(values: readonly unknown[]) {
  const tags = values.filter((value) => typeof value === 'string');
  return [...new Set(tags.map(normalizeArticleTag).filter(Boolean))];
}
