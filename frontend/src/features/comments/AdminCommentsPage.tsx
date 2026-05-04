import { useMemo, useState } from 'react';
import { Tabs, Card, Button, Tag, Typography, Space, Badge, Empty, message } from 'antd';
import { CheckOutlined, CloseOutlined, ReloadOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { listAdminComments, reviewAdminComment } from '../../api/comments';
import type { AdminComment, CommentStatus } from '../../api/comments';

const { Title, Text, Paragraph } = Typography;

const statusBorderColor: Record<CommentStatus, string> = {
  pending: '#F59E0B',
  approved: '#22C55E',
  rejected: '#EF4444',
};

const statusBadgeColor: Record<CommentStatus, 'warning' | 'success' | 'error'> = {
  pending: 'warning',
  approved: 'success',
  rejected: 'error',
};

const statusLabel: Record<CommentStatus, string> = {
  pending: '待审核',
  approved: '已通过',
  rejected: '已拒绝',
};

function formatDate(value: string | null) {
  if (!value) return '';
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value));
}

function CommentCard({
  comment,
  reviewing,
  onApprove,
  onReject,
}: {
  comment: AdminComment;
  reviewing: boolean;
  onApprove: (id: string) => void;
  onReject: (id: string) => void;
}) {
  return (
    <Card
      style={{
        borderRadius: 8,
        padding: 20,
        background: '#FFFFFF',
        border: '1px solid #E2E8F0',
        borderLeft: `4px solid ${statusBorderColor[comment.status]}`,
        boxShadow: '0 1px 3px 0 rgba(0,0,0,0.06)',
        marginBottom: 16,
      }}
      styles={{ body: { padding: 0 } }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 16 }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4, flexWrap: 'wrap' }}>
            <Text strong style={{ fontSize: 14, color: '#0F172A' }}>
              {comment.authorName}
            </Text>
            {comment.authorEmail && (
              <Text style={{ fontSize: 12, color: '#64748B' }}>{comment.authorEmail}</Text>
            )}
            <Tag
              color={statusBadgeColor[comment.status]}
              style={{ borderRadius: 6, fontSize: 12, lineHeight: '22px' }}
            >
              {statusLabel[comment.status]}
            </Tag>
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
            <Text style={{ fontSize: 12, color: '#64748B' }}>{formatDate(comment.createdAt)}</Text>
            {comment.reviewedAt && (
              <Text style={{ fontSize: 12, color: '#64748B' }}>
                审核于 {formatDate(comment.reviewedAt)}
              </Text>
            )}
          </div>
        </div>

        {comment.status === 'pending' && (
          <Space style={{ flexShrink: 0 }}>
            <Button
              type="primary"
              size="small"
              icon={<CheckOutlined />}
              loading={reviewing}
              style={{
                borderRadius: 8,
                background: '#22C55E',
                borderColor: '#22C55E',
              }}
              onClick={() => onApprove(comment.commentId)}
            >
              通过
            </Button>
            <Button
              size="small"
              danger
              icon={<CloseOutlined />}
              loading={reviewing}
              style={{ borderRadius: 8 }}
              onClick={() => onReject(comment.commentId)}
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
  const [activeTab, setActiveTab] = useState<CommentStatus>('pending');
  const queryClient = useQueryClient();
  const { data = [], isLoading, refetch } = useQuery({
    queryKey: ['admin-comments'],
    queryFn: listAdminComments,
  });

  const reviewMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: CommentStatus }) =>
      reviewAdminComment(id, status),
    onSuccess: async (_, variables) => {
      message.success(variables.status === 'approved' ? '评论已通过' : '评论已拒绝');
      await queryClient.invalidateQueries({ queryKey: ['admin-comments'] });
    },
  });

  const commentsByStatus = useMemo(
    () => ({
      pending: data.filter((comment) => comment.status === 'pending'),
      approved: data.filter((comment) => comment.status === 'approved'),
      rejected: data.filter((comment) => comment.status === 'rejected'),
    }),
    [data],
  );
  const filteredComments = commentsByStatus[activeTab];

  const renderComments = (emptyText: string) => (
    <div style={{ marginTop: 4 }}>
      {filteredComments.length === 0 && !isLoading ? (
        <Empty description={emptyText} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        filteredComments.map((comment) => (
          <CommentCard
            key={comment.commentId}
            comment={comment}
            reviewing={reviewMutation.isPending}
            onApprove={(id) => reviewMutation.mutate({ id, status: 'approved' })}
            onReject={(id) => reviewMutation.mutate({ id, status: 'rejected' })}
          />
        ))
      )}
    </div>
  );

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16 }}>
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
        <Button icon={<ReloadOutlined />} loading={isLoading} onClick={() => refetch()}>
          刷新
        </Button>
      </div>

      <Tabs
        activeKey={activeTab}
        onChange={(key) => setActiveTab(key as CommentStatus)}
        style={{ marginBottom: 0 }}
        items={[
          {
            key: 'pending',
            label: (
              <span>
                待审核
                <Badge
                  count={commentsByStatus.pending.length}
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
            children: renderComments('暂无待审核评论'),
          },
          {
            key: 'approved',
            label: '已通过',
            children: renderComments('暂无已通过评论'),
          },
          {
            key: 'rejected',
            label: '已拒绝',
            children: renderComments('暂无已拒绝评论'),
          },
        ]}
      />
    </div>
  );
}

export default AdminCommentsPage;
