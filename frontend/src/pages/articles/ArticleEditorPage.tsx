import { useEffect, useMemo, useState } from 'react';
import type { KeyboardEvent, ReactNode } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Card, Form, Input, Select, Button, Typography, Modal, Space, message, Alert, Skeleton, Tooltip, Upload, Switch, InputNumber, Popconfirm, Drawer, Table, Tag } from 'antd';
import { HistoryOutlined, UploadOutlined, LockOutlined, EyeOutlined, UnlockOutlined } from '@ant-design/icons';
import type { UploadProps } from 'antd/es/upload/interface';
import type { ColumnsType } from 'antd/es/table';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createAdminArticle,
  getArticleRevision,
  listAdminArticles,
  listArticleRevisions,
  markAdminArticlePrivate,
  publishAdminArticle,
  restoreAdminArticleToDraft,
  unpublishAdminArticle,
  updateAdminArticle,
} from '../../api/articles';
import type { AdminArticlePayload, ArticleFormData, ArticleRevisionSummary } from '../../api/articles';
import { diffLines } from './revisionDiff';
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
  featured: false,
  featuredRank: 100,
  featuredAt: null,
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
    featured: values.featured ?? false,
    featuredRank: values.featuredRank ?? 100,
    featuredAt: values.featuredAt ?? null,
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
      || values.featured
      || values.featuredRank !== 100
      || Boolean(values.featuredAt)
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

const revisionStatusConfig: Record<string, { color: string; label: string }> = {
  draft: { color: 'default', label: '草稿' },
  published: { color: 'success', label: '已发布' },
  private: { color: 'purple', label: '私有' },
  archived: { color: 'warning', label: '已归档' },
};

