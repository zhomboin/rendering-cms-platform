import { Alert, Button, Select, Space, Spin, Table, Tabs, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { ReloadOutlined } from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef } from 'react';
import { Navigate, useSearchParams } from 'react-router-dom';
import { listAdminArticles } from '../../api/articles';
import { getAuthUser } from '../../api/auth';
import { ApiRequestError } from '../../api/client';
import { getPostBridgeConnection, listPostBridgeAttempts } from '../../api/postbridge';
import type { PostBridgeAttempt } from '../../api/postbridge';
import {
  attemptDetailKey,
  attemptListKey,
  attemptStatusOptions,
  formatPostBridgeTime,
  PostBridgeAttemptPanel,
  PostBridgeStatus,
  shouldPollAttempt,
} from './PostBridgeAttemptPanel';
import { postBridgeConnectionKey, PostBridgeConnectionPanel } from './PostBridgeConnectionPanel';
import { PostBridgePreparation } from './PostBridgePreparation';

const { Title, Text } = Typography;
const pageSize = 10;

function PostBridgeWorkspace() {
  const [params, setParams] = useSearchParams();
  const queryClient = useQueryClient();
  const requestIds = useRef(new Map<string, string>());
  const articleId = params.get('articleId') || undefined;
  const status = attemptStatusOptions.find(
    (option) => option.value === params.get('status'),
  )?.value;
  const attemptId = params.get('attemptId');
  const pageParam = Number(params.get('page'));
  const page = Number.isSafeInteger(pageParam) && pageParam > 0 ? pageParam : 1;

  function updateParams(changes: Record<string, string | undefined>) {
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      for (const [key, value] of Object.entries(changes)) {
        if (value) next.set(key, value);
        else next.delete(key);
      }
      return next;
    });
  }

  const connection = useQuery({
    queryKey: postBridgeConnectionKey,
    queryFn: getPostBridgeConnection,
    retry: false,
  });
  const enabled = connection.data?.enabled === true;
  const distributionEnabled = enabled && connection.data?.signingConfigured === true;
  const tab =
    params.get('tab') === 'connection' || !distributionEnabled ? 'connection' : 'attempts';
  const articles = useQuery({
    queryKey: ['admin-articles'],
    queryFn: listAdminArticles,
    enabled: distributionEnabled && tab === 'attempts',
  });
  const attempts = useQuery({
    queryKey: [...attemptListKey, { articleId, status, page }],
    queryFn: () => listPostBridgeAttempts({ articleId, status, page, pageSize }),
    enabled: distributionEnabled && tab === 'attempts',
    retry: false,
    refetchInterval: ({ state: { data } }) =>
      data?.items.some(shouldPollAttempt) ? 10_000 : false,
  });
  const selectedArticle = articles.data?.find((article) => article.articleId === articleId);
  const canPrepare = Boolean(
    connection.data?.signingConfigured &&
      selectedArticle?.status === 'published' &&
      selectedArticle.version,
  );
  const articleTitles = new Map(
    articles.data?.map((article) => [article.articleId, article.title]),
  );
  const columns: ColumnsType<PostBridgeAttempt> = [
    {
      title: '文章',
      key: 'articleId',
      ellipsis: true,
      render: (_, attempt) => articleTitles.get(attempt.articleId) || attempt.articleId,
    },
    { title: '目标账号', dataIndex: 'targetAccountId', width: 130, ellipsis: true },
    {
      title: '状态',
      key: 'status',
      width: 205,
      render: (_, attempt) => (
        <Space size={4} wrap>
          <PostBridgeStatus attempt={attempt} />
          {attempt.revoked && <Text type="secondary">已撤销</Text>}
        </Space>
      ),
    },
    {
      title: '更新于',
      dataIndex: 'updatedAt',
      width: 155,
      render: (value: string) => formatPostBridgeTime(value),
    },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_, attempt) => (
        <Button
          type="link"
          onClick={() => updateParams({ attemptId: attempt.id, prepare: undefined })}
        >
          详情
        </Button>
      ),
    },
  ];
  const disabledConnection =
    connection.error instanceof ApiRequestError && connection.error.status === 404;
  const records = (
    <Space orientation="vertical" size={16} style={{ display: 'flex' }}>
      <Space wrap>
        <Select
          aria-label="文章筛选"
          allowClear
          showSearch={{ optionFilterProp: 'label' }}
          value={articleId}
          placeholder="全部文章"
          options={articles.data?.map((article) => ({
            value: article.articleId,
            label: article.title,
          }))}
          style={{ width: 224, maxWidth: '100%' }}
          onChange={(value) =>
            updateParams({
              articleId: value,
              page: undefined,
              prepare: undefined,
              attemptId: undefined,
            })
          }
        />
        <Select
          aria-label="分发状态筛选"
          allowClear
          value={status}
          placeholder="全部状态"
          options={attemptStatusOptions}
          style={{ width: 138 }}
          onChange={(value) =>
            updateParams({ status: value, page: undefined, attemptId: undefined })
          }
        />
        <Button
          icon={<ReloadOutlined />}
          loading={attempts.isFetching}
          onClick={() => {
            void attempts.refetch();
          }}
        >
          刷新
        </Button>
        {canPrepare && (
          <Button
            type="primary"
            onClick={() => updateParams({ prepare: '1', attemptId: undefined })}
          >
            创建准备稿
          </Button>
        )}
      </Space>
      {articles.error && <Alert type="error" showIcon message={articles.error.message} />}
      {attempts.error && <Alert type="error" showIcon message={attempts.error.message} />}
      <Table<PostBridgeAttempt>
        rowKey="id"
        columns={columns}
        dataSource={attempts.data?.items ?? []}
        loading={attempts.isPending}
        locale={{ emptyText: '暂无分发记录' }}
        scroll={{ x: 760 }}
        pagination={{
          current: page,
          pageSize,
          total: attempts.data?.total ?? 0,
          showSizeChanger: false,
          onChange: (nextPage) =>
            updateParams({ page: nextPage === 1 ? undefined : String(nextPage) }),
        }}
      />
    </Space>
  );

  return (
    <section style={{ minWidth: 0 }}>
      <Title level={1} style={{ margin: '0 0 12px', fontSize: 24 }}>
        PostBridge
      </Title>
      {connection.isPending && <Spin />}
      {disabledConnection && <Alert type="warning" showIcon message="PostBridge 未启用" />}
      {connection.error && !disabledConnection && (
        <Alert
          type="error"
          showIcon
          message={connection.error.message}
          action={
            <Button
              size="small"
              onClick={() => {
                void connection.refetch();
              }}
            >
              重试
            </Button>
          }
        />
      )}
      {connection.data?.enabled === false && (
        <Alert type="warning" showIcon message="PostBridge 未启用" />
      )}
      {enabled && connection.data && (
        <Tabs
          activeKey={tab}
          onChange={(key) =>
            updateParams({
              tab: key === 'connection' ? key : undefined,
              prepare: undefined,
              attemptId: undefined,
            })
          }
          items={[
            {
              key: 'attempts',
              label: '分发记录',
              disabled: !distributionEnabled,
              children: tab === 'attempts' ? records : null,
            },
            {
              key: 'connection',
              label: '连接配置',
              children:
                tab === 'connection' ? (
                  <PostBridgeConnectionPanel connection={connection.data} />
                ) : null,
            },
          ]}
        />
      )}
      {enabled &&
        tab === 'attempts' &&
        params.get('prepare') === '1' &&
        canPrepare &&
        selectedArticle && (
          <PostBridgePreparation
            key={selectedArticle.articleId}
            article={selectedArticle}
            requestIds={requestIds.current}
            onClose={() => updateParams({ prepare: undefined })}
            onCreate={(attempt) => {
              queryClient.setQueryData(attemptDetailKey(attempt.id), attempt);
              void queryClient.invalidateQueries({ queryKey: attemptListKey });
              updateParams({
                prepare: undefined,
                attemptId: attempt.id,
                status: undefined,
                page: undefined,
              });
            }}
          />
        )}
      {enabled && tab === 'attempts' && attemptId && (
        <PostBridgeAttemptPanel
          key={attemptId}
          attemptId={attemptId}
          articleTitles={articleTitles}
          onClose={() => updateParams({ attemptId: undefined })}
        />
      )}
    </section>
  );
}

export default function PostBridgePage() {
  if (getAuthUser()?.role !== 'admin') return <Navigate to="/admin/articles" replace />;
  return <PostBridgeWorkspace />;
}
