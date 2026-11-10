import { useEffect, useState } from 'react';
import { compileArticle } from '../../markdown/engine.js';
import type { ArticleMarkdown } from '../../markdown/engine.js';

type Result = { source: string; article?: ArticleMarkdown; error?: string };

export function useArticleMarkdown(source: string) {
  const [result, setResult] = useState<Result>();
  useEffect(() => {
    let cancelled = false;
    compileArticle(source).then(
      (article) => {
        if (!cancelled) setResult({ source, article });
      },
      (error: unknown) => {
        if (!cancelled)
          setResult({ source, error: error instanceof Error ? error.message : '正文解析失败' });
      },
    );
    return () => {
      cancelled = true;
    };
  }, [source]);
  return result?.source === source
    ? { ...result, loading: false }
    : { source, loading: true, article: undefined, error: undefined };
}
