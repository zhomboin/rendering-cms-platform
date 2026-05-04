import { Row, Col, Card, Statistic, Table, Typography } from 'antd';
import {
  ArrowUpOutlined,
  FileTextOutlined,
  MessageOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { useQuery } from '@tanstack/react-query';
import { getAdminAnalyticsSummary } from '../../../api/analytics';
import type { AnalyticsSummary, HotArticle } from '../../../api/analytics';

const { Title } = Typography;

const hotColumns: ColumnsType<HotArticle> = [
  {
    title: '排名',
    dataIndex: 'rank',
    key: 'rank',
    width: 72,
    render: (rank: number) => (
      <span style={{ fontWeight: rank <= 3 ? 600 : 400, color: rank <= 3 ? '#4F46E5' : '#64748B' }}>
        {rank}
      </span>
    ),
  },
  {
    title: '标题',
    dataIndex: 'title',
    key: 'title',
  },
  {
    title: '访问量',
    dataIndex: 'views',
    key: 'views',
    width: 100,
    align: 'right',
  },
];

function AdminDashboardPage() {
  const { data, isLoading } = useQuery({
    queryKey: ['analytics-summary'],
    queryFn: getAdminAnalyticsSummary,
  });

  const weeklyData = data?.last7Days ?? [];
  const hotArticles = data?.hotArticles ?? [];
  const maxVisits = Math.max(...weeklyData.map((d) => d.views), 1);
  const stats = [
    {
      key: 'visits',
      label: '今日访问',
      value: data?.todayViews ?? 0,
      suffix: '',
      subtitle: '来自站点日聚合统计',
      icon: <ArrowUpOutlined />,
      iconColor: '#22C55E',
      valueColor: '#22C55E',
    },
    {
      key: 'articles',
      label: '热门文章',
      value: hotArticles.length,
      suffix: '',
      subtitle: '按累计访问量排序',
      icon: <FileTextOutlined />,
      iconColor: '#4F46E5',
      valueColor: '#0F172A',
    },
    {
      key: 'comments',
      label: '近 7 天访问',
      value: weeklyData.reduce((sum, item) => sum + item.views, 0),
      suffix: '',
      subtitle: '近 7 天站点访问总量',
      icon: <MessageOutlined />,
      iconColor: '#F59E0B',
      valueColor: '#F59E0B',
    },
  ];

  return (
    <div>
      {/* Page title */}
      <Title
        level={4}
        style={{
          fontSize: 24,
          fontWeight: 700,
          marginBottom: 24,
          marginTop: 0,
          color: '#0F172A',
        }}
      >
        仪表盘
      </Title>

      {/* ---- Top stat cards ---- */}
      <Row gutter={[20, 20]}>
        {stats.map((s) => (
          <Col key={s.key} xs={24} sm={12} lg={8}>
            <Card
              style={{
                borderRadius: 24,
                padding: 24,
                background: '#FFFFFF',
                boxShadow: '0 1px 3px 0 rgba(0,0,0,0.06)',
                border: '1px solid #E2E8F0',
                height: '100%',
              }}
              styles={{ body: { padding: 0 } }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <Statistic
                  title={
                    <span style={{ fontSize: 14, color: '#64748B', fontWeight: 400 }}>{s.label}</span>
                  }
                  value={s.value}
                  valueStyle={{
                    fontSize: 32,
                    fontWeight: 700,
                    color: s.valueColor,
                    lineHeight: 1.2,
                  }}
                  suffix={s.suffix}
                />
                <div
                  style={{
                    width: 48,
                    height: 48,
                    borderRadius: 16,
                    background: `${s.iconColor}0f`,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: 22,
                    color: s.iconColor,
                    flexShrink: 0,
                  }}
                >
                  {s.icon}
                </div>
              </div>
              <div style={{ marginTop: 8, fontSize: 13, color: '#64748B' }}>
                {s.subtitle}
              </div>
            </Card>
          </Col>
        ))}
      </Row>

      {/* ---- Middle: weekly trend bar chart ---- */}
      <Card
        title={
          <span style={{ fontSize: 18, fontWeight: 700, color: '#0F172A' }}>
            近 7 天访问趋势
          </span>
        }
        style={{
          marginTop: 24,
          borderRadius: 24,
          border: '1px solid #E2E8F0',
          boxShadow: '0 1px 3px 0 rgba(0,0,0,0.06)',
        }}
        styles={{ header: { borderBottom: 'none', paddingBottom: 0 } }}
      >
        <div style={{ display: 'flex', alignItems: 'flex-end', gap: 12, height: 200, padding: '0 8px' }}>
          {weeklyData.map((d) => {
            const barHeight = Math.max((d.views / maxVisits) * 160, 12);
            return (
              <div
                key={d.date}
                style={{
                  flex: 1,
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  gap: 8,
                }}
              >
                <span style={{ fontSize: 11, color: '#64748B', lineHeight: 1.2 }}>{d.views}</span>
                <div
                  style={{
                    width: '100%',
                    maxWidth: 48,
                    height: barHeight,
                    borderRadius: '6px 6px 0 0',
                    background: 'linear-gradient(180deg, #4F46E5 0%, #818CF8 100%)',
                    transition: 'height 0.3s ease',
                    minHeight: 4,
                  }}
                  aria-label={`${d.date}: ${d.views} 次访问`}
                />
                <span style={{ fontSize: 11, color: '#64748B', marginTop: 4 }}>{d.date.slice(5)}</span>
              </div>
            );
          })}
        </div>
      </Card>

      {/* ---- Bottom: hot articles table ---- */}
      <Card
        title={
          <span style={{ fontSize: 18, fontWeight: 700, color: '#0F172A' }}>
            热门文章
          </span>
        }
        style={{
          marginTop: 24,
          borderRadius: 24,
          border: '1px solid #E2E8F0',
          boxShadow: '0 1px 3px 0 rgba(0,0,0,0.06)',
        }}
        styles={{ header: { borderBottom: '1px solid #E2E8F0' } }}
      >
        <Table
          columns={hotColumns}
          dataSource={hotArticles}
          pagination={false}
          loading={isLoading}
          rowKey="rank"
          style={{ borderRadius: 8 }}
        />
      </Card>
    </div>
  );
}

export default AdminDashboardPage;
