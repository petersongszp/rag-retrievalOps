import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { RetrievalDebugPage } from '@/components/admin/retrieval-debug-page';
import type { RetrievalDebugTrace } from '@/types/kb';

const mockPush = vi.fn();
const mockReplace = vi.fn();

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  useSearchParams: () => new URLSearchParams('request_id=req_test_123'),
}));

vi.mock('@/services/api/client', () => ({
  default: {
    get: vi.fn(),
  },
}));

import apiClient from '@/services/api/client';
const mockedGet = vi.mocked(apiClient.get);

const fullTrace: RetrievalDebugTrace = {
  request_id: 'req_test_123',
  debug_available: true,
  kb_ids: [1, 2],
  original_query: 'What is RAG',
  rewritten_query: 'What is retrieval augmented generation',
  rewrite_strategy: 'domain_terms',
  rewrite_gain_bucket: 'high',
  term_hits: ['retrieval', 'augmented'],
  route_final_queries: { dense: 'dense query', sparse: 'sparse query' },
  route_hits: [
    {
      route: 'dense',
      query: 'dense query',
      hits: [
        { chunk_id: 'c1', file_name: 'doc1.pdf', score: 0.95, route: 'dense' },
      ],
      contribution: 0.7,
      latency_ms: 50,
    },
    {
      route: 'sparse',
      query: 'sparse query',
      hits: [],
      contribution: 0.3,
      latency_ms: 30,
    },
  ],
  fusion_results: { before: [{ chunk_id: 'f1' }], after: [{ chunk_id: 'f1' }] },
  dedupe_results: { before_count: 10, after_count: 8, removed: [{ chunk_id: 'dup1' }] },
  rerank_results: {
    before: [{ chunk_id: 'r1', score: 0.8 }],
    after: [{ chunk_id: 'r1', rerank_score: 0.95 }],
    rerank_model: 'bge-reranker-v2',
    rerank_version: 'v1',
  },
  filter_results: { before_count: 8, after_count: 5, truncate_reason: 'token_budget' },
  parent_child: {
    parent_child_enabled: true,
    parent_fill_strategy: 'sibling_window',
    parent_fill_count: 3,
    parent_fill_tokens: 1200,
    child_hits: [{ chunk_id: 'child1', file_name: 'doc1.pdf', score: 0.9, route: 'dense' }],
    parent_contexts: [{ chunk_id: 'parent1', file_name: 'doc1.pdf' }],
  },
  topk_decision: {
    candidate_topk: 20,
    final_topk: 5,
    score_distribution: 'long_tail',
    rerank_gap: 0.15,
    evidence_density: 0.8,
    token_budget: 4000,
    topk_decision_reason: 'evidence_density_sufficient',
  },
  evidence_gate: {
    evidence_gate_result: 'passed',
    thresholds: { min_rerank_score: 0.5, min_density: 0.3, min_citation_coverage: 0.6 },
  },
  citation_check: {
    citation_supported: true,
    citation_support_score: 0.92,
    unsupported_claims: [],
    citation_check_version: 'v1',
  },
  degradation: { enabled: false },
  contract_gaps: [],
  created_at: '2026-01-01T00:00:00Z',
  final_results: [],
};

const WAIT_OPTS = { timeout: 8000 };

describe('RetrievalDebugPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders page heading element', async () => {
    mockedGet.mockResolvedValueOnce(fullTrace);
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 2 })).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('fetches debug trace with request_id from search params', async () => {
    mockedGet.mockResolvedValueOnce(fullTrace);
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(mockedGet).toHaveBeenCalledWith(
        expect.stringContaining('req_test_123')
      );
    }, WAIT_OPTS);
  });

  it('displays request_id after loading', async () => {
    mockedGet.mockResolvedValueOnce(fullTrace);
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(screen.getByText('req_test_123')).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('displays original and rewritten query from trace', async () => {
    mockedGet.mockResolvedValueOnce(fullTrace);
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(screen.getAllByText('What is RAG').length).toBeGreaterThanOrEqual(1);
    }, WAIT_OPTS);
  });

  it('shows error alert on API failure', async () => {
    mockedGet.mockRejectedValueOnce(new Error('Network error'));
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(screen.getByText(/Network error/)).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders fallback tags for missing fields', async () => {
    mockedGet.mockResolvedValueOnce({
      request_id: 'req_gap',
      debug_available: false,
      contract_gaps: ['route_hits', 'parent_child'],
    } as RetrievalDebugTrace);
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(screen.getAllByText('暂未返回').length).toBeGreaterThan(0);
    }, WAIT_OPTS);
  });

  it('renders route contribution and latency values', async () => {
    mockedGet.mockResolvedValueOnce(fullTrace);
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(screen.getByText('0.7')).toBeInTheDocument();
      expect(screen.getByText('50')).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders parent-child enabled flag in header', async () => {
    mockedGet.mockResolvedValueOnce(fullTrace);
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(screen.getAllByText('true').length).toBeGreaterThanOrEqual(1);
    }, WAIT_OPTS);
  });

  it('renders TopK decision reason and values', async () => {
    mockedGet.mockResolvedValueOnce(fullTrace);
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(screen.getByText('evidence_density_sufficient')).toBeInTheDocument();
      expect(screen.getByText('20')).toBeInTheDocument();
      expect(screen.getByText('5')).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders evidence gate result', async () => {
    mockedGet.mockResolvedValueOnce(fullTrace);
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(screen.getByText('passed')).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders citation support score', async () => {
    mockedGet.mockResolvedValueOnce(fullTrace);
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(screen.getByText('0.92')).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders degradation info when enabled', async () => {
    mockedGet.mockResolvedValueOnce({
      ...fullTrace,
      degradation: { enabled: true, reason: 'rerank_timeout', error_code: 'RERANK_504', fallback_strategy: 'skip_rerank' },
    });
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(screen.getByText('rerank_timeout')).toBeInTheDocument();
      expect(screen.getByText('RERANK_504')).toBeInTheDocument();
      expect(screen.getByText('skip_rerank')).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders kb_ids values', async () => {
    mockedGet.mockResolvedValueOnce(fullTrace);
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(screen.getByText('1, 2')).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders threshold values in evidence gate', async () => {
    mockedGet.mockResolvedValueOnce(fullTrace);
    render(<RetrievalDebugPage />);
    await waitFor(() => {
      expect(screen.getAllByText(/最低重排得分/).length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText(/最低证据密度/).length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText(/最低引用覆盖率/).length).toBeGreaterThanOrEqual(1);
    }, WAIT_OPTS);
  });
});
