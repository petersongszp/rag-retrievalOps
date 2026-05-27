'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import dayjs from 'dayjs';
import { ArrowRightOutlined, ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Empty, List, Space, Statistic, Tag, Typography } from 'antd';
import { KB_ADMIN_API } from '@/config/api';
import apiClient from '@/services/api/client';
import type { EvalReport, EvalRun, ListResponse } from '@/types/kb';

const { Title, Paragraph, Text } = Typography;

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

function metricColor(value: number) {
  if (value > 0) {
    return '#16a34a';
  }
  if (value < 0) {
    return '#dc2626';
  }
  return undefined;
}

export function QualityMonitorPage() {
  const [latestRun, setLatestRun] = useState<EvalRun | null>(null);
  const [latestReport, setLatestReport] = useState<EvalReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadLatestReport = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const runsResponse = (await apiClient.get(KB_ADMIN_API.LIST_EVAL_RUNS, {
        params: { status: 'succeeded', page: 1, page_size: 1 },
      })) as ListResponse<EvalRun>;

      const run = runsResponse.items?.[0];
      if (!run) {
        setLatestRun(null);
        setLatestReport(null);
        return;
      }

      const reportResponse = (await apiClient.get(KB_ADMIN_API.GET_EVAL_REPORT(run.run_id))) as EvalReport;
      setLatestRun(run);
      setLatestReport(reportResponse);
    } catch (loadError) {
      setLatestRun(null);
      setLatestReport(null);
      setError(normalizeError(loadError, '加载质量监控摘要失败'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadLatestReport();
  }, [loadLatestReport]);

  const quickChecks = useMemo(() => latestReport?.gate.checks.slice(0, 4) ?? [], [latestReport]);
  const quickContribution = useMemo(
    () => latestReport?.contribution.slice(0, 3) ?? [],
    [latestReport]
  );

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            质量监控
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            展示最近一次成功评测的核心质量摘要。更完整的运行管理和失败样本分析继续在 Evaluation 模块中完成。
          </Paragraph>
        </div>
        <Button icon={<ReloadOutlined />} onClick={() => void loadLatestReport()}>
          刷新
        </Button>
      </div>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      {loading ? (
        <Card loading />
      ) : !latestRun || !latestReport ? (
        <Card>
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有成功的评测报告可供展示" />
        </Card>
      ) : (
        <>
          <Card
            title="最近一次成功报告"
            extra={
              <Link href={`/evaluation/reports/${latestRun.run_id}`}>
                <Button type="primary" icon={<ArrowRightOutlined />}>
                  查看完整报告
                </Button>
              </Link>
            }
          >
            <Space direction="vertical" size="middle" className="w-full">
              <Space size="large" wrap>
                <div>
                  <Text type="secondary">Run ID</Text>
                  <div>
                    <Text code>{latestRun.run_id}</Text>
                  </div>
                </div>
                <div>
                  <Text type="secondary">Generated At</Text>
                  <div>{formatTime(latestReport.generated_at)}</div>
                </div>
                <div>
                  <Text type="secondary">Baseline</Text>
                  <div>{latestReport.baseline}</div>
                </div>
                <div>
                  <Text type="secondary">Candidate</Text>
                  <div>{latestReport.candidate}</div>
                </div>
                <div>
                  <Text type="secondary">Gate</Text>
                  <div>
                    <Tag color={latestReport.gate.passed ? 'success' : 'error'}>
                      {latestReport.gate.passed ? 'passed' : 'failed'}
                    </Tag>
                  </div>
                </div>
              </Space>
            </Space>
          </Card>

          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <Card>
              <Statistic
                title="Recall@K Delta"
                value={latestReport.comparison.recall_delta}
                precision={4}
                valueStyle={{ color: metricColor(latestReport.comparison.recall_delta) }}
              />
            </Card>
            <Card>
              <Statistic
                title="MRR Delta"
                value={latestReport.comparison.mrr_delta}
                precision={4}
                valueStyle={{ color: metricColor(latestReport.comparison.mrr_delta) }}
              />
            </Card>
            <Card>
              <Statistic
                title="nDCG Delta"
                value={latestReport.comparison.ndcg_delta}
                precision={4}
                valueStyle={{ color: metricColor(latestReport.comparison.ndcg_delta) }}
              />
            </Card>
            <Card>
              <Statistic
                title="Citation Delta"
                value={latestReport.comparison.citation_accuracy_delta}
                precision={4}
                valueStyle={{ color: metricColor(latestReport.comparison.citation_accuracy_delta) }}
              />
            </Card>
            <Card>
              <Statistic
                title="P95 Latency Delta"
                value={latestReport.comparison.p95_latency_delta_ms}
                precision={0}
                suffix="ms"
                valueStyle={{ color: metricColor(-latestReport.comparison.p95_latency_delta_ms) }}
              />
            </Card>
            <Card>
              <Statistic
                title="P95 Latency Ratio"
                value={latestReport.comparison.p95_latency_delta_ratio}
                precision={4}
                valueStyle={{ color: metricColor(-latestReport.comparison.p95_latency_delta_ratio) }}
              />
            </Card>
            <Card>
              <Statistic title="Dataset Size" value={latestReport.dataset_size} precision={0} />
            </Card>
            <Card>
              <Statistic title="Gate Checks" value={latestReport.gate.checks.length} precision={0} />
            </Card>
          </div>

          <div className="grid gap-4 xl:grid-cols-2">
            <Card title="Gate 摘要">
              {quickChecks.length === 0 ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前报告没有 gate 细项" />
              ) : (
                <List
                  dataSource={quickChecks}
                  renderItem={(item) => (
                    <List.Item>
                      <Space direction="vertical" size={2}>
                        <Space>
                          <Text strong>{item.name}</Text>
                          <Tag color={item.passed ? 'success' : 'error'}>
                            {item.passed ? 'passed' : 'failed'}
                          </Tag>
                        </Space>
                        <Text type="secondary">
                          actual={item.actual} expected={item.expected}
                        </Text>
                        <Text>{item.message}</Text>
                      </Space>
                    </List.Item>
                  )}
                />
              )}
            </Card>

            <Card title="贡献摘要">
              {quickContribution.length === 0 ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前报告没有贡献分析数据" />
              ) : (
                <List
                  dataSource={quickContribution}
                  renderItem={(item) => (
                    <List.Item>
                      <Space direction="vertical" size={2}>
                        <Text strong>
                          {item.strategy} vs {item.compared_to}
                        </Text>
                        <Space wrap>
                          <Text>Recall {item.recall_delta.toFixed(4)}</Text>
                          <Text>MRR {item.mrr_delta.toFixed(4)}</Text>
                          <Text>nDCG {item.ndcg_delta.toFixed(4)}</Text>
                          <Text>Citation {item.citation_accuracy_delta.toFixed(4)}</Text>
                          <Text>P95 {item.p95_latency_delta_ms.toFixed(0)} ms</Text>
                        </Space>
                      </Space>
                    </List.Item>
                  )}
                />
              )}
            </Card>
          </div>
        </>
      )}
    </div>
  );
}
