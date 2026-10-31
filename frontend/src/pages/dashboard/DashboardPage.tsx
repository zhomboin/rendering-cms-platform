import { useState } from 'react';
import { Button, Segmented } from 'antd';
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { getDashboardAlbums, getDashboardArticles, getDashboardOverview } from '../../api/dashboard';
import { getAdminAnalyticsTrend } from '../../api/analytics';
import { DashboardPanel, DashboardQuery } from './DashboardShared';
import { DashboardTrend } from './DashboardTrend';
import { DashboardAlbumGallery, DashboardArticleRanking } from './DashboardCollections';
import { DashboardContent, DashboardMetrics, DashboardRecentContent, DashboardResources, DashboardTodos } from './DashboardOverviewBlocks';
import { timestamp } from './dashboardFormat';
import './DashboardPage.css';

// 切换区间时重新加载，避免短暂展示其它区间或已过期的快照。
const queryOptions = { retry: false, staleTime: 0, gcTime: 0 } as const;

export default function DashboardPage() {
  const [days, setDays] = useState(30);
  const overview = useQuery({ queryKey: ['dashboard-overview', days], queryFn: () => getDashboardOverview(days), ...queryOptions });
  const articles = useQuery({ queryKey: ['dashboard-articles', days], queryFn: () => getDashboardArticles(days), ...queryOptions });
  const albums = useQuery({ queryKey: ['dashboard-albums', days], queryFn: () => getDashboardAlbums(days), ...queryOptions });
  const trend = useQuery({ queryKey: ['dashboard-trend', days], queryFn: () => getAdminAnalyticsTrend(days), ...queryOptions });
  const isRefreshing = [overview, articles, albums, trend].some(query => query.isFetching);
  const refresh = () => { void Promise.all([overview.refetch(), articles.refetch(), albums.refetch(), trend.refetch()]); };

  return <div className="dashboard">
    <div className="dashboard-toolbar">
      <div><div className="dashboard-eyebrow">内容与访问</div><h1>运营概览</h1>
        <p>{overview.data ? overview.data.startDate + ' — ' + overview.data.endDate : '近 ' + days + ' 天'}<span className="dashboard-toolbar-separator">·</span>
          {overview.data ? (overview.isError ? '刷新失败 · 上次成功于 ' : '更新于 ') + timestamp(overview.data.generatedAt) : '正在获取最新数据'}</p></div>
      <div className="dashboard-actions">
        <Segmented aria-label="统计区间" value={days} onChange={value => setDays(Number(value))} options={[7, 30, 90].map(value => ({ label: value + ' 天', value }))} />
        <Button aria-label="刷新数据" icon={<ReloadOutlined aria-hidden="true" />} onClick={refresh} loading={isRefreshing}>刷新数据</Button>
        <Link to="/admin/articles/new" className="dashboard-new-article" aria-label="新建文章"><PlusOutlined /><span>新建文章</span></Link>
      </div>
    </div>
    <section className="dashboard-metrics-section" aria-label="关键指标">
      <DashboardQuery query={overview} loadingLabel="正在加载指标">{data => <DashboardMetrics data={data} days={days} />}</DashboardQuery>
    </section>
    <div className="dashboard-two-columns">
      <DashboardPanel title="访问趋势" extra={<span className="dashboard-panel-hint">所选区间</span>}>
        <DashboardQuery query={trend}>{data => <DashboardTrend data={data} />}</DashboardQuery>
      </DashboardPanel>
      <DashboardPanel title="内容状态" extra={<Link to="/admin/articles">管理文章 →</Link>}>
        <DashboardQuery query={overview}>{data => <DashboardContent data={data} />}</DashboardQuery>
      </DashboardPanel>
    </div>
    <div className="dashboard-two-columns dashboard-middle">
      <DashboardPanel title="文章排行" extra={<span className="dashboard-panel-hint">近 {days} 天</span>}>
        <DashboardQuery query={articles}>{data => <DashboardArticleRanking data={data} days={days} />}</DashboardQuery>
      </DashboardPanel>
      <DashboardPanel title="运营待办" extra={<span className="dashboard-panel-hint">当前事项</span>}>
        <DashboardQuery query={overview}>{data => <DashboardTodos data={data} />}</DashboardQuery>
      </DashboardPanel>
    </div>
    <DashboardPanel title="相册一览" extra={<Link to="/admin/albums">管理相册 →</Link>}>
      <div className="dashboard-section-note">近 {days} 天 · 已发布优先 · 区间访问排序</div>
      <DashboardQuery query={albums}>{data => <DashboardAlbumGallery data={data} />}</DashboardQuery>
    </DashboardPanel>
    <div className="dashboard-two-columns dashboard-bottom">
      <DashboardPanel title="近期内容" extra={<Link to="/admin/articles">全部文章 →</Link>}>
        <DashboardQuery query={overview}>{data => <DashboardRecentContent data={data} />}</DashboardQuery>
      </DashboardPanel>
      <DashboardPanel title="资源概况" extra={<Link to="/admin/assets">管理资源 →</Link>}>
        <DashboardQuery query={overview}>{data => <DashboardResources data={data} />}</DashboardQuery>
      </DashboardPanel>
    </div>
  </div>;
}
