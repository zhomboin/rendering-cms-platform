import { Progress, Tag, Typography } from 'antd';

export interface UploadProgressItem {
  uid: string;
  name: string;
  percent?: number;
  status: 'uploading' | 'processing' | 'done' | 'error';
  error?: string;
}

const statusLabels = {
  uploading: '上传中',
  processing: '处理中',
  done: '已上传',
  error: '上传失败',
};

const statusColors = {
  uploading: 'processing',
  processing: 'processing',
  done: 'success',
  error: 'error',
};

export function uploadTransferProgress(
  loaded: number,
  total?: number,
): Pick<UploadProgressItem, 'percent' | 'status'> {
  if (!total || total <= 0) return { status: 'uploading' };
  const percent = Math.min(100, Math.max(0, Math.floor((loaded / total) * 100)));
  return { percent, status: percent === 100 ? 'processing' : 'uploading' };
}

export function UploadProgressList({ items }: { items: UploadProgressItem[] }) {
  if (!items.length) return null;

  return (
    <div className="cms-upload-progress" role="list" aria-label="上传进度">
      {items.map(({ uid, name, percent, status, error }) => (
        <div key={uid} role="listitem" className="cms-upload-row">
          <div className="cms-upload-row-meta">
            <Typography.Text ellipsis title={name} className="cms-upload-name">
              {name}
            </Typography.Text>
            {percent !== undefined && <span className="cms-upload-percent">{percent}%</span>}
            <Tag color={statusColors[status]} role="status">
              {statusLabels[status]}
            </Tag>
          </div>
          {percent !== undefined && (
            <Progress
              percent={percent}
              aria-label={`${name}的传输进度`}
              status={status === 'error' ? 'exception' : status === 'done' ? 'success' : 'normal'}
              showInfo={false}
              size={['100%', 7]}
            />
          )}
          {error && (
            <Typography.Text type="danger" className="cms-upload-error">
              {error}
            </Typography.Text>
          )}
        </div>
      ))}
    </div>
  );
}
