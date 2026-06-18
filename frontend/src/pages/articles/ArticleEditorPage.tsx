import { useEffect, useMemo, useState } from 'react';
import type { KeyboardEvent, ReactNode } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Form, Input, Select, Button, Typography, Modal, Space, message, Alert, Skeleton, Tooltip, Upload } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import type { UploadProps } from 'antd/es/upload/interface';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createAdminArticle,
  listAdminArticles,
  publishAdminArticle,
  updateAdminArticle,
} from '../../api/articles';
import type { AdminArticlePayload, ArticleFormData } from '../../api/articles';
import { uploadAdminAsset } from '../../api/assets';
import { MdxPreview } from './MdxPreview';
import './ArticleEditorPage.css';

const { Title, Text } = Typography;
const { TextArea } = Input;

const initialFormData: ArticleFormData = {
  title: '',
  slug: '',
  articleName: '',
  summary: '',
  tags: [],
  bodyMdx: '',
  coverImageUrl: '',
};

const ARTICLE_DRAFT_KEY_PREFIX = 'rendering-cms-article-editor-draft';

type MarkdownAction = {
  id: 'bold' | 'italic' | 'underline' | 'strike' | 'inlineCode' | 'codeBlock' | 'link' | 'image';
  icon: ReactNode;
  title: string;
  shortcut?: string;
  prefix?: string;
  suffix?: string;
  placeholder?: string;
};

type ImageInsertForm = {
  altText: string;
  imageUrl: string;
};

const markdownActions: MarkdownAction[] = [
  { id: 'bold', icon: <strong>B</strong>, title: '加粗', shortcut: 'Ctrl+B', prefix: '**', suffix: '**', placeholder: '加粗文本' },
  { id: 'italic', icon: <em>I</em>, title: '斜体', shortcut: 'Ctrl+I', prefix: '*', suffix: '*', placeholder: '斜体文本' },
  { id: 'underline', icon: <span style={{ textDecoration: 'underline' }}>U</span>, title: '下划线', shortcut: 'Ctrl+U', prefix: '<u>', suffix: '</u>', placeholder: '下划线文本' },
  { id: 'strike', icon: <span style={{ textDecoration: 'line-through' }}>S</span>, title: '删除线', prefix: '~~', suffix: '~~', placeholder: '删除线文本' },
  { id: 'inlineCode', icon: <CodeInlineIcon />, title: '行内代码', prefix: '`', suffix: '`', placeholder: 'code' },
  { id: 'codeBlock', icon: <CodeBlockIcon />, title: '代码块', prefix: '```ts\n', suffix: '\n```', placeholder: 'const example = true;' },
  { id: 'link', icon: <LinkIcon />, title: '添加链接', shortcut: 'Ctrl+K', prefix: '[', suffix: '](https://example.com)', placeholder: '链接文本' },
  { id: 'image', icon: <ImageIcon />, title: '插入图片' },
];

function toPayload(values: ArticleFormData): AdminArticlePayload {
  return {
    articleName: values.articleName,
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
    articleName: values.articleName ?? '',
    summary: values.summary ?? '',
    tags: Array.isArray(values.tags) ? values.tags : [],
    bodyMdx: values.bodyMdx ?? '',
    coverImageUrl: values.coverImageUrl ?? '',
  };
}

