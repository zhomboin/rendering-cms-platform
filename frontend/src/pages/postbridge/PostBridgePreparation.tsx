import { Alert, Button, Checkbox, Drawer, Form, Select, Space } from 'antd';
import { useMutation, useQuery } from '@tanstack/react-query';
import type { AdminArticleRecord } from '../../api/articles';
import { createPostBridgePreparation, getPostBridgeTargets } from '../../api/postbridge';
import type { PostBridgeAttempt, PostBridgePreparationInput } from '../../api/postbridge';

interface PreparationForm {
  targetAccountId: string;
  textOnly?: boolean;
}

interface PostBridgePreparationProps {
  article: AdminArticleRecord;
  requestIds: Map<string, string>;
  onCreate: (attempt: PostBridgeAttempt) => void;
  onClose: () => void;
}

export const postBridgeTargetsKey = ['postbridge-targets'];

function newPreparationRequestId() {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('');
  return [hex.slice(0, 8), hex.slice(8, 12), hex.slice(12, 16), hex.slice(16, 20), hex.slice(20)].join('-');
}

export function PostBridgePreparation({ article, requestIds, onCreate, onClose }: PostBridgePreparationProps) {
  const [form] = Form.useForm<PreparationForm>();
  const targetAccountId = Form.useWatch('targetAccountId', form);
  const textOnly = Form.useWatch('textOnly', form);
  const { articleId, version } = article;
  const hasCover = Boolean(article.coverImageUrl?.trim());
  const targets = useQuery({ queryKey: postBridgeTargetsKey, queryFn: getPostBridgeTargets, staleTime: 60_000, retry: false });
  const options = (targets.data?.items ?? [])
    .filter(target => target.platform === 'csdn' && target.operations.includes('save_draft') && target.body_formats.includes('markdown'))
    .map(target => ({ value: target.account_id, label: target.display_name || target.account_id }));
  const prepare = useMutation({ mutationFn: (payload: PostBridgePreparationInput) => createPostBridgePreparation(articleId, payload) });

  function createPreparation(values: PreparationForm) {
    if (prepare.isPending || !version || !options.some(option => option.value === values.targetAccountId) || (hasCover && !values.textOnly)) return;
    const onlyText = Boolean(values.textOnly);
    const key = JSON.stringify([articleId, version, values.targetAccountId, onlyText]);
    const requestId = requestIds.get(key) ?? newPreparationRequestId();
    requestIds.set(key, requestId);
    prepare.mutate({ requestId, expectedVersion: version, targetAccountId: values.targetAccountId, textOnly: onlyText }, {
      onSuccess: (attempt) => {
        requestIds.delete(key);
        onCreate(attempt);
      },
    });
  }

  return (
    <Drawer title="创建准备稿" size={480} open onClose={() => { if (!prepare.isPending) onClose(); }}>
      <Form form={form} layout="vertical" onFinish={createPreparation} disabled={prepare.isPending}>
        <Form.Item label="文章">
          <span>{article.title}</span>
        </Form.Item>
        <Form.Item name="targetAccountId" label="目标账号" rules={[{ required: true, message: '请选择目标账号' }]}>
          <Select options={options} virtual={false} loading={targets.isPending} placeholder="请选择" />
        </Form.Item>
        {hasCover && (
          <Form.Item name="textOnly" valuePropName="checked">
            <Checkbox>省略封面</Checkbox>
          </Form.Item>
        )}
        <Space orientation="vertical" size={12} style={{ display: 'flex' }}>
          {targets.error && (
            <Alert type="error" showIcon message={targets.error.message} action={<Button size="small" onClick={() => { void targets.refetch(); }}>重试</Button>} />
          )}
          {!targets.isPending && !targets.error && options.length === 0 && <Alert type="warning" showIcon message="暂无可用的 CSDN 目标账号" />}
          {prepare.error && <Alert type="error" showIcon message={prepare.error.message} />}
          <Button type="primary" htmlType="submit" loading={prepare.isPending} disabled={!version || !targetAccountId || (hasCover && !textOnly)}>
            创建准备稿
          </Button>
        </Space>
      </Form>
    </Drawer>
  );
}
