import { Alert, Button, Descriptions, Drawer, Empty, Popconfirm, Space, Spin, Tag } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef } from 'react';
import { getPostBridgeAttempt, runPostBridgeAttemptAction } from '../../api/postbridge';
import type {
  PostBridgeAttempt,
  PostBridgeAttemptAction,
  PostBridgeAttemptStatus,
} from '../../api/postbridge';

const attemptStatuses: Record<PostBridgeAttemptStatus, { label: string; color: string }> = {
  preparing: { label: '准备中', color: 'processing' },
  prepared: { label: '待确认', color: 'warning' },
  confirmed: { label: '已确认', color: 'blue' },
  submitting: { label: '提交中', color: 'processing' },
  in_progress: { label: '执行中', color: 'processing' },
  completed: { label: '已完成', color: 'success' },
  needs_attention: { label: '需核对', color: 'warning' },
  revoked: { label: '已撤销', color: 'default' },
  expired: { label: '已过期', color: 'default' },
  failed: { label: '失败', color: 'error' },
  cancelled: { label: '已取消', color: 'default' },
};

export const attemptStatusOptions = Object.entries(attemptStatuses).map(([value, { label }]) => ({
  value,
  label,
}));
export const attemptListKey = ['postbridge-attempts'];
export const attemptDetailKey = (id: string) => ['postbridge-attempt', id];

const activeStatuses = new Set<PostBridgeAttemptStatus>([
  'preparing',
  'prepared',
  'confirmed',
  'submitting',
  'in_progress',
]);

export function shouldPollAttempt(attempt: PostBridgeAttempt) {
  return attempt.cancelPending || (!attempt.revoked && activeStatuses.has(attempt.status));
}

function safeExternalURL(value?: string) {
  if (!value) return undefined;
  try {
    const url = new URL(value);
    return url.protocol === 'https:' && !url.username && !url.password ? url.href : undefined;
  } catch {
    return undefined;
  }
}

const childStatuses: Record<string, { label: string; color: string }> = {
  queued: { label: '排队中', color: 'processing' },
  running: { label: '执行中', color: 'processing' },
  waiting_user: { label: '待人工处理', color: 'warning' },
  reconciling: { label: '核对中', color: 'warning' },
  failed: { label: '失败', color: 'error' },
  cancelled: { label: '已取消', color: 'default' },
};

export function PostBridgeStatus({ attempt }: { attempt: PostBridgeAttempt }) {
  const childStatus = attempt.childStatus && childStatuses[attempt.childStatus];
  const status = childStatus || attemptStatuses[attempt.status];
  if (attempt.externalStatus === 'draft_saved')
    return <Tag color="success">已保存到 CSDN 草稿箱</Tag>;
  if (attempt.externalStatus === 'unknown') return <Tag color="warning">结果未知</Tag>;
  return <Tag color={status?.color}>{status?.label ?? '状态待核对'}</Tag>;
}

const dateFormat = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
});

export function formatPostBridgeTime(value?: string) {
  if (!value) return '—';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '—' : dateFormat.format(date);
}

interface PostBridgeAttemptPanelProps {
  attemptId: string;
  articleTitles: ReadonlyMap<string, string>;
  onClose: () => void;
}

