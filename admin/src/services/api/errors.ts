export type APIErrorCode =
  | 'unauthorized'
  | 'invalid_token'
  | 'forbidden'
  | 'tenant_suspended'
  | 'not_found'
  | 'email_exists'
  | 'weak_password'
  | 'quota_exceeded'
  | 'rate_limited'
  | 'unknown';

export type QuotaErrorDetails = {
  quota_type?: string;
  current?: number;
  limit?: number;
  reset_at?: string;
};

export class APIError extends Error {
  status: number;
  code: APIErrorCode;
  details?: QuotaErrorDetails;
  response?: unknown;

  constructor({
    message,
    status,
    code,
    details,
    response,
  }: {
    message: string;
    status: number;
    code: APIErrorCode;
    details?: QuotaErrorDetails;
    response?: unknown;
  }) {
    super(message);
    this.name = 'APIError';
    this.status = status;
    this.code = code;
    this.details = details;
    this.response = response;
  }
}

function extractPayload(error: unknown): { status: number; body?: any } {
  if (
    error &&
    typeof error === 'object' &&
    'response' in error &&
    error.response &&
    typeof error.response === 'object'
  ) {
    const response = error.response as { status?: number; data?: unknown };
    return {
      status: response.status ?? 0,
      body: response.data,
    };
  }

  return { status: 0 };
}

function inferCode(status: number, message: string, body?: any): APIErrorCode {
  const serverCode =
    typeof body?.code === 'string'
      ? body.code.toLowerCase()
      : typeof body?.error === 'string'
        ? body.error.toLowerCase()
        : '';
  const text = `${serverCode} ${message}`.toLowerCase();

  if (status === 401) {
    if (text.includes('expired') || text.includes('invalid token')) {
      return 'invalid_token';
    }
    return 'unauthorized';
  }
  if (status === 403) {
    if (text.includes('tenant') && (text.includes('disabled') || text.includes('suspended'))) {
      return 'tenant_suspended';
    }
    return 'forbidden';
  }
  if (status === 404) {
    return 'not_found';
  }
  if (status === 409) {
    return 'email_exists';
  }
  if (status === 422 || (status === 400 && text.includes('password'))) {
    return 'weak_password';
  }
  if (status === 429) {
    if (text.includes('quota')) {
      return 'quota_exceeded';
    }
    return 'rate_limited';
  }

  return 'unknown';
}

export function normalizeAPIError(error: unknown): APIError {
  if (error instanceof APIError) {
    return error;
  }

  const { status, body } = extractPayload(error);
  const message =
    (typeof body?.message === 'string' && body.message) ||
    (typeof body?.error === 'string' && body.error) ||
    (error && typeof error === 'object' && 'message' in error && typeof error.message === 'string'
      ? error.message
      : '请求失败，请稍后重试');
  const code = inferCode(status, message, body);
  const details = body?.data && typeof body.data === 'object' ? (body.data as QuotaErrorDetails) : undefined;

  return new APIError({
    message,
    status,
    code,
    details,
    response:
      error && typeof error === 'object' && 'response' in error ? (error as { response: unknown }).response : undefined,
  });
}

export function isUnauthorizedError(error: unknown): boolean {
  const normalized = normalizeAPIError(error);
  return normalized.status === 401;
}

export function isQuotaError(error: unknown): boolean {
  const normalized = normalizeAPIError(error);
  return normalized.code === 'quota_exceeded' || normalized.code === 'rate_limited';
}

export function getErrorMessage(error: unknown, fallback = '请求失败，请稍后重试'): string {
  const normalized = normalizeAPIError(error);
  return normalized.message || fallback;
}

export function formatQuotaError(error: unknown): string {
  const normalized = normalizeAPIError(error);
  if (!normalized.details) {
    return normalized.message;
  }

  const parts = [normalized.message];
  if (normalized.details.quota_type) {
    parts.push(`配额类型：${normalized.details.quota_type}`);
  }
  if (
    typeof normalized.details.current === 'number' &&
    typeof normalized.details.limit === 'number'
  ) {
    parts.push(`当前 ${normalized.details.current} / 上限 ${normalized.details.limit}`);
  }
  if (normalized.details.reset_at) {
    parts.push(`重置时间：${normalized.details.reset_at}`);
  }
  return parts.join('，');
}
