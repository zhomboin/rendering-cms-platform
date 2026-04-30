import { useState } from 'react';
import { Tabs, Card, Button, Tag, Typography, Space, Badge } from 'antd';
import { CheckOutlined, CloseOutlined } from '@ant-design/icons';
import { message } from 'antd';

const { Title, Text, Paragraph } = Typography;

type CommentStatus = 'pending' | 'approved' | 'rejected';

interface Comment {
  id: number;
  author: string;
  email: string;
  body: string;
  articleTitle: string;
  articleId: number;
  createdAt: string;
  status: CommentStatus;
  reviewedAt?: string;
}

const mockComments: Comment[] = [
  {
    id: 1,
    author: '张三',
    email: 'zhangsan@example.com',
    body: '写得非常好，对 React Server Components 的理解又加深了一层。特别是关于数据获取的部分，讲得很透彻。',
    articleTitle: '深入理解 React Server Components',
    articleId: 1,
    createdAt: '2026-04-30 14:23',
    status: 'pending',
  },
  {
    id: 2,
    author: '李四',
    email: 'lisi@example.com',
    body: '请问 Go 的 goroutine 和线程的主要区别是什么？文章里提到了一些但没有深入展开。',
    articleTitle: 'Go 语言并发编程实战',
    articleId: 2,
    createdAt: '2026-04-30 12:05',
    status: 'pending',
  },
  {
    id: 3,
    author: '王五',
    email: 'wangwu@example.com',
    body: '好文章！期待更多 TypeScript 类型体操的内容。',
    articleTitle: 'TypeScript 类型体操进阶',
    articleId: 3,
    createdAt: '2026-04-30 10:47',
    status: 'pending',
  },
  {
    id: 4,
    author: '赵六',
    email: 'zhaoliu@example.com',
    body: 'PostgreSQL 的索引优化一直是个难点，这篇文章总结得很清晰。',
    articleTitle: 'PostgreSQL 索引优化指南',
    articleId: 4,
    createdAt: '2026-04-30 09:12',
    status: 'pending',
  },
  {
    id: 5,
    author: '孙七',
    email: 'sunqi@example.com',
    body: 'Chi 的路由设计确实简洁，最近也在用这个库。',
    articleTitle: '使用 Chi 构建 RESTful API',
    articleId: 5,
    createdAt: '2026-04-29 22:30',
    status: 'pending',
  },
  {
    id: 6,
    author: '周八',
    email: 'zhouba@example.com',
    body: '写得很好，已收藏！希望后续能有更多实战案例。',
    articleTitle: '深入理解 React Server Components',
    articleId: 1,
    createdAt: '2026-04-29 16:18',
    status: 'approved',
    reviewedAt: '2026-04-30 08:00',
  },
  {
    id: 7,
    author: '吴九',
    email: 'wujiu@example.com',
    body: '这篇文章对新手很友好，推荐给团队的小伙伴们了。',
    articleTitle: 'Go 语言并发编程实战',
    articleId: 2,
    createdAt: '2026-04-28 20:45',
    status: 'approved',
    reviewedAt: '2026-04-29 10:30',
  },
  {
    id: 8,
    author: '郑十',
    email: 'zhengshi@example.com',
    body: '内容不错但有些错别字，希望作者能修正一下。',
    articleTitle: 'TypeScript 类型体操进阶',
    articleId: 3,
    createdAt: '2026-04-27 15:33',
    status: 'rejected',
    reviewedAt: '2026-04-28 09:00',
  },
];

const statusBorderColor: Record<CommentStatus, string> = {
  pending: '#F59E0B',
  approved: '#22C55E',
  rejected: '#EF4444',
};

const statusBadgeColor: Record<CommentStatus, string> = {
  pending: 'warning',
  approved: 'success',
  rejected: 'error',
};

const statusLabel: Record<CommentStatus, string> = {
  pending: '待审核',
  approved: '已通过',
  rejected: '已拒绝',
};

