import { useMemo, useState } from 'react';
import { App, Tabs, Card, Button, Tag, Typography, Space, Badge, Empty } from 'antd';
import { CheckOutlined, CloseOutlined, ReloadOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { listAdminComments, reviewAdminComment } from '../../api/comments';
import type { AdminComment, CommentStatus } from '../../api/comments';

const { Title, Text, Paragraph } = Typography;

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
        marginBottom: 16,
      }}
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'flex-start',
          gap: 16,
          flexWrap: 'wrap',
        }}
      >
        <div style={{ flex: 1, minWidth: 0 }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              marginBottom: 4,
              flexWrap: 'wrap',
            }}
          >
            <Text strong style={{ fontSize: 14, color: 'var(--cms-ink)' }}>
              {comment.authorName}
            </Text>
            {comment.authorEmail && (
              <Text style={{ fontSize: 12, color: 'var(--cms-muted)' }}>{comment.authorEmail}</Text>
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
              color: 'var(--cms-ink)',
              marginBottom: 8,
              lineHeight: 1.6,
              whiteSpace: 'pre-wrap',
              overflowWrap: 'anywhere',
            }}
          >
            {comment.body}
          </Paragraph>

          <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
            <Text style={{ fontSize: 12, color: 'var(--cms-primary)' }}>
              文章：{comment.articleTitle}
            </Text>
            <Text style={{ fontSize: 12, color: 'var(--cms-muted)' }}>
              {formatDate(comment.createdAt)}
            </Text>
            {comment.reviewedAt && (
              <Text style={{ fontSize: 12, color: 'var(--cms-muted)' }}>
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
                background: 'var(--cms-success)',
                borderColor: 'var(--cms-success)',
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

function CommentsPage() {
  const { message } = App.useApp();
  const [activeTab, setActiveTab] = useState<CommentStatus>('pending');
  const [reviewingId, setReviewingId] = useState<string | null>(null);
  const queryClient = useQueryClient();
  const {
    data = [],
    isLoading,
    refetch,
  } = useQuery({
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
    onSettled: () => {
      setReviewingId(null);
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
            reviewing={reviewingId === comment.commentId}
            onApprove={(id) => {
              setReviewingId(id);
              reviewMutation.mutate({ id, status: 'approved' });
            }}
            onReject={(id) => {
              setReviewingId(id);
              reviewMutation.mutate({ id, status: 'rejected' });
            }}
          />
        ))
      )}
    </div>
  );

  return (
    <div>
      <div className="cms-page-heading">
        <Title level={1}>评论审核</Title>
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
                    backgroundColor: 'var(--cms-warning)',
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

export default CommentsPage;
