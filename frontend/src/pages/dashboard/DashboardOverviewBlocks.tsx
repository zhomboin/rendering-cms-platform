import { Link } from 'react-router-dom';
import type { DashboardOverview } from '../../api/dashboard';
import { DashboardEmpty, DashboardImage } from './DashboardShared';
import { bytes, date, number, statusLabel } from './dashboardFormat';

export function DashboardMetrics({ data, days }: { data: DashboardOverview; days: number }) {
  const metrics = [
    ['今日站点访问', data.metrics.todayViews, '今日快照'],
    ['区间站点访问', data.metrics.periodViews, '近 ' + days + ' 天'],
    ['文章访问', data.metrics.articleViews, '近 ' + days + ' 天'],
    ['相册访问', data.metrics.albumViews, '近 ' + days + ' 天'],
    ['新增评论', data.metrics.newComments, '近 ' + days + ' 天 · 全部状态'],
    ['待审核评论', data.metrics.pendingComments, '当前待审核'],
  ] as const;
  return <div className="dashboard-metrics">{metrics.map(([label, value, hint]) =>
    <div className="dashboard-metric" key={label}>
      <span className="dashboard-metric-label">{label}</span><strong>{number(value)}</strong><span className="dashboard-muted">{hint}</span>
    </div>)}</div>;
}

export function DashboardContent({ data }: { data: DashboardOverview }) {
  const { content } = data;
  const states = [
    { label: '已发布', value: content.publishedArticles, color: '#4F46E5' },
    { label: '草稿', value: content.draftArticles, color: '#64748B' },
    { label: '私有', value: content.privateArticles, color: '#8B5E34' },
    { label: '已归档', value: content.archivedArticles, color: '#94A3B8' },
  ];
  const total = states.reduce((sum, state) => sum + state.value, 0);
  return <>
    <div className="dashboard-content-total"><strong>{number(total)}</strong><span className="dashboard-muted">篇文章 · 当前状态</span></div>
    <div className="dashboard-state-bar" aria-hidden="true">{states.map(state =>
      <span key={state.label} style={{ width: (total ? state.value / total * 100 : 0) + '%', background: state.color }} />)}</div>
    <dl className="dashboard-state-list">{states.map(state => <div key={state.label}>
      <dt><i style={{ background: state.color }} />{state.label}</dt><dd>{number(state.value)}</dd>
    </div>)}</dl>
    <div className="dashboard-content-albums"><span>已发布相册 <strong>{number(content.publishedAlbums)}</strong></span>
      <span>草稿相册 <strong>{number(content.draftAlbums)}</strong></span><span>照片 <strong>{number(content.photoCount)}</strong></span></div>
    <p className="dashboard-footnote">私有文章仅后台可见。</p>
  </>;
}

export function DashboardTodos({ data }: { data: DashboardOverview }) {
  const hasTasks = data.drafts.length || data.pendingComments.length || data.content.missingCoverArticles;
  return <>
    {!hasTasks ? <DashboardEmpty>暂无待处理事项</DashboardEmpty> : <div className="dashboard-todos">
      {data.drafts.length > 0 && <div className="dashboard-todo-group">
        <h3>继续撰写 <span>{number(data.content.draftArticles)} 篇草稿</span></h3>
        {data.drafts.map(article => <div className="dashboard-todo-item" key={article.articleId}>
          <Link to={'/admin/articles/' + article.articleId + '/edit'} className="dashboard-title-link">{article.title}</Link>
          <span className="dashboard-muted">{date(article.updatedAt)} 更新</span>
        </div>)}
      </div>}
      {data.pendingComments.length > 0 && <div className="dashboard-todo-group">
        <h3>待审核评论 <span>当前 {number(data.metrics.pendingComments)} 条</span></h3>
        {data.pendingComments.map(comment => <div className="dashboard-todo-comment" key={comment.commentId}>
          <div><strong>{comment.authorName}</strong><span className="dashboard-muted">{date(comment.createdAt)}</span></div>
          <p>{comment.body}</p><span className="dashboard-muted dashboard-truncate">来自《{comment.articleTitle}》</span>
        </div>)}
        <Link className="dashboard-inline-link" to="/admin/comments">前往评论审核 →</Link>
      </div>}
      {data.content.missingCoverArticles > 0 && <div className="dashboard-cover-task"><span>已发布文章缺封面 <strong>{number(data.content.missingCoverArticles)}</strong></span>
        <Link to="/admin/articles">管理文章 →</Link></div>}
    </div>}
  </>;
}

export function DashboardRecentContent({ data }: { data: DashboardOverview }) {
  if (!data.recentArticles.length) return <DashboardEmpty>暂无近期内容</DashboardEmpty>;
  return <div className="dashboard-recent-list">{data.recentArticles.map(article =>
    <div className="dashboard-recent-item" key={article.articleId}>
      <DashboardImage src={article.coverImageUrl} label={article.title} className="dashboard-recent-cover" />
      <div className="dashboard-grow"><Link className="dashboard-title-link" to={'/admin/articles/' + article.articleId + '/read'}>{article.title}</Link>
        <div className="dashboard-item-meta"><span className={'dashboard-status ' + article.status}>{statusLabel[article.status]}</span><span>{date(article.updatedAt)} 更新</span></div>
      </div><Link className="dashboard-edit-link" to={'/admin/articles/' + article.articleId + '/edit'} aria-label={'编辑' + article.title}>编辑</Link>
    </div>)}</div>;
}

export function DashboardResources({ data }: { data: DashboardOverview }) {
  return <>
    <dl className="dashboard-resource-stats">
      <div><dt>已登记资源</dt><dd>{number(data.resources.assetCount)}<small> 个</small></dd></div>
      <div><dt>图片资源</dt><dd>{number(data.resources.imageCount)}<small> 张</small></dd></div>
      <div><dt>资源登记大小</dt><dd>{bytes(data.resources.totalBytes)}</dd></div>
    </dl>
    {!data.recentImages.length ? <DashboardEmpty>暂无图片资源</DashboardEmpty>
      : <div className="dashboard-resource-images">{data.recentImages.map(image => <Link key={image.assetId} to="/admin/assets" title={image.filename}>
        <DashboardImage src={image.publicUrl} label={image.filename} />
        <span className="dashboard-truncate">{image.filename}</span>
      </Link>)}</div>}
    <p className="dashboard-footnote">按当前资源登记汇总，不代表对象存储实际用量。</p>
  </>;
}
