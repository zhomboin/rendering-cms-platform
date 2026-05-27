const AUTH_TOKEN_KEY = 'rendering-cms-token';
const AUTH_REFRESH_TOKEN_KEY = 'rendering-cms-refresh-token';
const AUTH_USER_KEY = 'rendering-cms-user';

export interface AuthUser {
  userId: string;
  email: string;
  name: string;
  role: 'admin' | 'editor' | string;
}

export function setAuthToken(token: string) {
	window.localStorage.setItem(AUTH_TOKEN_KEY, token);
}

export function setRefreshToken(token: string) {
	window.localStorage.setItem(AUTH_REFRESH_TOKEN_KEY, token);
}

export function clearAuthToken() {
	window.localStorage.removeItem(AUTH_TOKEN_KEY);
	window.localStorage.removeItem(AUTH_REFRESH_TOKEN_KEY);
	window.localStorage.removeItem(AUTH_USER_KEY);
}

export function getAuthToken() {
	return window.localStorage.getItem(AUTH_TOKEN_KEY);
}

export function getRefreshToken() {
	return window.localStorage.getItem(AUTH_REFRESH_TOKEN_KEY);
}

export function setAuthUser(user: AuthUser) {
  window.localStorage.setItem(AUTH_USER_KEY, JSON.stringify(user));
}

export function getAuthUser(): AuthUser | null {
  const value = window.localStorage.getItem(AUTH_USER_KEY);
  if (!value) return null;

  try {
    return JSON.parse(value) as AuthUser;
  } catch {
    window.localStorage.removeItem(AUTH_USER_KEY);
    return null;
  }
}
