import { apiDelete, apiGet, apiPatch, apiPost } from './client';

const base = '/admin/mcp';

export const permissionOptions = [
  { value: 'create_draft', label: '创建草稿' },
  { value: 'get_article', label: '读取文章' },
  { value: 'update_draft', label: '修改草稿' },
  { value: 'article_status', label: '查询文章状态' },
  { value: 'publish_article', label: '发布文章' },
] as const;
export type McpPermission = (typeof permissionOptions)[number]['value'];
export const defaultPermissions = permissionOptions
  .map((option) => option.value)
  .filter((name) => name !== 'publish_article');

export interface SettingsInput {
  enabled: boolean;
  origin: string;
  adminOrigin: string;
  blogOrigin: string;
  revision: number;
}
export interface McpSettings extends SettingsInput {
  endpoint: string;
  issuer: string;
}
export interface McpConnection {
  id: string;
  label: string;
  allowPublish: boolean;
  permissions: McpPermission[];
  permissionRevision: number;
  revoked: boolean;
  oauth: boolean;
  clientName: string;
  lastSeenAt: string | null;
}
export interface GrantInput {
  label: string;
  permissions: McpPermission[];
  oauthClient?: string;
  oauthSubject?: string;
}
export interface GrantResult {
  id: string;
  endpoint: string;
  token?: string;
}
export interface McpOperation {
  id: string;
  connection: string;
  operation: string;
  outcome: 'success' | 'failed' | 'pending';
  createdAt: string;
  articleId: string | null;
}

export const getSettings = () => apiGet<McpSettings>(base + '/settings');
export const saveSettings = (value: SettingsInput) => apiPatch<void>(base + '/settings', value);
export const getConnections = () => apiGet<McpConnection[]>(base + '/connections');
export const createConnection = (input: GrantInput) => apiPost<GrantResult>(base + '/connections', input);
export const setPermissions = (id: string, permissions: McpPermission[], expectedRevision: number) =>
  apiPatch<void>(base + '/connections/' + id, { permissions, expectedRevision });
export const revokeConnection = (id: string) => apiDelete<void>(base + '/connections/' + id);
export const getOperations = (before: string) =>
  apiGet<{ items: McpOperation[]; next: string }>(
    base + '/operations' + (before ? '?before=' + encodeURIComponent(before) : ''),
  );
