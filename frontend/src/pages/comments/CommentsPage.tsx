import { useState } from 'react';
import {
  Alert,
  App,
  Badge,
  Button,
  Card,
  Checkbox,
  Empty,
  Pagination,
  Space,
  Tabs,
  Tag,
  Typography,
} from 'antd';
import { CheckOutlined, CloseOutlined, ReloadOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { listAdminComments, reviewAdminComment } from '../../api/comments';
import type { AdminComment, CommentStatus } from '../../api/comments';
import './CommentsPage.css';

const { Title, Text } = Typography;
const commentsKey = ['admin-comments'] as const;
const pageSize = 10;
type ReviewStatus = Exclude<CommentStatus, 'pending'>;

const statuses = {
  pending: { label: '待审核', color: 'warning' },
  approved: { label: '已通过', color: 'success' },
  rejected: { label: '已拒绝', color: 'error' },
} as const;

const commentDate = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
});

function CommentRow({
  comment,
  selected,
  reviewing,
  onSelect,
  onReview,
}: {
  comment: AdminComment;
  selected: boolean;
  reviewing: boolean;
  onSelect: (checked: boolean) => void;
  onReview: (status: ReviewStatus) => void;
}) {
  const pending = comment.status === 'pending';

  return (
    <li className="comment-review-row">
      {pending && (
        <Checkbox
          aria-label={`选择 ${comment.authorName} 的评论`}
          checked={selected}
          disabled={reviewing}
          onChange={(event) => onSelect(event.target.checked)}
        />
      )}
      <div className="comment-review-content">
        <div className="comment-review-meta">
          <Text strong>{comment.authorName}</Text>
          {comment.authorEmail && <Text type="secondary">{comment.authorEmail}</Text>}
          <Tag color={statuses[comment.status].color}>{statuses[comment.status].label}</Tag>
        </div>
        <p className="comment-review-body">{comment.body}</p>
        <div className="comment-review-meta">
          <Text type="secondary">文章：{comment.articleTitle}</Text>
          <Text type="secondary">{commentDate.format(new Date(comment.createdAt))}</Text>
          {comment.reviewedAt && (
            <Text type="secondary">审核于 {commentDate.format(new Date(comment.reviewedAt))}</Text>
          )}
        </div>
      </div>
      {pending && (
        <Space className="comment-review-actions">
          <Button
            type="text"
            icon={<CheckOutlined />}
            disabled={reviewing}
            onClick={() => onReview('approved')}
          >
            通过
          </Button>
          <Button
            type="text"
            danger
            icon={<CloseOutlined />}
            disabled={reviewing}
            onClick={() => onReview('rejected')}
          >
            拒绝
          </Button>
        </Space>
      )}
    </li>
  );
}

