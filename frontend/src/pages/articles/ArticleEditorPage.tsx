import { useEffect, useMemo, useState } from 'react';
import type { KeyboardEvent, ReactNode } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Form, Input, Select, Button, Typography, Modal, Space, message, Alert, Skeleton, Tooltip } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createAdminArticle,
  listAdminArticles,
  publishAdminArticle,
  updateAdminArticle,
} from '../../api/articles';
import type { AdminArticlePayload, ArticleFormData } from '../../api/articles';
import { MdxPreview } from './MdxPreview';

const { Title, Text } = Typography;
const { TextArea } = Input;

const initialFormData: ArticleFormData = {
  title: '',
  slug: '',
  summary: '',
  tags: [],
  bodyMdx: '',
  coverImageUrl: '',
};

const ARTICLE_DRAFT_KEY_PREFIX = 'rendering-cms-article-editor-draft';

type MarkdownAction = {
  id: 'bold' | 'italic' | 'underline' | 'strike' | 'inlineCode' | 'codeBlock' | 'link';
  icon: ReactNode;
  title: string;
  shortcut?: string;
  prefix: string;
  suffix: string;
  placeholder: string;
};

const markdownActions: MarkdownAction[] = [
  { id: 'bold', icon: <strong>B</strong>, title: '加粗', shortcut: 'Ctrl+B', prefix: '**', suffix: '**', placeholder: '加粗文本' },
  { id: 'italic', icon: <em>I</em>, title: '斜体', shortcut: 'Ctrl+I', prefix: '*', suffix: '*', placeholder: '斜体文本' },
  { id: 'underline', icon: <span style={{ textDecoration: 'underline' }}>U</span>, title: '下划线', shortcut: 'Ctrl+U', prefix: '<u>', suffix: '</u>', placeholder: '下划线文本' },
  { id: 'strike', icon: <span style={{ textDecoration: 'line-through' }}>S</span>, title: '删除线', prefix: '~~', suffix: '~~', placeholder: '删除线文本' },
  { id: 'inlineCode', icon: <CodeInlineIcon />, title: '行内代码', prefix: '`', suffix: '`', placeholder: 'code' },
  { id: 'codeBlock', icon: <CodeBlockIcon />, title: '代码块', prefix: '```ts\n', suffix: '\n```', placeholder: 'const example = true;' },
  { id: 'link', icon: <LinkIcon />, title: '添加链接', shortcut: 'Ctrl+K', prefix: '[', suffix: '](https://example.com)', placeholder: '链接文本' },
];

function toPayload(values: ArticleFormData): AdminArticlePayload {
  return {
    slug: values.slug,
    title: values.title,
    summary: values.summary ?? '',
    bodyMdx: values.bodyMdx,
    tags: values.tags ?? [],
    featured: false,
    coverImageUrl: values.coverImageUrl ?? '',
  };
}

type StoredArticleDraft = {
  values: ArticleFormData;
  updatedAt: string;
};

function getArticleDraftKey(articleId: string | undefined) {
  return `${ARTICLE_DRAFT_KEY_PREFIX}:${articleId ?? 'new'}`;
}

function normalizeFormData(values: Partial<ArticleFormData>): ArticleFormData {
  return {
    title: values.title ?? '',
    slug: values.slug ?? '',
    summary: values.summary ?? '',
    tags: Array.isArray(values.tags) ? values.tags : [],
    bodyMdx: values.bodyMdx ?? '',
    coverImageUrl: values.coverImageUrl ?? '',
  };
}

function hasMeaningfulDraft(values: ArticleFormData) {
  return Boolean(
    values.title.trim()
      || values.slug.trim()
      || values.summary.trim()
      || values.bodyMdx.trim()
      || values.coverImageUrl.trim()
      || values.tags.length > 0,
  );
}

