import { Link } from 'react-router-dom';
import type { DashboardAlbums, DashboardArticles } from '../../api/dashboard';
import { DashboardEmpty, DashboardImage } from './DashboardShared';
import { date, number } from './dashboardFormat';

export function DashboardArticleRanking({ data, days }: { data: DashboardArticles; days: number }) {
  if (!data.articles.length) return <DashboardEmpty>暂无已发布文章</DashboardEmpty>;
  return <div className="dashboard-ranking">
    <div className="dashboard-ranking-labels"><span>文章 / 按区间访问排序</span><span>近 {days} 天 / 累计</span></div>
    {data.articles.map((article, index) => <div className="dashboard-rank-row" key={article.articleId}>
      <span className="dashboard-rank-number">{String(index + 1).padStart(2, '0')}</span>
      <DashboardImage src={article.coverImageUrl} label={article.title} className="dashboard-ranking-cover" />
      <div className="dashboard-grow"><Link className="dashboard-title-link" to={'/admin/articles/' + article.articleId + '/read'}>{article.title}</Link>
        <div className="dashboard-item-meta">{article.tags.slice(0, 2).map(tag => <span className="dashboard-tag" key={tag}>{tag}</span>)}<span>{date(article.publishedAt)} 发布</span></div>
      </div>
      <div className="dashboard-rank-views"><strong>{number(article.periodViews)}</strong><span>{number(article.totalViews)} 累计</span></div>
    </div>)}
  </div>;
}

export function DashboardAlbumGallery({ data }: { data: DashboardAlbums }) {
  if (!data.albums.length) return <DashboardEmpty>暂无相册</DashboardEmpty>;
  return <div className="dashboard-album-grid">{data.albums.map(album => <article className="dashboard-album" key={album.albumId}>
    <Link className="dashboard-album-photo" to={'/admin/albums/' + album.albumId + '/photos'} aria-label={'管理' + album.title + '的图片'}>
      <DashboardImage src={album.coverUrl} label={album.title} />
      <span className={'dashboard-album-status ' + album.status}>{album.status === 'published' ? '已发布' : '草稿'}</span>
    </Link>
    <div className="dashboard-album-info"><Link to={'/admin/albums/' + album.albumId + '/photos'} className="dashboard-title-link">{album.title}</Link>
      <div className="dashboard-album-meta"><span>{number(album.photoCount)} 张照片</span><span>{number(album.periodViews)} 次访问</span></div>
      <div className="dashboard-album-footer"><span>{date(album.updatedAt)} 更新</span><Link to={'/admin/albums/' + album.albumId + '/edit'} aria-label={'编辑' + album.title}>编辑信息</Link></div>
    </div>
  </article>)}</div>;
}
