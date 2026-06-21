import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { StrategyCenterPage } from '@/components/admin/strategy-center-page';
import type {
  StrategyFlag,
  StrategyImpact,
  StrategyGateSummary,
  StrategyVersion,
  StrategyOperationLog,
} from '@/types/kb';

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock('@/services/api/client', () => ({
  default: {
    get: vi.fn(),
    patch: vi.fn(),
    post: vi.fn(),
  },
}));

import apiClient from '@/services/api/client';
const mockedGet = vi.mocked(apiClient.get);

const sampleFlags: StrategyFlag[] = [
  {
    flag_key: 'RAG_ENABLE_PARENT_CHILD_RETRIEVAL',
    label: 'Parent Child Retrieval',
    status: 'canary',
    enabled: true,
    rollout_percentage: 20,
    strategy_version: 'p3-parent-child-v1',
    risk_level: 'medium',
    updated_at: '2026-01-01T00:00:00Z',
  },
  {
    flag_key: 'RAG_ENABLE_STRATEGIC_TOPK',
    label: 'Strategic TopK',
    status: 'enabled',
    enabled: true,
    rollout_percentage: 100,
    strategy_version: 'p3-topk-v1',
    risk_level: 'low',
    updated_at: '2026-01-01T00:00:00Z',
  },
  {
    flag_key: 'RAG_ENABLE_EVIDENCE_REFUSAL',
    label: 'Evidence Refusal',
    status: 'shadow',
    enabled: false,
    rollout_percentage: 0,
    strategy_version: 'p3-evidence-v1',
    risk_level: 'high',
    updated_at: '2026-01-01T00:00:00Z',
  },
];

const sampleImpact: StrategyImpact = {
  flag_key: 'RAG_ENABLE_PARENT_CHILD_RETRIEVAL',
  range: '24h',
  from: '2026-01-01T00:00:00Z',
  to: '2026-01-02T00:00:00Z',
  sample_size: 500,
  parent_fill_gain: 0.4,
  rewrite_gain: 0.2,
  route_contribution: { dense: 0.65, sparse: 0.35 },
  evidence_refusal_rate: 0.05,
  refusal_false_positive_rate: 0.01,
  citation_support_score: 0.92,
  p95_latency_delta_ms: 50,
};

const sampleGates: StrategyGateSummary = {
  flag_key: 'RAG_ENABLE_PARENT_CHILD_RETRIEVAL',
  gate_status: 'passed',
  passed: true,
  failed_rules: [],
  baseline_report_id: 'rpt_baseline',
  candidate_report_id: 'rpt_candidate',
  last_eval_run_id: 'run_001',
};

const sampleVersions: StrategyVersion[] = [
  { version_id: 'v1', flag_key: 'RAG_ENABLE_PARENT_CHILD_RETRIEVAL', label: 'Initial version', created_at: '2026-01-01T00:00:00Z', created_by: 'admin', gate_status: 'passed' },
];

const sampleOperations: StrategyOperationLog[] = [
  { id: 'op1', operation: 'update_flag', flag_key: 'RAG_ENABLE_PARENT_CHILD_RETRIEVAL', from_status: 'disabled', to_status: 'canary', reason: 'canary test after offline gate passed', created_at: '2026-01-01T01:00:00Z' },
];

function mockFullLoad() {
  const emptyList = { items: [], total: 0, page: 1, page_size: 20 };
  mockedGet.mockImplementation((url: string | unknown, config?: unknown) => {
    const urlStr = typeof url === 'string' ? url : String(url);
    if (urlStr.includes('/strategy/flags')) return Promise.resolve({ items: sampleFlags });
    if (urlStr.includes('/strategy/operations')) {
      const params = (config as any)?.params;
      if (params?.flag_key) return Promise.resolve({ items: sampleOperations, total: 1, page: 1, page_size: 20 });
      return Promise.resolve(emptyList);
    }
    if (urlStr.includes('/strategy/impact')) return Promise.resolve(sampleImpact);
    if (urlStr.includes('/strategy/gates')) return Promise.resolve(sampleGates);
    if (urlStr.includes('/strategy/versions')) return Promise.resolve({ items: sampleVersions, total: 1, page: 1, page_size: 20 });
    return Promise.resolve({});
  });
}

const WAIT_OPTS = { timeout: 8000 };

