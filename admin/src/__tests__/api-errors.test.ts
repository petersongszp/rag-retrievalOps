import { describe, expect, it } from 'vitest';
import { formatQuotaError, normalizeAPIError } from '@/services/api/errors';

describe('api error normalization', () => {
  it('normalizes quota exceeded responses with details', () => {
    const error = normalizeAPIError({
      response: {
        status: 429,
        data: {
          code: 429,
          message: 'quota exceeded',
          data: {
            quota_type: 'api_calls_today',
            current: 120,
            limit: 100,
            reset_at: '2026-06-03T23:59:59Z',
          },
        },
      },
    });

    expect(error.code).toBe('quota_exceeded');
    expect(formatQuotaError(error)).toContain('api_calls_today');
    expect(formatQuotaError(error)).toContain('120 / 上限 100');
  });

  it('normalizes email conflict responses', () => {
    const error = normalizeAPIError({
      response: {
        status: 409,
        data: {
          code: 409,
          message: 'Email already registered',
        },
      },
    });

    expect(error.code).toBe('email_exists');
    expect(error.message).toBe('Email already registered');
  });
});
