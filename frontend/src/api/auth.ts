import { apiPost } from './client';
import {
  clearAuthToken,
  getAuthToken,
  getAuthUser,
  getRefreshToken,
  setAuthToken,
  setAuthUser,
  setRefreshToken,
} from './auth-token';
import type { AuthUser } from './auth-token';

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  refreshToken: string;
  user: AuthUser;
}

export interface RefreshTokenResponse {
  token: string;
  refreshToken: string;
}

export function loginAdmin(values: LoginRequest) {
  return apiPost<LoginResponse>('/auth/login', values);
}

export { clearAuthToken, getAuthToken, getAuthUser, getRefreshToken, setAuthToken, setAuthUser, setRefreshToken };
export type { AuthUser };
