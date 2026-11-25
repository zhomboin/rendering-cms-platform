import { useMemo, useRef, useState } from 'react';
import { createSearchParams, Link, useNavigate } from 'react-router-dom';
import { App, Table, Button, Tag, Input, Select, Space, Typography, Alert, Dropdown } from 'antd';
import type { MenuProps } from 'antd';
import {
  DeleteOutlined,
  PlusOutlined,
  ReloadOutlined,
  StopOutlined,
  LockOutlined,
  UnlockOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  listAdminArticles,
  unpublishAdminArticle,
  markAdminArticlePrivate,
  restoreAdminArticleToDraft,
} from '../../api/articles';
import type { AdminArticleRecord } from '../../api/articles';
import { getAuthUser } from '../../api/auth';
import { ArticleDeleteDialog, type DeleteTarget } from './ArticleDeleteDialog';
import './ArticleListPage.css';

const { Title } = Typography;
const { Search } = Input;

const statusOptions = [
  { value: '', label: '全部' },
  { value: 'draft', label: '草稿' },
  { value: 'published', label: '已发布' },
  { value: 'private', label: '私有' },
  { value: 'archived', label: '已归档' },
];

const statusConfig: Record<AdminArticleRecord['status'], { color: string; label: string }> = {
  draft: { color: 'default', label: '草稿' },
  published: { color: 'success', label: '已发布' },
  private: { color: 'processing', label: '私有' },
  archived: { color: 'default', label: '已归档' },
};

