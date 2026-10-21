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

export interface LoginMFAChallenge {
  mfaRequired: true;
  challengeToken: string;
  email: string;
  expiresIn: number;
}

export type LoginResult = LoginResponse | LoginMFAChallenge;

export function isLoginMFAChallenge(result: LoginResult): result is LoginMFAChallenge {
  return (result as LoginMFAChallenge).mfaRequired === true;
}

export interface MFAVerifyRequest {
  challengeToken: string;
  code: string;
}

export interface MFAResendRequest {
  challengeToken: string;
}

export interface MFAResendResponse {
  expiresIn: number;
}

export function loginAdmin(values: LoginRequest) {
  return apiPost<LoginResult>('/auth/login', values);
}

export function verifyLoginCode(values: MFAVerifyRequest) {
  return apiPost<LoginResponse>('/auth/mfa/verify', values);
}

export function resendLoginCode(values: MFAResendRequest) {
  return apiPost<MFAResendResponse>('/auth/mfa/resend', values);
}

export { clearAuthToken, getAuthToken, getAuthUser, getRefreshToken, setAuthToken, setAuthUser, setRefreshToken };
export type { AuthUser };
