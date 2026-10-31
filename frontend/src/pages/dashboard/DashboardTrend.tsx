import { useMemo } from 'react';
import type { AnalyticsTrend } from '../../api/analytics';
import { DashboardEmpty } from './DashboardShared';
import { number } from './dashboardFormat';

interface TrendRow { date: string; site: number; articles: number }

function trendRows(data: AnalyticsTrend): TrendRow[] {
  // 站点接口提供完整日期轴；忽略轴外文章记录，避免异常日期移动统计区间。
  const articles = new Map<string, number>();
  for (const item of data.articles) articles.set(item.date, (articles.get(item.date) ?? 0) + item.views);
  return [...data.site].sort((a, b) => a.date.localeCompare(b.date)).map(row => ({
    date: row.date, site: row.views, articles: articles.get(row.date) ?? 0,
  }));
}
function axisNumber(value: number) {
  if (value >= 1e12) return value.toExponential(0).replace('+', '');
  const unit = value >= 1e8 ? 1e8 : value >= 1e4 ? 1e4 : value >= 1000 ? 1000 : 1;
  const scaled = value / unit;
  const label = new Intl.NumberFormat('zh-CN', { useGrouping: false, maximumFractionDigits: scaled >= 100 ? 0 : 1 }).format(scaled >= 100 ? Math.floor(scaled) : scaled);
  return label + (unit === 1e8 ? '亿' : unit === 1e4 ? '万' : unit === 1000 ? '千' : '');
}
export function DashboardTrend({ data }: { data: AnalyticsTrend }) {
  const rows = useMemo(() => trendRows(data), [data]);
  const hasViews = rows.some(row => row.site > 0 || row.articles > 0);
  const max = Math.max(4, ...rows.flatMap(row => [row.site, row.articles]));
  const ceiling = Math.ceil(max / 4) * 4;
  const x = (i: number) => 108 + i / Math.max(rows.length - 1, 1) * 568;
  const y = (value: number) => 190 - value / ceiling * 162;
  const points = (key: 'site' | 'articles') => rows.map((row, i) => x(i) + ',' + y(row[key])).join(' ');
  const labelIndexes = [...new Set([0, Math.floor((rows.length - 1) / 2), rows.length - 1])];
  return <div className="dashboard-trend">
    <div className="dashboard-trend-caption"><span>近 {data.days} 天 · 每日访问计数</span>
      <div className="dashboard-legend"><span><i className="dashboard-dot site" />站点</span><span><i className="dashboard-dot articles" />文章合计</span></div>
    </div>
    {!hasViews ? <DashboardEmpty>所选区间暂无访问记录</DashboardEmpty> : <svg className="dashboard-chart" viewBox="0 0 704 226" role="img" aria-label={'近 ' + data.days + ' 天站点与文章访问趋势，详细数值见下方每日数据表'}>
      {[0, 1, 2, 3, 4].map(i => <g key={i}>
        <line x1="108" x2="676" y1={y(ceiling * i / 4)} y2={y(ceiling * i / 4)} stroke="#E2E8F0" />
        <text x="96" y={y(ceiling * i / 4) + 4} textAnchor="end">{axisNumber(ceiling * i / 4)}</text>
      </g>)}
      {labelIndexes.map(i => <text key={i} x={x(i)} y="214" textAnchor={i === 0 ? 'start' : i === rows.length - 1 ? 'end' : 'middle'}>{rows[i].date.slice(5)}</text>)}
      <polyline points={points('site')} fill="none" stroke="#4F46E5" strokeWidth="2.5" strokeLinejoin="round" />
      <polyline points={points('articles')} fill="none" stroke="#15803D" strokeWidth="2.5" strokeLinejoin="round" strokeDasharray="5 3" />
    </svg>}
    <details className="dashboard-chart-data">
      <summary>查看每日数据</summary>
      <div className="dashboard-data-scroll"><table>
        <caption className="dashboard-sr-only">每日站点与文章访问计数</caption>
        <thead><tr><th scope="col">日期</th><th scope="col">站点</th><th scope="col">文章合计</th></tr></thead>
        <tbody>{rows.map(row => <tr key={row.date}><td>{row.date}</td><td>{number(row.site)}</td><td>{number(row.articles)}</td></tr>)}</tbody>
      </table></div>
    </details>
    <p className="dashboard-footnote">按日、IP 哈希与内容去重，非标准 PV / UV。站点与文章分别计数，不相加。</p>
  </div>;
}