describe('StrategyCenterPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders page heading', async () => {
    mockFullLoad();
    render(<StrategyCenterPage />);
    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 2 })).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders flag labels from API', async () => {
    mockFullLoad();
    render(<StrategyCenterPage />);
    await waitFor(() => {
      expect(screen.getAllByText('Parent Child Retrieval').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('Strategic TopK').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('Evidence Refusal').length).toBeGreaterThanOrEqual(1);
    }, WAIT_OPTS);
  });

  it('renders flag status tags', async () => {
    mockFullLoad();
    render(<StrategyCenterPage />);
    await waitFor(() => {
      expect(screen.getAllByText('灰度中').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('已启用').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('影子模式').length).toBeGreaterThanOrEqual(1);
    }, WAIT_OPTS);
  });

  it('renders rollout percentage values', async () => {
    mockFullLoad();
    render(<StrategyCenterPage />);
    await waitFor(() => {
      expect(screen.getByText('20%')).toBeInTheDocument();
      expect(screen.getByText('100%')).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders risk level tags', async () => {
    mockFullLoad();
    render(<StrategyCenterPage />);
    await waitFor(() => {
      expect(screen.getAllByText('中风险').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('低风险').length).toBeGreaterThanOrEqual(1);
      expect(screen.getAllByText('高风险').length).toBeGreaterThanOrEqual(1);
    }, WAIT_OPTS);
  });

  it('shows error alert when flags API fails', async () => {
    mockedGet.mockRejectedValueOnce(new Error('Server unavailable'));
    render(<StrategyCenterPage />);
    await waitFor(() => {
      expect(screen.getByText(/Server unavailable/)).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders flag_key when a flag is selected', async () => {
    mockFullLoad();
    render(<StrategyCenterPage />);
    await waitFor(() => {
      expect(screen.getAllByText('RAG_ENABLE_PARENT_CHILD_RETRIEVAL').length).toBeGreaterThanOrEqual(1);
    }, WAIT_OPTS);
  });

  it('renders impact metrics values', async () => {
    mockFullLoad();
    render(<StrategyCenterPage />);
    await waitFor(() => {
      expect(screen.getByText('0.400')).toBeInTheDocument();
      expect(screen.getByText('0.200')).toBeInTheDocument();
      expect(screen.getByText('0.920')).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders gate summary data', async () => {
    mockFullLoad();
    render(<StrategyCenterPage />);
    await waitFor(() => {
      expect(screen.getByText('rpt_baseline')).toBeInTheDocument();
      expect(screen.getByText('rpt_candidate')).toBeInTheDocument();
      expect(screen.getByText('run_001')).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders version list', async () => {
    mockFullLoad();
    render(<StrategyCenterPage />);
    await waitFor(() => {
      expect(screen.getByText('v1')).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders operation log entries', async () => {
    mockFullLoad();
    render(<StrategyCenterPage />);
    await waitFor(() => {
      expect(screen.getByText('update_flag')).toBeInTheDocument();
      expect(screen.getByText('canary test after offline gate passed')).toBeInTheDocument();
    }, WAIT_OPTS);
  });

  it('renders contract gap list when present in impact', async () => {
    const impactWithGaps: StrategyImpact = { ...sampleImpact, contract_gaps: ['avg_context_tokens_delta'] };
    const emptyList = { items: [], total: 0, page: 1, page_size: 20 };
    mockedGet.mockImplementation((url: string | unknown, config?: unknown) => {
      const urlStr = typeof url === 'string' ? url : String(url);
      if (urlStr.includes('/strategy/flags')) return Promise.resolve({ items: sampleFlags });
      if (urlStr.includes('/strategy/operations')) return Promise.resolve(emptyList);
      if (urlStr.includes('/strategy/impact')) return Promise.resolve(impactWithGaps);
      if (urlStr.includes('/strategy/gates')) return Promise.resolve(sampleGates);
      if (urlStr.includes('/strategy/versions')) return Promise.resolve(emptyList);
      return Promise.resolve({});
    });
    render(<StrategyCenterPage />);
    await waitFor(() => {
      const elements = screen.queryAllByText(/avg_context_tokens_delta/);
      expect(elements.length).toBeGreaterThanOrEqual(1);
    }, WAIT_OPTS);
  });
});
