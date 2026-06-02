export type UserRole = 'owner' | 'admin' | 'member' | 'viewer';

export interface RegisterPayload {
  email: string;
  password: string;
  name: string;
  tenant_name: string;
}

export interface RegisterResponse {
  user_id: number;
  email: string;
  tenant_id: number;
}

export interface LoginPayload {
  email: string;
  password: string;
}

export interface SessionResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  user_id: number;
  role: UserRole;
  tenant_id: number;
}

export interface AuthSession extends SessionResponse {
  expires_at: number;
}

export interface AuthUser {
  user_id: number;
  email: string;
  name: string;
  role: UserRole;
  tenant_id: number;
  tenant_name: string;
  created_at?: string;
}

export interface ChangePasswordPayload {
  old_password: string;
  new_password: string;
}

export type AuthStatus = 'loading' | 'authenticated' | 'anonymous';
