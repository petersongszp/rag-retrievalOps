'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  BellOutlined,
  FolderOpenOutlined,
  ReloadOutlined,
  SyncOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { Alert, Button, Card, Col, Empty, Row, Segmented, Space, Statistic, Typography } from 'antd';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';
import type {
  MetricsOverview,
  MetricsOverviewBucketCount,
  MetricsOverviewCostBreakdown,
  MetricsOverviewBucketP95,
  MetricsOverviewBucketRate,
  MetricsRange,
} from '@/types/kb';
import { useKnowledgeBaseContext } from './knowledge-base-provider';
import { ActionEmpty } from './ui/action-empty';
import { InlineHelp } from './ui/inline-help';
import { MetricCard } from './ui/metric-card';
import { PageHeader } from './ui/page-header';

const { Paragraph, Text, Title } = Typography;

interface DashboardStats {
  kb_count: number;
  document_count: number;
  processing_job_count: number;
  failed_job_count: number;
}

type NumericPoint = {
  label: string;
  value: number;
};

function formatBucketLabel(bucket: string, range: MetricsRange) {
  const date = new Date(bucket);
  if (range === '7d') {
    return `${date.getUTCMonth() + 1}/${date.getUTCDate()} ${String(date.getUTCHours()).padStart(2, '0')}:00`;
  }
  return `${String(date.getUTCHours()).padStart(2, '0')}:${String(date.getUTCMinutes()).padStart(2, '0')}`;
}

function formatPercent(value: number) {
  return `${(value * 100).toFixed(1)}%`;
}

function buildPolyline(points: NumericPoint[]) {
  if (points.length === 0) {
    return '';
  }

  const width = 220;
  const height = 88;
  const padding = 8;
  const values = points.map((point) => point.value);
  const max = Math.max(...values);
  const min = Math.min(...values);
  const span = max - min || 1;

  return points
    .map((point, index) => {
      const x = padding + (index * (width - padding * 2)) / Math.max(points.length - 1, 1);
      const y = height - padding - ((point.value - min) / span) * (height - padding * 2);
      return `${x},${y}`;
    })
    .join(' ');
}

function TrendSparkline({ data, color }: { data: NumericPoint[]; color: string }) {
  if (data.length === 0) {
    return (
      <div className="flex h-[88px] items-center justify-center rounded-xl bg-slate-50 text-sm text-slate-400">
        暂无数据
      </div>
    );
  }

  const polyline = buildPolyline(data);
  const lastValue = data[data.length - 1];

  return (
    <div className="space-y-2">
      <svg viewBox="0 0 220 88" className="h-[88px] w-full rounded-xl bg-slate-50">
        <polyline fill="none" stroke={color} strokeWidth="3" points={polyline} strokeLinecap="round" />
      </svg>
      <div className="flex items-center justify-between text-xs text-slate-500">
        <span>{data[0]?.label}</span>
        <span className="font-medium text-slate-700">
          {lastValue.label} · {lastValue.value.toFixed(1)}
        </span>
      </div>
    </div>
  );
}

function TrendMetricCard({
  title,
  value,
  helper,
  data,
  color,
}: {
  title: string;
  value: string;
  helper: string;
  data: NumericPoint[];
  color: string;
}) {
  return (
    <Card bodyStyle={{ padding: 20 }} className="h-full admin-section-card">
      <Space direction="vertical" size={14} className="w-full">
        <div>
          <Space size={6}>
            <Text type="secondary">{title}</Text>
            <InlineHelp title={helper} />
          </Space>
          <div className="mt-1 text-2xl font-semibold text-slate-900">{value}</div>
          <Text type="secondary">{helper}</Text>
        </div>
        <TrendSparkline data={data} color={color} />
      </Space>
    </Card>
  );
}

