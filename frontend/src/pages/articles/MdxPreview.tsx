import { Button, Empty, Tooltip, Typography } from 'antd';
import { MdxBlockRenderer } from './MdxBlockRenderer';
import { parsePreviewBlocks } from './MdxPreviewParser';

const { Text } = Typography;

type MdxPreviewProps = {
  source: string;
  fillHeight?: boolean;
  onEnterFullscreen?: () => void;
};

export function MdxPreview({ source, fillHeight = false, onEnterFullscreen }: MdxPreviewProps) {
  const blocks = parsePreviewBlocks(source);

  return (
    <section className="mdx-preview" aria-label="MDX 预览">
      <div className="mdx-preview__header" style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', gap: 16, alignItems: 'flex-start' }}>
        <div>
          <Text strong style={{ display: 'block', color: '#0F172A', marginBottom: 4 }}>
            MDX 预览
          </Text>
          <Text style={{ fontSize: 12, color: '#64748B' }}>
            用于快速检查正文结构，复杂组件以源码形式展示。
          </Text>
        </div>
        {onEnterFullscreen && (
          <Tooltip title="全屏写作预览">
            <Button
              aria-label="全屏写作预览"
              icon={<FullscreenIcon />}
              onClick={onEnterFullscreen}
            />
          </Tooltip>
        )}
      </div>

      <div
        className={fillHeight ? 'mdx-preview__body mdx-preview__body--fill' : 'mdx-preview__body'}
        style={{
          minHeight: fillHeight ? 'calc(100vh - 145px)' : 420,
          maxHeight: fillHeight ? 'calc(100vh - 145px)' : 720,
          overflow: 'auto',
          padding: 24,
          border: '1px solid #E2E8F0',
          borderRadius: 8,
          background: '#FFFFFF',
        }}
      >
        {blocks.length === 0 ? (
          <Empty description="输入 MDX 正文后在这里预览" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        ) : (
          <MdxBlockRenderer blocks={blocks} variant="preview" />
        )}
      </div>
    </section>
  );
}

function FullscreenIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M8 3H3v5" />
      <path d="M16 3h5v5" />
      <path d="M8 21H3v-5" />
      <path d="M16 21h5v-5" />
    </svg>
  );
}
