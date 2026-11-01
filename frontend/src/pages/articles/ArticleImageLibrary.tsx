import { useState } from 'react';
import { Alert, Button, Checkbox, Empty, Input, Modal, Select, Spin } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { listAdminAssets } from '../../api/assets';
import { listAdminAlbums, listAdminAlbumPhotos } from '../../api/albums';
import { assetToImage, imageUrlKey, photoToImage, uniqueArticleImages } from './articleImageUtils';
import type { ArticleImage } from './articleImageUtils';
import './ArticleImageLibrary.css';

interface Props {
  source: 'albums' | 'assets';
  selected: ArticleImage[];
  onToggle: (image: ArticleImage, checked: boolean) => void;
}

export function ArticleImageLibrary({ source, selected, onToggle }: Props) {
  const [albumId, setAlbumId] = useState<string>();
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const [preview, setPreview] = useState<ArticleImage>();
  const albums = useQuery({
    queryKey: ['article-image-albums'],
    queryFn: listAdminAlbums,
    enabled: source === 'albums',
    retry: false,
    staleTime: 0,
  });
  const assets = useQuery({
    queryKey: ['article-image-assets'],
    queryFn: listAdminAssets,
    enabled: source === 'assets',
    retry: false,
    staleTime: 0,
  });
  const photos = useQuery({
    queryKey: ['article-image-photos', albumId],
    queryFn: () => listAdminAlbumPhotos(albumId as string),
    enabled: source === 'albums' && Boolean(albumId),
    retry: false,
    staleTime: 0,
  });
  const query = source === 'assets' ? assets : photos;
  const images = uniqueArticleImages(source === 'assets'
    ? (assets.data ?? []).flatMap(asset => { const image = assetToImage(asset); return image ? [image] : []; })
    : (photos.data ?? []).flatMap(photo => { const image = photoToImage(photo); return image ? [image] : []; }));
  const filtered = images.filter(image => image.alt.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()));
  const pageCount = Math.max(1, Math.ceil(filtered.length / 12));
  const currentPage = Math.min(page, pageCount);
  const visible = filtered.slice((currentPage - 1) * 12, currentPage * 12);
  const selectedUrls = new Set(selected.map(image => imageUrlKey(image.url)));
  const error = source === 'albums' && albums.error ? albums.error : query.error;
  const loading = source === 'albums' && (albums.isPending || albums.isFetching)
    || (source === 'assets' || Boolean(albumId)) && (query.isPending || query.isFetching);
  const retry = () => source === 'albums' && albums.error ? void albums.refetch() : void query.refetch();

  return (
    <div className="article-image-library">
      <p className="article-image-library__note">仅显示可长期公开访问的图片。</p>
      {source === 'albums' && (
        <Select
          aria-label="选择相册"
          placeholder="选择相册"
          value={albumId}
          showSearch
          optionFilterProp="label"
          virtual={false}
          style={{ width: '100%' }}
          options={(albums.data ?? []).map(album => ({ value: album.albumId, label: album.title }))}
          onChange={value => { setAlbumId(value); setSearch(''); setPage(1); }}
        />
      )}
      <Input
        aria-label="搜索图片"
        placeholder="搜索图片名称"
        value={search}
        allowClear
        onChange={event => { setSearch(event.target.value); setPage(1); }}
      />
      {error ? (
        <Alert
          type="error"
          showIcon
          title={error instanceof Error ? error.message : '图库读取失败'}
          action={<Button aria-label="重试" onClick={retry}>重试</Button>}
        />
      ) : loading ? (
        <div className="article-image-library__state" role="status"><Spin /> 正在加载图片</div>
      ) : source === 'albums' && !albumId ? (
        <Empty description={albums.data?.length ? '请先选择相册' : '暂无相册'} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : visible.length === 0 ? (
        <Empty description={search.trim() ? '没有匹配的图片' : '暂无可用图片'} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <>
          <div className="article-image-library__grid">
            {visible.map(image => (
              <div className="article-image-library__item" key={imageUrlKey(image.url)}>
                <Button className="article-image-library__preview" aria-label={'预览 ' + image.alt} onClick={() => setPreview(image)}>
                  <img src={image.url} alt="" loading="lazy" />
                  <span>预览</span>
                </Button>
                <div className="article-image-library__caption">
                  <Checkbox
                    aria-label={'选择 ' + image.alt}
                    checked={selectedUrls.has(imageUrlKey(image.url))}
                    onChange={event => onToggle(image, event.target.checked)}
                  />
                  <span title={image.alt}>{image.alt || '图片'}</span>
                </div>
              </div>
            ))}
          </div>
          <div className="article-image-library__pagination" aria-label="图片分页">
            <Button aria-label="上一页" disabled={currentPage <= 1} onClick={() => setPage(currentPage - 1)}>上一页</Button>
            <span>{currentPage} / {pageCount} 页 · {filtered.length} 张</span>
            <Button aria-label="下一页" disabled={currentPage >= pageCount} onClick={() => setPage(currentPage + 1)}>下一页</Button>
          </div>
        </>
      )}
      <Modal title="图片预览" open={Boolean(preview)} footer={null} onCancel={() => setPreview(undefined)} width={760}>
        {preview && <><img className="article-image-library__large" src={preview.url} alt={preview.alt} /><p className="article-image-library__name">{preview.alt}</p></>}
      </Modal>
    </div>
  );
}
