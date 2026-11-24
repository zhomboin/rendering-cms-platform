import { apiDelete, apiGet, apiPatch, apiPost } from './client';

const base = '/admin/mcp';

const articlePermissions = [
  { value: 'create_draft', label: '创建草稿' },
  { value: 'get_article', label: '读取文章' },
  { value: 'update_draft', label: '修改草稿' },
  { value: 'article_status', label: '查询文章状态' },
  { value: 'publish_article', label: '发布文章' },
] as const;
const albumPermissions = [
  { value: 'create_album', label: '创建相册' },
  { value: 'list_albums', label: '列出相册' },
  { value: 'get_album', label: '读取相册详情' },
  { value: 'update_album', label: '修改相册' },
  { value: 'publish_album', label: '发布相册' },
  { value: 'unpublish_album', label: '下架相册' },
] as const;
const photoPermissions = [
  { value: 'list_album_photos', label: '查看照片' },
  { value: 'upload_album_photo', label: '上传照片' },
  { value: 'update_album_photo', label: '编辑照片说明' },
  { value: 'reorder_album_photos', label: '调整照片顺序' },
  { value: 'set_album_cover', label: '设置相册封面' },
  { value: 'delete_album_photo', label: '删除照片' },
] as const;
export const permissionGroups = [
  { label: '文章', options: articlePermissions },
  { label: '相册', options: albumPermissions },
  { label: '照片', options: photoPermissions },
] as const;
export const permissionOptions = [...articlePermissions, ...albumPermissions, ...photoPermissions];
export type McpPermission = (typeof permissionOptions)[number]['value'];
export type McpReadScope = 'own' | 'all';
export const readScopeLabels = { own: '仅本连接创建', all: '全部可读内容' } as const;
export const defaultPermissions = articlePermissions
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
  readScope: McpReadScope;
  revoked: boolean;
  oauth: boolean;
  clientName: string;
  lastSeenAt: string | null;
}
export interface GrantInput {
  label: string;
  readScope: McpReadScope;
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
  albumId: string | null;
}

export const getSettings = () => apiGet<McpSettings>(base + '/settings');
export const saveSettings = (value: SettingsInput) => apiPatch<void>(base + '/settings', value);
export const getConnections = () => apiGet<McpConnection[]>(base + '/connections');
export const createConnection = (input: GrantInput) => apiPost<GrantResult>(base + '/connections', input);
export const setPermissions = (
  id: string,
  permissions: McpPermission[],
  readScope: McpReadScope,
  expectedRevision: number,
) =>
  apiPatch<void>(base + '/connections/' + id, {
    permissions,
    readScope,
    expectedRevision,
  });
export const revokeConnection = (id: string) => apiDelete<void>(base + '/connections/' + id);
export const getOperations = (before: string) =>
  apiGet<{ items: McpOperation[]; next: string }>(
    base + '/operations' + (before ? '?before=' + encodeURIComponent(before) : ''),
  );
