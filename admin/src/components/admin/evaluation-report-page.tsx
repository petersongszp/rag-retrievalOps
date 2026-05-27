'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import dayjs from 'dayjs';
import {
  ArrowLeftOutlined,
  DownloadOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Empty,
  Form,
  Input,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { KB_ADMIN_API } from '@/config/api';
import apiClient from '@/services/api/client';
import type {
  EvalFailureCase,
  EvalFailureReason,
  EvalGateCheck,
  EvalReport,
  EvalRun,
  EvalStrategyDelta,
  EvalStrategyResult,
  ListResponse,
} from '@/types/kb';

const { Title, Paragraph, Text } = Typography;

const PAGE_SIZE = 10;

type FailureFilterValues = {
  failure_reason?: EvalFailureReason;
  query_type?: string;
  tag?: string;
};

type MetricRow = {
  key: string;
  metric: string;
  baseline?: number;
  candidate?: number;
  delta?: number;
  deltaRatio?: number;
  unit?: 'ms';
  inverse?: boolean;
};

const failureReasonOptions: Array<{ label: string; value: EvalFailureReason }> = [
  { label: 'recall_miss', value: 'recall_miss' },
  { label: 'citation_miss', value: 'citation_miss' },
  { label: 'mrr_drop', value: 'mrr_drop' },
  { label: 'ndcg_drop', value: 'ndcg_drop' },
  { label: 'latency_regression', value: 'latency_regression' },
  { label: 'gate_failed', value: 'gate_failed' },
  { label: 'trace_missing', value: 'trace_missing' },
];

function normalizeError(error: unknown, fallback: string): string {
  if (
    error &&
    typeof error === 'object' &&
    'message' in error &&
    typeof error.message === 'string'
  ) {
    return error.message;
  }

  return fallback;
}

function formatTime(value?: string): string {
  if (!value) {
    return '-';
  }
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : value;
}

function formatMetric(value?: number, digits = 4): string {
  if (typeof value !== 'number' || Number.isNaN(value)) {
    return '-';
  }
  return value.toFixed(digits);
}

function formatLatency(value?: number): string {
  if (typeof value !== 'number' || Number.isNaN(value)) {
    return '-';
  }
  return `${value.toFixed(0)} ms`;
}

function formatDelta(
  value?: number,
  options?: { digits?: number; suffix?: string; inverse?: boolean }
) {
  const digits = options?.digits ?? 4;
  const suffix = options?.suffix ?? '';
  const inverse = options?.inverse ?? false;

  if (typeof value !== 'number' || Number.isNaN(value)) {
    return <Text type="secondary">-</Text>;
  }

  const positive = value > 0;
  const negative = value < 0;
  const type = inverse
    ? positive
      ? 'danger'
      : negative
        ? 'success'
        : undefined
    : positive
      ? 'success'
      : negative
        ? 'danger'
        : undefined;

  return <Text type={type}>{`${positive ? '+' : ''}${value.toFixed(digits)}${suffix}`}</Text>;
}

function gateColor(passed: boolean): string {
  return passed ? 'success' : 'error';
}

function failureReasonColor(reason: EvalFailureReason): string {
  switch (reason) {
    case 'gate_failed':
      return 'error';
    case 'latency_regression':
      return 'volcano';
    case 'trace_missing':
      return 'gold';
    default:
      return 'warning';
  }
}

function buildFailureParams(filters: FailureFilterValues, page: number) {
  return {
    page,
    page_size: PAGE_SIZE,
    ...(filters.failure_reason ? { failure_reason: filters.failure_reason } : {}),
    ...(filters.query_type?.trim() ? { query_type: filters.query_type.trim() } : {}),
    ...(filters.tag?.trim() ? { tag: filters.tag.trim() } : {}),
  };
}

function pickStrategyResult(report: EvalReport | null, strategyName?: string): EvalStrategyResult | undefined {
  if (!report || !strategyName) {
    return undefined;
  }
  return report.results.find((item) => item.strategy.name === strategyName);
}

function normalizeFailureResponse(
  payload: ListResponse<EvalFailureCase> | EvalFailureCase[]
): ListResponse<EvalFailureCase> {
  if (Array.isArray(payload)) {
    return {
      items: payload,
      total: payload.length,
      page: 1,
      page_size: payload.length,
    };
  }
  return payload;
}

type EvaluationReportPageProps = {
  runId: string;
};

export function EvaluationReportPage({ runId }: EvaluationReportPageProps) {
  const [messageApi, contextHolder] = message.useMessage();
  const [failureForm] = Form.useForm<FailureFilterValues>();

  const [run, setRun] = useState<EvalRun | null>(null);
  const [report, setReport] = useState<EvalReport | null>(null);
  const [pageLoading, setPageLoading] = useState(true);
  const [pageError, setPageError] = useState<string | null>(null);

  const [failureFilters, setFailureFilters] = useState<FailureFilterValues>({});
  const [failureCases, setFailureCases] = useState<EvalFailureCase[]>([]);
  const [failureTotal, setFailureTotal] = useState(0);
  const [failurePage, setFailurePage] = useState(1);
  const [failureLoading, setFailureLoading] = useState(false);
  const [failureError, setFailureError] = useState<string | null>(null);

  const baselineResult = useMemo(() => pickStrategyResult(report, report?.baseline), [report]);
  const candidateResult = useMemo(() => pickStrategyResult(report, report?.candidate), [report]);

  const metricRows = useMemo<MetricRow[]>(() => {
    if (!report) {
      return [];
    }

    return [
      {
        key: 'recall',
        metric: 'Recall@K',
        baseline: baselineResult?.metrics.recall_at_k,
        candidate: candidateResult?.metrics.recall_at_k,
        delta: report.comparison.recall_delta,
      },
      {
        key: 'mrr',
        metric: 'MRR',
        baseline: baselineResult?.metrics.mrr,
        candidate: candidateResult?.metrics.mrr,
        delta: report.comparison.mrr_delta,
      },
      {
        key: 'ndcg',
        metric: 'nDCG',
        baseline: baselineResult?.metrics.ndcg,
        candidate: candidateResult?.metrics.ndcg,
        delta: report.comparison.ndcg_delta,
      },
      {
        key: 'citation',
        metric: 'Citation Accuracy',
        baseline: baselineResult?.metrics.citation_accuracy,
        candidate: candidateResult?.metrics.citation_accuracy,
        delta: report.comparison.citation_accuracy_delta,
      },
      {
        key: 'p50',
        metric: 'P50 Latency',
        baseline: baselineResult?.metrics.p50_latency_ms,
        candidate: candidateResult?.metrics.p50_latency_ms,
        unit: 'ms',
        inverse: true,
      },
      {
        key: 'p95',
        metric: 'P95 Latency',
        baseline: baselineResult?.metrics.p95_latency_ms,
        candidate: candidateResult?.metrics.p95_latency_ms,
        delta: report.comparison.p95_latency_delta_ms,
        deltaRatio: report.comparison.p95_latency_delta_ratio,
        unit: 'ms',
        inverse: true,
      },
      {
        key: 'avg',
        metric: 'Avg Latency',
        baseline: baselineResult?.metrics.avg_latency_ms,
        candidate: candidateResult?.metrics.avg_latency_ms,
        unit: 'ms',
        inverse: true,
      },
    ];
  }, [baselineResult, candidateResult, report]);

  const loadFailureCases = useCallback(
    async (filters: FailureFilterValues, page: number) => {
      try {
        setFailureLoading(true);
        setFailureError(null);

        const response = (await apiClient.get(KB_ADMIN_API.LIST_EVAL_FAILURE_CASES(runId), {
          params: buildFailureParams(filters, page),
        })) as ListResponse<EvalFailureCase> | EvalFailureCase[];

        const normalized = normalizeFailureResponse(response);
        setFailureCases(normalized.items ?? []);
        setFailureTotal(normalized.total ?? 0);
        setFailurePage(normalized.page ?? page);
      } catch (error) {
        setFailureCases([]);
        setFailureTotal(0);
        setFailureError(normalizeError(error, '加载失败样本失败'));
      } finally {
        setFailureLoading(false);
      }
    },
    [runId]
  );

  const loadPage = useCallback(async () => {
    try {
      setPageLoading(true);
      setPageError(null);

      const [runResponse, reportResponse] = await Promise.all([
        apiClient.get(KB_ADMIN_API.GET_EVAL_RUN(runId)) as Promise<EvalRun>,
        apiClient.get(KB_ADMIN_API.GET_EVAL_REPORT(runId)) as Promise<EvalReport>,
      ]);

      setRun(runResponse);
      setReport(reportResponse);
    } catch (error) {
      setRun(null);
      setReport(null);
      setPageError(normalizeError(error, '加载评测报告失败'));
    } finally {
      setPageLoading(false);
    }
  }, [runId]);

  useEffect(() => {
    void loadPage();
    void loadFailureCases({}, 1);
  }, [loadFailureCases, loadPage]);

  const comparisonColumns = useMemo<ColumnsType<MetricRow>>(
    () => [
      { title: '指标', dataIndex: 'metric', key: 'metric', width: 180 },
      {
        title: 'Baseline',
        key: 'baseline',
        render: (_, record) =>
          record.unit === 'ms' ? formatLatency(record.baseline) : formatMetric(record.baseline),
      },
      {
        title: 'Candidate',
        key: 'candidate',
        render: (_, record) =>
          record.unit === 'ms' ? formatLatency(record.candidate) : formatMetric(record.candidate),
      },
      {
        title: 'Delta',
        key: 'delta',
        render: (_, record) =>
          record.unit === 'ms'
            ? formatDelta(record.delta, { digits: 0, suffix: ' ms', inverse: record.inverse })
            : formatDelta(record.delta, { inverse: record.inverse }),
      },
      {
        title: 'Delta Ratio',
        key: 'delta_ratio',
        render: (_, record) =>
          record.deltaRatio !== undefined ? (
            formatDelta(record.deltaRatio, { inverse: record.inverse })
          ) : (
            <Text type="secondary">-</Text>
          ),
      },
    ],
    []
  );

  const contributionColumns = useMemo<ColumnsType<EvalStrategyDelta>>(
    () => [
      { title: 'Strategy', dataIndex: 'strategy', key: 'strategy', width: 180 },
      { title: 'Compared To', dataIndex: 'compared_to', key: 'compared_to', width: 180 },
      {
        title: 'Recall Delta',
        dataIndex: 'recall_delta',
        key: 'recall_delta',
        render: (value: number) => formatDelta(value),
      },
      {
        title: 'MRR Delta',
        dataIndex: 'mrr_delta',
        key: 'mrr_delta',
        render: (value: number) => formatDelta(value),
      },
      {
        title: 'nDCG Delta',
        dataIndex: 'ndcg_delta',
        key: 'ndcg_delta',
        render: (value: number) => formatDelta(value),
      },
      {
        title: 'Citation Delta',
        dataIndex: 'citation_accuracy_delta',
        key: 'citation_accuracy_delta',
        render: (value: number) => formatDelta(value),
      },
      {
        title: 'P95 Latency Delta',
        dataIndex: 'p95_latency_delta_ms',
        key: 'p95_latency_delta_ms',
        render: (value: number) => formatDelta(value, { digits: 0, suffix: ' ms', inverse: true }),
      },
    ],
    []
  );

  const gateColumns = useMemo<ColumnsType<EvalGateCheck>>(
    () => [
      { title: 'Name', dataIndex: 'name', key: 'name', width: 220 },
      {
        title: 'Actual',
        dataIndex: 'actual',
        key: 'actual',
        render: (value: number) => formatMetric(value),
      },
      {
        title: 'Expected',
        dataIndex: 'expected',
        key: 'expected',
        render: (value: number) => formatMetric(value),
      },
      {
        title: 'Passed',
        dataIndex: 'passed',
        key: 'passed',
        width: 120,
        render: (value: boolean) => <Tag color={gateColor(value)}>{value ? 'passed' : 'failed'}</Tag>,
      },
      { title: 'Message', dataIndex: 'message', key: 'message', ellipsis: true },
    ],
    []
  );

  const failureColumns = useMemo<ColumnsType<EvalFailureCase>>(
    () => [
      {
        title: 'Case ID',
        dataIndex: 'case_id',
        key: 'case_id',
        width: 180,
        render: (value: string) => <Text code>{value}</Text>,
      },
      {
        title: 'Query',
        dataIndex: 'query',
        key: 'query',
        ellipsis: true,
      },
      {
        title: 'Reason',
        dataIndex: 'failure_reason',
        key: 'failure_reason',
        width: 160,
        render: (value: EvalFailureReason) => <Tag color={failureReasonColor(value)}>{value}</Tag>,
      },
      {
        title: 'Recall Delta',
        key: 'recall_delta',
        width: 120,
        render: (_, record) => formatDelta(record.delta.recall_delta),
      },
      {
        title: 'MRR Delta',
        key: 'mrr_delta',
        width: 120,
        render: (_, record) => formatDelta(record.delta.mrr_delta),
      },
      {
        title: 'nDCG Delta',
        key: 'ndcg_delta',
        width: 120,
        render: (_, record) => formatDelta(record.delta.ndcg_delta),
      },
      {
        title: 'Citation Delta',
        key: 'citation_delta',
        width: 140,
        render: (_, record) => formatDelta(record.delta.citation_accuracy_delta),
      },
      {
        title: 'Latency Delta',
        key: 'latency_delta_ms',
        width: 140,
        render: (_, record) =>
          formatDelta(record.delta.latency_delta_ms, {
            digits: 0,
            suffix: ' ms',
            inverse: true,
          }),
      },
      {
        title: 'Trace',
        key: 'trace',
        width: 240,
        render: (_, record) => (
          <Space wrap>
            {record.baseline_request_id ? (
              <Link href={`/trace-logs/retrieval?request_id=${record.baseline_request_id}`}>
                <Button size="small">Baseline Trace</Button>
              </Link>
            ) : (
              <Tag color="warning">未生成 baseline trace</Tag>
            )}
            {record.candidate_request_id ? (
              <Link href={`/trace-logs/retrieval?request_id=${record.candidate_request_id}`}>
                <Button size="small">Candidate Trace</Button>
              </Link>
            ) : (
              <Tag color="warning">未生成 candidate trace</Tag>
            )}
          </Space>
        ),
      },
    ],
    []
  );

  const exportReport = (format: 'json' | 'markdown') => {
    if (typeof window === 'undefined') {
      return;
    }

    window.open(KB_ADMIN_API.EXPORT_EVAL_REPORT(runId, format), '_blank', 'noopener,noreferrer');
    messageApi.success(`已发起 ${format.toUpperCase()} 导出`);
  };

  return (
    <div className="space-y-6">
      {contextHolder}

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Space size="small" wrap>
            <Link href="/evaluation/runs">
              <Button icon={<ArrowLeftOutlined />}>返回运行列表</Button>
            </Link>
          </Space>
          <Title level={2} style={{ marginTop: 16, marginBottom: 8 }}>
            评测报告
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            展示 baseline 与 candidate 的离线评测对比、门禁结果、贡献分析以及失败样本下钻。
          </Paragraph>
        </div>
        <Space wrap>
          <Button icon={<ReloadOutlined />} onClick={() => void loadPage()}>
            刷新
          </Button>
          <Button icon={<DownloadOutlined />} onClick={() => exportReport('json')}>
            导出 JSON
          </Button>
          <Button icon={<DownloadOutlined />} onClick={() => exportReport('markdown')}>
            导出 Markdown
          </Button>
        </Space>
      </div>

      {pageError ? <Alert type="error" showIcon message={pageError} /> : null}

      {pageLoading ? (
        <Card loading />
      ) : !report ? (
        <Card>
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前运行还没有可用报告" />
        </Card>
      ) : (
        <>
          <Descriptions bordered size="small" column={2}>
            <Descriptions.Item label="Run ID">{run?.run_id ?? runId}</Descriptions.Item>
            <Descriptions.Item label="Gate">
              <Tag color={gateColor(report.gate.passed)}>{report.gate.passed ? 'passed' : 'failed'}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="Dataset Size">{report.dataset_size}</Descriptions.Item>
            <Descriptions.Item label="Generated At">{formatTime(report.generated_at)}</Descriptions.Item>
            <Descriptions.Item label="Baseline">{report.baseline}</Descriptions.Item>
            <Descriptions.Item label="Candidate">{report.candidate}</Descriptions.Item>
            <Descriptions.Item label="Run Status">
              {run ? <Tag color={run.status === 'succeeded' ? 'success' : 'warning'}>{run.status}</Tag> : <Tag color="warning">Contract gap</Tag>}
            </Descriptions.Item>
            <Descriptions.Item label="Error">
              {run?.error_msg ? run.error_msg : <Text type="secondary">-</Text>}
            </Descriptions.Item>
          </Descriptions>

          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <Card>
              <Statistic title="Recall@K Delta" value={report.comparison.recall_delta} precision={4} />
            </Card>
            <Card>
              <Statistic title="MRR Delta" value={report.comparison.mrr_delta} precision={4} />
            </Card>
            <Card>
              <Statistic title="nDCG Delta" value={report.comparison.ndcg_delta} precision={4} />
            </Card>
            <Card>
              <Statistic title="Citation Delta" value={report.comparison.citation_accuracy_delta} precision={4} />
            </Card>
            <Card>
              <Statistic title="Baseline P50" value={baselineResult?.metrics.p50_latency_ms} precision={0} suffix="ms" />
            </Card>
            <Card>
              <Statistic title="Candidate P50" value={candidateResult?.metrics.p50_latency_ms} precision={0} suffix="ms" />
            </Card>
            <Card>
              <Statistic title="Baseline P95" value={baselineResult?.metrics.p95_latency_ms} precision={0} suffix="ms" />
            </Card>
            <Card>
              <Statistic title="Candidate P95" value={candidateResult?.metrics.p95_latency_ms} precision={0} suffix="ms" />
            </Card>
          </div>

          <Card title="指标对比">
            <Table<MetricRow> rowKey="key" pagination={false} dataSource={metricRows} columns={comparisonColumns} />
          </Card>

          <Card title="贡献分析">
            {report.contribution.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前报告没有贡献分析数据" />
            ) : (
              <Table<EvalStrategyDelta>
                rowKey={(record) => `${record.strategy}-${record.compared_to}`}
                pagination={false}
                dataSource={report.contribution}
                columns={contributionColumns}
              />
            )}
          </Card>

          <Card
            title="Gate 检查"
            extra={<Tag color={gateColor(report.gate.passed)}>{report.gate.passed ? 'passed' : 'failed'}</Tag>}
          >
            <Table<EvalGateCheck> rowKey="name" pagination={false} dataSource={report.gate.checks} columns={gateColumns} />
          </Card>

          <Card title="失败样本">
            <Space direction="vertical" size="large" className="w-full">
              <Form
                form={failureForm}
                layout="vertical"
                onFinish={(values) => {
                  const nextFilters: FailureFilterValues = {
                    failure_reason: values.failure_reason,
                    query_type: values.query_type?.trim() || undefined,
                    tag: values.tag?.trim() || undefined,
                  };
                  setFailureFilters(nextFilters);
                  void loadFailureCases(nextFilters, 1);
                }}
              >
                <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                  <Form.Item label="Failure Reason" name="failure_reason">
                    <Select allowClear placeholder="全部原因" options={failureReasonOptions} />
                  </Form.Item>
                  <Form.Item label="Query Type" name="query_type">
                    <Input placeholder="例如 factual / multi-hop" />
                  </Form.Item>
                  <Form.Item label="Tag" name="tag">
                    <Input placeholder="按单个标签筛选" />
                  </Form.Item>
                </div>
                <Space>
                  <Button type="primary" htmlType="submit" icon={<SearchOutlined />} loading={failureLoading}>
                    查询失败样本
                  </Button>
                  <Button
                    onClick={() => {
                      failureForm.resetFields();
                      const nextFilters = {} as FailureFilterValues;
                      setFailureFilters(nextFilters);
                      void loadFailureCases(nextFilters, 1);
                    }}
                  >
                    重置
                  </Button>
                </Space>
              </Form>

              {failureError ? <Alert type="error" showIcon message={failureError} /> : null}

              {failureCases.length === 0 && !failureLoading ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前筛选条件下没有失败样本" />
              ) : (
                <Table<EvalFailureCase>
                  rowKey={(record) => `${record.case_id}-${record.failure_reason}`}
                  loading={failureLoading}
                  columns={failureColumns}
                  dataSource={failureCases}
                  expandable={{
                    rowExpandable: (record) =>
                      Boolean(record.query_type) ||
                      Boolean(record.tags?.length) ||
                      Boolean(record.baseline_request_id) ||
                      Boolean(record.candidate_request_id),
                    expandedRowRender: (record) => (
                      <Descriptions bordered size="small" column={1}>
                        <Descriptions.Item label="Query Type">{record.query_type || '-'}</Descriptions.Item>
                        <Descriptions.Item label="Tags">
                          {record.tags?.length ? record.tags.join(', ') : '-'}
                        </Descriptions.Item>
                        <Descriptions.Item label="Baseline Metrics">
                          <pre className="mb-0 whitespace-pre-wrap rounded bg-slate-50 p-3 text-xs">
                            {JSON.stringify(record.baseline_metrics, null, 2)}
                          </pre>
                        </Descriptions.Item>
                        <Descriptions.Item label="Candidate Metrics">
                          <pre className="mb-0 whitespace-pre-wrap rounded bg-slate-50 p-3 text-xs">
                            {JSON.stringify(record.candidate_metrics, null, 2)}
                          </pre>
                        </Descriptions.Item>
                      </Descriptions>
                    ),
                  }}
                  pagination={{
                    current: failurePage,
                    pageSize: PAGE_SIZE,
                    total: failureTotal,
                    onChange: (page) => void loadFailureCases(failureFilters, page),
                  }}
                />
              )}
            </Space>
          </Card>
        </>
      )}
    </div>
  );
}
