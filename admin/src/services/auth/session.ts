import type { AuthSession, SessionResponse } from '@/types/auth';

const SESSION_STORAGE_KEY = 'admin.auth.session';
const EXPIRY_SKEW_MS = 30_000;

type SessionListener = (session: AuthSession | null) => void;

let cachedSession: AuthSession | null | undefined;
const listeners = new Set<SessionListener>();

function isBrowser(): boolean {
  return typeof window !== 'undefined';
}

function notify(session: AuthSession | null): void {
  listeners.forEach((listener) => listener(session));
}

function normalizeSession(session: SessionResponse): AuthSession {
  return {
    ...session,
    expires_at: Date.now() + session.expires_in * 1000,
  };
}

function readSessionFromStorage(): AuthSession | null {
  if (!isBrowser()) {
    return null;
  }

  const raw = window.localStorage.getItem(SESSION_STORAGE_KEY);
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw) as AuthSession;
    if (
      !parsed ||
      typeof parsed.access_token !== 'string' ||
      typeof parsed.refresh_token !== 'string' ||
      typeof parsed.expires_at !== 'number'
    ) {
      window.localStorage.removeItem(SESSION_STORAGE_KEY);
      return null;
    }
    return parsed;
  } catch {
    window.localStorage.removeItem(SESSION_STORAGE_KEY);
    return null;
  }
}

export function getStoredSession(): AuthSession | null {
  if (cachedSession !== undefined) {
    return cachedSession;
  }

  cachedSession = readSessionFromStorage();
  return cachedSession;
}

export function setStoredSession(session: SessionResponse | AuthSession): AuthSession {
  const normalized =
    'expires_at' in session ? (session as AuthSession) : normalizeSession(session as SessionResponse);

  cachedSession = normalized;

  if (isBrowser()) {
    window.localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(normalized));
  }

  notify(normalized);
  return normalized;
}

export function clearStoredSession(): void {
  cachedSession = null;

  if (isBrowser()) {
    window.localStorage.removeItem(SESSION_STORAGE_KEY);
  }

  notify(null);
}

export function getAccessToken(): string | null {
  return getStoredSession()?.access_token ?? null;
}

export function getRefreshToken(): string | null {
  return getStoredSession()?.refresh_token ?? null;
}

export function hasValidSession(): boolean {
  const session = getStoredSession();
  if (!session) {
    return false;
  }

  return session.expires_at > Date.now() + EXPIRY_SKEW_MS;
}

export function subscribeToSession(listener: SessionListener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function resetSessionCache(): void {
  cachedSession = undefined;
}
