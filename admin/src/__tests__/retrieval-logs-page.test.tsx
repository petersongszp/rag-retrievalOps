import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { RetrievalLogsPage } from '@/components/admin/retrieval-logs-page';
import type { KBRetrieveLog, ListResponse } from '@/types/kb';

const mockPush = vi.fn();
const mockReplace = vi.fn();
const mockSearchParams = new URLSearchParams('');

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  useSearchParams: () => mockSearchParams,
}));

vi.mock('@/services/api/client', () => ({
  default: {
    get: vi.fn(),
  },
}));

vi.mock('@/components/admin/knowledge-base-provider', () => ({
  useKnowledgeBaseContext: () => ({
    bases: [{ id: 1, name: 'KB One' }],
    selectedBase: { id: 1, name: 'KB One' },
  }),
}));

import apiClient from '@/services/api/client';

const mockedGet = vi.mocked(apiClient.get);

const listResponse: ListResponse<KBRetrieveLog> = {
  items: [
    {
      id: 1,
      request_id: 'req_hit',
      user_id: 100,
      kb_ids: '1',
      query: 'cached query',
      top_k: 5,
      candidate_topk: 10,
      final_topk: 5,
      token_budget: 4000,
      rewrite_applied: false,
      final_count: 3,
      truncated_count: 0,
      dense_hits: 0,
      sparse_hits: 0,
      dense_contribution: 0,
      sparse_contribution: 0,
      result_status: 'success',
      embedding_ms: 0,
      search_ms: 0,
      postprocess_ms: 0,
      rerank_ms: 0,
      duration_ms: 12,
      timeout_ms: 3000,
      semantic_cache_hit: true,
      embedding_cache_hit: true,
      created_at: '2026-06-09T12:00:00Z',
    },
    {
      id: 2,
      request_id: 'req_miss',
      user_id: 100,
      kb_ids: '1',
      query: 'uncached query',
      top_k: 5,
      candidate_topk: 10,
      final_topk: 5,
      token_budget: 4000,
      rewrite_applied: false,
      final_count: 2,
      truncated_count: 0,
      dense_hits: 1,
      sparse_hits: 1,
      dense_contribution: 1,
      sparse_contribution: 1,
      result_status: 'success',
      embedding_ms: 5,
      search_ms: 10,
      postprocess_ms: 2,
      rerank_ms: 1,
      duration_ms: 18,
      timeout_ms: 3000,
      semantic_cache_hit: false,
      embedding_cache_hit: false,
      created_at: '2026-06-09T12:01:00Z',
    },
  ],
  total: 2,
  page: 1,
  page_size: 20,
};

const detailResponse: KBRetrieveLog = {
  id: 1,
  request_id: 'req_hit',
  user_id: 100,
  kb_ids: '1',
  query: 'cached query',
  final_query: 'cached query',
  top_k: 5,
  candidate_topk: 10,
  final_topk: 5,
  token_budget: 4000,
  rewrite_applied: false,
  final_count: 3,
  truncated_count: 0,
  dense_hits: 0,
  sparse_hits: 0,
  dense_contribution: 0,
  sparse_contribution: 0,
  result_status: 'success',
  embedding_ms: 0,
  search_ms: 0,
  postprocess_ms: 0,
  rerank_ms: 0,
  duration_ms: 12,
  timeout_ms: 3000,
  semantic_cache_enabled: true,
  semantic_cache_hit: true,
  semantic_cache_lookup_ms: 4,
  semantic_cache_similarity: 0.9321,
  semantic_cache_reason: 'threshold_passed',
  semantic_cache_entry_id: 'entry_123',
  embedding_cache_enabled: true,
  embedding_cache_hit: true,
  embedding_cache_lookup_ms: 1,
  embedding_cache_reason: 'hit',
  created_at: '2026-06-09T12:00:00Z',
};

describe('RetrievalLogsPage cache UI', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedGet.mockImplementation((url) => {
      if (typeof url === 'string' && url.includes('/retrieve/audit/req_hit')) {
        return Promise.resolve(detailResponse);
      }
      return Promise.resolve(listResponse);
    });
  });

  it('renders semantic cache hit and miss tags in retrieval log list', async () => {
    render(<RetrievalLogsPage />);

    await waitFor(() => {
      expect(screen.getByText('cached query')).toBeInTheDocument();
    });

    expect(screen.getAllByText('命中').length).toBeGreaterThan(0);
    expect(screen.getAllByText('未命中').length).toBeGreaterThan(0);
  });

  it('renders semantic cache and embedding cache details in the drawer', async () => {
    render(<RetrievalLogsPage />);

    await waitFor(() => {
      expect(screen.getByText('cached query')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('cached query'));

    await waitFor(() => {
      expect(screen.getAllByText('语义缓存').length).toBeGreaterThan(0);
    });

    expect(screen.getAllByText('命中').length).toBeGreaterThan(0);
    expect(screen.getByText('threshold_passed')).toBeInTheDocument();
    expect(screen.getByText('entry_123')).toBeInTheDocument();
    expect(screen.getByText('0.9321')).toBeInTheDocument();
    expect(screen.getAllByText('向量缓存').length).toBeGreaterThan(0);
    expect(screen.getByText('hit')).toBeInTheDocument();
  });
});
