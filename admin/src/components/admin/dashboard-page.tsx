'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import {
  AlertOutlined,
  BellOutlined,
  ClockCircleOutlined,
  FileAddOutlined,
  FolderOpenOutlined,
  KeyOutlined,
  ReloadOutlined,
  SearchOutlined,
  SyncOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { Alert, Button, Card, Col, Row, Segmented, Space, Statistic, Typography } from 'antd';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';
import type {
  MetricsOverview,
  MetricsOverviewBucketCount,
  MetricsOverviewBucketP95,
  MetricsOverviewBucketRate,
  MetricsOverviewCostBreakdown,
  MetricsRange,
} from '@/types/kb';
import { useKnowledgeBaseContext } from './knowledge-base-provider';
import { ActionEmpty } from './ui/action-empty';
import { InlineHelp } from './ui/inline-help';
import { MetricCard } from './ui/metric-card';
import { PageHeader } from './ui/page-header';
import { StatusBadge } from './ui/status-badge';

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

type TodoItem = {
  key: string;
  title: string;
  description: string;
  href: string;
  tone: 'warning' | 'error' | 'processing';
};

type QuickAction = {
  key: string;
  title: string;
  description: string;
  href: string;
  icon: React.ReactNode;
};

const EMPTY_RATE_THRESHOLD = 0.15;
const P95_THRESHOLD_MS = 1500;

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

function formatDuration(value: number) {
  if (value >= 1000) {
    return `${(value / 1000).toFixed(1)} 秒`;
  }
  return `${Math.round(value)} 毫秒`;
}

function formatRefreshTime(value: string | null) {
  if (!value) {
    return '尚未刷新';
  }
  return new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    month: '2-digit',
    day: '2-digit',
  }).format(new Date(value));
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
        暂无趋势数据
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
    <Card styles={{ body: { padding: 20 } }} className="h-full admin-section-card">
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