function ErrorTopNCard({ metrics }: { metrics: MetricsOverview | null }) {
  const items = metrics?.error_type_topn ?? [];

  return (
    <Card title="失败类型 TopN" className="h-full admin-section-card">
      {items.length === 0 ? (
        <ActionEmpty
          title="当前时间窗内没有失败日志"
          description="说明当前链路较稳定，可以继续关注趋势变化。"
        />
      ) : (
        <Space direction="vertical" size={14} className="w-full">
          {items.map((item, index) => {
            const max = items[0]?.count || 1;
            const width = `${Math.max((item.count / max) * 100, 10)}%`;
            return (
              <div key={`${item.error_code}-${index}`} className="space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <Text strong>{item.error_code}</Text>
                  <Text type="secondary">{item.count}</Text>
                </div>
                <div className="h-2 rounded-full bg-slate-100">
                  <div className="h-2 rounded-full bg-rose-500" style={{ width }} />
                </div>
              </div>
            );
          })}
        </Space>
      )}
    </Card>
  );
}

export function DashboardPage() {
  const router = useRouter();
  const { selectedBase, isPermissionDenied } = useKnowledgeBaseContext();
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [statsError, setStatsError] = useState<string | null>(null);
  const [statsPermissionDenied, setStatsPermissionDenied] = useState(false);
  const [metricsRange, setMetricsRange] = useState<MetricsRange>('24h');
  const [metrics, setMetrics] = useState<MetricsOverview | null>(null);
  const [metricsLoading, setMetricsLoading] = useState(false);
  const [metricsError, setMetricsError] = useState<string | null>(null);

  useEffect(() => {
    const loadStats = async () => {
      try {
        setLoading(true);
        setStatsError(null);
        setStatsPermissionDenied(false);
        const data = (await apiClient.get(KB_ADMIN_API.DASHBOARD_STATS)) as DashboardStats;
        setStats(data);
      } catch (error) {
        const nextError = error instanceof Error ? error.message : '加载概览数据失败';
        setStatsError(nextError);
        setStatsPermissionDenied(
          Boolean(
            error &&
              typeof error === 'object' &&
              'response' in error &&
              error.response &&
              typeof error.response === 'object' &&
              'status' in error.response &&
              error.response.status === 403
          )
        );
      } finally {
        setLoading(false);
      }
    };

    void loadStats();
  }, []);

  useEffect(() => {
    const loadMetrics = async () => {
      try {
        setMetricsLoading(true);
        setMetricsError(null);
        const data = (await apiClient.get(KB_ADMIN_API.METRICS_OVERVIEW, {
          params: {
            range: metricsRange,
            ...(selectedBase?.id ? { kb_id: selectedBase.id } : {}),
          },
        })) as MetricsOverview;
        setMetrics(data);
      } catch (error) {
        setMetricsError(error instanceof Error ? error.message : '加载监控指标失败');
        setMetrics(null);
      } finally {
        setMetricsLoading(false);
      }
    };

    void loadMetrics();
  }, [metricsRange, selectedBase?.id]);

  const jobsHref = selectedBase ? `/knowledge-bases/${selectedBase.id}` : '/knowledge-bases';

  const ingestSuccessSeries = useMemo(
    () =>
      (metrics?.ingest_success_rate ?? []).map((item: MetricsOverviewBucketRate) => ({
        label: formatBucketLabel(item.bucket, metricsRange),
        value: Number((item.rate * 100).toFixed(1)),
      })),
    [metrics?.ingest_success_rate, metricsRange]
  );

  const requestCountSeries = useMemo(
    () =>
      (metrics?.retrieve_request_count ?? []).map((item: MetricsOverviewBucketCount) => ({
        label: formatBucketLabel(item.bucket, metricsRange),
        value: item.count,
      })),
    [metrics?.retrieve_request_count, metricsRange]
  );

  const p95Series = useMemo(
    () =>
      (metrics?.retrieve_p95_ms ?? []).map((item: MetricsOverviewBucketP95) => ({
        label: formatBucketLabel(item.bucket, metricsRange),
        value: item.p95_ms,
      })),
    [metrics?.retrieve_p95_ms, metricsRange]
  );

  const emptyRateSeries = useMemo(
    () =>
      (metrics?.retrieve_empty_rate ?? []).map((item: MetricsOverviewBucketRate) => ({
        label: formatBucketLabel(item.bucket, metricsRange),
        value: Number((item.rate * 100).toFixed(1)),
      })),
    [metrics?.retrieve_empty_rate, metricsRange]
  );

  const costSeries = useMemo(
    () =>
      (metrics?.cost_overview ?? []).map((item: MetricsOverviewCostBreakdown) => ({
        label: formatBucketLabel(item.bucket, metricsRange),
        value: Number(item.cost_per_1k_queries.toFixed(4)),
      })),
    [metrics?.cost_overview, metricsRange]
  );

  const latestIngestRate = metrics?.ingest_success_rate.at(-1)?.rate ?? 0;
  const totalRequests = (metrics?.retrieve_request_count ?? []).reduce(
    (sum, item) => sum + item.count,
    0
  );
  const latestP95 = metrics?.retrieve_p95_ms.at(-1)?.p95_ms ?? 0;
  const latestEmptyRate = metrics?.retrieve_empty_rate.at(-1)?.rate ?? 0;
  const latestCostPer1K = metrics?.cost_overview?.at(-1)?.cost_per_1k_queries ?? 0;
  const latestAvgContextTokens = metrics?.cost_overview?.at(-1)?.avg_context_tokens ?? 0;

  return (
    <div className="admin-page">
      <PageHeader
        title="工作台"
        subtitle={`当前知识库：${selectedBase?.name ?? '未选择'}。在这里快速查看系统状态、核心指标和治理入口。`}
        extra={
          <Button icon={<ReloadOutlined />} onClick={() => router.refresh()}>
            刷新页面
          </Button>
        }
      />

      {isPermissionDenied && (
        <Alert
          type="error"
          showIcon
          message="权限不足"
          description="当前账号无权访问知识库列表（403）。请联系管理员确认权限配置。"
        />
      )}

      {statsError && !statsPermissionDenied ? <Alert type="error" showIcon message={statsError} /> : null}

      {statsPermissionDenied && (
        <Alert
          type="error"
          showIcon
          message="权限不足"
          description="当前账号无权访问概览统计数据（403）。请联系管理员确认权限配置。"
        />
      )}

      <Row gutter={[16, 16]}>
        <Col xs={12} md={6}>
          <MetricCard
            label="知识库数量"
            value={<Statistic value={stats?.kb_count ?? 0} loading={loading} />}
            helper="查看已接入的知识库资产"
            onClick={() => router.push('/knowledge-bases')}
          />
        </Col>
        <Col xs={12} md={6}>
          <MetricCard
            label="文档总数"
            value={<Statistic value={stats?.document_count ?? 0} loading={loading} />}
            helper="查看文档沉淀与入库规模"
            onClick={() => router.push('/knowledge-bases')}
          />
        </Col>
        <Col xs={12} md={6}>
          <MetricCard
            label="处理中入库任务"
            value={
              <Statistic
                value={stats?.processing_job_count ?? 0}
                loading={loading}
                prefix={<SyncOutlined spin={!!stats && stats.processing_job_count > 0} />}
              />
            }
            helper="查看正在执行的解析和入库任务"
            onClick={() => router.push(jobsHref)}
          />
        </Col>
        <Col xs={12} md={6}>
          <MetricCard
            label="失败入库任务"
            value={
              <Statistic
                value={stats?.failed_job_count ?? 0}
                loading={loading}
                valueStyle={stats && stats.failed_job_count > 0 ? { color: '#cf1322' } : undefined}
                prefix={stats && stats.failed_job_count > 0 ? <WarningOutlined /> : undefined}
              />
            }
            helper="优先处理失败任务，避免影响检索质量"
            onClick={() => router.push(jobsHref)}
          />
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={8}>
          <MetricCard
            label="每千次问答成本"
            value={<Statistic value={latestCostPer1K} precision={4} suffix="/1k" />}
            helper="查看成本趋势和高成本请求"
            onClick={() => router.push('/cost-ops/cost')}
          />
        </Col>
        <Col xs={24} md={8}>
          <MetricCard
            label="审计覆盖入口"
            value={<Statistic value={metrics?.retrieve_request_count?.length ?? 0} prefix={<FolderOpenOutlined />} />}
            helper="查看关键操作记录和追溯详情"
            onClick={() => router.push('/audit')}
          />
        </Col>
        <Col xs={24} md={8}>
          <MetricCard
            label="治理告警入口"
            value={<Statistic value={metrics?.error_type_topn?.length ?? 0} prefix={<BellOutlined />} />}
            helper="查看告警确认、解决与风险处理"
            onClick={() => router.push('/alerts')}
          />
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={12}>
          <Card title="知识库管理" extra={<Link href="/knowledge-bases">打开</Link>} className="admin-section-card">
            <Paragraph style={{ marginBottom: 0 }}>
              管理知识库与文档，上传文件并触发入库任务。
            </Paragraph>
          </Card>
        </Col>
        <Col xs={24} md={12}>
          <Card title="检索调优" extra={<Link href="/retrieval-lab">打开</Link>} className="admin-section-card">
            <Paragraph style={{ marginBottom: 0 }}>
              运行检索验证，查看相关结果、引用来源与链路编号。
            </Paragraph>
          </Card>
        </Col>
      </Row>

      <Card className="admin-section-card">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <Title level={4} style={{ marginBottom: 4 }}>
              监控指标
            </Title>
            <Paragraph type="secondary" style={{ marginBottom: 0 }}>
              基于结构化日志聚合生成，支持 1h / 24h / 7d 时间窗。
            </Paragraph>
          </div>
          <Segmented<MetricsRange>
            options={[
              { label: '1h', value: '1h' },
              { label: '24h', value: '24h' },
              { label: '7d', value: '7d' },
            ]}
            value={metricsRange}
            onChange={(value) => setMetricsRange(value)}
          />
        </div>

        <div className="mt-6 space-y-4">
          {metricsError ? <Alert type="error" showIcon message={metricsError} /> : null}
          {metricsLoading ? (
            <div className="py-12 text-center text-slate-500">监控指标加载中...</div>
          ) : (
            <Row gutter={[16, 16]}>
              <Col xs={24} md={12} xl={6}>
                <TrendMetricCard
                  title="入库成功率"
                  value={formatPercent(latestIngestRate)}
                  helper="终态任务中 completed 的占比"
                  data={ingestSuccessSeries}
                  color="#0f766e"
                />
              </Col>
              <Col xs={24} md={12} xl={6}>
                <TrendMetricCard
                  title="检索请求量"
                  value={String(totalRequests)}
                  helper="当前时间窗内的总检索次数"
                  data={requestCountSeries}
                  color="#1d4ed8"
                />
              </Col>
              <Col xs={24} md={12} xl={6}>
                <TrendMetricCard
                  title="检索 P95"
                  value={`${latestP95} ms`}
                  helper="按时间桶计算的 P95 耗时"
                  data={p95Series}
                  color="#c2410c"
                />
              </Col>
              <Col xs={24} md={12} xl={6}>
                <TrendMetricCard
                  title="空结果率"
                  value={formatPercent(latestEmptyRate)}
                  helper="result_status = no_result 的占比"
                  data={emptyRateSeries}
                  color="#7c3aed"
                />
              </Col>
              <Col xs={24} md={12} xl={6}>
                <TrendMetricCard
                  title="每千次问答成本"
                  value={`$${latestCostPer1K.toFixed(4)}`}
                  helper={`平均上下文 ${latestAvgContextTokens.toFixed(0)} tokens`}
                  data={costSeries}
                  color="#8b5cf6"
                />
              </Col>
              <Col xs={24}>
                <ErrorTopNCard metrics={metrics} />
              </Col>
            </Row>
          )}
        </div>
      </Card>
    </div>
  );
}
