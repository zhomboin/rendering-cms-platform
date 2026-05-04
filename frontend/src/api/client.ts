import axios from 'axios';
import type { AxiosError } from 'axios';
import { clearAuthToken, getAuthToken } from './auth-token';

const API_BASE = import.meta.env.VITE_API_BASE ?? 'http://127.0.0.1:8080/api/v1';

interface ApiError {
  status: number;
  message: string;
}

class ApiRequestError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiRequestError';
    this.status = status;
  }
}

const apiClient = axios.create({
  baseURL: API_BASE,
  timeout: 10000,
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json',
  },
});

apiClient.interceptors.request.use((config) => {
  const token = getAuthToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  } else {
    delete config.headers.Authorization;
  }

  return config;
});

apiClient.interceptors.response.use(
  (response) => response,
  (error: AxiosError<{ error?: string; message?: string }>) => {
    const apiError = toApiRequestError(error);
    if (apiError.status === 401) {
      clearAuthToken();
      redirectAdminToLogin();
    }
    return Promise.reject(apiError);
  },
);

function toApiRequestError(error: AxiosError<{ error?: string; message?: string }>) {
  const status = error.response?.status ?? 0;
  const responseMessage = error.response?.data?.error ?? error.response?.data?.message;
  const message = responseMessage ?? (status > 0 ? `请求失败 (${status})` : error.message);
  return new ApiRequestError(status, message);
}

function redirectAdminToLogin() {
  if (typeof window === 'undefined') return;

  const currentPath = `${window.location.pathname}${window.location.search}${window.location.hash}`;
  const isAdminPage = window.location.pathname.startsWith('/admin');
  const isLoginPage = window.location.pathname === '/admin/login';
  if (!isAdminPage || isLoginPage) return;

  const redirect = encodeURIComponent(currentPath);
  window.location.replace(`/admin/login?redirect=${redirect}`);
}

export async function apiGet<T>(path: string): Promise<T> {
  const response = await apiClient.get<T>(path);
  return response.data;
}

export async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  const response = await apiClient.post<T>(path, body);
  return response.data;
}

export async function apiPatch<T>(path: string, body?: unknown): Promise<T> {
  const response = await apiClient.patch<T>(path, body);
  return response.data;
}

export { ApiRequestError, apiClient };
export type { ApiError };