function formatRevisionTime(value: string | null) {
  if (!value) return '-';
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

function revisionListColumns(currentVersion?: number): ColumnsType<ArticleRevisionSummary> {
  return [
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      width: 110,
      render: (version: number) =>
        version === currentVersion ? <Tag color="processing">v{version} 当前</Tag> : `v${version}`,
    },
    { title: '标题', dataIndex: 'title', key: 'title', ellipsis: true },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (status: string) => {
        const config = revisionStatusConfig[status];
        return <Tag color={config?.color}>{config?.label ?? status}</Tag>;
      },
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 150,
      render: (value: string | null) => formatRevisionTime(value),
    },
  ];
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
  const [revisionDrawerOpen, setRevisionDrawerOpen] = useState(false);
  const [selectedVersion, setSelectedVersion] = useState<number | null>(null);
  const isEdit = Boolean(id);
  const draftKey = useMemo(() => getArticleDraftKey(id), [id]);

  const articlesQuery = useQuery({
    queryKey: ['admin-articles'],
    queryFn: listAdminArticles,
    enabled: isEdit,
  });

  const revisionsQuery = useQuery({
    queryKey: ['article-revisions', id],
    queryFn: () => listArticleRevisions(id as string),
    enabled: revisionDrawerOpen && isEdit,
  });

  const revisionDetailQuery = useQuery({
    queryKey: ['article-revision', id, selectedVersion],
    queryFn: () => getArticleRevision(id as string, selectedVersion as number),
    enabled: revisionDrawerOpen && selectedVersion !== null,
  });

  const currentArticle = useMemo(
    () => articlesQuery.data?.find((article) => article.articleId === id),
    [articlesQuery.data, id],
  );
  const isPublishedArticle = currentArticle?.status === 'published';
  const isPrivateArticle = currentArticle?.status === 'private';
  const isEditableArticle = currentArticle?.status === 'draft' || currentArticle?.status === 'private';
  const isLockedArticle = Boolean(currentArticle) && !isEditableArticle;

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
        featured: currentArticle.featured ?? false,
        featuredRank: currentArticle.featuredRank ?? 100,
        featuredAt: currentArticle.featuredAt ?? null,
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
      message.success(article.status === 'private' ? '私有文章已保存' : '草稿已保存');
      await queryClient.invalidateQueries({ queryKey: ['admin-articles'] });
      await queryClient.invalidateQueries({ queryKey: ['article-revisions', id] });
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
      await queryClient.invalidateQueries({ queryKey: ['article-revisions', id] });
      navigate('/admin/articles');
    },
    onError: (error) => {
      setPublishModalOpen(false);
      message.error(error instanceof Error ? error.message : '文章发布失败');
    },
  });

  const unpublishMutation = useMutation({
    mutationFn: () => unpublishAdminArticle(id as string),
    onSuccess: async () => {
      message.success('文章已下架并恢复为草稿，修改后可再次发布');
      await queryClient.invalidateQueries({ queryKey: ['admin-articles'] });
      await queryClient.invalidateQueries({ queryKey: ['article-revisions', id] });
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '文章下架失败');
    },
  });

  const markPrivateMutation = useMutation({
    mutationFn: () => markAdminArticlePrivate(id as string),
    onSuccess: async () => {
      message.success('文章已设为私有，仅后台可见，可继续编辑');
      await queryClient.invalidateQueries({ queryKey: ['admin-articles'] });
      await queryClient.invalidateQueries({ queryKey: ['article-revisions', id] });
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '文章设为私有失败');
    },
  });

  const restoreDraftMutation = useMutation({
    mutationFn: () => restoreAdminArticleToDraft(id as string),
    onSuccess: async () => {
      message.success('私有文章已转为草稿，如需公开可点击发布');
      await queryClient.invalidateQueries({ queryKey: ['admin-articles'] });
      await queryClient.invalidateQueries({ queryKey: ['article-revisions', id] });
    },
    onError: (error) => {
      message.error(error instanceof Error ? error.message : '文章转为草稿失败');
    },
  });

  const revisionDetail = revisionDetailQuery.data;
  const revisionMetaChanges = useMemo(() => {
    if (!currentArticle || !revisionDetail) return [];
    const changes: Array<{ label: string; current: string; revision: string }> = [];
    const compare = (label: string, currentValue: string, revisionValue: string) => {
      if (currentValue !== revisionValue) {
        changes.push({ label, current: currentValue || '（空）', revision: revisionValue || '（空）' });
      }
    };
    compare('标题', currentArticle.title, revisionDetail.title);
    compare('英文名', currentArticle.articleName, revisionDetail.articleName);
    compare('摘要', currentArticle.summary ?? '', revisionDetail.summary ?? '');
    compare('标签', (currentArticle.tags ?? []).join('、'), (revisionDetail.tags ?? []).join('、'));
    compare('首页精选', currentArticle.featured ? '是' : '否', revisionDetail.featured ? '是' : '否');
    compare('精选排序', String(currentArticle.featuredRank ?? 100), String(revisionDetail.featuredRank ?? 100));
    compare('封面图片', currentArticle.coverImageUrl ?? '', revisionDetail.coverImageUrl ?? '');
    return changes;
  }, [currentArticle, revisionDetail]);

  const revisionBodyDiff = useMemo(() => {
    if (!revisionDetail) return [];
    return diffLines(currentArticle?.bodyMdx ?? '', revisionDetail.bodyMdx ?? '');
  }, [currentArticle, revisionDetail]);

  const handleRestoreRevision = () => {
    if (!revisionDetail || !currentArticle) return;
    Modal.confirm({
      title: `恢复到 v${revisionDetail.version}？`,
      content: '该版本内容会载入编辑器并暂存为本地草稿，不会自动保存；请检查内容后手动保存草稿。',
      okText: '载入版本',
      cancelText: '取消',
      onOk: () => {
        const restoredValues: ArticleFormData = {
          title: revisionDetail.title,
          slug: currentArticle.slug,
          articleName: revisionDetail.articleName,
          summary: revisionDetail.summary ?? '',
          tags: revisionDetail.tags ?? [],
          bodyMdx: revisionDetail.bodyMdx ?? '',
          featured: revisionDetail.featured ?? false,
          featuredRank: revisionDetail.featuredRank ?? 100,
          featuredAt: revisionDetail.featuredAt ?? null,
          coverImageUrl: revisionDetail.coverImageUrl ?? '',
        };
        form.setFieldsValue(restoredValues);
        saveArticleDraft(draftKey, restoredValues);
        setSelectedVersion(null);
        setRevisionDrawerOpen(false);
        message.success('已载入历史版本内容，确认后保存草稿');
      },
    });
  };

  const handleSaveDraft = async () => {
    if (isLockedArticle) {
      message.warning('请先下架文章恢复为草稿，再修改内容');
      return;
    }
    const values = await form.validateFields();
    saveMutation.mutate(values);
  };

  const handlePublish = async () => {
    if (isLockedArticle) {
      message.warning('请先下架文章恢复为草稿，修改后再发布');
      return;
    }
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
      <div className="article-editor__header">
        <Title level={1} className="article-editor__title">
          {isEdit ? '编辑文章' : '新建文章'}
        </Title>
        {isEdit && (
          <Space>
            <Button
              icon={<EyeOutlined />}
              onClick={() => navigate(`/admin/articles/${id}/read`)}
            >
              阅读视图
            </Button>
            <Button
              icon={<HistoryOutlined />}
              onClick={() => {
                setSelectedVersion(null);
                setRevisionDrawerOpen(true);
              }}
            >
              历史版本
            </Button>
          </Space>
        )}
      </div>

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

      {isPublishedArticle && (
        <Alert
          type="warning"
          showIcon
          title="已发布文章当前为只读编辑态"
          description="为避免直接覆盖线上内容，请先下架文章恢复为草稿，修改后再次发布。"
          action={
            <Popconfirm
              title="确认下架这篇文章？"
              description="下架后立即对公众不可见，文章恢复为草稿。"
              okText="下架"
              okButtonProps={{ danger: true }}
              cancelText="取消"
              onConfirm={() => unpublishMutation.mutate()}
            >
              <Button danger loading={unpublishMutation.isPending}>
                下架为草稿
              </Button>
            </Popconfirm>
          }
          style={{ marginBottom: 20 }}
        />
      )}

      {isPrivateArticle && (
        <Alert
          type="info"
          showIcon
          title="私有文章，仅后台可见"
          description="这篇内容不会出现在公开站点；可直接编辑保存，每次保存都会记录历史版本。如需公开，请先转为草稿再发布。"
          action={
            <Popconfirm
              title="确认转为草稿？"
              description="转为草稿后仍不会自动公开，需要再执行发布才会出现在公开站点。"
              okText="转为草稿"
              cancelText="取消"
              onConfirm={() => restoreDraftMutation.mutate()}
            >
              <Button icon={<UnlockOutlined />} loading={restoreDraftMutation.isPending}>
                转为草稿
              </Button>
            </Popconfirm>
          }
          style={{ marginBottom: 20 }}
        />
      )}

      {currentArticle?.status === 'archived' && (
        <Alert
          type="warning"
          showIcon
          title="已归档文章为只读状态"
          description="归档文章暂不支持在后台修改。"
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

            <Space size={16} align="start" wrap style={{ width: '100%', marginBottom: 20 }}>
              <Form.Item name="featured" label="首页精选" valuePropName="checked" style={{ marginBottom: 0 }}>
                <Switch
                  onChange={(checked) => {
                    if (checked && !form.getFieldValue('featuredAt')) {
                      form.setFieldsValue({ featuredAt: new Date().toISOString() });
                    }
                  }}
                />
              </Form.Item>

              <Form.Item name="featuredRank" label="精选排序" style={{ marginBottom: 0 }}>
                <InputNumber min={1} max={999} precision={0} style={{ width: 140 }} />
              </Form.Item>
            </Space>
            <Form.Item name="featuredAt" hidden>
              <Input />
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
          <Button
            onClick={handleSaveDraft}
            loading={saveMutation.isPending}
            disabled={isLockedArticle}
            style={{ borderColor: '#4F46E5', color: '#4F46E5' }}
          >
            {isPrivateArticle ? '保存' : '保存草稿'}
          </Button>
          {currentArticle?.status === 'draft' && (
            <Popconfirm
              title="确认设为私有？"
              description="设为私有后仅在后台可见，公开站点不展示；之后仍可继续编辑。"
              okText="设为私有"
              cancelText="取消"
              onConfirm={() => markPrivateMutation.mutate()}
            >
              <Button
                icon={<LockOutlined />}
                loading={markPrivateMutation.isPending}
                style={{ borderColor: '#7C3AED', color: '#7C3AED' }}
              >
                设为私有
              </Button>
            </Popconfirm>
          )}
          <Tooltip title={isPrivateArticle ? '私有文章需先转为草稿，再执行发布' : ''}>
            <span>
              <Button
                type="primary"
                size="large"
                loading={publishMutation.isPending}
                disabled={isLockedArticle || isPrivateArticle}
                style={{ backgroundColor: '#4F46E5', borderColor: '#4F46E5', fontWeight: 600 }}
                onClick={() => setPublishModalOpen(true)}
              >
                发布
              </Button>
            </span>
          </Tooltip>
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

      <Drawer
        title="历史版本"
        placement="right"
        size={560}
        open={revisionDrawerOpen}
        onClose={() => {
          setRevisionDrawerOpen(false);
          setSelectedVersion(null);
        }}
      >
        {selectedVersion === null ? (
          <Table<ArticleRevisionSummary>
            size="small"
            rowKey="version"
            loading={revisionsQuery.isLoading}
            dataSource={revisionsQuery.data ?? []}
            pagination={{ pageSize: 8, showSizeChanger: false }}
            onRow={(record) => ({
              onClick: () => setSelectedVersion(record.version),
              style: { cursor: 'pointer' },
            })}
            columns={revisionListColumns(currentArticle?.version)}
          />
        ) : (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <div className="revision-drawer__toolbar">
              <Button size="small" onClick={() => setSelectedVersion(null)}>
                ← 返回版本列表
              </Button>
              <Tooltip
                title={isEditableArticle ? '' : '请先将文章恢复为可编辑状态（草稿或私有），再载入历史版本'}
              >
                <span>
                  <Button
                    type="primary"
                    size="small"
                    style={{ backgroundColor: '#4F46E5', borderColor: '#4F46E5' }}
                    disabled={!isEditableArticle || !revisionDetail}
                    onClick={handleRestoreRevision}
                  >
                    恢复此版本
                  </Button>
                </span>
              </Tooltip>
            </div>

            {revisionDetailQuery.isLoading && <Skeleton active paragraph={{ rows: 6 }} />}
            {revisionDetailQuery.error && (
              <Alert
                type="error"
                showIcon
                title={revisionDetailQuery.error instanceof Error ? revisionDetailQuery.error.message : '修订版本读取失败'}
              />
            )}
            {revisionDetail && (
              <>
                <div>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    对比方向：当前内容 → v{revisionDetail.version}（{formatRevisionTime(revisionDetail.updatedAt)} 的快照）。
                  </Text>
                </div>

                {revisionMetaChanges.length > 0 ? (
                  <div className="revision-meta">
                    {revisionMetaChanges.map((change) => (
                      <div className="revision-meta__row" key={change.label}>
                        <span className="revision-meta__label">{change.label}</span>
                        <span className="revision-meta__value revision-meta__value--current">{change.current}</span>
                        <span className="revision-meta__arrow">→</span>
                        <span className="revision-meta__value revision-meta__value--target">{change.revision}</span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <Text type="secondary">元数据无差异。</Text>
                )}

                <div>
                  <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>
                    正文差异：绿色为 v{revisionDetail.version} 内容（恢复后生效），红色为当前内容（恢复后移除）。
                  </Text>
                  {revisionBodyDiff.every((line) => line.type === 'same') ? (
                    <Alert type="info" showIcon title="正文无差异" />
                  ) : (
                    <div className="revision-diff">
                      {revisionBodyDiff.map((line, index) => (
                        <div key={index} className={`revision-diff__line revision-diff__line--${line.type}`}>
                          {line.type === 'add' ? '+ ' : line.type === 'remove' ? '− ' : '  '}
                          {line.text || ' '}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </>
            )}
          </Space>
        )}
      </Drawer>
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
