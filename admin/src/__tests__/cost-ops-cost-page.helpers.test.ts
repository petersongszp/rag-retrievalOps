import dayjs from 'dayjs';
import { describe, expect, it } from 'vitest';
import {
  buildCostChartGeometry,
  buildCostListRows,
  buildCostWindowParams,
  type CostChartDatum,
} from '@/components/admin/cost-ops-cost-page.helpers';
import type { CostTimeseriesPoint } from '@/types/kb';

describe('cost ops helpers', () => {
  it('builds day window params in Asia/Shanghai', () => {
    expect(buildCostWindowParams('day', dayjs('2026-06-08'))).toEqual({
      start_time: '2026-06-08T00:00:00+08:00',
      end_time: '2026-06-08T23:59:59.999999999+08:00',
      bucket: '1h',
      tz: 'Asia/Shanghai',
    });
  });

  it('builds month window params in Asia/Shanghai', () => {
    expect(buildCostWindowParams('month', dayjs('2026-06-08'))).toEqual({
      start_time: '2026-06-01T00:00:00+08:00',
      end_time: '2026-06-30T23:59:59.999999999+08:00',
      bucket: '1d',
      tz: 'Asia/Shanghai',
    });
  });

  it('filters zero cost rows and sorts newest first', () => {
    const items: CostTimeseriesPoint[] = [
      {
        bucket: '2026-06-08T05:00:00+08:00',
        total_estimated_cost: 0.2,
        cost_per_1k_queries: 20,
        avg_context_tokens: 120,
      },
      {
        bucket: '2026-06-08T02:00:00+08:00',
        total_estimated_cost: 0,
        cost_per_1k_queries: 0,
        avg_context_tokens: 10,
      },
      {
        bucket: '2026-06-08T09:00:00+08:00',
        total_estimated_cost: 0.8,
        cost_per_1k_queries: 80,
        avg_context_tokens: 240,
      },
    ];

    expect(buildCostListRows(items).map((item) => item.bucket)).toEqual([
      '2026-06-08T09:00:00+08:00',
      '2026-06-08T05:00:00+08:00',
    ]);
  });

  it('keeps zero value points in chart geometry', () => {
    const data: CostChartDatum[] = [
      { bucket: '2026-06-08T00:00:00+08:00', label: '00:00', value: 0.5 },
      { bucket: '2026-06-08T01:00:00+08:00', label: '01:00', value: 0 },
      { bucket: '2026-06-08T02:00:00+08:00', label: '02:00', value: 1.2 },
    ];

    const geometry = buildCostChartGeometry(data);
    expect(geometry.points).toHaveLength(3);
    expect(geometry.bars).toHaveLength(3);
    expect(geometry.linePath).not.toBe('');
  });
});
