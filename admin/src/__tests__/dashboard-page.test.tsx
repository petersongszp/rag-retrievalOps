import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { DashboardPage } from '@/components/admin/dashboard-page';

const mockPush = vi.fn();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
}));

vi.mock('@/components/admin/knowledge-base-provider', () => ({
  useKnowledgeBaseContext: () => ({
    selectedBase: { id: 7, name: '客服知识库' },
    isPermissionDenied: false,
  }),
}));

vi.mock('@/services/api/client', () => ({
  default: {
    get: vi.fn(),
  },
}));

import apiClient from '@/services/api/client';

const mockedGet = vi.mocked(apiClient.get);

const statsPayload = {
  kb_count: 3,
  document_count: 42,
  processing_job_count: 2,
  failed_job_count: 1,
};

const metricsPayload = {
  range: '24h',
  ingest_success_rate: [{ bucket: '2026-06-21T10:00:00Z', rate: 0.92, total: 20 }],
  retrieve_request_count: [{ bucket: '2026-06-21T10:00:00Z', count: 80 }],
  retrieve_p95_ms: [{ bucket: '2026-06-21T10:00:00Z', p95_ms: 1800 }],
  retrieve_empty_rate: [{ bucket: '2026-06-21T10:00:00Z', rate: 0.2, total: 80 }],
  error_type_topn: [{ error_code: 'timeout', count: 5 }],
  cost_overview: [
    {
      bucket: '2026-06-21T10:00:00Z',
      total_cost: 12,
      cost_per_1k_queries: 3.56,
      embedding_cost: 1,
      retrieval_cost: 2,
      rerank_cost: 1,
      llm_cost: 8,
      vector_storage_cost: 0,
      avg_context_tokens: 2200,
    },
  ],
};

describe('DashboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders workspace heading and current knowledge base', async () => {
    mockedGet.mockResolvedValueOnce(statsPayload);
    mockedGet.mockResolvedValueOnce(metricsPayload);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 2, name: '工作台' })).toBeInTheDocument();
      expect(screen.getByText(/当前知识库：客服知识库/)).toBeInTheDocument();
    });
  });

  it('shows actionable todo items for failures and regressions', async () => {
    mockedGet.mockResolvedValueOnce(statsPayload);
    mockedGet.mockResolvedValueOnce(metricsPayload);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText('处理失败入库任务')).toBeInTheDocument();
      expect(screen.getByText('检查响应耗时')).toBeInTheDocument();
      expect(screen.getByText('查看高频错误类型')).toBeInTheDocument();
    });
  });

  it('shows onboarding empty state when both requests fail', async () => {
    mockedGet.mockRejectedValueOnce(new Error('stats unavailable'));
    mockedGet.mockRejectedValueOnce(new Error('metrics unavailable'));

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText('工作台还没有可展示的数据')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: '去创建知识库' })).toBeInTheDocument();
    });
  });
});
