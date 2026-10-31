import { useEffect, useRef, useState } from 'react';
import { Button, Progress, Space, Typography, Upload, message } from 'antd';
import { PictureOutlined } from '@ant-design/icons';
import type { RcFile, UploadRequestOption } from '@rc-component/upload/lib/interface';
import { photoContentTypes, uploadAdminAlbumPhoto } from '../../api/albums';

const { Text } = Typography;
const maxPhotoBytes = 20 * 1024 * 1024;

interface UploadItem {
  uid: string;
  name: string;
  percent?: number;
  status: 'uploading' | 'processing' | 'done' | 'error';
  error?: string;
}

interface AlbumPhotoUploadProps {
  albumId: string;
  disabled: boolean;
  onUploaded: () => void;
}

const statusLabels = {
  uploading: '上传中',
  processing: '处理中',
  done: '已上传',
  error: '上传失败',
};

export default function AlbumPhotoUpload({ albumId, disabled, onUploaded }: AlbumPhotoUploadProps) {
  const [uploads, setUploads] = useState<UploadItem[]>([]);
  const controllers = useRef(new Map<string, AbortController>());

  useEffect(() => {
    const active = controllers.current;
    return () => {
      for (const controller of active.values()) controller.abort();
      active.clear();
    };
  }, []);

  const updateUpload = (uid: string, changes: Partial<UploadItem>) => {
    setUploads(items => items.map(item => item.uid === uid ? { ...item, ...changes } : item));
  };

  const handleUpload = (options: UploadRequestOption) => {
    const file = options.file as RcFile;
    const controller = new AbortController();
    controllers.current.set(file.uid, controller);
    setUploads(items => [...items, { uid: file.uid, name: file.name, status: 'uploading' }]);
    const alt = file.name.replace(/\.[^.]+$/, '').replace(/[-_]+/g, ' ').trim() || file.name;

    void uploadAdminAlbumPhoto(albumId, file, { alt }, {
      signal: controller.signal,
      onProgress: ({ loaded, total }) => {
        if (controller.signal.aborted) return;
        const percent = total && total > 0
          ? Math.min(100, Math.floor(loaded / total * 100))
          : undefined;
        updateUpload(file.uid, {
          percent,
          status: total && loaded >= total ? 'processing' : 'uploading',
        });
        if (percent !== undefined) options.onProgress?.({ percent });
      },
    }).then(photo => {
      if (controller.signal.aborted) return;
      updateUpload(file.uid, { status: 'done', percent: 100 });
      options.onSuccess?.(photo);
      onUploaded();
    }).catch((error: unknown) => {
      if (controller.signal.aborted) return;
      const uploadError = error instanceof Error ? error : new Error('照片上传失败');
      updateUpload(file.uid, { status: 'error', error: uploadError.message });
      options.onError?.(uploadError);
    }).finally(() => {
      controllers.current.delete(file.uid);
    });
    return { abort: () => controller.abort() };
  };

  return (
    <>
      <Upload.Dragger
        multiple
        showUploadList={false}
        // 记录由本组件维护，避免隐藏列表长期持有原始文件。
        fileList={[]}
        accept={photoContentTypes.join(',')}
        disabled={disabled}
        beforeUpload={file => {
          if (!photoContentTypes.includes(file.type)) {
            message.error(file.name + ' 不是 PNG/JPEG/WebP 图片');
            return Upload.LIST_IGNORE;
          }
          if (file.size > maxPhotoBytes) {
            message.error(file.name + ' 超过 20MB 上限');
            return Upload.LIST_IGNORE;
          }
          return true;
        }}
        customRequest={handleUpload}
      >
        <p className="ant-upload-drag-icon"><PictureOutlined /></p>
        <p className="ant-upload-text">点击或拖拽照片到这里上传</p>
        <p className="ant-upload-hint">PNG / JPEG / WebP，单张不超过 20MB</p>
      </Upload.Dragger>
      {uploads.length > 0 && (
        <div style={{ marginTop: 16, minWidth: 0 }}>
          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button
              type="text"
              disabled={!uploads.some(item => item.status === 'done' || item.status === 'error')}
              onClick={() => setUploads(items => items.filter(item => item.status === 'uploading' || item.status === 'processing'))}
            >
              清除记录
            </Button>
          </div>
          <div role="list" aria-label="上传进度" style={{ maxHeight: 280, overflowY: 'auto', minWidth: 0 }}>
            {uploads.map(item => (
              <div key={item.uid} role="listitem" style={{ padding: '8px 0', minWidth: 0 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 16, minWidth: 0 }}>
                  <Text ellipsis title={item.name} style={{ flex: 1, minWidth: 0 }}>{item.name}</Text>
                  <Space size={8} style={{ flexShrink: 0 }}>
                    {item.percent !== undefined && <Text type="secondary">{item.percent}%</Text>}
                    <Text type={item.status === 'error' ? 'danger' : item.status === 'done' ? 'success' : 'secondary'}>
                      {statusLabels[item.status]}
                    </Text>
                  </Space>
                </div>
                <Progress
                  percent={item.percent ?? 0}
                  status={item.status === 'error' ? 'exception' : item.status === 'done' ? 'success' : 'active'}
                  showInfo={false}
                  size="small"
                />
                {item.error && <Text type="danger" style={{ overflowWrap: 'anywhere' }}>{item.error}</Text>}
              </div>
            ))}
          </div>
        </div>
      )}
    </>
  );
}
