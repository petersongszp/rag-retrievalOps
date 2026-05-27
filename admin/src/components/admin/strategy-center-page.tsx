'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { ReloadOutlined } from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Empty,
  Select,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { KB_ADMIN_API } from '@/config/api';
import apiClient from '@/services/api/client';
import type {
  ListResponse,
  MetricsRange,
  StrategyFlag,
  StrategyGateSummary,
  StrategyImpact,
  StrategyOperationLog,
} from '@/types/kb';

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

function statusColor(status?: string): string {
  switch (status) {
    case 'enabled':
      return 'success';
    case 'shadow':
    case 'canary':
      return 'processing';
    case 'rolling_back':
      return 'warning';
    case 'error':
      return 'error';
    default:
      return 'default';
  }
}

function renderMetric(value?: number, digits = 3) {
  if (value === undefined || value === null) {
    return <Tag color="warning">契约缺口</Tag>;
  }
  return value.toFixed(digits);
}

const impactRanges: MetricsRange[] = ['1h', '24h', '7d'];

export function StrategyCenterPage() {
  const [flags, setFlags] = useState<StrategyFlag[]>([]);
  const [selectedFlagKey, setSelectedFlagKey] = useState<string>('');
  const [selectedRange, setSelectedRange] = useState<MetricsRange>('24h');
  const [impact, setImpact] = useState<StrategyImpact | null>(null);
  const [gates, setGates] = useState<StrategyGateSummary | null>(null);
  const [operations, setOperations] = useState<StrategyOperationLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const selectedFlag = useMemo(
    () => flags.find((item) => item.flag_key === selectedFlagKey) ?? null,
    [flags, selectedFlagKey]
  );

  const operationColumns = useMemo<ColumnsType<StrategyOperationLog>>(
    () => [
      {
        title: 'Operation',
        dataIndex: 'operation',
        key: 'operation',
        width: 140,
        render: (value?: string) => value || <Tag color="warning">契约缺口</Tag>,
      },
      {
        title: 'From',
        dataIndex: 'from_status',
        key: 'from_status',
        width: 120,
        render: (value?: string) => <Tag color={statusColor(value)}>{value || 'n/a'}</Tag>,
      },
      {
        title: 'To',
        dataIndex: 'to_status',
        key: 'to_status',
        width: 120,
        render: (value?: string) => <Tag color={statusColor(value)}>{value || 'n/a'}</Tag>,
      },
      {
        title: 'Reason',
        dataIndex: 'reason',
        key: 'reason',
        ellipsis: true,
      },
      {
        title: 'Created At',
        dataIndex: 'created_at',
        key: 'created_at',
        width: 180,
      },
    ],
    []
  );

  const loadStrategyCenter = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const flagResponse = (await apiClient.get(KB_ADMIN_API.LIST_STRATEGY_FLAGS)) as {
        items?: StrategyFlag[];
      };
      const nextFlags = flagResponse.items ?? [];
      setFlags(nextFlags);

      const activeFlagKey = selectedFlagKey || nextFlags[0]?.flag_key || '';
      setSelectedFlagKey(activeFlagKey);

      if (!activeFlagKey) {
        setImpact(null);
        setGates(null);
        setOperations([]);
        return;
      }

      const [impactResponse, gateResponse, operationResponse] = await Promise.all([
        apiClient.get(KB_ADMIN_API.GET_STRATEGY_IMPACT, {
          params: { flag_key: activeFlagKey, range: selectedRange },
        }) as Promise<StrategyImpact>,
        apiClient.get(KB_ADMIN_API.GET_STRATEGY_GATES, {
          params: { flag_key: activeFlagKey },
        }) as Promise<StrategyGateSummary>,
        apiClient.get(KB_ADMIN_API.LIST_STRATEGY_OPERATIONS, {
          params: { flag_key: activeFlagKey, page: 1, page_size: 5 },
        }) as Promise<ListResponse<StrategyOperationLog>>,
      ]);

      setImpact(impactResponse);
      setGates(gateResponse);
      setOperations(operationResponse.items ?? []);
    } catch (loadError) {
      setError(normalizeError(loadError, '加载策略中心失败'));
      setImpact(null);
      setGates(null);
      setOperations([]);
    } finally {
      setLoading(false);
    }
  }, [selectedFlagKey, selectedRange]);

  useEffect(() => {
    void loadStrategyCenter();
  }, [loadStrategyCenter]);

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            策略中心
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            先把 Phase 3
            策略的契约、路由和基础观测面连起来，后续阶段再继续补灰度编辑、版本对比和更细的调试视图。
          </Paragraph>
        </div>
        <Space wrap>
          <Link href="/evaluation/runs">
            <Button>查看评测运行</Button>
          </Link>
          <Button icon={<ReloadOutlined />} onClick={() => void loadStrategyCenter()}>
            刷新
          </Button>
        </Space>
      </div>

      <Card>
        <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_200px]">
          <div>
            <Text type="secondary">Strategy Flag</Text>
            <Select
              className="mt-2 w-full"
              value={selectedFlagKey || undefined}
              placeholder="选择一个策略开关"
              options={flags.map((item) => ({
                label: item.label || item.flag_key,
                value: item.flag_key,
              }))}
              onChange={(value) => setSelectedFlagKey(value)}
            />
          </div>
          <div>
            <Text type="secondary">Range</Text>
            <Select
              className="mt-2 w-full"
              value={selectedRange}
              options={impactRanges.map((value) => ({ label: value, value }))}
              onChange={(value) => setSelectedRange(value)}
            />
          </div>
        </div>
      </Card>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      {loading ? (
        <Card>
          <div className="flex justify-center py-10">
            <Spin />
          </div>
        </Card>
      ) : !selectedFlag ? (
        <Card>
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有可展示的策略开关。" />
        </Card>
      ) : (
        <>
          <div className="grid gap-4 xl:grid-cols-3">
            <Card title="当前状态">
              <Descriptions column={1} size="small" bordered>
                <Descriptions.Item label="Flag">{selectedFlag.flag_key}</Descriptions.Item>
                <Descriptions.Item label="Label">
                  {selectedFlag.label || <Tag color="warning">契约缺口</Tag>}
                </Descriptions.Item>
                <Descriptions.Item label="Status">
                  <Tag color={statusColor(selectedFlag.status)}>
                    {selectedFlag.status || 'unknown'}
                  </Tag>
                </Descriptions.Item>
                <Descriptions.Item label="Rollout">
                  {selectedFlag.rollout_percentage ?? <Tag color="warning">契约缺口</Tag>}
                </Descriptions.Item>
                <Descriptions.Item label="Version">
                  {selectedFlag.strategy_version || <Tag color="warning">契约缺口</Tag>}
                </Descriptions.Item>
              </Descriptions>
            </Card>

            <Card title="Impact 摘要">
              {impact ? (
                <Descriptions column={1} size="small" bordered>
                  <Descriptions.Item label="Sample Size">
                    {impact.sample_size ?? <Tag color="warning">契约缺口</Tag>}
                  </Descriptions.Item>
                  <Descriptions.Item label="Rewrite Gain">
                    {renderMetric(impact.rewrite_gain)}
                  </Descriptions.Item>
                  <Descriptions.Item label="Parent Fill Gain">
                    {renderMetric(impact.parent_fill_gain)}
                  </Descriptions.Item>
                  <Descriptions.Item label="P95 Latency Delta">
                    {renderMetric(impact.p95_latency_delta_ms, 0)}
                  </Descriptions.Item>
                  <Descriptions.Item label="Sample Too Small">
                    {impact.sample_size_too_small ? (
                      <Tag color="warning">true</Tag>
                    ) : (
                      <Tag color="success">false</Tag>
                    )}
                  </Descriptions.Item>
                </Descriptions>
              ) : (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有 impact 数据。" />
              )}
            </Card>

            <Card title="Gate 摘要">
              {gates ? (
                <Descriptions column={1} size="small" bordered>
                  <Descriptions.Item label="Gate Status">
                    <Tag color={gates.passed ? 'success' : 'warning'}>
                      {gates.gate_status || 'unknown'}
                    </Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="Passed">
                    {gates.passed ? (
                      <Tag color="success">true</Tag>
                    ) : (
                      <Tag color="error">false</Tag>
                    )}
                  </Descriptions.Item>
                  <Descriptions.Item label="Last Eval Run">
                    {gates.last_eval_run_id || <Tag color="warning">契约缺口</Tag>}
                  </Descriptions.Item>
                  <Descriptions.Item label="Failed Rules">
                    {gates.failed_rules?.length ? (
                      gates.failed_rules.join(', ')
                    ) : (
                      <Tag color="default">none</Tag>
                    )}
                  </Descriptions.Item>
                </Descriptions>
              ) : (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有 gate 数据。" />
              )}
            </Card>
          </div>

          {impact?.contract_gaps?.length ? (
            <Alert
              type="warning"
              showIcon
              message="Impact 存在契约缺口"
              description={impact.contract_gaps.join(', ')}
            />
          ) : null}

          <Card title="最近操作">
            {operations.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有策略操作日志。" />
            ) : (
              <Table<StrategyOperationLog>
                rowKey="id"
                size="small"
                columns={operationColumns}
                dataSource={operations}
                pagination={false}
              />
            )}
          </Card>
        </>
      )}
    </div>
  );
}
