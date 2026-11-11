import { useEffect, useId, useMemo, useRef, useState } from 'react';
import type { AnalyticsTrend } from '../../api/analytics';
import { DashboardEmpty } from './DashboardShared';
import { number } from './dashboardFormat';

interface TrendRow {
  date: string;
  site: number;
  articles: number;
}

function trendRows(data: AnalyticsTrend): TrendRow[] {
  // 站点接口提供完整日期轴；忽略轴外文章记录，避免异常日期移动统计区间。
  const articles = new Map<string, number>();
  for (const item of data.articles)
    articles.set(item.date, (articles.get(item.date) ?? 0) + item.views);
  return [...data.site]
    .sort((a, b) => a.date.localeCompare(b.date))
    .map((row) => ({
      date: row.date,
      site: row.views,
      articles: articles.get(row.date) ?? 0,
    }));
}
function axisNumber(value: number) {
  if (value >= 1e12) return value.toExponential(0).replace('+', '');
  const unit = value >= 1e8 ? 1e8 : value >= 1e4 ? 1e4 : value >= 1000 ? 1000 : 1;
  const scaled = value / unit;
  const label = new Intl.NumberFormat('zh-CN', {
    useGrouping: false,
    maximumFractionDigits: scaled >= 100 ? 0 : 1,
  }).format(scaled >= 100 ? Math.floor(scaled) : scaled);
  return label + (unit === 1e8 ? '亿' : unit === 1e4 ? '万' : unit === 1000 ? '千' : '');
}
function useChartWidth() {
  const ref = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(704);
  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    const measure = () => {
      const measuredWidth = element.clientWidth;
      if (measuredWidth > 0) setWidth(measuredWidth);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);
  return { ref, width };
}

export function DashboardTrend({ data }: { data: AnalyticsTrend }) {
  const fillId = useId();
  const { ref, width } = useChartWidth();
  const rows = useMemo(() => trendRows(data), [data]);
  const hasViews = rows.some((row) => row.site > 0 || row.articles > 0);
  const max = Math.max(4, ...rows.flatMap((row) => [row.site, row.articles]));
  const ceiling = Math.ceil(max / 4) * 4;
  const height = Math.round(Math.min(340, Math.max(170, 145 + width / 7)));
  const plotTop = 16;
  const plotBottom = height - 26;
  const axisStart = ceiling >= 1000 ? 60 : 36;
  const axisEnd = width - 8;
  const x = (i: number) => axisStart + (i / Math.max(rows.length - 1, 1)) * (axisEnd - axisStart);
  const y = (value: number) => plotBottom - (value / ceiling) * (plotBottom - plotTop);
  const points = (key: 'site' | 'articles') =>
    rows.map((row, i) => x(i) + ',' + y(row[key])).join(' ');
  const labelCount = Math.min(
    rows.length,
    width < 390 ? 3 : width < 720 ? 5 : width < 1120 ? 7 : 9,
  );
  const labelIndexes = Array.from({ length: labelCount }, (_, index) =>
    Math.floor((index * (rows.length - 1)) / Math.max(labelCount - 1, 1)),
  );
  return (
    <div className="dashboard-trend" ref={ref}>
      <div className="dashboard-trend-caption">
        <span>近 {data.days} 天 · 每日访问计数</span>
        <div className="dashboard-legend">
          <span>
            <i className="dashboard-dot site" />
            站点
          </span>
          <span>
            <i className="dashboard-dot articles" />
            文章合计
          </span>
        </div>
      </div>
      {!hasViews ? (
        <DashboardEmpty>所选区间暂无访问记录</DashboardEmpty>
      ) : (
        <svg
          className="dashboard-chart"
          viewBox={`0 0 ${width} ${height}`}
          role="img"
          aria-label={'近 ' + data.days + ' 天站点与文章访问趋势，详细数值见下方每日数据表'}
        >
          <defs>
            <linearGradient id={fillId} x2="0" y2="1">
              <stop stopColor="var(--cms-primary)" stopOpacity=".18" />
              <stop offset="1" stopColor="var(--cms-primary)" stopOpacity=".02" />
            </linearGradient>
          </defs>
          <polygon
            points={[
              points('site'),
              x(rows.length - 1) + ',' + plotBottom,
              x(0) + ',' + plotBottom,
            ].join(' ')}
            fill={'url(#' + fillId + ')'}
          />
          {[0, 1, 2, 3, 4].map((i) => (
            <g key={i}>
              <line
                x1={axisStart}
                x2={axisEnd}
                y1={y((ceiling * i) / 4)}
                y2={y((ceiling * i) / 4)}
                stroke="var(--cms-line)"
              />
              <text x={axisStart - 8} y={y((ceiling * i) / 4) + 4} textAnchor="end">
                {axisNumber((ceiling * i) / 4)}
              </text>
            </g>
          ))}
          {labelIndexes.map((i) => (
            <text
              key={i}
              x={x(i)}
              y={height - 6}
              textAnchor={i === 0 ? 'start' : i === rows.length - 1 ? 'end' : 'middle'}
            >
              {rows[i].date.slice(5)}
            </text>
          ))}
          <polyline
            points={points('site')}
            fill="none"
            stroke="var(--cms-primary)"
            strokeWidth="2.5"
            strokeLinejoin="round"
          />
          <polyline
            points={points('articles')}
            fill="none"
            stroke="var(--cms-success)"
            strokeWidth="2.5"
            strokeLinejoin="round"
            strokeDasharray="5 3"
          />
        </svg>
      )}
      <details className="dashboard-chart-data">
        <summary>查看每日数据</summary>
        <div className="dashboard-data-scroll">
          <table>
            <caption className="dashboard-sr-only">每日站点与文章访问计数</caption>
            <thead>
              <tr>
                <th scope="col">日期</th>
                <th scope="col">站点</th>
                <th scope="col">文章合计</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={row.date}>
                  <td>{row.date}</td>
                  <td>{number(row.site)}</td>
                  <td>{number(row.articles)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </details>
      <p className="dashboard-footnote">
        按日、IP 哈希与内容去重，非标准 PV / UV。站点与文章分别计数，不相加。
      </p>
    </div>
  );
}
