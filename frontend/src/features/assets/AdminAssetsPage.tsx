import { useState } from 'react';
import { Upload, Table, Button, Tag, Typography, Space, message } from 'antd';
import { InboxOutlined, DownloadOutlined, DeleteOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type { UploadProps } from 'antd/es/upload/interface';

const { Title, Text } = Typography;
const { Dragger } = Upload;

const allowedTypes = ['image/png', 'image/jpeg', 'image/webp', 'application/pdf', 'text/plain', 'application/zip'];

interface AssetFile {
  id: number;
  filename: string;
  contentType: string;
  byteSize: number;
  createdAt: string;
}

const mockFiles: AssetFile[] = [
  {
    id: 1,
    filename: 'blog-banner-hero.png',
    contentType: 'image/png',
    byteSize: 2_456_000,
    createdAt: '2026-04-28 14:30',
  },
  {
    id: 2,
    filename: 'architecture-diagram.webp',
    contentType: 'image/webp',
    byteSize: 384_000,
    createdAt: '2026-04-27 10:15',
  },
  {
    id: 3,
    filename: 'api-specification-v2.pdf',
    contentType: 'application/pdf',
    byteSize: 1_230_000,
    createdAt: '2026-04-26 16:45',
  },
  {
    id: 4,
    filename: 'deployment-guide.txt',
    contentType: 'text/plain',
    byteSize: 12_500,
    createdAt: '2026-04-25 09:20',
  },
  {
    id: 5,
    filename: 'postgres-backup-20260424.zip',
    contentType: 'application/zip',
    byteSize: 8_450_000,
    createdAt: '2026-04-24 03:00',
  },
  {
    id: 6,
    filename: 'profile-avatar.jpg',
    contentType: 'image/jpeg',
    byteSize: 156_000,
    createdAt: '2026-04-23 11:30',
  },
];

const contentTypeMap: Record<string, string> = {
  'image/png': 'PNG',
  'image/jpeg': 'JPEG',
  'image/webp': 'WebP',
  'application/pdf': 'PDF',
  'text/plain': 'TXT',
  'application/zip': 'ZIP',
};

const contentTypeColor: Record<string, string> = {
  'image/png': 'blue',
  'image/jpeg': 'blue',
  'image/webp': 'blue',
  'application/pdf': 'red',
  'text/plain': 'default',
  'application/zip': 'orange',
};

function formatBytes(bytes: number): string {
  if (bytes >= 1_000_000) {
    return `${(bytes / 1_000_000).toFixed(1)} MB`;
  }
  if (bytes >= 1_000) {
    return `${(bytes / 1_000).toFixed(0)} KB`;
  }
  return `${bytes} B`;
}

const columns: ColumnsType<AssetFile> = [
  {
    title: '文件名',
    dataIndex: 'filename',
    key: 'filename',
    render: (name: string) => (
      <Text style={{ color: '#0F172A', fontWeight: 500 }}>{name}</Text>
    ),
  },
  {
    title: '类型',
    dataIndex: 'contentType',
    key: 'contentType',
    width: 100,
    render: (type: string) => (
      <Tag
        color={contentTypeColor[type] || 'default'}
        style={{ borderRadius: 6, fontSize: 12, lineHeight: '22px' }}
      >
        {contentTypeMap[type] || type}
      </Tag>
    ),
  },
  {
    title: '大小',
    dataIndex: 'byteSize',
    key: 'byteSize',
    width: 100,
    align: 'right' as const,
    render: (size: number) => (
      <Text style={{ color: '#64748B' }}>{formatBytes(size)}</Text>
    ),
  },
  {
    title: '上传时间',
    dataIndex: 'createdAt',
    key: 'createdAt',
    width: 160,
    render: (date: string) => (
      <Text style={{ color: '#64748B' }}>{date}</Text>
    ),
  },
  {
    title: '操作',
    key: 'actions',
    width: 140,
    render: (_: unknown, record: AssetFile) => (
      <Space>
        <Button
          type="text"
          size="small"
          icon={<DownloadOutlined />}
          style={{ color: '#4F46E5', borderRadius: 8 }}
          onClick={() => message.info(`下载 ${record.filename}`)}
        >
          下载
        </Button>
        <Button
          type="text"
          size="small"
          danger
          icon={<DeleteOutlined />}
          style={{ borderRadius: 8 }}
          onClick={() => message.warning(`删除 ${record.filename}`)}
        >
          删除
        </Button>
      </Space>
    ),
  },
];

const draggerProps: UploadProps = {
  name: 'file',
  multiple: true,
  accept: allowedTypes.join(','),
  showUploadList: false,
  beforeUpload: () => false,
  onChange(info) {
    const { file } = info;
    if (file.status !== 'removed') {
      message.success(`${file.name} 已添加到上传队列`);
    }
  },
  onDrop() {
    message.info('文件已拖拽上传');
  },
};

function AdminAssetsPage() {
  const [files] = useState<AssetFile[]>(mockFiles);

  return (
    <div>
      <Title
        level={4}
        style={{
          fontSize: 24,
          fontWeight: 700,
          marginBottom: 24,
          marginTop: 0,
          color: '#0F172A',
        }}
      >
        资源管理
      </Title>

      <Dragger
        {...draggerProps}
        style={{
          borderRadius: 24,
          background: '#FAFAFA',
          border: '2px dashed #E2E8F0',
          padding: '32px 0',
          marginBottom: 24,
        }}
      >
        <p
          style={{
            fontSize: 48,
            color: '#4F46E5',
            margin: 0,
            lineHeight: 1,
            marginBottom: 12,
          }}
        >
          <InboxOutlined />
        </p>
        <p
          style={{
            fontSize: 16,
            color: '#0F172A',
            fontWeight: 500,
            margin: 0,
            marginBottom: 8,
          }}
        >
          点击或拖拽文件到此处上传
        </p>
        <p
          style={{
            fontSize: 13,
            color: '#64748B',
            margin: 0,
          }}
        >
          支持 PNG、JPEG、WebP、PDF、TXT、ZIP，单文件最大 20MB
        </p>
      </Dragger>

      <div
        style={{
          background: '#FFFFFF',
          borderRadius: 24,
          border: '1px solid #E2E8F0',
          boxShadow: '0 1px 3px 0 rgba(0,0,0,0.06)',
          padding: 20,
        }}
      >
        <Table
          columns={columns}
          dataSource={files}
          rowKey="id"
          pagination={false}
          style={{ borderRadius: 8 }}
          onRow={() => ({
            style: { height: 48 },
          })}
        />
      </div>
    </div>
  );
}

export default AdminAssetsPage;
