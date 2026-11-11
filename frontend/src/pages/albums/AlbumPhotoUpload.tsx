import { useEffect, useRef, useState } from 'react';
import { App, Button, Upload } from 'antd';
import { PictureOutlined } from '@ant-design/icons';
import type { RcFile, UploadRequestOption } from '@rc-component/upload/lib/interface';
import { photoContentTypes, uploadAdminAlbumPhoto } from '../../api/albums';
import { validateUploadSize } from '../../api/uploadLimits';

import { UploadProgressList, uploadTransferProgress } from '../../components/UploadProgressList';
import type { UploadProgressItem } from '../../components/UploadProgressList';

interface AlbumPhotoUploadProps {
  albumId: string;
  disabled: boolean;
  onUploaded: () => void;
}

export default function AlbumPhotoUpload({ albumId, disabled, onUploaded }: AlbumPhotoUploadProps) {
  const { message } = App.useApp();
  const [uploads, setUploads] = useState<UploadProgressItem[]>([]);
  const controllers = useRef(new Map<string, AbortController>());

  useEffect(() => {
    const active = controllers.current;
    return () => {
      for (const controller of active.values()) controller.abort();
      active.clear();
    };
  }, []);

  const updateUpload = (uid: string, changes: Partial<UploadProgressItem>) => {
    setUploads((items) => items.map((item) => (item.uid === uid ? { ...item, ...changes } : item)));
  };

  const handleUpload = (options: UploadRequestOption) => {
    const file = options.file as RcFile;
    const controller = new AbortController();
    controllers.current.set(file.uid, controller);
    setUploads((items) => [...items, { uid: file.uid, name: file.name, status: 'uploading' }]);
    const alt =
      file.name
        .replace(/\.[^.]+$/, '')
        .replace(/[-_]+/g, ' ')
        .trim() || file.name;

    void uploadAdminAlbumPhoto(
      albumId,
      file,
      { alt },
      {
        signal: controller.signal,
        onProgress: ({ loaded, total }) => {
          if (controller.signal.aborted) return;
          const progress = uploadTransferProgress(loaded, total);
          updateUpload(file.uid, progress);
          if (progress.percent !== undefined) options.onProgress?.({ percent: progress.percent });
        },
      },
    )
      .then((photo) => {
        if (controller.signal.aborted) return;
        updateUpload(file.uid, { status: 'done', percent: 100 });
        options.onSuccess?.(photo);
        onUploaded();
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        const uploadError = error instanceof Error ? error : new Error('照片上传失败');
        updateUpload(file.uid, { status: 'error', error: uploadError.message });
        options.onError?.(uploadError);
      })
      .finally(() => {
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
        beforeUpload={(file) => {
          if (!photoContentTypes.includes(file.type)) {
            message.error(file.name + ' 不是 PNG/JPEG/WebP 图片');
            return Upload.LIST_IGNORE;
          }
          try {
            validateUploadSize(file);
          } catch (error) {
            message.error(
              file.name + ' ' + (error instanceof Error ? error.message : '图片大小无效'),
            );
            return Upload.LIST_IGNORE;
          }
          return true;
        }}
        customRequest={handleUpload}
      >
        <p className="ant-upload-drag-icon">
          <PictureOutlined />
        </p>
        <p className="ant-upload-text">点击或拖拽照片到这里上传</p>
        <p className="ant-upload-hint">PNG / JPEG / WebP，单张不超过 50MB</p>
      </Upload.Dragger>
      {uploads.length > 0 && (
        <div style={{ marginTop: 16, minWidth: 0 }}>
          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button
              type="text"
              disabled={!uploads.some((item) => item.status === 'done' || item.status === 'error')}
              onClick={() =>
                setUploads((items) =>
                  items.filter(
                    (item) => item.status === 'uploading' || item.status === 'processing',
                  ),
                )
              }
            >
              清除记录
            </Button>
          </div>
          <UploadProgressList items={uploads} />
        </div>
      )}
    </>
  );
}
