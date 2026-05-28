'use client';

import { useEffect, useMemo, useState } from 'react';
import { ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Empty, Segmented, Space, Statistic, Table, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';
import type { CostSummary, CostTimeseriesPoint, HighCostQuery, MetricsRange } from '@/types/kb';
import { useKnowledgeBaseContext } from './knowledge-base-provider';

const { Paragraph, Text, Title } = Typography;

export function CostOpsCostPage() {
  const { selectedBase } = useKnowledgeBaseContext();
  const [range, setRange] = useState<MetricsRange>('24h');
  const [summary, setSummary] = useState<CostSummary | null>(null);
  const [timeseries, setTimeseries] = useState<CostTimeseriesPoint[]>([]);
  const [highCostQueries, setHighCostQueries] = useState<HighCostQuery[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);
      const params = {
        range,
        ...(selectedBase?.id ? { kb_id: selectedBase.id } : {}),
      };
      const [summaryResp, timeseriesResp, highCostResp] = await Promise.all([
        apiClient.get(KB_ADMIN_API.COST_SUMMARY, { params }) as Promise<CostSummary>,
        apiClient.get(KB_ADMIN_API.COST_TIMESERIES, { params }) as Promise<{ items?: CostTimeseriesPoint[] }>,
        apiClient.get(KB_ADMIN_API.HIGH_COST_QUERIES, { params: { ...params, page: 1, page_size: 10 } }) as Promise<{ items?: HighCostQuery[] }>,
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
  };

  useEffect(() => {
    void loadData();
  }, [range, selectedBase?.id]);

  const columns = useMemo<ColumnsType<HighCostQuery>>(
    () => [
      { title: 'Request ID', dataIndex: 'request_id', key: 'request_id', render: (value: string) => <Text code>{value}</Text> },
      { title: '策略版本', dataIndex: 'strategy_version', key: 'strategy_version', render: (value?: string) => value || '-' },
      { title: '模型', dataIndex: 'model_name', key: 'model_name', render: (value?: string) => value || '-' },
      { title: '成本', dataIndex: 'estimated_cost', key: 'estimated_cost', render: (value?: number) => value?.toFixed(4) ?? '-' },
      { title: '上下文 Tokens', dataIndex: 'context_tokens', key: 'context_tokens', render: (value?: number) => value ?? '-' },
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
            查看请求级成本归因、时间趋势和高成本查询。当前知识库：{selectedBase?.name ?? '全部知识库'}
          </Paragraph>
        </div>
        <Space>
          <Segmented
            value={range}
            options={[
              { label: '1h', value: '1h' },
              { label: '24h', value: '24h' },
              { label: '7d', value: '7d' },
            ]}
            onChange={(value) => setRange(value as MetricsRange)}
          />
          <Button icon={<ReloadOutlined />} onClick={() => void loadData()} loading={loading}>
            刷新
          </Button>
        </Space>
      </div>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      <div className="grid gap-4 md:grid-cols-4">
        <Card><Statistic title="总成本" value={summary?.total_estimated_cost ?? 0} precision={4} loading={loading} /></Card>
        <Card><Statistic title="每千次问答成本" value={summary?.cost_per_1k_queries ?? 0} precision={4} loading={loading} /></Card>
        <Card><Statistic title="平均上下文 Tokens" value={summary?.avg_context_tokens ?? 0} precision={0} loading={loading} /></Card>
        <Card><Statistic title="高成本 Query" value={summary?.high_cost_query_count ?? 0} loading={loading} /></Card>
      </div>

      <Card title="成本趋势">
        {timeseries.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前时间窗内没有成本数据" />
        ) : (
          <div className="space-y-2">
            {timeseries.map((item) => (
              <div key={item.bucket} className="flex items-center justify-between rounded border border-slate-200 px-3 py-2 text-sm">
                <span>{new Date(item.bucket).toLocaleString()}</span>
                <span>{item.total_estimated_cost?.toFixed?.(4) ?? '0.0000'}</span>
              </div>
            ))}
          </div>
        )}
      </Card>

      <Card title="高成本 Query Top 10">
        <Table rowKey="request_id" columns={columns} dataSource={highCostQueries} pagination={false} />
      </Card>
    </div>
  );
}