function CommentCard({
  comment,
  onApprove,
  onReject,
}: {
  comment: Comment;
  onApprove: (id: number) => void;
  onReject: (id: number) => void;
}) {
  return (
    <Card
      style={{
        borderRadius: 24,
        padding: 20,
        background: '#FFFFFF',
        border: `1px solid #E2E8F0`,
        borderLeft: `4px solid ${statusBorderColor[comment.status]}`,
        boxShadow: '0 1px 3px 0 rgba(0,0,0,0.06)',
        marginBottom: 16,
      }}
      styles={{ body: { padding: 0 } }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
            <Text strong style={{ fontSize: 14, color: '#0F172A' }}>
              {comment.author}
            </Text>
            <Text style={{ fontSize: 12, color: '#64748B' }}>{comment.email}</Text>
            {comment.status !== 'pending' && (
              <Tag
                color={statusBadgeColor[comment.status] as 'success' | 'error' | 'warning'}
                style={{ borderRadius: 6, fontSize: 12, lineHeight: '22px', marginLeft: 8 }}
              >
                {statusLabel[comment.status]}
              </Tag>
            )}
          </div>

          <Paragraph
            style={{
              fontSize: 14,
              color: '#334155',
              marginBottom: 8,
              lineHeight: 1.6,
              whiteSpace: 'pre-wrap',
            }}
          >
            {comment.body}
          </Paragraph>

          <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
            <Text style={{ fontSize: 12, color: '#4F46E5' }}>
              文章：{comment.articleTitle}
            </Text>
            <Text style={{ fontSize: 12, color: '#94A3B8' }}>|</Text>
            <Text style={{ fontSize: 12, color: '#64748B' }}>{comment.createdAt}</Text>
            {comment.reviewedAt && (
              <>
                <Text style={{ fontSize: 12, color: '#94A3B8' }}>|</Text>
                <Text style={{ fontSize: 12, color: '#64748B' }}>
                  审核于 {comment.reviewedAt}
                </Text>
              </>
            )}
          </div>
        </div>

        {comment.status === 'pending' && (
          <Space style={{ marginLeft: 20, flexShrink: 0 }}>
            <Button
              type="primary"
              size="small"
              icon={<CheckOutlined />}
              style={{
                borderRadius: 8,
                background: '#22C55E',
                borderColor: '#22C55E',
              }}
              onClick={() => onApprove(comment.id)}
            >
              通过
            </Button>
            <Button
              size="small"
              danger
              icon={<CloseOutlined />}
              style={{ borderRadius: 8 }}
              onClick={() => onReject(comment.id)}
            >
              拒绝
            </Button>
          </Space>
        )}
      </div>
    </Card>
  );
}

function AdminCommentsPage() {
  const [comments, setComments] = useState<Comment[]>(mockComments);
  const [activeTab, setActiveTab] = useState<string>('pending');

  const pendingCount = comments.filter((c) => c.status === 'pending').length;

  const handleApprove = (id: number) => {
    setComments((prev) =>
      prev.map((c) =>
        c.id === id
          ? { ...c, status: 'approved' as const, reviewedAt: '2026-04-30 15:00' }
          : c,
      ),
    );
    message.success('评论已通过');
  };

  const handleReject = (id: number) => {
    setComments((prev) =>
      prev.map((c) =>
        c.id === id
          ? { ...c, status: 'rejected' as const, reviewedAt: '2026-04-30 15:00' }
          : c,
      ),
    );
    message.success('评论已拒绝');
  };

  const filteredComments = comments.filter((c) => {
    if (activeTab === 'pending') return c.status === 'pending';
    if (activeTab === 'approved') return c.status === 'approved';
    if (activeTab === 'rejected') return c.status === 'rejected';
    return true;
  });

  return (
    <div>
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
        评论审核
      </Title>

      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        style={{ marginBottom: 0 }}
        items={[
          {
            key: 'pending',
            label: (
              <span>
                待审核
                <Badge
                  count={pendingCount}
                  size="small"
                  style={{
                    backgroundColor: '#F59E0B',
                    marginLeft: 6,
                    fontSize: 11,
                    minWidth: 18,
                    height: 18,
                    lineHeight: '18px',
                  }}
                />
              </span>
            ),
            children: (
              <div style={{ marginTop: 4 }}>
                {filteredComments.length === 0 && (
                  <Text style={{ color: '#64748B' }}>暂无待审核评论</Text>
                )}
                {filteredComments.map((c) => (
                  <CommentCard
                    key={c.id}
                    comment={c}
                    onApprove={handleApprove}
                    onReject={handleReject}
                  />
                ))}
              </div>
            ),
          },
          {
            key: 'approved',
            label: '已通过',
            children: (
              <div style={{ marginTop: 4 }}>
                {filteredComments.length === 0 && (
                  <Text style={{ color: '#64748B' }}>暂无已通过评论</Text>
                )}
                {filteredComments.map((c) => (
                  <CommentCard
                    key={c.id}
                    comment={c}
                    onApprove={handleApprove}
                    onReject={handleReject}
                  />
                ))}
              </div>
            ),
          },
          {
            key: 'rejected',
            label: '已拒绝',
            children: (
              <div style={{ marginTop: 4 }}>
                {filteredComments.length === 0 && (
                  <Text style={{ color: '#64748B' }}>暂无已拒绝评论</Text>
                )}
                {filteredComments.map((c) => (
                  <CommentCard
                    key={c.id}
                    comment={c}
                    onApprove={handleApprove}
                    onReject={handleReject}
                  />
                ))}
              </div>
            ),
          },
        ]}
      />
    </div>
  );
}

export default AdminCommentsPage;
