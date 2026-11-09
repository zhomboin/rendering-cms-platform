import { apiClient, apiGet, apiPost } from './client';

const connectionPath = '/admin/postbridge';
const attemptsPath = connectionPath + '/attempts';

export interface PostBridgeConnection {
  enabled: boolean;
  signingConfigured: boolean;
  registrationId?: string;
  registrationStatus?: string;
}

export interface PostBridgeRegistration {
  registrationId: string;
  status: string;
  recorded: boolean;
}

export interface PostBridgeTarget {
  account_id: string;
  display_name: string;
  platform: string;
  operations: string[];
  body_formats: string[];
}

export interface PostBridgeTargets {
  items: PostBridgeTarget[];
}

export interface PostBridgeActivation {
  status: 'active';
  targetsVerified: boolean;
  items?: PostBridgeTarget[];
}

export type PostBridgeAttemptStatus =
  | 'preparing'
  | 'prepared'
  | 'confirmed'
  | 'submitting'
  | 'in_progress'
  | 'completed'
  | 'needs_attention'
  | 'revoked'
  | 'expired'
  | 'failed'
  | 'cancelled';

export interface PostBridgeAttempt {
  id: string;
  articleId: string;
  sourceVersion: number;
  targetAccountId: string;
  status: PostBridgeAttemptStatus;
  previewUrl?: string;
  detailUrl?: string;
  externalStatus?: string;
  childStatus?: string;
  outcome?: string;
  lastConfirmedAt?: string | null;
  remoteUrl?: string;
  errorCode?: string;
  revoked: boolean;
  createdAt: string;
  updatedAt: string;
  authorizationExpiresAt: string;
  canSubmit: boolean;
  canCancel: boolean;
  canRefresh: boolean;
  cancelPending: boolean;
  cancelResult?: 'not_submitted' | 'accepted' | 'not_cancellable' | 'not_found';
}

export interface PostBridgeAttemptPage {
  items: PostBridgeAttempt[];
  page: number;
  pageSize: number;
  total: number;
}

export interface PostBridgePreparationInput {
  requestId: string;
  expectedVersion: number;
  targetAccountId: string;
  textOnly: boolean;
}

export type PostBridgeAttemptAction = 'refresh' | 'submit' | 'cancel';

export function getPostBridgeConnection() {
  return apiGet<PostBridgeConnection>(connectionPath + '/connection');
}

export function registerPostBridge(payload: { name: string; invitationToken: string }) {
  return apiPost<PostBridgeRegistration>(connectionPath + '/registrations', payload);
}

export async function activatePostBridge() {
  const response = await apiClient.post<PostBridgeActivation>(
    connectionPath + '/activate',
    undefined,
    { timeout: 25000 },
  );
  return response.data;
}

export function getPostBridgeTargets() {
  return apiGet<PostBridgeTargets>(connectionPath + '/targets');
}

export function createPostBridgePreparation(
  articleId: string,
  payload: PostBridgePreparationInput,
) {
  return apiPost<PostBridgeAttempt>(
    '/admin/articles/' + encodeURIComponent(articleId) + '/postbridge/preparations',
    payload,
  );
}

export function listPostBridgeAttempts(filters: {
  articleId?: string;
  status?: string;
  page: number;
  pageSize: number;
}) {
  const query = new URLSearchParams({
    page: String(filters.page),
    pageSize: String(filters.pageSize),
  });
  if (filters.articleId) query.set('articleId', filters.articleId);
  if (filters.status) query.set('status', filters.status);
  return apiGet<PostBridgeAttemptPage>(attemptsPath + '?' + query.toString());
}

export function getPostBridgeAttempt(id: string) {
  return apiGet<PostBridgeAttempt>(attemptsPath + '/' + encodeURIComponent(id));
}

export function runPostBridgeAttemptAction(id: string, action: PostBridgeAttemptAction) {
  return apiPost<PostBridgeAttempt>(attemptsPath + '/' + encodeURIComponent(id) + '/' + action);
}
