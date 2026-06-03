import { afterEach, describe, expect, it, vi } from 'vitest';
import apiClient, { refreshClient, setUnauthorizedHandler } from '@/services/api/client';
import { clearStoredSession, getStoredSession, resetSessionCache, setStoredSession } from '@/services/auth/session';

type AdapterConfig = {
  url?: string;
  method?: string;
  headers?: Record<string, string>;
  data?: unknown;
};

function makeSuccess(config: AdapterConfig, data: unknown) {
  return Promise.resolve({
    status: 200,
    statusText: 'OK',
    config,
    headers: {},
    data,
  });
}

function makeFailure(config: AdapterConfig, status: number, data: unknown) {
  return Promise.reject({
    config,
    response: {
      status,
      statusText: 'Error',
      config,
      headers: {},
      data,
    },
    message: typeof data === 'object' && data && 'message' in data ? data.message : 'request failed',
  });
}

describe('api client refresh flow', () => {
  afterEach(() => {
    clearStoredSession();
    resetSessionCache();
    setUnauthorizedHandler(null);
    apiClient.defaults.adapter = undefined;
    refreshClient.defaults.adapter = undefined;
  });

  it('refreshes once and replays concurrent requests', async () => {
    setStoredSession({
      access_token: 'old-token',
      refresh_token: 'refresh-token',
      expires_in: 3600,
      user_id: 1,
      role: 'owner',
      tenant_id: 1,
    });

    let refreshCalls = 0;
    let protectedCalls = 0;

    apiClient.defaults.adapter = async (config) => {
      protectedCalls += 1;
      const authHeader = String(config.headers?.Authorization || '');
      if (authHeader === 'Bearer new-token') {
        return makeSuccess(config, { code: 200, message: 'Success', data: { ok: true } });
      }
      return makeFailure(config, 401, { code: 401, message: 'Invalid or expired token' });
    };

    refreshClient.defaults.adapter = async (config) => {
      refreshCalls += 1;
      return makeSuccess(config, {
        code: 200,
        message: 'Success',
        data: {
          access_token: 'new-token',
          refresh_token: 'new-refresh-token',
          expires_in: 7200,
          user_id: 1,
          role: 'owner',
          tenant_id: 1,
        },
      });
    };

    const [first, second] = await Promise.all([apiClient.get('/protected'), apiClient.get('/protected')]);

    expect(first).toEqual({ ok: true });
    expect(second).toEqual({ ok: true });
    expect(refreshCalls).toBe(1);
    expect(protectedCalls).toBe(4);
    expect(getStoredSession()?.access_token).toBe('new-token');
  });

  it('clears the session and calls unauthorized handler when refresh fails', async () => {
    setStoredSession({
      access_token: 'old-token',
      refresh_token: 'refresh-token',
      expires_in: 3600,
      user_id: 1,
      role: 'owner',
      tenant_id: 1,
    });

    const unauthorizedSpy = vi.fn();
    setUnauthorizedHandler(unauthorizedSpy);

    apiClient.defaults.adapter = async (config) =>
      makeFailure(config, 401, { code: 401, message: 'Invalid or expired token' });

    refreshClient.defaults.adapter = async (config) =>
      makeFailure(config, 401, { code: 401, message: 'Invalid refresh token' });

    await expect(apiClient.get('/protected')).rejects.toMatchObject({
      status: 401,
    });

    expect(getStoredSession()).toBeNull();
    expect(unauthorizedSpy).toHaveBeenCalledTimes(1);
  });
});