function hasMeaningfulDraft(values: ArticleFormData) {
  return Boolean(
    values.title.trim()
      || values.articleName.trim()
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
  const [imageForm] = Form.useForm<ImageInsertForm>();
  const bodyMdx = Form.useWatch('bodyMdx', form) ?? '';
  const [publishModalOpen, setPublishModalOpen] = useState(false);
  const [imageModalOpen, setImageModalOpen] = useState(false);
  const [imageUploading, setImageUploading] = useState(false);
  const [writerFullscreen, setWriterFullscreen] = useState(false);
  const [mobilePane, setMobilePane] = useState<'edit' | 'preview'>('edit');
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
        articleName: currentArticle.articleName,
        summary: currentArticle.summary,
        tags: currentArticle.tags,
        bodyMdx: currentArticle.bodyMdx,
        coverImageUrl: currentArticle.coverImageUrl ?? '',
      };
      const draft = loadArticleDraft(draftKey);
      form.setFieldsValue({
        ...articleValues,
        ...(draft?.values ?? {}),
        slug: articleValues.slug,
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

  const getEditorSelection = () => {
    const editor = document.querySelector<HTMLTextAreaElement>('[data-article-body-editor="true"]');
    const currentValue = form.getFieldValue('bodyMdx') ?? '';
    const selectionStart = editor?.selectionStart ?? currentValue.length;
    const selectionEnd = editor?.selectionEnd ?? currentValue.length;
    return {
      currentValue,
      selectionStart,
      selectionEnd,
      selectedText: currentValue.slice(selectionStart, selectionEnd),
    };
  };

  const insertIntoEditor = (prefix: string, suffix: string, fallbackText: string) => {
    const { currentValue, selectionStart, selectionEnd, selectedText } = getEditorSelection();
    const insertedText = selectedText || fallbackText;
    const nextValue = [
      currentValue.slice(0, selectionStart),
      prefix,
      insertedText,
      suffix,
      currentValue.slice(selectionEnd),
    ].join('');
    const nextSelectionStart = selectionStart + prefix.length;
    const nextSelectionEnd = nextSelectionStart + insertedText.length;

    form.setFieldsValue({ bodyMdx: nextValue });
    window.requestAnimationFrame(() => {
      const nextEditor = document.querySelector<HTMLTextAreaElement>('[data-article-body-editor="true"]');
      nextEditor?.focus();
      nextEditor?.setSelectionRange(nextSelectionStart, nextSelectionEnd);
    });
  };

  const insertTextIntoEditor = (text: string) => {
    const { currentValue, selectionStart, selectionEnd } = getEditorSelection();
    const nextValue = [
      currentValue.slice(0, selectionStart),
      text,
      currentValue.slice(selectionEnd),
    ].join('');
    const nextSelection = selectionStart + text.length;

    form.setFieldsValue({ bodyMdx: nextValue });
    window.requestAnimationFrame(() => {
      const nextEditor = document.querySelector<HTMLTextAreaElement>('[data-article-body-editor="true"]');
      nextEditor?.focus();
      nextEditor?.setSelectionRange(nextSelection, nextSelection);
    });
  };

  const openImageModal = () => {
    const { selectedText } = getEditorSelection();
    imageForm.setFieldsValue({
      altText: selectedText,
      imageUrl: '',
    });
    setImageModalOpen(true);
  };

  const applyMarkdownAction = (action: MarkdownAction) => {
    if (action.id === 'image') {
      openImageModal();
      return;
    }
    insertIntoEditor(action.prefix ?? '', action.suffix ?? '', action.placeholder ?? '');
  };

  const handleInsertImage = async () => {
    const values = await imageForm.validateFields();
    const imageUrl = values.imageUrl.trim();
    const altText = values.altText.trim() || '图片';
    const { currentValue, selectionStart, selectionEnd } = getEditorSelection();
    const beforeSelection = currentValue.slice(0, selectionStart);
    const afterSelection = currentValue.slice(selectionEnd);
    const prefix = beforeSelection.trimEnd()
      ? beforeSelection.endsWith('\n\n')
        ? ''
        : beforeSelection.endsWith('\n')
          ? '\n'
          : '\n\n'
      : '';
    const suffix = afterSelection.trimStart()
      ? afterSelection.startsWith('\n\n')
        ? ''
        : afterSelection.startsWith('\n')
          ? '\n'
          : '\n\n'
      : '';
    insertTextIntoEditor(`${prefix}![${altText}](${imageUrl})${suffix}`);
    setImageModalOpen(false);
  };

  const imageUploadProps: UploadProps = {
    accept: 'image/png,image/jpeg,image/webp',
    maxCount: 1,
    showUploadList: false,
    customRequest: async (options) => {
      const file = options.file as File;
      if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
        const error = new Error('仅支持 PNG、JPEG、WebP 图片');
        message.error(error.message);
        options.onError?.(error);
        return;
      }

      setImageUploading(true);
      try {
        const asset = await uploadAdminAsset(file, 'blog-image');
        if (!asset.publicUrl) {
          throw new Error('图片已上传，但当前存储未返回可公开访问的 URL');
        }
        imageForm.setFieldsValue({
          imageUrl: asset.publicUrl,
          altText: imageForm.getFieldValue('altText') || file.name.replace(/\.[^.]+$/, ''),
        });
        message.success('图片已上传，可插入正文');
        options.onSuccess?.(asset);
      } catch (error) {
        const uploadError = error instanceof Error ? error : new Error('图片上传失败');
        message.error(uploadError.message);
        options.onError?.(uploadError);
      } finally {
        setImageUploading(false);
      }
    },
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
        className="article-editor__format-button"
      />
    </Tooltip>
  );

  const renderEditorWorkspace = (fullscreen: boolean) => (
    <div className={fullscreen ? 'article-editor__writer article-editor__writer--fullscreen' : 'article-editor__writer'}>
      <div className="article-editor__writer-header">
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

      <Space className="article-editor__toolbar" size={8} wrap>
        {markdownActions.map(renderFormatButton)}
      </Space>

      <Form.Item name="bodyMdx" rules={[{ required: true, message: '请输入文章正文' }]} style={{ marginBottom: 0 }}>
        <TextArea
          data-article-body-editor="true"
          className="article-editor__body-input"
          rows={fullscreen ? 30 : 22}
          placeholder="使用 Markdown/MDX 格式编写文章正文"
          style={{
            fontFamily: "'Fira Code', 'JetBrains Mono', 'SF Mono', Consolas, monospace",
            lineHeight: 1.7,
          }}
        />
      </Form.Item>
    </div>
  );

  if (isEdit && articlesQuery.isLoading) {
    return (
      <div className="article-editor article-editor--loading">
        <Skeleton active paragraph={{ rows: 12 }} />
      </div>
    );
  }

  return (
    <div className="article-editor">
      <Title level={1} className="article-editor__title">
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

      <Card className="article-editor__card" styles={{ body: { padding: 0 } }}>
        <div className="article-editor__mobile-tabs">
          <Button
            type={mobilePane === 'edit' ? 'primary' : 'text'}
            aria-pressed={mobilePane === 'edit'}
            onClick={() => setMobilePane('edit')}
          >
            编辑
          </Button>
          <Button
            type={mobilePane === 'preview' ? 'primary' : 'text'}
            aria-pressed={mobilePane === 'preview'}
            onClick={() => setMobilePane('preview')}
          >
            预览
          </Button>
        </div>
        <div
          onKeyDown={handleEditorShortcut}
          className="article-editor__workspace"
        >
          <Form
            form={form}
            className={mobilePane === 'preview' ? 'article-editor__form article-editor__pane-hidden-mobile' : 'article-editor__form'}
            layout="vertical"
            initialValues={initialFormData}
            onValuesChange={(_, values) => saveArticleDraft(draftKey, values)}
          >
            <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入文章标题' }]}>
              <Input placeholder="输入文章标题" size="large" />
            </Form.Item>

            <Form.Item
              name="slug"
              label="短链"
            >
              <Input prefix="/articles/" placeholder="保存后生成" size="large" disabled />
            </Form.Item>

            <Form.Item
              name="articleName"
              label="英文名"
              rules={[
                { required: true, message: '请输入文章英文名' },
                { pattern: /^[a-z0-9]+(?:-[a-z0-9]+)*$/, message: '英文名只能使用小写字母、数字和中划线' },
              ]}
            >
              <Input placeholder="my-article-name" size="large" />
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
                className="article-editor__fullscreen"
              >
                <div
                  className="article-editor__fullscreen-grid"
                >
                  {renderEditorWorkspace(true)}
                  <div className="article-editor__fullscreen-preview">
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
            <div className={mobilePane === 'edit' ? 'article-editor__preview article-editor__pane-hidden-mobile' : 'article-editor__preview'}>
              <MdxPreview
                source={bodyMdx}
                onEnterFullscreen={() => setWriterFullscreen(true)}
              />
            </div>
          )}
        </div>
      </Card>

      <div
        className="article-editor__actions"
      >
        <Button onClick={() => navigate('/admin/articles')}>取消</Button>
        <Space className="article-editor__actions-main" size={12}>
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

      <Modal
        title="插入图片"
        open={imageModalOpen}
        onOk={handleInsertImage}
        onCancel={() => setImageModalOpen(false)}
        okText="插入正文"
        cancelText="取消"
        confirmLoading={imageUploading}
      >
        <Form form={imageForm} layout="vertical" initialValues={{ altText: '', imageUrl: '' }}>
          <Form.Item
            name="imageUrl"
            label="图片 URL"
            rules={[
              { required: true, message: '请输入图片 URL 或上传图片' },
              {
                validator: (_, value: string) => {
                  if (!value) return Promise.resolve();
                  if (/^(https?:\/\/|\/)/.test(value.trim())) return Promise.resolve();
                  return Promise.reject(new Error('图片 URL 必须以 http(s):// 或 / 开头'));
                },
              },
            ]}
          >
            <Input placeholder="https://example.com/image.webp" />
          </Form.Item>

          <Form.Item name="altText" label="替代文本">
            <Input placeholder="用于无障碍和图片加载失败时展示" />
          </Form.Item>

          <Upload {...imageUploadProps}>
            <Button icon={<UploadOutlined />} loading={imageUploading}>
              上传图片并回填 URL
            </Button>
          </Upload>
        </Form>
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

function ImageIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <rect x="3" y="5" width="18" height="14" rx="2" />
      <circle cx="8" cy="10" r="1.5" />
      <path d="m21 16-5-5L5 19" />
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
