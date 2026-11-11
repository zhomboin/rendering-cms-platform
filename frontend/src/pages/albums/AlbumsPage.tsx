import { useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  App,
  Table,
  Button,
  Tag,
  Input,
  Select,
  Space,
  Typography,
  Alert,
  Popconfirm,
  Image,
} from 'antd';
import {
  PlusOutlined,
  PictureOutlined,
  EditOutlined,
  ReloadOutlined,
  StopOutlined,
  CloudUploadOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  deleteAdminAlbum,
  listAdminAlbums,
  publishAdminAlbum,
  unpublishAdminAlbum,
} from '../../api/albums';
import type { AdminAlbum } from '../../api/albums';

const { Title } = Typography;
const { Search } = Input;

const statusOptions = [
  { value: '', label: '全部' },
  { value: 'draft', label: '草稿' },
  { value: 'published', label: '已发布' },
];

const statusConfig: Record<AdminAlbum['status'], { color: string; label: string }> = {
  draft: { color: 'default', label: '草稿' },
  published: { color: 'success', label: '已发布' },
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

export default function AlbumsPage() {
  const { message } = App.useApp();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [searchText, setSearchText] = useState<string>('');
  const {
    data = [],
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['admin-albums'],
    queryFn: listAdminAlbums,
  });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['admin-albums'] });

  const publishMutation = useMutation({
    mutationFn: publishAdminAlbum,
    onSuccess: () => {
      message.success('相册已发布');
      void invalidate();
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '相册发布失败');
    },
  });

  const unpublishMutation = useMutation({
    mutationFn: unpublishAdminAlbum,
    onSuccess: () => {
      message.success('相册已下架');
      void invalidate();
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '相册下架失败');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteAdminAlbum,
    onSuccess: () => {
      message.success('相册已删除');
      void invalidate();
    },
    onError: (mutationError) => {
      message.error(mutationError instanceof Error ? mutationError.message : '相册删除失败');
    },
  });

  const filteredAlbums = useMemo(
    () =>
      data.filter((album) => {
        const matchStatus = !statusFilter || album.status === statusFilter;
        const keyword = searchText.trim().toLowerCase();
        const matchTitle =
          !keyword ||
          album.title.toLowerCase().includes(keyword) ||
          album.slug.toLowerCase().includes(keyword);
        return matchStatus && matchTitle;
      }),
    [data, searchText, statusFilter],
  );

  const columns: ColumnsType<AdminAlbum> = [
    {
      title: '封面',
      dataIndex: 'coverUrl',
      key: 'coverUrl',
      width: 96,
      render: (url: string | null, record: AdminAlbum) =>
        url ? (
          <div onClick={(event) => event.stopPropagation()}>
            <Image
              src={url}
              alt="相册封面"
              width={64}
              height={44}
              style={{ objectFit: 'cover', borderRadius: 6 }}
              aria-label={'预览' + record.title + '封面'}
              preview={{ cover: '预览' }}
            />
          </div>
        ) : (
          <div
            style={{
              width: 64,
              height: 44,
              borderRadius: 6,
              background: 'var(--cms-well)',
              display: 'grid',
              placeItems: 'center',
              color: 'var(--cms-muted)',
              fontSize: 12,
            }}
          >
            无封面
          </div>
        ),
    },
    {
      title: '标题',
      dataIndex: 'title',
      key: 'title',
      render: (text: string, record: AdminAlbum) => (
        <Link
          className="cms-table-title"
          to={'/admin/albums/' + record.albumId + '/edit'}
          onClick={(event) => event.stopPropagation()}
        >
          {text}
        </Link>
      ),
    },
    {
      title: 'slug',
      dataIndex: 'slug',
      key: 'slug',
      width: 180,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: AdminAlbum['status']) => {
        const cfg = statusConfig[status];
        return <Tag color={cfg.color}>{cfg.label}</Tag>;
      },
    },
    {
      title: '照片数',
      dataIndex: 'photoCount',
      key: 'photoCount',
      width: 90,
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
      render: (val: string | null) => formatDate(val),
    },
    {
      title: '操作',
      key: 'action',
      width: 340,
      render: (_: unknown, record: AdminAlbum) => (
        <Space size={0}>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={(e) => {
              e.stopPropagation();
              navigate(`/admin/albums/${record.albumId}/edit`);
            }}
          >
            基本信息
          </Button>
          <Button
            aria-label="图片管理"
            type="link"
            size="small"
            icon={<PictureOutlined />}
            onClick={(e) => {
              e.stopPropagation();
              navigate(`/admin/albums/${record.albumId}/photos`);
            }}
          >
            图片管理
          </Button>
          {record.status === 'draft' && (
            <Button
              type="link"
              size="small"
              icon={<CloudUploadOutlined />}
              onClick={(e) => {
                e.stopPropagation();
                publishMutation.mutate(record.albumId);
              }}
            >
              发布
            </Button>
          )}
          {record.status === 'published' && (
            <Popconfirm
              title="确认下架该相册？"
              description="下架后不再公开展示，可再次发布。"
              okText="下架"
              okButtonProps={{ danger: true }}
              cancelText="取消"
              onConfirm={(e) => {
                e?.stopPropagation();
                unpublishMutation.mutate(record.albumId);
              }}
              onCancel={(e) => e?.stopPropagation()}
            >
              <Button
                type="link"
                size="small"
                danger
                icon={<StopOutlined />}
                onClick={(e) => e.stopPropagation()}
              >
                下架
              </Button>
            </Popconfirm>
          )}
          <Popconfirm
            title="确认删除该相册？"
            description="相册及照片将被删除，此操作不可恢复。"
            okText="删除"
            okButtonProps={{ danger: true }}
            cancelText="取消"
            onConfirm={(e) => {
              e?.stopPropagation();
              deleteMutation.mutate(record.albumId);
            }}
            onCancel={(e) => e?.stopPropagation()}
          >
            <Button
              type="link"
              size="small"
              danger
              icon={<DeleteOutlined />}
              onClick={(e) => e.stopPropagation()}
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div className="cms-page-heading">
        <Title level={1}>相册管理</Title>
        <Space>
          <Button icon={<ReloadOutlined />} loading={isLoading} onClick={() => refetch()}>
            刷新
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => navigate('/admin/albums/new')}
          >
            新建相册
          </Button>
        </Space>
      </div>

      {error && (
        <Alert
          type="error"
          showIcon
          message={error instanceof Error ? error.message : '后台相册列表读取失败'}
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
          placeholder="搜索标题或短链"
          allowClear
          onSearch={setSearchText}
          onChange={(e) => {
            if (!e.target.value) setSearchText('');
          }}
          style={{ width: 280, maxWidth: '100%' }}
        />
      </div>

      <Table<AdminAlbum>
        columns={columns}
        dataSource={filteredAlbums}
        rowKey="albumId"
        loading={isLoading}
        pagination={{
          pageSize: 10,
          showSizeChanger: false,
          showTotal: (total) => `共 ${total} 个相册`,
        }}
        scroll={{ x: 1120 }}
        onRow={(record) => ({
          onClick: () => navigate(`/admin/albums/${record.albumId}/edit`),
          style: { cursor: 'pointer', height: 56 },
        })}
      />
    </div>
  );
}
