import { Upload, Table, Button, Tag, Typography, message } from 'antd';
import { InboxOutlined, DownloadOutlined, ReloadOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type { UploadProps } from 'antd/es/upload/interface';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  allowedAssetContentTypes,
  getAdminAssetDownloadUrl,
  listAdminAssets,
  uploadAdminAsset,
} from '../../../api/assets';
import type { AssetFile } from '../../../api/assets';

const { Title, Text } = Typography;
const { Dragger } = Upload;

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
  if (bytes >= 1_000_000) return `${(bytes / 1_000_000).toFixed(1)} MB`;
  if (bytes >= 1_000) return `${(bytes / 1_000).toFixed(0)} KB`;
  return `${bytes} B`;
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

function AdminAssetsPage() {
  const queryClient = useQueryClient();
  const { data = [], isLoading, refetch } = useQuery({
    queryKey: ['admin-assets'],
    queryFn: listAdminAssets,
  });

  const handleDownload = async (asset: AssetFile) => {
    try {
      const downloadUrl = await getAdminAssetDownloadUrl(asset.assetId);
      window.open(downloadUrl, '_blank', 'noopener,noreferrer');
    } catch (error) {
      message.error(error instanceof Error ? error.message : '下载链接生成失败');
    }
  };

  const columns: ColumnsType<AssetFile> = [
    {
      title: '文件名',
      dataIndex: 'filename',
      key: 'filename',
      render: (name: string) => <Text style={{ color: '#0F172A', fontWeight: 500 }}>{name}</Text>,
    },
    {
      title: '类型',
      dataIndex: 'contentType',
      key: 'contentType',
      width: 100,
      render: (type: string) => (
        <Tag color={contentTypeColor[type] || 'default'} style={{ borderRadius: 6, fontSize: 12, lineHeight: '22px' }}>
          {contentTypeMap[type] || type}
        </Tag>
      ),
    },
    {
      title: '大小',
      dataIndex: 'byteSize',
      key: 'byteSize',
      width: 100,
      align: 'right',
      render: (size: number) => <Text style={{ color: '#64748B' }}>{formatBytes(size)}</Text>,
    },
    {
      title: '上传时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      render: (date: string) => <Text style={{ color: '#64748B' }}>{formatDate(date)}</Text>,
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      render: (_: unknown, record: AssetFile) => (
        <Button
          type="text"
          size="small"
          icon={<DownloadOutlined />}
          style={{ color: '#4F46E5', borderRadius: 8 }}
          onClick={() => handleDownload(record)}
        >
          下载
        </Button>
      ),
    },
  ];

  const draggerProps: UploadProps = {
    name: 'file',
    multiple: false,
    accept: allowedAssetContentTypes.join(','),
    showUploadList: false,
    customRequest: async (options) => {
      const file = options.file as File;
      if (!allowedAssetContentTypes.includes(file.type)) {
        message.error('不支持的文件类型');
        options.onError?.(new Error('不支持的文件类型'));
        return;
      }
      try {
        const asset = await uploadAdminAsset(file);
        message.success(`${file.name} 已上传`);
        await queryClient.invalidateQueries({ queryKey: ['admin-assets'] });
        options.onSuccess?.(asset);
      } catch (error) {
        const uploadError = error instanceof Error ? error : new Error('上传失败');
        message.error(uploadError.message);
        options.onError?.(uploadError);
      }
    },
    onDrop() {
      message.info('文件已加入上传处理');
    },
  };

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16 }}>
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
        <Button icon={<ReloadOutlined />} loading={isLoading} onClick={() => refetch()}>
          刷新
        </Button>
      </div>

      <Dragger
        {...draggerProps}
        style={{
          borderRadius: 8,
          background: '#FAFAFA',
          border: '2px dashed #E2E8F0',
          padding: '32px 0',
          marginBottom: 24,
        }}
      >
        <p style={{ fontSize: 48, color: '#4F46E5', margin: 0, lineHeight: 1, marginBottom: 12 }}>
          <InboxOutlined />
        </p>
        <p style={{ fontSize: 16, color: '#0F172A', fontWeight: 500, margin: 0, marginBottom: 8 }}>
          点击或拖拽文件到此处上传
        </p>
        <p style={{ fontSize: 13, color: '#64748B', margin: 0 }}>
          支持 PNG、JPEG、WebP、PDF、TXT、ZIP，单文件最大 20MB
        </p>
      </Dragger>

      <div
        style={{
          background: '#FFFFFF',
          borderRadius: 8,
          border: '1px solid #E2E8F0',
          boxShadow: '0 1px 3px 0 rgba(0,0,0,0.06)',
          padding: 20,
        }}
      >
        <Table
          columns={columns}
          dataSource={data}
          rowKey="assetId"
          loading={isLoading}
          pagination={{ pageSize: 10 }}
          style={{ borderRadius: 8 }}
          onRow={() => ({ style: { height: 48 } })}
        />
      </div>
    </div>
  );
}

export default AdminAssetsPage;