function RiskDistributionCard({ metrics }: { metrics: MetricsOverview | null }) {
  const items = metrics?.error_type_topn ?? [];

  return (
    <Card title="风险分布" className="h-full admin-section-card">
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
  const [statsLoading, setStatsLoading] = useState(true);
  const [statsError, setStatsError] = useState<string | null>(null);
  const [statsPermissionDenied, setStatsPermissionDenied] = useState(false);

  const [metricsRange, setMetricsRange] = useState<MetricsRange>('24h');
  const [metrics, setMetrics] = useState<MetricsOverview | null>(null);
  const [metricsLoading, setMetricsLoading] = useState(false);
  const [metricsError, setMetricsError] = useState<string | null>(null);

  const [refreshing, setRefreshing] = useState(false);
  const [lastRefreshedAt, setLastRefreshedAt] = useState<string | null>(null);

  const jobsHref = selectedBase ? `/knowledge-bases/${selectedBase.id}` : '/knowledge-bases';
  const uploadHref = selectedBase ? `/knowledge-bases/${selectedBase.id}` : '/knowledge-bases';

  const loadStats = async () => {
    try {
      setStatsLoading(true);
      setStatsError(null);
      setStatsPermissionDenied(false);
      const data = (await apiClient.get(KB_ADMIN_API.DASHBOARD_STATS)) as DashboardStats;
      setStats(data);
    } catch (error) {
      const nextError = error instanceof Error ? error.message : '加载工作台统计失败';
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
      setStats(null);
    } finally {
      setStatsLoading(false);
    }
  };

  const loadMetrics = async (range: MetricsRange) => {
    try {
      setMetricsLoading(true);
      setMetricsError(null);
      const data = (await apiClient.get(KB_ADMIN_API.METRICS_OVERVIEW, {
        params: {
          range,
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

  const refreshAll = async (range: MetricsRange = metricsRange) => {
    try {
      setRefreshing(true);
      await Promise.all([loadStats(), loadMetrics(range)]);
      setLastRefreshedAt(new Date().toISOString());
    } finally {
      setRefreshing(false);
    }
  };

  useEffect(() => {
    void loadStats();
  }, []);

  useEffect(() => {
    void loadMetrics(metricsRange);
    setLastRefreshedAt(new Date().toISOString());
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [metricsRange, selectedBase?.id]);

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
  const hasAnyData = Boolean(stats || metrics);

  const systemStatus = useMemo(() => {
    if (statsError || metricsError || statsPermissionDenied) {
      return {
        tone: 'error' as const,
        title: '存在阻塞问题',
        description: '部分关键数据暂时无法读取，请优先检查权限或接口状态。',
      };
    }

    if ((stats?.failed_job_count ?? 0) > 0 || latestP95 > P95_THRESHOLD_MS || latestEmptyRate > EMPTY_RATE_THRESHOLD) {
      return {
        tone: 'warning' as const,
        title: '需要关注风险',
        description: '工作台发现失败任务、空结果率或响应耗时存在异常波动。',
      };
    }

    if (statsLoading || metricsLoading) {
      return {
        tone: 'processing' as const,
        title: '正在刷新状态',
        description: '平台正在同步最新统计和趋势指标。',
      };
    }

    return {
      tone: 'success' as const,
      title: '系统运行正常',
      description: '当前入库、检索和治理指标整体稳定。',
    };
  }, [latestEmptyRate, latestP95, metricsError, metricsLoading, stats?.failed_job_count, statsError, statsLoading, statsPermissionDenied]);

  const todoItems = useMemo<TodoItem[]>(() => {
    const items: TodoItem[] = [];

    if ((stats?.failed_job_count ?? 0) > 0) {
      items.push({
        key: 'failed-jobs',
        title: '处理失败入库任务',
        description: `当前有 ${stats?.failed_job_count ?? 0} 个失败入库任务，建议优先查看原因并重试。`,
        href: jobsHref,
        tone: 'error',
      });
    }

    if (latestEmptyRate > EMPTY_RATE_THRESHOLD) {
      items.push({
        key: 'empty-rate',
        title: '检查空结果率',
        description: `当前空结果率为 ${formatPercent(latestEmptyRate)}，建议进入检索调优确认召回情况。`,
        href: '/retrieval-lab',
        tone: 'warning',
      });
    }

    if (latestP95 > P95_THRESHOLD_MS) {
      items.push({
        key: 'p95',
        title: '检查响应耗时',
        description: `当前 P95 响应耗时为 ${formatDuration(latestP95)}，建议查看链路与成本情况。`,
        href: '/trace-logs/retrieval',
        tone: 'warning',
      });
    }

    if ((metrics?.error_type_topn?.length ?? 0) > 0) {
      const topError = metrics?.error_type_topn?.[0];
      items.push({
        key: 'error-types',
        title: '查看高频错误类型',
        description: `当前最高频错误为 ${topError?.error_code ?? '未知错误'}，建议进入告警或链路追踪继续定位。`,
        href: '/alerts',
        tone: 'processing',
      });
    }

    return items;
  }, [jobsHref, latestEmptyRate, latestP95, metrics?.error_type_topn, stats?.failed_job_count]);

  const quickActions = useMemo<QuickAction[]>(
    () => [
      {
        key: 'create-kb',
        title: '新建知识库',
        description: '新增一个知识库，开始沉淀业务文档。',
        href: '/knowledge-bases',
        icon: <FolderOpenOutlined />,
      },
      {
        key: 'upload-docs',
        title: '上传文档',
        description: selectedBase ? `继续为 ${selectedBase.name} 上传文档。` : '进入知识库后上传文档并触发入库。',
        href: uploadHref,
        icon: <FileAddOutlined />,
      },
      {
        key: 'retrieval-verify',
        title: '开始检索验证',
        description: '输入真实问题，查看结果、引用来源和链路编号。',
        href: '/retrieval-lab',
        icon: <SearchOutlined />,
      },
      {
        key: 'quality-report',
        title: '查看质量报告',
        description: '查看最近的质量趋势与评测结论。',
        href: '/quality-monitor',
        icon: <BellOutlined />,
      },
      {
        key: 'create-api-key',
        title: '创建接入密钥',
        description: '为应用接入准备新的调用凭据。',
        href: '/api-keys',
        icon: <KeyOutlined />,
      },
    ],
    [selectedBase?.name, uploadHref]
  );

  return (
    <div className="admin-page">
      <PageHeader
        title="工作台"
        subtitle={`当前知识库：${selectedBase?.name ?? '未选择'}。从这里查看系统状态、待处理事项和下一步动作。`}
        extra={
          <>
            <Text className="admin-subtle-text">最近刷新：{formatRefreshTime(lastRefreshedAt)}</Text>
            <Button
              icon={<ReloadOutlined />}
              loading={refreshing}
              onClick={() => void refreshAll(metricsRange)}
            >
              刷新数据
            </Button>
          </>
        }
      />

      {isPermissionDenied ? (
        <Alert
          type="error"
          showIcon
          message="权限不足"
          description="当前账号无权访问知识库列表（403）。请联系管理员确认权限配置。"
        />
      ) : null}

      {statsError && !statsPermissionDenied ? <Alert type="error" showIcon message={statsError} /> : null}
      {metricsError ? <Alert type="warning" showIcon message={metricsError} /> : null}

      {!hasAnyData && !statsLoading && !metricsLoading ? (
        <Card className="admin-section-card">
          <ActionEmpty
            title="工作台还没有可展示的数据"
            description="建议先创建知识库并上传文档，随后再开始检索验证和质量评测。"
            action={
              <Button type="primary" onClick={() => router.push('/knowledge-bases')}>
                去创建知识库
              </Button>
            }
          />
        </Card>
      ) : null}

      <Card className="admin-section-card">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <Space direction="vertical" size={8}>
            <Space size={10} align="center">
              <StatusBadge status={systemStatus.tone} label={systemStatus.title} />
              <Text className="admin-subtle-text">{systemStatus.description}</Text>
            </Space>
            <Space wrap size={12}>
              <Text className="admin-subtle-text">
                <SyncOutlined /> 入库成功率 {formatPercent(latestIngestRate)}
              </Text>
              <Text className="admin-subtle-text">
                <ClockCircleOutlined /> P95 响应耗时 {formatDuration(latestP95)}
              </Text>
              <Text className="admin-subtle-text">
                <AlertOutlined /> 空结果率 {formatPercent(latestEmptyRate)}
              </Text>
            </Space>
          </Space>
          <StatusBadge
            status={(stats?.failed_job_count ?? 0) > 0 ? 'error' : 'success'}
            label={(stats?.failed_job_count ?? 0) > 0 ? '存在失败入库任务' : '暂无失败入库任务'}
          />
        </div>
      </Card>

      <Row gutter={[16, 16]}>
        <Col xs={12} md={6}>
          <MetricCard
            label="知识库数量"
            value={<Statistic value={stats?.kb_count ?? 0} loading={statsLoading} />}
            helper="查看已接入的知识库资产"
            onClick={() => router.push('/knowledge-bases')}
          />
        </Col>
        <Col xs={12} md={6}>
          <MetricCard
            label="文档总数"
            value={<Statistic value={stats?.document_count ?? 0} loading={statsLoading} />}
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
                loading={statsLoading}
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
                loading={statsLoading}
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
        <Col xs={24} xl={12}>
          <Card title="待处理事项" className="admin-section-card">
            {todoItems.length === 0 ? (
              <ActionEmpty
                title="当前没有待处理风险"
                description="系统状态稳定，可以继续进行检索验证或查看趋势变化。"
                action={
                  <Button type="link" onClick={() => router.push('/retrieval-lab')}>
                    去做检索验证
                  </Button>
                }
              />
            ) : (
              <Space direction="vertical" size={12} className="w-full">
                {todoItems.map((item) => (
                  <div
                    key={item.key}
                    className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-3"
                  >
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <Space direction="vertical" size={6}>
                        <StatusBadge status={item.tone} label={item.title} />
                        <Text>{item.description}</Text>
                      </Space>
                      <Button type="link" onClick={() => router.push(item.href)}>
                        立即处理
                      </Button>
                    </div>
                  </div>
                ))}
              </Space>
            )}
          </Card>
        </Col>
        <Col xs={24} xl={12}>
          <Card title="快捷操作" className="admin-section-card">
            <div className="grid gap-3 md:grid-cols-2">
              {quickActions.map((action) => (
                <button
                  key={action.key}
                  type="button"
                  className="rounded-lg border border-slate-200 bg-slate-50 px-4 py-4 text-left transition hover:border-blue-200 hover:bg-blue-50"
                  onClick={() => router.push(action.href)}
                >
                  <Space direction="vertical" size={8} className="w-full">
                    <Space size={10}>
                      <span className="rounded-md bg-white p-2 text-blue-600 shadow-sm">{action.icon}</span>
                      <Text strong>{action.title}</Text>
                    </Space>
                    <Text className="admin-subtle-text">{action.description}</Text>
                  </Space>
                </button>
              ))}
            </div>
          </Card>
        </Col>
      </Row>

      <Card className="admin-section-card">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <Title level={4} style={{ marginBottom: 4 }}>
              趋势与风险
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
          {metricsLoading ? (
            <div className="py-12 text-center text-slate-500">监控指标加载中...</div>
          ) : (
            <Row gutter={[16, 16]}>
              <Col xs={24} md={12} xl={8}>
                <TrendMetricCard
                  title="入库成功率"
                  value={formatPercent(latestIngestRate)}
                  helper="终态任务中 completed 的占比"
                  data={ingestSuccessSeries}
                  color="#0f766e"
                />
              </Col>
              <Col xs={24} md={12} xl={8}>
                <TrendMetricCard
                  title="检索请求量"
                  value={String(totalRequests)}
                  helper="当前时间窗内的总检索次数"
                  data={requestCountSeries}
                  color="#1d4ed8"
                />
              </Col>
              <Col xs={24} md={12} xl={8}>
                <TrendMetricCard
                  title="P95 响应耗时"
                  value={formatDuration(latestP95)}
                  helper="按时间桶统计的 95 分位响应耗时"
                  data={p95Series}
                  color="#c2410c"
                />
              </Col>
              <Col xs={24} md={12} xl={8}>
                <TrendMetricCard
                  title="空结果率"
                  value={formatPercent(latestEmptyRate)}
                  helper="检索结果为空的请求占比"
                  data={emptyRateSeries}
                  color="#7c3aed"
                />
              </Col>
              <Col xs={24} md={12} xl={8}>
                <TrendMetricCard
                  title="每千次问答成本"
                  value={`$${latestCostPer1K.toFixed(4)}`}
                  helper={`平均上下文 ${latestAvgContextTokens.toFixed(0)} tokens`}
                  data={costSeries}
                  color="#8b5cf6"
                />
              </Col>
              <Col xs={24} md={12} xl={8}>
                <RiskDistributionCard metrics={metrics} />
              </Col>
            </Row>
          )}
        </div>
      </Card>
    </div>
  );
}