function loadArticleDraft(key: string): StoredArticleDraft | null {
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

function saveArticleDraft(key: string, values: Partial<ArticleFormData>) {
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

function clearArticleDraft(key: string) {
  try {
    window.localStorage.removeItem(key);
  } catch {
    // Ignore storage errors after successful remote persistence.
  }
}

export default function ArticleEditorPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ArticleFormData>();
  const bodyMdx = Form.useWatch('bodyMdx', form) ?? '';
  const [publishModalOpen, setPublishModalOpen] = useState(false);
  const [writerFullscreen, setWriterFullscreen] = useState(false);
  const [restoredDraftAt, setRestoredDraftAt] = useState<string | null>(null);
  const isEdit = Boolean(id);
  const draftKey = useMemo(() => getArticleDraftKey(id), [id]);

  const articlesQuery = useQuery({
    queryKey: ['admin-articles'],
    queryFn: listAdminArticles,
    enabled: isEdit,
  });

  const currentArticle = useMemo(
    () => articlesQuery.data?.find((article) => article.articleId === id),
    [articlesQuery.data, id],
  );

  useEffect(() => {
    setRestoredDraftAt(null);
    if (!isEdit) {
      const draft = loadArticleDraft(draftKey);
      form.setFieldsValue(draft?.values ?? initialFormData);
      if (draft) setRestoredDraftAt(draft.updatedAt);
      return;
    }
    if (currentArticle) {
      const articleValues = {
        title: currentArticle.title,
        slug: currentArticle.slug,
        summary: currentArticle.summary,
        tags: currentArticle.tags,
        bodyMdx: currentArticle.bodyMdx,
        coverImageUrl: currentArticle.coverImageUrl ?? '',
      };
      const draft = loadArticleDraft(draftKey);
      form.setFieldsValue({
        ...articleValues,
        ...(draft?.values ?? {}),
      });
      if (draft) setRestoredDraftAt(draft.updatedAt);
    }
  }, [currentArticle, draftKey, form, isEdit]);

  const saveMutation = useMutation({
    mutationFn: async (values: ArticleFormData) => {
      if (isEdit && id) {
        return updateAdminArticle(id, toPayload(values));
      }
      return createAdminArticle(toPayload(values));
    },
    onSuccess: async (article) => {
      clearArticleDraft(draftKey);
      message.success('草稿已保存');
      await queryClient.invalidateQueries({ queryKey: ['admin-articles'] });
      if (!isEdit) navigate(`/admin/articles/${article.articleId}/edit`, { replace: true });
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '草稿保存失败');
    },
  });

  const publishMutation = useMutation({
    mutationFn: async (values: ArticleFormData) => {
      const saved = isEdit && id
        ? await updateAdminArticle(id, toPayload(values))
        : await createAdminArticle(toPayload(values));
      return publishAdminArticle(saved.articleId);
    },
    onSuccess: async () => {
      clearArticleDraft(draftKey);
      message.success('文章已发布');
      setPublishModalOpen(false);
      await queryClient.invalidateQueries({ queryKey: ['admin-articles'] });
      navigate('/admin/articles');
    },
    onError: (error) => {
      setPublishModalOpen(false);
      message.error(error instanceof Error ? error.message : '文章发布失败');
    },
  });

  const handleSaveDraft = async () => {
    const values = await form.validateFields();
    saveMutation.mutate(values);
  };

  const handlePublish = async () => {
    const values = await form.validateFields();
    publishMutation.mutate(values);
  };

  const applyMarkdownAction = (action: MarkdownAction) => {
    const editor = document.querySelector<HTMLTextAreaElement>('[data-article-body-editor="true"]');
    const currentValue = form.getFieldValue('bodyMdx') ?? '';
    const selectionStart = editor?.selectionStart ?? currentValue.length;
    const selectionEnd = editor?.selectionEnd ?? currentValue.length;
    const selectedText = currentValue.slice(selectionStart, selectionEnd);
    const insertedText = selectedText || action.placeholder;
    const nextValue = [
      currentValue.slice(0, selectionStart),
      action.prefix,
      insertedText,
      action.suffix,
      currentValue.slice(selectionEnd),
    ].join('');
    const nextSelectionStart = selectionStart + action.prefix.length;
    const nextSelectionEnd = nextSelectionStart + insertedText.length;

    form.setFieldsValue({ bodyMdx: nextValue });
    window.requestAnimationFrame(() => {
      const nextEditor = document.querySelector<HTMLTextAreaElement>('[data-article-body-editor="true"]');
      nextEditor?.focus();
      nextEditor?.setSelectionRange(nextSelectionStart, nextSelectionEnd);
    });
  };

  const handleEditorShortcut = (event: KeyboardEvent<HTMLDivElement>) => {
    if (!event.ctrlKey && !event.metaKey) return;
    if (event.key.toLowerCase() === 's') {
      event.preventDefault();
      void handleSaveDraft();
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      setPublishModalOpen(true);
    }
    const actionByKey: Partial<Record<string, MarkdownAction['id']>> = {
      b: 'bold',
      i: 'italic',
      u: 'underline',
      k: 'link',
    };
    const actionId = actionByKey[event.key.toLowerCase()];
    const action = markdownActions.find((item) => item.id === actionId);
    if (action) {
      event.preventDefault();
      applyMarkdownAction(action);
    }
  };

  const renderFormatButton = (action: MarkdownAction) => (
    <Tooltip key={action.id} title={action.shortcut ? `${action.title} (${action.shortcut})` : action.title}>
      <Button
        aria-label={action.shortcut ? `${action.title} ${action.shortcut}` : action.title}
        htmlType="button"
        icon={action.icon}
        onClick={() => applyMarkdownAction(action)}
        style={{ width: 36, height: 36 }}
      />
    </Tooltip>
  );

  const renderEditorWorkspace = (fullscreen: boolean) => (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, alignItems: 'center', marginBottom: 12 }}>
        <Text strong>MDX 正文</Text>
        <Tooltip title={fullscreen ? '退出全屏写作' : '全屏写作：正文和预览并排展示'}>
          <Button
            aria-label={fullscreen ? '退出全屏写作' : '全屏写作'}
            htmlType="button"
            icon={fullscreen ? <ExitFullscreenIcon /> : <FullscreenIcon />}
            onClick={() => setWriterFullscreen((value) => !value)}
          />
        </Tooltip>
      </div>

      <Space size={8} wrap style={{ marginBottom: 12 }}>
        {markdownActions.map(renderFormatButton)}
      </Space>

      <Form.Item name="bodyMdx" rules={[{ required: true, message: '请输入文章正文' }]} style={{ marginBottom: 0 }}>
        <TextArea
          data-article-body-editor="true"
          rows={fullscreen ? 30 : 22}
          placeholder="使用 Markdown/MDX 格式编写文章正文"
          style={{
            height: fullscreen ? 'calc(100vh - 250px)' : undefined,
            fontFamily: "'Fira Code', 'JetBrains Mono', 'SF Mono', Consolas, monospace",
            lineHeight: 1.7,
          }}
        />
      </Form.Item>
    </div>
  );

  if (isEdit && articlesQuery.isLoading) {
    return (
      <div style={{ padding: 24 }}>
        <Skeleton active paragraph={{ rows: 12 }} />
      </div>
    );
  }

  return (
    <div style={{ padding: 24 }}>
      <Title level={1} style={{ margin: '0 0 24px', fontSize: 24, fontWeight: 700 }}>
        {isEdit ? '编辑文章' : '新建文章'}
      </Title>

      {articlesQuery.error && (
        <Alert type="error" showIcon title={articlesQuery.error instanceof Error ? articlesQuery.error.message : '文章读取失败'} style={{ marginBottom: 20 }} />
      )}

      {restoredDraftAt && (
        <Alert
          type="info"
          showIcon
          title="已恢复本地未保存草稿"
          description={`恢复时间：${new Date(restoredDraftAt).toLocaleString()}`}
          style={{ marginBottom: 20 }}
        />
      )}

      <Card style={{ borderRadius: 8, border: '1px solid #E2E8F0', boxShadow: '0 1px 3px rgba(0,0,0,0.06)' }} styles={{ body: { padding: 32 } }}>
        <div
          onKeyDown={handleEditorShortcut}
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 360px), 1fr))',
            gap: 28,
            alignItems: 'start',
          }}
        >
          <Form
            form={form}
            layout="vertical"
            initialValues={initialFormData}
            onValuesChange={(_, values) => saveArticleDraft(draftKey, values)}
          >
            <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入文章标题' }]}>
              <Input placeholder="输入文章标题" size="large" />
            </Form.Item>

            <Form.Item
              name="slug"
              label="Slug"
              rules={[
                { required: true, message: '请输入 URL Slug' },
                { pattern: /^[a-z0-9]+(?:-[a-z0-9]+)*$/, message: 'Slug 只能使用小写字母、数字和中划线' },
              ]}
            >
              <Input prefix="/articles/" placeholder="my-article-slug" size="large" />
            </Form.Item>

            <Form.Item name="summary" label="摘要">
              <TextArea rows={3} placeholder="文章摘要" showCount maxLength={300} />
            </Form.Item>

            <Form.Item name="tags" label="标签">
              <Select mode="tags" placeholder="输入标签后按回车添加" style={{ width: '100%' }} tokenSeparators={[',', '，']} />
            </Form.Item>

            {!writerFullscreen && renderEditorWorkspace(false)}
            {writerFullscreen && (
              <div
                style={{
                  position: 'fixed',
                  inset: 16,
                  zIndex: 1100,
                  padding: 24,
                  borderRadius: 8,
                  background: '#FFFFFF',
                  boxShadow: '0 24px 80px rgba(15, 23, 42, 0.24)',
                  overflow: 'auto',
                }}
              >
                <div
                  style={{
                    display: 'grid',
                    gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 420px), 1fr))',
                    gap: 24,
                    height: '100%',
                    alignItems: 'start',
                  }}
                >
                  {renderEditorWorkspace(true)}
                  <div style={{ minHeight: 0 }}>
                    <MdxPreview source={bodyMdx} fillHeight />
                  </div>
                </div>
              </div>
            )}

            <Text
              style={{
                display: 'block',
                color: '#64748B',
                fontSize: 12,
                marginTop: 8,
                marginBottom: 20,
              }}
            >
              预览会实时更新；复杂 MDX 组件会按源码展示，最终渲染以 Rendering 博客为准。
            </Text>

            <Form.Item name="coverImageUrl" label="封面图片 URL">
              <Input placeholder="https://example.com/image.jpg" />
            </Form.Item>
          </Form>

          {!writerFullscreen && (
            <div style={{ position: 'sticky', top: 24 }}>
              <MdxPreview
                source={bodyMdx}
                onEnterFullscreen={() => setWriterFullscreen(true)}
              />
            </div>
          )}
        </div>
      </Card>

      <div
        style={{
          position: 'sticky',
          bottom: 0,
          marginTop: 24,
          padding: '16px 0',
          background: '#FFFFFF',
          borderTop: '1px solid #E2E8F0',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
        }}
      >
        <Button onClick={() => navigate('/admin/articles')}>取消</Button>
        <Space size={12}>
          <Button onClick={handleSaveDraft} loading={saveMutation.isPending} style={{ borderColor: '#4F46E5', color: '#4F46E5' }}>
            保存草稿
          </Button>
          <Button
            type="primary"
            size="large"
            loading={publishMutation.isPending}
            style={{ backgroundColor: '#4F46E5', borderColor: '#4F46E5', fontWeight: 600 }}
            onClick={() => setPublishModalOpen(true)}
          >
            发布
          </Button>
        </Space>
      </div>

      <Modal
        title="确认发布"
        open={publishModalOpen}
        onOk={handlePublish}
        onCancel={() => setPublishModalOpen(false)}
        okText="确认发布"
        cancelText="取消"
        confirmLoading={publishMutation.isPending}
      >
        <p>确定要发布这篇文章吗？发布后将对所有读者可见。</p>
      </Modal>
    </div>
  );
}

function CodeInlineIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="m8 9-4 3 4 3" />
      <path d="m16 9 4 3-4 3" />
    </svg>
  );
}

function CodeBlockIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M4 5h16" />
      <path d="M4 19h16" />
      <path d="m9 9-3 3 3 3" />
      <path d="m15 9 3 3-3 3" />
    </svg>
  );
}

function LinkIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M10 13a5 5 0 0 0 7.1 0l2-2a5 5 0 0 0-7.1-7.1l-1.1 1.1" />
      <path d="M14 11a5 5 0 0 0-7.1 0l-2 2A5 5 0 0 0 12 20.1l1.1-1.1" />
    </svg>
  );
}

function FullscreenIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M8 3H3v5" />
      <path d="M16 3h5v5" />
      <path d="M8 21H3v-5" />
      <path d="M16 21h5v-5" />
    </svg>
  );
}

function ExitFullscreenIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M4 14h6v6" />
      <path d="M20 14h-6v6" />
      <path d="M4 10h6V4" />
      <path d="M20 10h-6V4" />
    </svg>
  );
}
