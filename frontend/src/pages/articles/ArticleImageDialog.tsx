import { useEffect, useRef, useState } from 'react';
import { Button, Form, Input, Modal, Tabs, Upload, message } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import type { UploadProps } from 'antd/es/upload/interface';
import { uploadAdminAsset } from '../../api/assets';
import { ArticleImageLibrary } from './ArticleImageLibrary';
import { imageUrlKey, uniqueArticleImages } from './articleImageUtils';
import type { ArticleImage } from './articleImageUtils';

interface Props {
  initialAlt: string;
  onInsert: (images: ArticleImage[]) => void;
  onCancel: () => void;
}

type ImageForm = { imageUrl: string; altText: string };

export function ArticleImageDialog({ initialAlt, onInsert, onCancel }: Props) {
  const [form] = Form.useForm<ImageForm>();
  const [tab, setTab] = useState('manual');
  const [selected, setSelected] = useState<ArticleImage[]>([]);
  const [uploading, setUploading] = useState(false);
  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);

  const insert = async () => {
    if (tab !== 'manual') {
      if (selected.length) onInsert(selected);
      return;
    }
    try {
      const values = await form.validateFields();
      onInsert([{ url: values.imageUrl.trim(), alt: values.altText.trim() || '图片' }]);
    } catch {
      // 字段错误由表单展示。
    }
  };

  const uploadProps: UploadProps = {
    accept: 'image/png,image/jpeg,image/webp',
    maxCount: 1,
    showUploadList: false,
    customRequest: async options => {
      const file = options.file as File;
      if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
        const error = new Error('仅支持 PNG、JPEG、WebP 图片');
        message.error(error.message);
        options.onError?.(error);
        return;
      }
      setUploading(true);
      try {
        const asset = await uploadAdminAsset(file, 'blog-image');
        if (!mounted.current) return;
        if (!asset.publicUrl) throw new Error('图片已上传，但当前存储未返回可公开访问的 URL');
        form.setFieldsValue({ imageUrl: asset.publicUrl, altText: form.getFieldValue('altText') || file.name.replace(/\.[^.]+$/, '') });
        message.success('图片已上传，可插入正文');
        options.onSuccess?.(asset);
      } catch (error) {
        if (!mounted.current) return;
        const uploadError = error instanceof Error ? error : new Error('图片上传失败');
        message.error(uploadError.message);
        options.onError?.(uploadError);
      } finally {
        if (mounted.current) setUploading(false);
      }
    },
  };

  return (
    <Modal
      title="插入图片"
      open
      width={720}
      className="article-image-dialog"
      onOk={() => void insert()}
      onCancel={onCancel}
      okText="插入正文"
      cancelText="取消"
      cancelButtonProps={{ 'aria-label': '取消' }}
      okButtonProps={{ 'aria-label': '插入正文', loading: uploading, disabled: uploading || tab !== 'manual' && selected.length === 0 }}
      focusTriggerAfterClose={false}
    >
      <Tabs activeKey={tab} onChange={setTab} items={[
        { key: 'manual', label: '上传 / 链接' },
        { key: 'albums', label: '相册' },
        { key: 'assets', label: '资源' },
      ]} />
      <div hidden={tab !== 'manual'}>
        <Form form={form} layout="vertical" initialValues={{ altText: initialAlt, imageUrl: '' }}>
          <Form.Item name="imageUrl" label="图片 URL" rules={[
            { required: true, message: '请输入图片 URL 或上传图片' },
            { validator: (_, value: string) => {
              if (!value || /^(https?:\/\/|\/)/.test(value.trim())) return Promise.resolve();
              return Promise.reject(new Error('图片 URL 必须以 http(s):// 或 / 开头'));
            } },
          ]}>
            <Input aria-label="图片 URL" placeholder="https://example.com/image.webp" />
          </Form.Item>
          <Form.Item name="altText" label="替代文本"><Input placeholder="用于无障碍和图片加载失败时展示" /></Form.Item>
          <Upload {...uploadProps}><Button icon={<UploadOutlined />} loading={uploading}>上传图片并回填 URL</Button></Upload>
        </Form>
      </div>
      {tab !== 'manual' && (
        <ArticleImageLibrary
          key={tab}
          source={tab === 'albums' ? 'albums' : 'assets'}
          selected={selected}
          onToggle={(image, checked) => setSelected(current => checked
            ? uniqueArticleImages([...current, image])
            : current.filter(item => imageUrlKey(item.url) !== imageUrlKey(image.url)))}
        />
      )}
      {selected.length > 0 && (
        <div className="article-image-dialog__selection">
          <span>已选择 {selected.length} 张</span>
          <Button aria-label="清空选择" size="small" onClick={() => setSelected([])}>清空选择</Button>
        </div>
      )}
    </Modal>
  );
}
