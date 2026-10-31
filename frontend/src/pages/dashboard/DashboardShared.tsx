import { useState } from 'react';
import type { ReactNode } from 'react';
import { Alert, Button, Skeleton } from 'antd';
import type { UseQueryResult } from '@tanstack/react-query';

export function DashboardPanel({ title, extra, children, className = '' }: {
  title: string; extra?: ReactNode; children: ReactNode; className?: string;
}) {
  return <section className={'dashboard-panel ' + className} aria-label={title}>
    <header className="dashboard-panel-header"><h2>{title}</h2>{extra}</header>
    <div className="dashboard-panel-body">{children}</div>
  </section>;
}

export function DashboardQuery<T>({ query, children, loadingLabel = '正在加载数据' }: {
  query: UseQueryResult<T, Error>; children: (data: T) => ReactNode; loadingLabel?: string;
}) {
  if (query.isPending) return <div role="status" aria-label={loadingLabel}><span className="dashboard-sr-only">{loadingLabel}</span><Skeleton active paragraph={{ rows: 3 }} title={false} /></div>;
  if (query.isError && !query.data) return <Alert type="error" showIcon title="数据加载失败"
    description={<><p>{query.error.message}</p><Button aria-label="重试" onClick={() => void query.refetch()} loading={query.isFetching}>重试</Button></>} />;
  if (!query.data) return null;
  return <>
    {query.isError && <Alert className="dashboard-refresh-error" type="warning" showIcon title="刷新失败，正在显示上次成功数据"
      description={<><span>{query.error.message}</span><Button aria-label="重试" size="small" onClick={() => void query.refetch()} loading={query.isFetching}>重试</Button></>} />}
    {children(query.data)}
  </>;
}

export function DashboardEmpty({ children }: { children: ReactNode }) {
  return <div className="dashboard-empty">{children}</div>;
}

export function DashboardImage({ src, className = '', label }: { src: string; className?: string; label: string }) {
  const [failedUrl, setFailedUrl] = useState<string>();
  const unavailable = !src || failedUrl === src;
  return <div className={'dashboard-image ' + className}>
    {unavailable
      ? <span role="img" aria-label={label + (src ? '图片暂不可用' : '暂无封面')} className="dashboard-image-placeholder">{src ? '图片暂不可用' : '暂无封面'}</span>
      : <img src={src} alt={label} loading="lazy" decoding="async" width="240" height="160" onError={() => setFailedUrl(src)} />}
  </div>;
}