export default function CommentsPage() {
  const { message, modal } = App.useApp();
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState<CommentStatus>('pending');
  const [selected, setSelected] = useState<string[]>([]);
  const [page, setPage] = useState(1);
  const [reviewError, setReviewError] = useState('');
  const comments = useQuery({ queryKey: commentsKey, queryFn: listAdminComments });
  const data = comments.data ?? [];
  const filtered = data.filter((comment) => comment.status === activeTab);
  const currentPage = Math.min(page, Math.max(1, Math.ceil(filtered.length / pageSize)));
  const visible = filtered.slice((currentPage - 1) * pageSize, currentPage * pageSize);
  const selectedOnPage =
    activeTab === 'pending'
      ? visible
          .filter(({ commentId }) => selected.includes(commentId))
          .map(({ commentId }) => commentId)
      : [];
  const pendingCount = data.filter(({ status }) => status === 'pending').length;

  const review = useMutation({
    mutationFn: async ({ ids, status }: { ids: string[]; status: ReviewStatus }) => {
      const updated = new Map<string, Awaited<ReturnType<typeof reviewAdminComment>>>();
      const errors: string[] = [];
      // 每页最多处理十条，逐条提交以限制并发并记录每条结果。
      for (const id of ids) {
        try {
          updated.set(id, await reviewAdminComment(id, status));
        } catch (error) {
          errors.push(error instanceof Error ? error.message : '评论审核失败');
        }
      }
      return { updated, errors };
    },
    onMutate: () => setReviewError(''),
    onSuccess: async ({ updated, errors }, { status }) => {
      await queryClient.cancelQueries({ queryKey: commentsKey });
      queryClient.setQueryData<AdminComment[]>(commentsKey, (current) =>
        current?.map((item) => {
          const result = updated.get(item.commentId);
          return result ? { ...item, status: result.status, reviewedAt: result.reviewedAt } : item;
        }),
      );
      setSelected((current) => current.filter((id) => !updated.has(id)));
      const result = `${updated.size} 条评论${statuses[status].label}`;
      if (errors.length) {
        setReviewError(`${result}，${errors.length} 条失败。${errors[0]}`);
      } else {
        void message.success(result);
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: commentsKey }),
        queryClient.invalidateQueries({ queryKey: ['dashboard-overview'] }),
      ]);
    },
  });

  const reviewSelected = (status: ReviewStatus) => {
    if (selectedOnPage.length === 0 || review.isPending) return;
    const action = status === 'approved' ? '通过' : '拒绝';
    modal.confirm({
      title: `${action}选中的 ${selectedOnPage.length} 条评论？`,
      okText: `批量${action}`,
      okButtonProps: { danger: status === 'rejected' },
      cancelText: '取消',
      onOk: () => review.mutateAsync({ ids: selectedOnPage, status }),
    });
  };

  return (
    <div>
      <div className="cms-page-heading">
        <Title level={1}>评论审核</Title>
        <Button
          icon={<ReloadOutlined />}
          loading={comments.isFetching}
          disabled={review.isPending}
          onClick={() => comments.refetch()}
        >
          刷新
        </Button>
      </div>

      <Tabs
        activeKey={activeTab}
        onChange={(key) => {
          setActiveTab(key as CommentStatus);
          setSelected([]);
          setPage(1);
          setReviewError('');
        }}
        items={[
          {
            key: 'pending',
            label: (
              <Space size={6}>
                待审核 <Badge count={pendingCount} color="var(--cms-warning)" />
              </Space>
            ),
            disabled: review.isPending,
          },
          { key: 'approved', label: '已通过', disabled: review.isPending },
          { key: 'rejected', label: '已拒绝', disabled: review.isPending },
        ].map((tab) => ({
          ...tab,
          children: tab.key === activeTab && (
            <Card loading={comments.isPending} className="comment-review-panel">
              {comments.error && <Alert type="error" showIcon title={comments.error.message} />}
              {reviewError && <Alert type="error" showIcon title={reviewError} />}
              {activeTab === 'pending' && visible.length > 0 && (
                <div className="comment-review-toolbar">
                  <Checkbox
                    checked={selectedOnPage.length === visible.length}
                    indeterminate={
                      selectedOnPage.length > 0 && selectedOnPage.length < visible.length
                    }
                    disabled={review.isPending}
                    onChange={(event) =>
                      setSelected(
                        event.target.checked ? visible.map(({ commentId }) => commentId) : [],
                      )
                    }
                  >
                    全选本页
                  </Checkbox>
                  <Text type="secondary" role="status">
                    已选 {selectedOnPage.length} 条
                  </Text>
                  <Space wrap className="comment-review-actions">
                    <Button
                      disabled={review.isPending || selectedOnPage.length === 0}
                      onClick={() => reviewSelected('approved')}
                    >
                      批量通过
                    </Button>
                    <Button
                      danger
                      disabled={review.isPending || selectedOnPage.length === 0}
                      onClick={() => reviewSelected('rejected')}
                    >
                      批量拒绝
                    </Button>
                  </Space>
                </div>
              )}
              {visible.length > 0 ? (
                <ul className="comment-review-list">
                  {visible.map((comment) => (
                    <CommentRow
                      key={comment.commentId}
                      comment={comment}
                      selected={selectedOnPage.includes(comment.commentId)}
                      reviewing={review.isPending}
                      onSelect={(checked) =>
                        setSelected((current) =>
                          checked
                            ? [...current, comment.commentId]
                            : current.filter((id) => id !== comment.commentId),
                        )
                      }
                      onReview={(status) => review.mutate({ ids: [comment.commentId], status })}
                    />
                  ))}
                </ul>
              ) : (
                !comments.error && (
                  <Empty
                    description={'暂无' + statuses[activeTab].label + '评论'}
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                  />
                )
              )}
              {filtered.length > pageSize && (
                <Pagination
                  align="end"
                  current={currentPage}
                  pageSize={pageSize}
                  total={filtered.length}
                  showSizeChanger={false}
                  disabled={review.isPending}
                  onChange={(value) => {
                    setPage(value);
                    setSelected([]);
                  }}
                />
              )}
            </Card>
          ),
        }))}
      />
    </div>
  );
}