function formatDate(value: string | null) {
  if (!value) return '-';
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

export default function ArticleListPage() {
  const { message, modal } = App.useApp();
  const navigate = useNavigate();
  const isAdmin = getAuthUser()?.role === 'admin';
  const queryClient = useQueryClient();
  const refreshButton = useRef<HTMLButtonElement>(null);
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [searchText, setSearchText] = useState<string>('');
  const {
    data = [],
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['admin-articles'],
    queryFn: listAdminArticles,
  });

  const unpublishMutation = useMutation({
    mutationFn: unpublishAdminArticle,
    onSuccess: () => {
      message.success('文章已下架并恢复为草稿');
      void queryClient.invalidateQueries({ queryKey: ['admin-articles'] });
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '文章下架失败');
    },
  });

  const markPrivateMutation = useMutation({
    mutationFn: markAdminArticlePrivate,
    onSuccess: () => {
      message.success('文章已设为私有，仅后台可见');
      void queryClient.invalidateQueries({ queryKey: ['admin-articles'] });
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '文章设为私有失败');
    },
  });

  const restoreDraftMutation = useMutation({
    mutationFn: restoreAdminArticleToDraft,
    onSuccess: () => {
      message.success('私有文章已转为草稿');
      void queryClient.invalidateQueries({ queryKey: ['admin-articles'] });
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '文章转为草稿失败');
    },
  });

  const filteredArticles = useMemo(
    () =>
      data.filter((article) => {
        const matchStatus = !statusFilter || article.status === statusFilter;
        const keyword = searchText.trim().toLowerCase();
        const matchTitle =
          !keyword ||
          article.title.toLowerCase().includes(keyword) ||
          article.slug.toLowerCase().includes(keyword) ||
          article.articleName.toLowerCase().includes(keyword);
        return matchStatus && matchTitle;
      }),
    [data, searchText, statusFilter],
  );

  const columns: ColumnsType<AdminArticleRecord> = [
    {
      title: '标题',
      dataIndex: 'title',
      key: 'title',
      render: (text: string, record: AdminArticleRecord) => (
        <Link className="cms-table-title" to={'/admin/articles/' + record.articleId + '/edit'}>
          {text}
        </Link>
      ),
    },
    {
      title: '英文名',
      dataIndex: 'articleName',
      key: 'articleName',
      width: 168,
    },
    {
      title: '短链',
      dataIndex: 'slug',
      key: 'slug',
      width: 100,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 96,
      render: (status: AdminArticleRecord['status']) => {
        const cfg = statusConfig[status];
        return (
          <Tag color={cfg.color} icon={status === 'private' ? <LockOutlined /> : undefined}>
            {cfg.label}
          </Tag>
        );
      },
    },
    {
      title: '标签',
      dataIndex: 'tags',
      key: 'tags',
      render: (tags: string[]) => (
        <Space size={4} wrap>
          {tags.map((tag) => (
            <Tag key={tag} style={{ marginRight: 0 }}>
              {tag}
            </Tag>
          ))}
        </Space>
      ),
    },
    {
      title: '发布时间',
      dataIndex: 'publishedAt',
      key: 'publishedAt',
      width: 152,
      render: (val: string | null) => formatDate(val),
    },
    {
      title: '操作',
      key: 'action',
      width: 196,
      render: (_: unknown, record: AdminArticleRecord) => {
        const articlePath = `/admin/articles/${record.articleId}`;
        const items: MenuProps['items'] = [];

        if (record.status === 'draft') {
          items.push({
            key: 'private',
            icon: <LockOutlined aria-hidden="true" />,
            label: '设为私有',
            onClick: () =>
              modal.confirm({
                title: '确认设为私有？',
                content: '设为私有后仅在后台可见，可转为草稿再发布。',
                okText: '设为私有',
                cancelText: '取消',
                onOk: () => markPrivateMutation.mutateAsync(record.articleId),
              }),
          });
        }
        if (record.status === 'private') {
          items.push({
            key: 'draft',
            icon: <UnlockOutlined aria-hidden="true" />,
            label: '转为草稿',
            onClick: () =>
              modal.confirm({
                title: '确认转为草稿？',
                content: '转为草稿后需要再执行发布才会出现在公开站点。',
                okText: '转为草稿',
                cancelText: '取消',
                onOk: () => restoreDraftMutation.mutateAsync(record.articleId),
              }),
          });
        }
        if (record.status === 'published') {
          if (isAdmin && record.version) {
            items.push(
              {
                key: 'csdn',
                label: (
                  <Link
                    to={{
                      pathname: '/admin/postbridge',
                      search: createSearchParams({
                        articleId: record.articleId,
                        prepare: '1',
                      }).toString(),
                    }}
                  >
                    保存到 CSDN 草稿箱
                  </Link>
                ),
              },
              { type: 'divider' },
            );
          }
          items.push({
            key: 'unpublish',
            label: '下架',
            icon: <StopOutlined aria-hidden="true" />,
            danger: true,
            onClick: () =>
              modal.confirm({
                title: '确认下架这篇文章？',
                content: `「${record.title}」将恢复为草稿，并立即停止公开展示。`,
                okText: '下架',
                okButtonProps: { danger: true },
                cancelText: '取消',
                onOk: () => unpublishMutation.mutateAsync(record.articleId),
              }),
          });
        }

        const version = record.version;
        if (isAdmin && version !== undefined) {
          items.push({
            key: 'delete',
            icon: <DeleteOutlined aria-hidden="true" />,
            label: '删除文章',
            danger: true,
            onClick: () =>
              setDeleteTarget({ articleId: record.articleId, title: record.title, version }),
          });
        }

        return (
          <Space size={0} className="article-list-actions">
            <Button type="link" size="small" onClick={() => navigate(`${articlePath}/read`)}>
              阅读
            </Button>
            <Button type="link" size="small" onClick={() => navigate(`${articlePath}/edit`)}>
              编辑
            </Button>
            {items.length > 0 && (
              <Dropdown autoFocus destroyOnHidden trigger={['click']} menu={{ items }}>
                <Button
                  type="text"
                  size="small"
                  id={`article-actions-${record.articleId}`}
                  aria-label={`更多操作：${record.title}`}
                  aria-haspopup="menu"
                >
                  更多
                </Button>
              </Dropdown>
            )}
          </Space>
        );
      },
    },
  ];

  return (
    <div>
      <div className="cms-page-heading">
        <Title level={1}>文章管理</Title>
        <Space>
          <Button
            ref={refreshButton}
            icon={<ReloadOutlined />}
            loading={isLoading}
            onClick={() => refetch()}
          >
            刷新
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => navigate('/admin/articles/new')}
          >
            新建文章
          </Button>
        </Space>
      </div>

      {error && (
        <Alert
          type="error"
          showIcon
          message={error instanceof Error ? error.message : '后台文章列表读取失败'}
          style={{ marginBottom: 20 }}
        />
      )}

      <div className="cms-list-filters">
        <Select
          aria-label="状态筛选"
          value={statusFilter}
          onChange={setStatusFilter}
          options={statusOptions}
          style={{ width: 140 }}
          placeholder="状态筛选"
        />
        <Search
          placeholder="搜索标题、英文名或短链"
          allowClear
          onSearch={setSearchText}
          onChange={(e) => {
            if (!e.target.value) setSearchText('');
          }}
          style={{ width: 280, maxWidth: '100%' }}
        />
      </div>

      {deleteTarget && (
        <ArticleDeleteDialog
          target={deleteTarget}
          onClose={() => {
            const trigger = document.getElementById(`article-actions-${deleteTarget.articleId}`);
            (trigger ?? refreshButton.current)?.focus();
            setDeleteTarget(null);
          }}
        />
      )}

      <Table<AdminArticleRecord>
        columns={columns}
        dataSource={filteredArticles}
        rowKey="articleId"
        loading={isLoading}
        pagination={{
          pageSize: 10,
          showSizeChanger: false,
          showTotal: (total) => `共 ${total} 篇文章`,
        }}
        scroll={{ x: 960 }}
      />
    </div>
  );
}
