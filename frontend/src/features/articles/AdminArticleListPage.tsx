import { useNavigate } from 'react-router-dom';
import { Table, Button, Tag, Input, Select, Space, Typography, Popconfirm } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';

const { Title } = Typography;
const { Search } = Input;

interface ArticleItem {
  id: number;
  title: string;
  slug: string;
  status: 'draft' | 'published' | 'archived';
  tags: string[];
  publishedAt: string | null;
}

const mockArticles: ArticleItem[] = [
  {
    id: 1,
    title: 'Go 语言并发编程实战',
    slug: 'go-concurrency-in-practice',
    status: 'published',
    tags: ['Go', '并发'],
    publishedAt: '2026-04-28 10:00',
  },
  {
    id: 2,
    title: 'React 19 新特性解读',
    slug: 'react-19-new-features',
    status: 'published',
    tags: ['React', '前端'],
    publishedAt: '2026-04-25 14:30',
  },
  {
    id: 3,
    title: '使用 Chi 构建 RESTful API',
    slug: 'building-rest-api-with-chi',
    status: 'draft',
    tags: ['Go', 'API'],
    publishedAt: null,
  },
  {
    id: 4,
    title: 'PostgreSQL 索引优化指南',
    slug: 'postgresql-index-optimization',
    status: 'published',
    tags: ['PostgreSQL', '数据库'],
    publishedAt: '2026-04-20 09:15',
  },
  {
    id: 5,
    title: '深入理解 MDX 与内容管理',
    slug: 'understanding-mdx-content-management',
    status: 'draft',
    tags: ['MDX', 'CMS'],
    publishedAt: null,
  },
  {
    id: 6,
    title: 'TypeScript 5.0 装饰器完全指南',
    slug: 'typescript-5-decorators-guide',
    status: 'archived',
    tags: ['TypeScript'],
    publishedAt: '2026-03-15 11:00',
  },
  {
    id: 7,
    title: 'Docker 多阶段构建最佳实践',
    slug: 'docker-multi-stage-build-best-practices',
    status: 'published',
    tags: ['Docker', 'DevOps'],
    publishedAt: '2026-04-10 16:45',
  },
  {
    id: 8,
    title: '使用 sqlc 生成类型安全 Go 代码',
    slug: 'type-safe-go-with-sqlc',
    status: 'draft',
    tags: ['Go', 'sqlc', '数据库'],
    publishedAt: null,
  },
];

const statusOptions = [
  { value: '', label: '全部' },
  { value: 'draft', label: '草稿' },
  { value: 'published', label: '已发布' },
  { value: 'archived', label: '已归档' },
];

const statusConfig: Record<
  ArticleItem['status'],
  { color: string; label: string }
> = {
  draft: { color: 'default', label: '草稿' },
  published: { color: 'success', label: '已发布' },
  archived: { color: 'warning', label: '已归档' },
};

export default function AdminArticleListPage() {
  const navigate = useNavigate();
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [searchText, setSearchText] = useState<string>('');

  const filteredArticles = mockArticles.filter((article) => {
    const matchStatus = !statusFilter || article.status === statusFilter;
    const matchTitle =
      !searchText ||
      article.title.toLowerCase().includes(searchText.toLowerCase());
    return matchStatus && matchTitle;
  });

  const handleDelete = (id: number) => {
    console.log('Delete article', id);
  };

  const columns: ColumnsType<ArticleItem> = [
    {
      title: '标题',
      dataIndex: 'title',
      key: 'title',
      render: (text: string, record: ArticleItem) => (
        <a
          onClick={() => navigate(`/admin/articles/${record.id}/edit`)}
          style={{ color: '#4F46E5', fontWeight: 500 }}
        >
          {text}
        </a>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 110,
      render: (status: ArticleItem['status']) => {
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
      width: 170,
      render: (val: string | null) => val ?? '-',
    },
    {
      title: '操作',
      key: 'action',
      width: 140,
      render: (_: unknown, record: ArticleItem) => (
        <Space size={8}>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => navigate(`/admin/articles/${record.id}/edit`)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确认删除"
            description="确定要删除这篇文章吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确认"
            cancelText="取消"
          >
            <Button type="link" size="small" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: 24,
        }}
      >
        <Title level={1} style={{ margin: 0, fontSize: 24, fontWeight: 700 }}>
          文章管理
        </Title>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          style={{ backgroundColor: '#4F46E5', borderColor: '#4F46E5' }}
          onClick={() => navigate('/admin/articles/new')}
        >
          新建文章
        </Button>
      </div>

      <div
        style={{
          display: 'flex',
          gap: 12,
          marginBottom: 20,
          flexWrap: 'wrap',
        }}
      >
        <Select
          value={statusFilter}
          onChange={setStatusFilter}
          options={statusOptions}
          style={{ width: 140 }}
          placeholder="状态筛选"
        />
        <Search
          placeholder="搜索文章标题..."
          allowClear
          onSearch={setSearchText}
          onChange={(e) => {
            if (!e.target.value) setSearchText('');
          }}
          style={{ width: 280 }}
        />
      </div>

      <Table<ArticleItem>
        columns={columns}
        dataSource={filteredArticles}
        rowKey="id"
        pagination={{
          pageSize: 10,
          showSizeChanger: false,
          showTotal: (total) => `共 ${total} 篇文章`,
        }}
        onRow={(record) => ({
          onClick: () => navigate(`/admin/articles/${record.id}/edit`),
          style: { cursor: 'pointer', height: 48 },
        })}
        rowClassName={() => 'admin-article-row'}
        style={{ borderCollapse: 'collapse' }}
      />
    </div>
  );
}
