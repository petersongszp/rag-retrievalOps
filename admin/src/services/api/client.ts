import axios, {
  AxiosHeaders,
  AxiosInstance,
  AxiosResponse,
  InternalAxiosRequestConfig,
} from 'axios';
import { AUTH_API, API_BASE_URL } from '@/config/api';
import {
  clearStoredSession,
  getAccessToken,
  getRefreshToken,
  setStoredSession,
} from '@/services/auth/session';
import type { AuthSession, SessionResponse } from '@/types/auth';
import { APIError, normalizeAPIError } from './errors';

type RetriableConfig = InternalAxiosRequestConfig & {
  _retry?: boolean;
};

type UnauthorizedHandler = (() => void) | null;

export const refreshClient: AxiosInstance = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

const apiClient: AxiosInstance = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

let refreshPromise: Promise<AuthSession | null> | null = null;
let unauthorizedHandler: UnauthorizedHandler = null;

function unwrapPayload(response: AxiosResponse) {
  const payload = response?.data;
  if (payload && typeof payload === 'object' && 'code' in payload) {
    if (payload.code === 200) {
      let data = payload.data;
      if (data && typeof data === 'object' && 'data' in data && Object.keys(data).length === 1) {
        data = (data as { data: unknown }).data;
      }
      return data;
    }
    throw new APIError({
      message: payload.message || '请求失败',
      status: response.status,
      code: 'unknown',
      response,
    });
  }

  return payload;
}

function normalizeRequestUrl(config: InternalAxiosRequestConfig) {
  const baseURL = config.baseURL || '';
  const requestUrl = config.url || '';
  if (baseURL.endsWith('/api') && requestUrl.startsWith('/api/')) {
    config.url = requestUrl.replace(/^\/api/, '');
  }
}

function ensureHeaders(config: InternalAxiosRequestConfig): AxiosHeaders {
  const headers = config.headers instanceof AxiosHeaders ? config.headers : new AxiosHeaders(config.headers);
  config.headers = headers;
  return headers;
}

function isAuthEndpoint(url?: string): boolean {
  if (!url) {
    return false;
  }

  return [AUTH_API.LOGIN, AUTH_API.REGISTER, AUTH_API.REFRESH].some((path) => url.includes(path));
}

function redirectToLogin() {
  if (unauthorizedHandler) {
    unauthorizedHandler();
    return;
  }

  if (typeof window === 'undefined') {
    return;
  }

  const path = `${window.location.pathname}${window.location.search}`;
  const next = path && path !== '/' && path !== '/login' && path !== '/register' ? `?next=${encodeURIComponent(path)}` : '';
  window.location.assign(`/login${next}`);
}

function handleUnauthorizedFailure() {
  clearStoredSession();
  redirectToLogin();
}

function shouldRefresh(error: unknown, config?: RetriableConfig): boolean {
  const normalized = normalizeAPIError(error);
  if (normalized.status !== 401) {
    return false;
  }
  if (!config || config._retry || isAuthEndpoint(config.url)) {
    return false;
  }
  return Boolean(getRefreshToken());
}

async function refreshSession(): Promise<AuthSession | null> {
  if (refreshPromise) {
    return refreshPromise;
  }

  const refreshToken = getRefreshToken();
  if (!refreshToken) {
    return null;
  }

  refreshPromise = refreshClient
    .post(AUTH_API.REFRESH, {
      refresh_token: refreshToken,
    })
    .then((response) => {
      const session = unwrapPayload(response) as SessionResponse;
      return setStoredSession(session);
    })
    .catch((error) => {
      throw normalizeAPIError(error);
    })
    .finally(() => {
      refreshPromise = null;
    });

  return refreshPromise;
}

export function setUnauthorizedHandler(handler: UnauthorizedHandler) {
  unauthorizedHandler = handler;
}

apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    normalizeRequestUrl(config);
    const headers = ensureHeaders(config);

    if (typeof FormData !== 'undefined' && config.data instanceof FormData) {
      headers.delete('Content-Type');
      headers.delete('content-type');
    }

    const accessToken = getAccessToken();
    if (accessToken && !headers.has('Authorization')) {
      headers.set('Authorization', `Bearer ${accessToken}`);
    }

    return config;
  },
  (error) => Promise.reject(normalizeAPIError(error))
);

apiClient.interceptors.response.use(
  (response: AxiosResponse) => unwrapPayload(response),
  async (error) => {
    const config = error?.config as RetriableConfig | undefined;

    if (shouldRefresh(error, config)) {
      try {
        const requestConfig = config as RetriableConfig;
        requestConfig._retry = true;
        const session = await refreshSession();
        if (!session) {
          handleUnauthorizedFailure();
          throw normalizeAPIError(error);
        }

        const headers = ensureHeaders(requestConfig);
        headers.set('Authorization', `Bearer ${session.access_token}`);
        return apiClient(requestConfig);
      } catch (refreshError) {
        handleUnauthorizedFailure();
        return Promise.reject(normalizeAPIError(refreshError));
      }
    }

    const normalized = normalizeAPIError(error);
    if (normalized.status === 401 && !isAuthEndpoint(config?.url)) {
      handleUnauthorizedFailure();
    }

    return Promise.reject(normalized);
  }
);

export default apiClient;
