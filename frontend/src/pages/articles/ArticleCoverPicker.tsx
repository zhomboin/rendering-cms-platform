import { useDeferredValue, useMemo, useState } from 'react';
import { AutoComplete, Button, Form, Modal, Tabs } from 'antd';
import { ArticleImageLibrary } from './ArticleImageLibrary';
import type { ArticleImage } from './articleImageUtils';
import { collectArticleImages } from './articleCoverImages';

export function ArticleCoverPicker({
  bodyMdx,
  onSelect,
}: {
  bodyMdx: string;
  onSelect: (url: string) => void;
}) {
  const coverSource = useDeferredValue(bodyMdx);
  const coverOptions = useMemo(
    () =>
      collectArticleImages(coverSource).map(({ url, alt }) => ({
        value: url,
        label: alt ? `${alt} ${url}` : url,
      })),
    [coverSource],
  );

  return (
    <Form.Item label="封面图片 URL" htmlFor="article-cover-url">
      <div className="article-editor__cover-field">
        <Form.Item name="coverImageUrl" noStyle>
          <AutoComplete
            id="article-cover-url"
            options={coverOptions}
            showSearch={{ filterOption: false }}
            virtual={false}
            placeholder="https://example.com/image.jpg"
            optionRender={({ data }) => (
              <span className="article-editor__cover-option" title={data.label}>
                {!data.value.startsWith('/') && <img src={data.value} alt="" loading="lazy" />}
                <span>{data.label}</span>
              </span>
            )}
          />
        </Form.Item>
        <CoverLibraryButton onSelect={onSelect} />
      </div>
    </Form.Item>
  );
}

function CoverLibraryButton({ onSelect }: { onSelect: (url: string) => void }) {
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<'albums' | 'assets'>('albums');
  const [selected, setSelected] = useState<ArticleImage>();

  return (
    <>
      <Button
        onClick={() => {
          setSource('albums');
          setSelected(undefined);
          setOpen(true);
        }}
      >
        从图库选择
      </Button>
      <Modal
        title="选择封面"
        open={open}
        destroyOnHidden
        className="article-image-dialog"
        width={720}
        okText="使用这张图片"
        cancelText="取消"
        okButtonProps={{ disabled: !selected }}
        onOk={() => {
          if (!selected) return;
          onSelect(selected.url);
          setOpen(false);
        }}
        onCancel={() => setOpen(false)}
      >
        <Tabs
          activeKey={source}
          items={[
            { key: 'albums', label: '相册' },
            { key: 'assets', label: '资源' },
          ]}
          onChange={(key) => {
            setSource(key === 'assets' ? 'assets' : 'albums');
            setSelected(undefined);
          }}
        />
        <ArticleImageLibrary
          key={source}
          source={source}
          single
          selected={selected ? [selected] : []}
          onToggle={setSelected}
        />
        {selected && (
          <p aria-live="polite" className="article-image-library__name">
            已选 {selected.alt}
          </p>
        )}
      </Modal>
    </>
  );
}
