'use client';

import dayjs, { type Dayjs } from 'dayjs';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { ReloadOutlined } from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  DatePicker,
  Empty,
  Segmented,
  Space,
  Statistic,
  Table,
  Typography,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';
import type { CostSummary, CostTimeseriesPoint, HighCostQuery } from '@/types/kb';
import { useKnowledgeBaseContext } from './knowledge-base-provider';
import {
  buildCostChartGeometry,
  buildCostListRows,
  buildCostWindowParams,
  formatCostAxisLabel,
  formatCostBucketLabel,
  getDefaultCostSelectedDate,
  type CostChartDatum,
  type CostDisplayMode,
  type CostViewMode,
} from './cost-ops-cost-page.helpers';

const { Paragraph, Text, Title } = Typography;

const BAR_COLOR = '#43c6c7';
const LINE_COLOR = '#2563eb';

function formatCostValue(value?: number) {
  if (!Number.isFinite(value)) {
    return '0.0000';
  }
  return Number(value).toFixed(4);
}

function formatTokenValue(value?: number) {
  if (!Number.isFinite(value)) {
    return '0';
  }
  return Math.round(value ?? 0).toLocaleString('en-US');
}

function formatBucketDate(value: string, viewMode: CostViewMode) {
  const parsed = dayjs(value);
  if (!parsed.isValid()) {
    return value;
  }
  return viewMode === 'month' ? parsed.format('YYYY-MM-DD') : parsed.format('YYYY-MM-DD HH:mm');
}

function buildChartData(items: CostTimeseriesPoint[], viewMode: CostViewMode): CostChartDatum[] {
  return items.map((item) => ({
    bucket: item.bucket,
    label: formatCostBucketLabel(item.bucket, viewMode),
    value: item.total_tokens ?? 0,
    tokensPer1KQueries: item.tokens_per_1k_queries,
    avgTokensPerQuery: item.avg_tokens_per_query,
  }));
}

function LegendItem({
  color,
  label,
  dashed = false,
}: {
  color: string;
  label: string;
  dashed?: boolean;
}) {
  return (
    <span className="inline-flex items-center gap-2 text-sm text-slate-600">
      <span
        aria-hidden
        className="inline-block"
        style={{
          width: 18,
          height: dashed ? 4 : 10,
          borderRadius: dashed ? 999 : 3,
          background: dashed ? 'transparent' : color,
          border: dashed ? `2px solid ${color}` : 'none',
        }}
      />
      {label}
    </span>
  );
}

