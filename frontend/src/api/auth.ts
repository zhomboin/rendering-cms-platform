import { apiPost } from './client';
import { clearAuthToken, getAuthToken, setAuthToken } from './auth-token';

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
}

export function loginAdmin(values: LoginRequest) {
  return apiPost<LoginResponse>('/auth/login', values);
}

export { clearAuthToken, getAuthToken, setAuthToken };
