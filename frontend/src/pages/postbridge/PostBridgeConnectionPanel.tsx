import { Alert, Button, Card, Descriptions, Form, Input, List, Space, Tag } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { activatePostBridge, getPostBridgeTargets, registerPostBridge } from '../../api/postbridge';
import type { PostBridgeConnection, PostBridgeRegistration, PostBridgeTarget } from '../../api/postbridge';
import { postBridgeTargetsKey } from './PostBridgePreparation';

export const postBridgeConnectionKey = ['postbridge-connection'];

interface RegistrationForm {
  name: string;
  invitationToken: string;
}

export function PostBridgeConnectionPanel({ connection }: { connection: PostBridgeConnection }) {
  const queryClient = useQueryClient();
  const [form] = Form.useForm<RegistrationForm>();
  const [registrationError, setRegistrationError] = useState('');
  const [unrecordedRegistration, setUnrecordedRegistration] = useState<PostBridgeRegistration>();
  const targets = useQuery({ queryKey: postBridgeTargetsKey, queryFn: getPostBridgeTargets, enabled: false, retry: false });
  const registration = useMutation({ mutationFn: registerPostBridge, gcTime: 0 });
  const activation = useMutation({
    mutationFn: activatePostBridge,
    onSuccess: (result) => {
      if (result.targetsVerified) queryClient.setQueryData(postBridgeTargetsKey, { items: result.items ?? [] });
    },
  });

  function submitRegistration(values: RegistrationForm) {
    if (registration.isPending) return;
    const payload = { name: values.name.trim(), invitationToken: values.invitationToken.trim() };
    form.resetFields(['invitationToken']);
    setRegistrationError('');
    registration.mutate(payload, {
      onSuccess: (result) => {
        setUnrecordedRegistration(result.recorded ? undefined : result);
        if (result.recorded) {
          queryClient.setQueryData(postBridgeConnectionKey, {
            ...connection, registrationId: result.registrationId, registrationStatus: result.status,
          });
        }
        registration.reset();
      },
      onError: (error) => {
        setRegistrationError(error.message);
        registration.reset();
      },
    });
  }

  const registrationId = unrecordedRegistration?.registrationId ?? connection.registrationId;
  const registrationStatus = unrecordedRegistration?.status ?? connection.registrationStatus;

  return (
    <Space orientation="vertical" size={16} style={{ display: 'flex', maxWidth: 700 }}>
      <Card title="连接状态">
        <Descriptions
          column={1}
          size="small"
          items={[
            { key: 'credentials', label: '签名凭据', children: <Tag color={connection.signingConfigured ? 'success' : 'default'}>{connection.signingConfigured ? '已配置' : '待配置'}</Tag> },
            { key: 'registration', label: '注册编号', children: <span style={{ overflowWrap: 'anywhere' }}>{registrationId || '—'}</span> },
            ...(registrationStatus ? [{
              key: 'registrationStatus', label: '注册状态', children: <Tag>{registrationStatus === 'pending' ? '待审批' : registrationStatus}</Tag>,
            }] : []),
          ]}
        />
        <Space orientation="vertical" size={12} style={{ display: 'flex', marginTop: 16 }}>
          {unrecordedRegistration && <Alert type="warning" showIcon message="注册记录未保存" />}
          {activation.error && <Alert type="error" showIcon message={activation.error.message} />}
          {targets.error && <Alert type="error" showIcon message={targets.error.message} />}
          <Space wrap>
            <Button disabled={!connection.signingConfigured || activation.isPending || activation.data?.status === 'active'} loading={activation.isPending} onClick={() => activation.mutate()}>
              激活密钥
            </Button>
            <Button disabled={!connection.signingConfigured || activation.isPending} loading={targets.isFetching} onClick={() => { void targets.refetch(); }}>
              查询目标
            </Button>
            {activation.data?.status === 'active' && <Tag color="success">密钥已激活</Tag>}
            {activation.data?.status === 'active' && !activation.data.targetsVerified && !targets.isSuccess && <Tag color="warning">目标待核对</Tag>}
          </Space>
          {targets.data && (
            <List<PostBridgeTarget>
              size="small"
              dataSource={targets.data.items}
              locale={{ emptyText: '暂无授权目标' }}
              renderItem={target => <List.Item><Space wrap><span>{target.display_name || target.account_id}</span><Tag>{target.platform.toUpperCase()}</Tag></Space></List.Item>}
            />
          )}
        </Space>
      </Card>
      <Card title="应用注册">
        <Form form={form} layout="vertical" onFinish={submitRegistration} disabled={registration.isPending}>
          <Form.Item name="name" label="应用名称" rules={[{ required: true, whitespace: true, message: '请输入应用名称' }]}>
            <Input maxLength={120} />
          </Form.Item>
          <Form.Item name="invitationToken" label="一次性邀请" rules={[{ required: true, whitespace: true, message: '请输入一次性邀请' }]}>
            <Input.Password visibilityToggle={false} autoComplete="off" />
          </Form.Item>
          <Space orientation="vertical" size={12} style={{ display: 'flex' }}>
            {registrationError && <Alert type="error" showIcon message={registrationError} />}
            <Button htmlType="submit" loading={registration.isPending}>注册应用</Button>
          </Space>
        </Form>
      </Card>
    </Space>
  );
}
