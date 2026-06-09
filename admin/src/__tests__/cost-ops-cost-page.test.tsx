import dayjs from 'dayjs';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { KB_ADMIN_API } from '@/config/api';
import type { CostSummary, CostTimeseriesPoint, HighCostQuery } from '@/types/kb';

vi.mock('@/services/api/client', () => ({
  default: {
    get: vi.fn(),
  },
}));

vi.mock('@/components/admin/knowledge-base-provider', () => ({
  useKnowledgeBaseContext: () => ({
    selectedBase: { id: 7, name: '测试知识库' },
  }),
}));

vi.mock('@/components/admin/cost-ops-cost-page.helpers', async () => {
  const actual = await vi.importActual<
    typeof import('@/components/admin/cost-ops-cost-page.helpers')
  >('@/components/admin/cost-ops-cost-page.helpers');
  return {
    ...actual,
    getDefaultCostSelectedDate: () => dayjs('2026-06-08'),
  };
});

import apiClient from '@/services/api/client';
import { CostOpsCostPage } from '@/components/admin/cost-ops-cost-page';

const mockedGet = vi.mocked(apiClient.get);

const summary: CostSummary = {
  range: 'custom',
  total_estimated_cost: 1.2345,
  total_tokens: 12345,
  tokens_per_1k_queries: 1234500,
  avg_tokens_per_query: 1234.5,
  cost_per_1k_queries: 123.45,
  avg_context_tokens: 256,
  high_cost_query_count: 2,
};

const timeseries: CostTimeseriesPoint[] = [
  {
    bucket: '2026-06-08T05:00:00+08:00',
    total_estimated_cost: 0.2,
    total_tokens: 2000,
    tokens_per_1k_queries: 2000000,
    avg_tokens_per_query: 2000,
    cost_per_1k_queries: 20,
    avg_context_tokens: 120,
  },
  {
    bucket: '2026-06-08T02:00:00+08:00',
    total_estimated_cost: 0,
    total_tokens: 0,
    tokens_per_1k_queries: 0,
    avg_tokens_per_query: 0,
    cost_per_1k_queries: 0,
    avg_context_tokens: 0,
  },
  {
    bucket: '2026-06-08T09:00:00+08:00',
    total_estimated_cost: 0.8,
    total_tokens: 8000,
    tokens_per_1k_queries: 8000000,
    avg_tokens_per_query: 8000,
    cost_per_1k_queries: 80,
    avg_context_tokens: 240,
  },
];

const highCostQueries: HighCostQuery[] = [
  {
    request_id: 'req-2',
    strategy_version: 'v2',
    model_name: 'mimo-v2.5-pro',
    estimated_cost: 0.8,
    context_tokens: 240,
    created_at: '2026-06-08T09:15:00+08:00',
  },
];

function mockCostApis() {
  mockedGet.mockImplementation((url: string | unknown, config?: unknown) => {
    const urlString = String(url);
    const params = (config as { params?: Record<string, unknown> } | undefined)?.params ?? {};

    if (urlString === KB_ADMIN_API.COST_SUMMARY) {
      return Promise.resolve({ ...summary, range: params.bucket === '1d' ? 'custom' : 'custom' });
    }
    if (urlString === KB_ADMIN_API.COST_TIMESERIES) {
      return Promise.resolve({ items: timeseries });
    }
    if (urlString === KB_ADMIN_API.HIGH_COST_QUERIES) {
      return Promise.resolve({ items: highCostQueries });
    }
    return Promise.resolve({});
  });
}

describe('CostOpsCostPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCostApis();
  });

  it('loads day view with hourly Shanghai window params', async () => {
    render(<CostOpsCostPage />);

    await waitFor(() => {
      expect(mockedGet).toHaveBeenCalled();
    });

    const summaryCall = mockedGet.mock.calls.find(([url]) => url === KB_ADMIN_API.COST_SUMMARY);
    expect(summaryCall?.[1]).toMatchObject({
      params: {
        bucket: '1h',
        tz: 'Asia/Shanghai',
        kb_id: 7,
        start_time: '2026-06-08T00:00:00+08:00',
        end_time: '2026-06-08T23:59:59.999999999+08:00',
      },
    });
  });

  it('switches to month view with daily Shanghai window params', async () => {
    const user = userEvent.setup();
    render(<CostOpsCostPage />);

    await waitFor(() => {
      expect(mockedGet).toHaveBeenCalledTimes(3);
    });

    await user.click(screen.getByText('月'));

    await waitFor(() => {
      const monthCall = mockedGet.mock.calls
        .filter(([url]) => url === KB_ADMIN_API.COST_SUMMARY)
        .find(
          ([, config]) => (config as { params?: Record<string, unknown> })?.params?.bucket === '1d'
        );
      expect(monthCall?.[1]).toMatchObject({
        params: {
          bucket: '1d',
          tz: 'Asia/Shanghai',
          kb_id: 7,
          start_time: '2026-06-01T00:00:00+08:00',
          end_time: '2026-06-30T23:59:59.999999999+08:00',
        },
      });
    });
  });

  it('shows non-zero detail rows in reverse chronological order in list mode', async () => {
    const user = userEvent.setup();
    render(<CostOpsCostPage />);

    await waitFor(() => {
      expect(screen.getByText('高成本 Query Top 10')).toBeInTheDocument();
    });

    await user.click(screen.getByText('列表'));

    await waitFor(() => {
      expect(screen.getByText('2026-06-08 09:00')).toBeInTheDocument();
      expect(screen.getByText('2026-06-08 05:00')).toBeInTheDocument();
    });

    expect(screen.queryByText('2026-06-08 02:00')).not.toBeInTheDocument();

    const newest = screen.getByText('2026-06-08 09:00');
    const older = screen.getByText('2026-06-08 05:00');
    expect(newest.compareDocumentPosition(older) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('shows tooltip details when hovering a chart bucket', async () => {
    render(<CostOpsCostPage />);

    await waitFor(() => {
      expect(screen.getByLabelText('Token 消耗趋势图')).toBeInTheDocument();
    });

    fireEvent.mouseEnter(screen.getByTestId('cost-chart-hit-2'));

    await waitFor(() => {
      expect(screen.getByText('2026-06-08 09:00')).toBeInTheDocument();
      expect(screen.getAllByText('总 Token 消耗').length).toBeGreaterThanOrEqual(2);
      expect(screen.getByText('每千次 Token 消耗')).toBeInTheDocument();
      expect(screen.getByText('平均每次 Token 消耗')).toBeInTheDocument();
      expect(screen.getAllByText('8,000').length).toBeGreaterThanOrEqual(2);
      expect(screen.getByText('8,000,000')).toBeInTheDocument();
    });
  });
});
