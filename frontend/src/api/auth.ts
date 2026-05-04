import { apiPost } from './client';
import { clearAuthToken, getAuthToken, getAuthUser, setAuthToken, setAuthUser } from './auth-token';
import type { AuthUser } from './auth-token';

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: AuthUser;
}

export function loginAdmin(values: LoginRequest) {
  return apiPost<LoginResponse>('/auth/login', values);
}

export { clearAuthToken, getAuthToken, getAuthUser, setAuthToken, setAuthUser };
export type { AuthUser };
