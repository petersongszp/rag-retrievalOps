import { afterEach, describe, expect, it } from 'vitest';
import {
  clearStoredSession,
  getAccessToken,
  getStoredSession,
  resetSessionCache,
  setStoredSession,
  subscribeToSession,
} from '@/services/auth/session';

describe('auth session storage', () => {
  afterEach(() => {
    clearStoredSession();
    resetSessionCache();
  });

  it('stores and reads the session with computed expiry', () => {
    const session = setStoredSession({
      access_token: 'access-token',
      refresh_token: 'refresh-token',
      expires_in: 3600,
      user_id: 1,
      role: 'owner',
      tenant_id: 2,
    });

    expect(session.expires_at).toBeGreaterThan(Date.now());
    expect(getAccessToken()).toBe('access-token');
    expect(getStoredSession()?.tenant_id).toBe(2);
  });

  it('notifies listeners when the session changes', () => {
    const seen: Array<string | null> = [];
    const unsubscribe = subscribeToSession((session) => {
      seen.push(session?.access_token ?? null);
    });

    setStoredSession({
      access_token: 'first-token',
      refresh_token: 'refresh-token',
      expires_in: 120,
      user_id: 1,
      role: 'owner',
      tenant_id: 9,
    });
    clearStoredSession();
    unsubscribe();

    expect(seen).toEqual(['first-token', null]);
  });
});