export function PostBridgeAttemptPanel({
  attemptId,
  articleTitles,
  onClose,
}: PostBridgeAttemptPanelProps) {
  const queryClient = useQueryClient();
  const waitingForPreview = useRef(false);
  const query = useQuery({
    queryKey: attemptDetailKey(attemptId),
    queryFn: () => getPostBridgeAttempt(attemptId),
    retry: false,
    refetchInterval: ({ state: { data } }) => (data && shouldPollAttempt(data) ? 5000 : false),
  });
  const action = useMutation({
    mutationFn: (kind: PostBridgeAttemptAction) => runPostBridgeAttemptAction(attemptId, kind),
    onSuccess: async (result) => {
      await queryClient.cancelQueries({ queryKey: attemptDetailKey(attemptId) });
      queryClient.setQueryData(attemptDetailKey(attemptId), result);
      void queryClient.invalidateQueries({ queryKey: attemptListKey });
    },
    onError: () => {
      void queryClient.invalidateQueries({ queryKey: attemptDetailKey(attemptId) });
    },
  });
  const { isPending, mutate } = action;
  const attempt = query.data;
  const canRefresh = attempt?.canRefresh;
  const previewUrl =
    !attempt?.revoked && attempt && ['prepared', 'confirmed'].includes(attempt.status)
      ? safeExternalURL(attempt.previewUrl)
      : undefined;

  useEffect(() => {
    function refreshAfterPreview() {
      if (
        document.visibilityState !== 'visible' ||
        !waitingForPreview.current ||
        !canRefresh ||
        isPending
      )
        return;
      waitingForPreview.current = false;
      mutate('refresh');
    }
    window.addEventListener('focus', refreshAfterPreview);
    document.addEventListener('visibilitychange', refreshAfterPreview);
    return () => {
      window.removeEventListener('focus', refreshAfterPreview);
      document.removeEventListener('visibilitychange', refreshAfterPreview);
    };
  }, [canRefresh, isPending, mutate]);

  const externalUnknown = attempt?.externalStatus === 'unknown';
  const cancellingDistribution =
    !externalUnknown && (attempt?.status === 'in_progress' || attempt?.status === 'submitting');
  const detailUrl = safeExternalURL(attempt?.detailUrl);
  const draftUrl =
    attempt?.externalStatus === 'draft_saved' ? safeExternalURL(attempt.remoteUrl) : undefined;
  return (
    <Drawer title="分发详情" size={520} open onClose={onClose}>
      <Space orientation="vertical" size={16} style={{ display: 'flex' }}>
        {query.isPending && <Spin />}
        {query.error && <Alert type="error" showIcon message={query.error.message} />}
        {action.error && <Alert type="error" showIcon message={action.error.message} />}
        {attempt && (
          <>
            <Descriptions
              column={1}
              size="small"
              items={[
                {
                  key: 'article',
                  label: '文章',
                  children: articleTitles.get(attempt.articleId) || attempt.articleId,
                },
                { key: 'target', label: '目标账号', children: attempt.targetAccountId },
                { key: 'version', label: '文章版本', children: attempt.sourceVersion },
                {
                  key: 'state',
                  label: '状态',
                  children: (
                    <Space size={4} wrap>
                      <PostBridgeStatus attempt={attempt} />
                      {attempt.revoked && <Tag>授权已撤销</Tag>}
                      {attempt.cancelPending && <Tag color="processing">取消处理中</Tag>}
                      {attempt.cancelResult === 'not_cancellable' && (
                        <Tag color="warning">无法取消</Tag>
                      )}
                      {attempt.cancelResult === 'not_found' && (
                        <Tag color="warning">暂未找到分发</Tag>
                      )}
                      {attempt.cancelResult === 'accepted' &&
                        !attempt.cancelPending &&
                        attempt.childStatus !== 'cancelled' &&
                        attempt.status !== 'cancelled' && <Tag>已请求取消</Tag>}
                    </Space>
                  ),
                },
                {
                  key: 'expires',
                  label: '授权到期',
                  children: formatPostBridgeTime(attempt.authorizationExpiresAt),
                },
                ...(attempt.lastConfirmedAt
                  ? [
                      {
                        key: 'checked',
                        label: '最近核对',
                        children: formatPostBridgeTime(attempt.lastConfirmedAt),
                      },
                    ]
                  : []),
                {
                  key: 'updated',
                  label: '更新于',
                  children: formatPostBridgeTime(attempt.updatedAt),
                },
              ]}
            />
            {externalUnknown && !detailUrl && (
              <Alert type="warning" showIcon message="请到 PostBridge 核对结果" />
            )}
            {attempt.errorCode && !externalUnknown && attempt.status === 'failed' && (
              <Alert type="error" showIcon message="分发未完成" />
            )}
            <Space wrap>
              {previewUrl && (
                <Button
                  href={previewUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  onClick={() => {
                    waitingForPreview.current = true;
                  }}
                >
                  打开 PostBridge 确认
                </Button>
              )}
              {canRefresh && (
                <Button
                  disabled={isPending}
                  loading={isPending && action.variables === 'refresh'}
                  onClick={() => {
                    waitingForPreview.current = false;
                    mutate('refresh');
                  }}
                >
                  刷新状态
                </Button>
              )}
              {attempt.canSubmit &&
                !attempt.revoked &&
                !externalUnknown &&
                attempt.externalStatus !== 'draft_saved' && (
                  <Button
                    type="primary"
                    disabled={isPending}
                    loading={isPending && action.variables === 'submit'}
                    onClick={() => mutate('submit')}
                  >
                    提交到 CSDN 草稿箱
                  </Button>
                )}
              {attempt.canCancel && !attempt.revoked && !attempt.cancelPending && (
                <Popconfirm
                  title={cancellingDistribution ? '取消分发并撤销授权？' : '撤销分发授权？'}
                  okText={cancellingDistribution ? '确认取消' : '确认撤销'}
                  cancelText="返回"
                  onConfirm={() => mutate('cancel')}
                >
                  <Button danger disabled={isPending}>
                    {cancellingDistribution ? '取消分发' : '撤销授权'}
                  </Button>
                </Popconfirm>
              )}
              {detailUrl && (
                <Button type="link" href={detailUrl} target="_blank" rel="noopener noreferrer">
                  {externalUnknown ? '在 PostBridge 核对' : '查看 PostBridge 记录'}
                </Button>
              )}
              {draftUrl && (
                <Button type="link" href={draftUrl} target="_blank" rel="noopener noreferrer">
                  打开 CSDN 草稿
                </Button>
              )}
            </Space>
          </>
        )}
        {!query.isPending && !query.error && !attempt && <Empty description="暂无分发详情" />}
      </Space>
    </Drawer>
  );
}
