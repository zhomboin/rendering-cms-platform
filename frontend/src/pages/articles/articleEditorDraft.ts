import type { AdminArticlePayload, ArticleFormData } from '../../api/articles';

export const initialFormData: ArticleFormData = {
  title: '',
  slug: '',
  articleName: '',
  summary: '',
  tags: [],
  bodyMdx: '',
  featured: false,
  featuredRank: 100,
  featuredAt: null,
  coverImageUrl: '',
};

const ARTICLE_DRAFT_KEY_PREFIX = 'rendering-cms-article-editor-draft';

export function normalizeArticleName(value: string) {
  return value
    .normalize('NFKD')
    .replace(/\p{Mark}/gu, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '');
}

export function toPayload(values: ArticleFormData): AdminArticlePayload {
  return {
    articleName: values.articleName,
    title: values.title,
    summary: values.summary ?? '',
    bodyMdx: values.bodyMdx,
    tags: values.tags ?? [],
    featured: values.featured ?? false,
    featuredRank: values.featuredRank ?? 100,
    featuredAt: values.featuredAt ?? null,
    coverImageUrl: values.coverImageUrl ?? '',
  };
}

type StoredArticleDraft = {
  values: ArticleFormData;
  updatedAt: string;
};

export function getArticleDraftKey(articleId: string | undefined) {
  return `${ARTICLE_DRAFT_KEY_PREFIX}:${articleId ?? 'new'}`;
}

function normalizeFormData(values: Partial<ArticleFormData>): ArticleFormData {
  return {
    title: values.title ?? '',
    slug: values.slug ?? '',
    articleName: values.articleName ?? '',
    summary: values.summary ?? '',
    tags: Array.isArray(values.tags) ? values.tags : [],
    bodyMdx: values.bodyMdx ?? '',
    featured: values.featured ?? false,
    featuredRank: values.featuredRank ?? 100,
    featuredAt: values.featuredAt ?? null,
    coverImageUrl: values.coverImageUrl ?? '',
  };
}

function hasMeaningfulDraft(values: ArticleFormData) {
  return Boolean(
    values.title.trim() ||
      values.articleName.trim() ||
      values.summary.trim() ||
      values.bodyMdx.trim() ||
      values.coverImageUrl.trim() ||
      values.featured ||
      values.featuredRank !== 100 ||
      Boolean(values.featuredAt) ||
      values.tags.length > 0,
  );
}

export function loadArticleDraft(key: string): StoredArticleDraft | null {
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as StoredArticleDraft;
    const values = normalizeFormData(parsed.values ?? {});
    if (!hasMeaningfulDraft(values)) return null;
    return {
      values,
      updatedAt: parsed.updatedAt,
    };
  } catch {
    window.localStorage.removeItem(key);
    return null;
  }
}

export function saveArticleDraft(key: string, values: Partial<ArticleFormData>) {
  try {
    const normalizedValues = normalizeFormData(values);
    if (!hasMeaningfulDraft(normalizedValues)) {
      window.localStorage.removeItem(key);
      return;
    }
    const draft: StoredArticleDraft = {
      values: normalizedValues,
      updatedAt: new Date().toISOString(),
    };
    window.localStorage.setItem(key, JSON.stringify(draft));
  } catch {
    // Local draft is best-effort and must not block writing in the editor.
  }
}

export function clearArticleDraft(key: string) {
  try {
    window.localStorage.removeItem(key);
  } catch {
    // Ignore storage errors after successful remote persistence.
  }
}
