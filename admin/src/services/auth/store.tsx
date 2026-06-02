'use client';

import { createContext, useContext, useEffect, useMemo, useState } from 'react';
import { message } from 'antd';
import apiClient from '@/services/api/client';
import { AUTH_API } from '@/config/api';
import type {
  AuthSession,
  AuthStatus,
  AuthUser,
  ChangePasswordPayload,
  LoginPayload,
  RegisterPayload,
  RegisterResponse,
  SessionResponse,
} from '@/types/auth';
import {
  clearStoredSession,
  getStoredSession,
  setStoredSession,
  subscribeToSession,
} from './session';

type AuthContextValue = {
  status: AuthStatus;
  session: AuthSession | null;
  user: AuthUser | null;
  login: (payload: LoginPayload) => Promise<AuthUser>;
  register: (payload: RegisterPayload) => Promise<RegisterResponse>;
  loadMe: () => Promise<AuthUser>;
  logout: (options?: { redirectTo?: string; silent?: boolean }) => void;
  changePassword: (payload: ChangePasswordPayload) => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

function normalizeError(error: unknown, fallback: string): string {
  if (
    error &&
    typeof error === 'object' &&
    'message' in error &&
    typeof error.message === 'string' &&
    error.message.trim()
  ) {
    return error.message;
  }

  if (
    error &&
    typeof error === 'object' &&
    'response' in error &&
    error.response &&
    typeof error.response === 'object' &&
    'data' in error.response &&
    error.response.data &&
    typeof error.response.data === 'object' &&
    'error' in error.response.data &&
    typeof error.response.data.error === 'string'
  ) {
    return error.response.data.error;
  }

  return fallback;
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>('loading');
  const [session, setSession] = useState<AuthSession | null>(null);
  const [user, setUser] = useState<AuthUser | null>(null);

  const logout = (options?: { redirectTo?: string; silent?: boolean }) => {
    clearStoredSession();
    setSession(null);
    setUser(null);
    setStatus('anonymous');

    if (!options?.silent && typeof window !== 'undefined') {
      window.location.assign(options?.redirectTo ?? '/login');
    }
  };

  const loadMe = async (): Promise<AuthUser> => {
    const profile = (await apiClient.get(AUTH_API.ME)) as AuthUser;
    setUser(profile);
    setStatus('authenticated');
    return profile;
  };

  const login = async (payload: LoginPayload): Promise<AuthUser> => {
    const nextSession = (await apiClient.post(AUTH_API.LOGIN, payload)) as SessionResponse;
    const normalized = setStoredSession(nextSession);
    setSession(normalized);
    return loadMe();
  };

  const register = async (payload: RegisterPayload): Promise<RegisterResponse> => {
    return (await apiClient.post(AUTH_API.REGISTER, payload)) as RegisterResponse;
  };

  const changePassword = async (payload: ChangePasswordPayload): Promise<void> => {
    await apiClient.put(AUTH_API.PASSWORD, payload);
  };

  useEffect(() => {
    const unsubscribe = subscribeToSession((nextSession) => {
      setSession(nextSession);
      if (!nextSession) {
        setUser(null);
        setStatus('anonymous');
      }
    });

    const initialSession = getStoredSession();
    setSession(initialSession);

    if (!initialSession) {
      setStatus('anonymous');
      return unsubscribe;
    }

    void loadMe().catch((error) => {
      message.warning(normalizeError(error, '登录状态已失效，请重新登录'));
      logout({ silent: true });
    });

    return unsubscribe;
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      status,
      session,
      user,
      login,
      register,
      loadMe,
      logout,
      changePassword,
    }),
    [status, session, user]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
}