function CostTrendChart({
  items,
  viewMode,
  totalTokens,
}: {
  items: CostTimeseriesPoint[];
  viewMode: CostViewMode;
  totalTokens?: number;
}) {
  const chartData = useMemo(() => buildChartData(items, viewMode), [items, viewMode]);
  const geometry = useMemo(() => buildCostChartGeometry(chartData), [chartData]);
  const [activeIndex, setActiveIndex] = useState<number | null>(null);
  const hasVisibleData = chartData.some((item) => item.value > 0);
  const activeDatum = activeIndex === null ? null : (chartData[activeIndex] ?? null);
  const activePoint = activeIndex === null ? null : (geometry.points[activeIndex] ?? null);

  if (!hasVisibleData) {
    return (
      <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前时间范围内没有 Token 消耗数据" />
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <div className="text-sm text-slate-500">当前时间窗总 Token 消耗</div>
          <div className="text-2xl font-semibold text-slate-900">
            {formatTokenValue(totalTokens)}
          </div>
        </div>
        <Space size={16} wrap>
          <LegendItem color={BAR_COLOR} label="总 Token 消耗" />
          <LegendItem color={LINE_COLOR} label="Token 趋势" dashed />
        </Space>
      </div>

      <div
        className="relative overflow-x-auto rounded-2xl border border-slate-100 bg-white px-2 py-3"
        onMouseLeave={() => setActiveIndex(null)}
      >
        {activeDatum && activePoint ? (
          <div
            className="pointer-events-none absolute z-10 min-w-[260px] rounded-2xl border border-slate-200 bg-white/95 p-4 shadow-xl shadow-slate-200/60 backdrop-blur"
            style={{
              left: `${Math.min(Math.max((activePoint.x / geometry.width) * 100 + 2, 10), 72)}%`,
              top: `${Math.min(Math.max((activePoint.y / geometry.height) * 100 - 8, 12), 52)}%`,
            }}
          >
            <div className="mb-3 text-sm font-semibold text-slate-700">
              {formatBucketDate(activeDatum.bucket, viewMode)}
            </div>
            <div className="space-y-3 text-sm">
              <div className="flex items-center justify-between gap-4">
                <span className="inline-flex items-center gap-2 text-slate-500">
                  <span className="h-3 w-3 rounded-full bg-blue-500" />总 Token 消耗
                </span>
                <span className="font-semibold text-slate-800">
                  {formatTokenValue(activeDatum.value)}
                </span>
              </div>
              <div className="flex items-center justify-between gap-4">
                <span className="inline-flex items-center gap-2 text-slate-500">
                  <span className="h-3 w-3 rounded-full bg-teal-400" />
                  每千次 Token 消耗
                </span>
                <span className="font-semibold text-slate-800">
                  {formatTokenValue(activeDatum.tokensPer1KQueries)}
                </span>
              </div>
              <div className="flex items-center justify-between gap-4">
                <span className="inline-flex items-center gap-2 text-slate-500">
                  <span className="h-3 w-3 rounded-full bg-slate-300" />
                  平均每次 Token 消耗
                </span>
                <span className="font-semibold text-slate-800">
                  {formatTokenValue(activeDatum.avgTokensPerQuery)}
                </span>
              </div>
            </div>
          </div>
        ) : null}

        <svg
          viewBox={`0 0 ${geometry.width} ${geometry.height}`}
          className="w-full min-w-[760px]"
          role="img"
          aria-label="Token 消耗趋势图"
        >
          {geometry.yTicks.map((tick) => {
            const y =
              geometry.baselineY -
              (tick / geometry.maxValue) * (geometry.baselineY - geometry.plotTop);
            return (
              <g key={`grid-${tick}`}>
                <line
                  x1={geometry.plotLeft}
                  x2={geometry.width - geometry.plotRight}
                  y1={y}
                  y2={y}
                  stroke="#e2e8f0"
                  strokeDasharray="4 6"
                />
                <text
                  x={geometry.plotLeft - 12}
                  y={y + 4}
                  textAnchor="end"
                  fontSize="12"
                  fill="#94a3b8"
                >
                  {formatCostAxisLabel(tick)}
                </text>
              </g>
            );
          })}

          <line
            x1={geometry.plotLeft}
            x2={geometry.width - geometry.plotRight}
            y1={geometry.baselineY}
            y2={geometry.baselineY}
            stroke="#cbd5e1"
          />

          {activePoint ? (
            <line
              x1={activePoint.x}
              x2={activePoint.x}
              y1={geometry.plotTop}
              y2={geometry.baselineY}
              stroke="#94a3b8"
              strokeDasharray="4 4"
            />
          ) : null}

          {geometry.bars.map((bar) => (
            <g key={`bar-${bar.label}-${bar.x}`}>
              <rect
                x={bar.x}
                y={bar.y}
                width={bar.width}
                height={Math.max(bar.height, 0)}
                rx="4"
                fill={BAR_COLOR}
                fillOpacity="0.9"
              />
            </g>
          ))}

          <path
            d={geometry.linePath}
            fill="none"
            stroke={LINE_COLOR}
            strokeWidth="3"
            strokeLinecap="round"
            strokeLinejoin="round"
          />

          {geometry.points.map((point) => (
            <g key={`point-${point.label}-${point.x}`}>
              <circle
                cx={point.x}
                cy={point.y}
                r="4.5"
                fill="#ffffff"
                stroke={LINE_COLOR}
                strokeWidth="2"
              />
            </g>
          ))}

          {geometry.xTicks.map((tick) =>
            tick.show ? (
              <text
                key={`x-${tick.label}-${tick.x}`}
                x={tick.x}
                y={geometry.height - 10}
                textAnchor="middle"
                fontSize="12"
                fill="#94a3b8"
              >
                {tick.label}
              </text>
            ) : null
          )}

          {geometry.hitAreas.map((area, index) => (
            <rect
              key={`hit-${area.label}-${area.x}`}
              x={area.x}
              y={area.y}
              width={area.width}
              height={area.height}
              fill="transparent"
              data-testid={`cost-chart-hit-${index}`}
              onMouseEnter={() => setActiveIndex(index)}
              onMouseMove={() => setActiveIndex(index)}
            />
          ))}
        </svg>
      </div>
    </div>
  );
}

export function CostOpsCostPage() {
  const { selectedBase } = useKnowledgeBaseContext();
  const [viewMode, setViewMode] = useState<CostViewMode>('day');
  const [displayMode, setDisplayMode] = useState<CostDisplayMode>('chart');
  const [selectedDate, setSelectedDate] = useState<Dayjs>(() => getDefaultCostSelectedDate());
  const [summary, setSummary] = useState<CostSummary | null>(null);
  const [timeseries, setTimeseries] = useState<CostTimeseriesPoint[]>([]);
  const [highCostQueries, setHighCostQueries] = useState<HighCostQuery[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const requestParams = useMemo(
    () => ({
      ...buildCostWindowParams(viewMode, selectedDate),
      ...(selectedBase?.id ? { kb_id: selectedBase.id } : {}),
    }),
    [selectedBase?.id, selectedDate, viewMode]
  );

  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [summaryResp, timeseriesResp, highCostResp] = await Promise.all([
        apiClient.get(KB_ADMIN_API.COST_SUMMARY, { params: requestParams }) as Promise<CostSummary>,
        apiClient.get(KB_ADMIN_API.COST_TIMESERIES, { params: requestParams }) as Promise<{
          items?: CostTimeseriesPoint[];
        }>,
        apiClient.get(KB_ADMIN_API.HIGH_COST_QUERIES, {
          params: { ...requestParams, page: 1, page_size: 10 },
        }) as Promise<{ items?: HighCostQuery[] }>,
      ]);
      setSummary(summaryResp);
      setTimeseries(timeseriesResp.items ?? []);
      setHighCostQueries(highCostResp.items ?? []);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '加载成本看板失败');
      setSummary(null);
      setTimeseries([]);
      setHighCostQueries([]);
    } finally {
      setLoading(false);
    }
  }, [requestParams]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  const detailRows = useMemo(() => buildCostListRows(timeseries), [timeseries]);

  const detailColumns = useMemo<ColumnsType<CostTimeseriesPoint>>(
    () => [
      {
        title: '时间',
        dataIndex: 'bucket',
        key: 'bucket',
        render: (value: string) => formatBucketDate(value, viewMode),
      },
      {
        title: '总成本',
        dataIndex: 'total_estimated_cost',
        key: 'total_estimated_cost',
        align: 'right',
        render: (value?: number) => formatCostValue(value),
      },
      {
        title: '每千次问答成本',
        dataIndex: 'cost_per_1k_queries',
        key: 'cost_per_1k_queries',
        align: 'right',
        render: (value?: number) => formatCostValue(value),
      },
      {
        title: '平均上下文 Tokens',
        dataIndex: 'avg_context_tokens',
        key: 'avg_context_tokens',
        align: 'right',
        render: (value?: number) =>
          Number.isFinite(value) ? Math.round(value ?? 0).toString() : '-',
      },
    ],
    [viewMode]
  );

  const queryColumns = useMemo<ColumnsType<HighCostQuery>>(
    () => [
      {
        title: 'Request ID',
        dataIndex: 'request_id',
        key: 'request_id',
        render: (value: string) => <Text code>{value}</Text>,
      },
      {
        title: '策略版本',
        dataIndex: 'strategy_version',
        key: 'strategy_version',
        render: (value?: string) => value || '-',
      },
      {
        title: '模型',
        dataIndex: 'model_name',
        key: 'model_name',
        render: (value?: string) => value || '-',
      },
      {
        title: '成本',
        dataIndex: 'estimated_cost',
        key: 'estimated_cost',
        align: 'right',
        render: (value?: number) => formatCostValue(value),
      },
      {
        title: '上下文 Tokens',
        dataIndex: 'context_tokens',
        key: 'context_tokens',
        align: 'right',
        render: (value?: number) => value ?? '-',
      },
      {
        title: '时间',
        dataIndex: 'created_at',
        key: 'created_at',
        render: (value?: string) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'),
      },
    ],
    []
  );

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            成本看板
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            查看请求级成本归因、时间变化趋势和高成本查询。当前知识库：
            {selectedBase?.name ?? '全部知识库'}
          </Paragraph>
        </div>

        <Space size={12} wrap>
          <Segmented
            value={viewMode}
            options={[
              { label: '日', value: 'day' },
              { label: '月', value: 'month' },
            ]}
            onChange={(value) => setViewMode(value as CostViewMode)}
          />
          <DatePicker
            allowClear={false}
            inputReadOnly
            picker={viewMode === 'month' ? 'month' : 'date'}
            value={selectedDate}
            format={viewMode === 'month' ? 'YYYY-MM' : 'YYYY-MM-DD'}
            onChange={(value) => {
              if (value) {
                setSelectedDate(value);
              }
            }}
          />
          <Segmented
            value={displayMode}
            options={[
              { label: '图表', value: 'chart' },
              { label: '列表', value: 'list' },
            ]}
            onChange={(value) => setDisplayMode(value as CostDisplayMode)}
          />
          <Button icon={<ReloadOutlined />} onClick={() => void loadData()} loading={loading}>
            刷新
          </Button>
        </Space>
      </div>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <Statistic
            title="总成本"
            value={summary?.total_estimated_cost ?? 0}
            precision={4}
            loading={loading}
          />
        </Card>
        <Card>
          <Statistic
            title="每千次问答成本"
            value={summary?.cost_per_1k_queries ?? 0}
            precision={4}
            loading={loading}
          />
        </Card>
        <Card>
          <Statistic
            title="平均上下文 Tokens"
            value={summary?.avg_context_tokens ?? 0}
            precision={0}
            loading={loading}
          />
        </Card>
        <Card>
          <Statistic
            title="高成本 Query"
            value={summary?.high_cost_query_count ?? 0}
            loading={loading}
          />
        </Card>
      </div>

      <Card
        title="Token 消耗变化"
        extra={<Text type="secondary">{viewMode === 'month' ? '按自然日统计' : '按小时统计'}</Text>}
        styles={{ body: { padding: 20 } }}
      >
        {displayMode === 'chart' ? (
          <CostTrendChart
            items={timeseries}
            viewMode={viewMode}
            totalTokens={summary?.total_tokens}
          />
        ) : detailRows.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="当前时间范围内还没有非零成本数据，建议切换时间范围或等待请求流量产生。"
          >
            <Button icon={<ReloadOutlined />} onClick={() => void loadData()} loading={loading}>
              重新加载
            </Button>
          </Empty>
        ) : (
          <Table
            rowKey="bucket"
            columns={detailColumns}
            dataSource={detailRows}
            pagination={false}
            size="middle"
          />
        )}
      </Card>

      <Card title="高成本查询 Top 10">
        {highCostQueries.length === 0 && !loading ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="当前时间范围内没有高成本查询，可以继续观察成本趋势。"
          />
        ) : (
          <Table
            rowKey="request_id"
            columns={queryColumns}
            dataSource={highCostQueries}
            pagination={false}
          />
        )}
      </Card>
    </div>
  );
}
