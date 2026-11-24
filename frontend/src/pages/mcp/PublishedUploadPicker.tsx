import { Alert, Button, Select, Space } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { listAdminAlbums } from '../../api/albums';

export function PublishedUploadPicker({
  id,
  value = [],
  onChange,
}: {
  id?: string;
  value?: string[];
  onChange?: (ids: string[]) => void;
}) {
  const albums = useQuery({ queryKey: ['mcp-upload-albums'], queryFn: listAdminAlbums, staleTime: 0 });
  const options = (albums.data ?? [])
    .filter((album) => album.status === 'published' || value.includes(album.albumId))
    .map((album) => ({
      value: album.albumId,
      label: album.status === 'published' ? album.title : album.title + '（已下架）',
    }));
  return (
    <Space orientation="vertical" size={4} style={{ display: 'flex' }}>
      <Select
        id={id}
        aria-label="可追加照片的已发布相册"
        mode="multiple"
        value={value}
        onChange={onChange}
        options={options}
        loading={albums.isFetching}
        showSearch={{ optionFilterProp: 'label' }}
        placeholder="选择相册"
        notFoundContent={albums.isFetching ? '正在读取相册' : '没有匹配的已发布相册'}
        style={{ width: '100%' }}
      />
      {albums.isError && (
        <Alert
          type="error"
          title="相册列表读取失败"
          action={
            <Button size="small" onClick={() => void albums.refetch()}>
              重试
            </Button>
          }
        />
      )}
    </Space>
  );
}
