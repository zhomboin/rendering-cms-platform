import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Table, Button, Tag, Input, Select, Space, Typography, Alert, Popconfirm, message } from 'antd';
import { PlusOutlined, EditOutlined, ReloadOutlined, StopOutlined, EyeOutlined, LockOutlined, UnlockOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { listAdminArticles, unpublishAdminArticle, markAdminArticlePrivate, restoreAdminArticleToDraft } from '../../api/articles';
import type { AdminArticleRecord } from '../../api/articles';

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
  private: { color: 'purple', label: '私有' },
  archived: { color: 'warning', label: '已归档' },
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
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [searchText, setSearchText] = useState<string>('');
  const { data = [], isLoading, error, refetch } = useQuery({
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
        const matchTitle = !keyword
          || article.title.toLowerCase().includes(keyword)
          || article.slug.toLowerCase().includes(keyword)
          || article.articleName.toLowerCase().includes(keyword);
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
        <a onClick={() => navigate(`/admin/articles/${record.articleId}/edit`)} style={{ color: '#4F46E5', fontWeight: 500 }}>
          {text}
        </a>
      ),
    },
    {
      title: '英文名',
      dataIndex: 'articleName',
      key: 'articleName',
      width: 220,
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
      width: 110,
      render: (status: AdminArticleRecord['status']) => {
        const cfg = statusConfig[status];
        return <Tag color={cfg.color}>{cfg.label}</Tag>;
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
      width: 180,
      render: (val: string | null) => formatDate(val),
    },
    {
      title: '操作',
      key: 'action',
      width: 240,
      render: (_: unknown, record: AdminArticleRecord) => (
        <Space size={0}>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={(e) => { e.stopPropagation(); navigate(`/admin/articles/${record.articleId}/read`); }}>
            阅读
          </Button>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={(e) => { e.stopPropagation(); navigate(`/admin/articles/${record.articleId}/edit`); }}>
            编辑
          </Button>
          {record.status === 'draft' && (
            <Popconfirm
              title="确认设为私有？"
              description="设为私有后仅在后台可见，公开站点不展示；可随时转为草稿再发布。"
              okText="设为私有"
              cancelText="取消"
              onConfirm={(e) => {
                e?.stopPropagation();
                markPrivateMutation.mutate(record.articleId);
              }}
              onCancel={(e) => e?.stopPropagation()}
            >
              <Button type="link" size="small" icon={<LockOutlined />} onClick={(e) => e.stopPropagation()}>
                私有
              </Button>
            </Popconfirm>
          )}
          {record.status === 'private' && (
            <Popconfirm
              title="确认转为草稿？"
              description="转为草稿后仍不会自动公开，需要再执行发布才会出现在公开站点。"
              okText="转为草稿"
              cancelText="取消"
              onConfirm={(e) => {
                e?.stopPropagation();
                restoreDraftMutation.mutate(record.articleId);
              }}
              onCancel={(e) => e?.stopPropagation()}
            >
              <Button type="link" size="small" icon={<UnlockOutlined />} onClick={(e) => e.stopPropagation()}>
                转草稿
              </Button>
            </Popconfirm>
          )}
          {record.status === 'published' && (
            <Popconfirm
              title="确认下架这篇文章？"
              description="下架后立即对公众不可见，文章恢复为草稿，可修改后再次发布。"
              okText="下架"
              okButtonProps={{ danger: true }}
              cancelText="取消"
              onConfirm={(e) => {
                e?.stopPropagation();
                unpublishMutation.mutate(record.articleId);
              }}
              onCancel={(e) => e?.stopPropagation()}
            >
              <Button type="link" size="small" danger icon={<StopOutlined />} onClick={(e) => e.stopPropagation()}>
                下架
              </Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24, gap: 16 }}>
        <Title level={1} style={{ margin: 0, fontSize: 24, fontWeight: 700 }}>
          文章管理
        </Title>
        <Space>
          <Button icon={<ReloadOutlined />} loading={isLoading} onClick={() => refetch()}>
            刷新
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            style={{ backgroundColor: '#4F46E5', borderColor: '#4F46E5' }}
            onClick={() => navigate('/admin/articles/new')}
          >
            新建文章
          </Button>
        </Space>
      </div>

      {error && (
        <Alert type="error" showIcon message={error instanceof Error ? error.message : '后台文章列表读取失败'} style={{ marginBottom: 20 }} />
      )}

      <div style={{ display: 'flex', gap: 12, marginBottom: 20, flexWrap: 'wrap' }}>
        <Select value={statusFilter} onChange={setStatusFilter} options={statusOptions} style={{ width: 140 }} placeholder="状态筛选" />
        <Search
          placeholder="搜索标题、英文名或短链"
          allowClear
          onSearch={setSearchText}
          onChange={(e) => {
            if (!e.target.value) setSearchText('');
          }}
          style={{ width: 280 }}
        />
      </div>

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
        scroll={{ x: 1120 }}
        onRow={(record) => ({
          onClick: () => navigate(`/admin/articles/${record.articleId}/edit`),
          style: { cursor: 'pointer', height: 48 },
        })}
        style={{ borderCollapse: 'collapse' }}
      />
    </div>
  );
}
