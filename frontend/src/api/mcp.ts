import { apiDelete, apiGet, apiPatch, apiPost } from './client';

const base = '/admin/mcp';

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
  revoked: boolean;
  oauth: boolean;
  clientName: string;
  lastSeenAt: string | null;
}
export interface GrantInput {
  label: string;
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
export const setPublish = (id: string, allowPublish: boolean) =>
  apiPatch<void>(base + '/connections/' + id, { allowPublish });
export const revokeConnection = (id: string) => apiDelete<void>(base + '/connections/' + id);
export const getOperations = (before: string) =>
  apiGet<{ items: McpOperation[]; next: string }>(
    base + '/operations' + (before ? '?before=' + encodeURIComponent(before) : ''),
  );
