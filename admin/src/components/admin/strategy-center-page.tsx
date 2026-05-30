'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { ReloadOutlined, RollbackOutlined, SettingOutlined } from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Spin,
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
  ListResponse,
  MetricsRange,
  StrategyFlag,
  StrategyFlagStatus,
  StrategyGateSummary,
  StrategyImpact,
  StrategyOperationLog,
  StrategyRollbackRequest,
  StrategyRollbackResult,
  StrategyVersion,
} from '@/types/kb';

const { Title, Paragraph, Text } = Typography;

const impactRanges: MetricsRange[] = ['1h', '24h', '7d'];
const strategyStatuses: StrategyFlagStatus[] = [
  'enabled',
  'disabled',
  'shadow',
  'canary',
  'rolling_back',
  'error',
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

function statusColor(status?: string): string {
  switch (status) {
    case 'enabled':
      return 'success';
    case 'shadow':
      return 'processing';
    case 'canary':
      return 'orange';
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
    return <Tag color="warning">Contract gap</Tag>;
  }
  return value.toFixed(digits);
}

function renderValue(value?: string | number | boolean | null) {
  if (value === undefined || value === null || value === '') {
    return <Tag color="warning">Contract gap</Tag>;
  }
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }
  return String(value);
}

type EditStrategyFormValues = {
  enabled: boolean;
  status: StrategyFlagStatus;
  rollout_percentage: number;
  reason: string;
};

type RollbackFormValues = {
  reason: string;
};

export function StrategyCenterPage() {
  const [messageApi, contextHolder] = message.useMessage();
  const [editForm] = Form.useForm<EditStrategyFormValues>();
  const [rollbackForm] = Form.useForm<RollbackFormValues>();

  const [flags, setFlags] = useState<StrategyFlag[]>([]);
  const [selectedFlagKey, setSelectedFlagKey] = useState<string>('');
  const [selectedRange, setSelectedRange] = useState<MetricsRange>('24h');

  const [flagsLoading, setFlagsLoading] = useState(true);
  const [flagsError, setFlagsError] = useState<string | null>(null);
  const [overviewOperations, setOverviewOperations] = useState<StrategyOperationLog[]>([]);

  const [impact, setImpact] = useState<StrategyImpact | null>(null);
  const [impactLoading, setImpactLoading] = useState(false);
  const [impactError, setImpactError] = useState<string | null>(null);

  const [gates, setGates] = useState<StrategyGateSummary | null>(null);
  const [gateLoading, setGateLoading] = useState(false);
  const [gateError, setGateError] = useState<string | null>(null);

  const [versions, setVersions] = useState<StrategyVersion[]>([]);
  const [versionsLoading, setVersionsLoading] = useState(false);
  const [versionsError, setVersionsError] = useState<string | null>(null);

  const [operations, setOperations] = useState<StrategyOperationLog[]>([]);
  const [operationsLoading, setOperationsLoading] = useState(false);
  const [operationsError, setOperationsError] = useState<string | null>(null);

  const [editOpen, setEditOpen] = useState(false);
  const [rollbackOpen, setRollbackOpen] = useState(false);
  const [rollbackAll, setRollbackAll] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [lastRollbackResult, setLastRollbackResult] = useState<StrategyRollbackResult | null>(null);

  const selectedFlag = useMemo(
    () => flags.find((item) => item.flag_key === selectedFlagKey) ?? null,
    [flags, selectedFlagKey]
  );

  const enabledCount = useMemo(() => flags.filter((item) => item.enabled).length, [flags]);
  const canaryCount = useMemo(() => flags.filter((item) => item.status === 'canary').length, [flags]);
  const errorCount = useMemo(() => flags.filter((item) => item.status === 'error').length, [flags]);
  const rollbackCount = useMemo(
    () => overviewOperations.filter((item) => item.operation === 'rollback').length,
    [overviewOperations]
  );

  const flagColumns = useMemo<ColumnsType<StrategyFlag>>(
    () => [
      {
        title: '策略',
        dataIndex: 'label',
        key: 'label',
        width: 220,
        render: (_, record) => (
          <Space direction="vertical" size={2} className="min-w-0">
            <Text
              strong
              ellipsis={{ tooltip: record.label || record.flag_key }}
              className="block min-w-0 leading-tight"
            >
              {record.label || record.flag_key}
            </Text>
            <Text type="secondary" className="block text-xs leading-tight" ellipsis>
              {record.flag_key}
            </Text>
          </Space>
        ),
      },
      {
        title: '状态',
        dataIndex: 'status',
        key: 'status',
        width: 110,
        render: (value?: string) => <Tag color={statusColor(value)}>{value || 'unknown'}</Tag>,
      },
      {
        title: '灰度',
        dataIndex: 'rollout_percentage',
        key: 'rollout_percentage',
        width: 90,
        render: (value?: number) => (value === undefined ? <Tag color="warning">gap</Tag> : `${value}%`),
      },
      {
        title: '风险',
        dataIndex: 'risk_level',
        key: 'risk_level',
        width: 90,
        render: (value?: string) => <Tag>{value || 'unknown'}</Tag>,
      },
    ],
    []
  );

  const versionColumns = useMemo<ColumnsType<StrategyVersion>>(
    () => [
      { title: 'Version ID', dataIndex: 'version_id', key: 'version_id', ellipsis: true },
      { title: 'Created By', dataIndex: 'created_by', key: 'created_by', width: 120 },
      {
        title: 'Gate',
        dataIndex: 'gate_status',
        key: 'gate_status',
        width: 120,
        render: (value?: string) => <Tag color={statusColor(value)}>{value || 'pending'}</Tag>,
      },
      { title: 'Created At', dataIndex: 'created_at', key: 'created_at', width: 180 },
    ],
    []
  );

  const operationColumns = useMemo<ColumnsType<StrategyOperationLog>>(
    () => [
      {
        title: 'Operation',
        dataIndex: 'operation',
        key: 'operation',
        width: 120,
        render: (value?: string) => value || <Tag color="warning">Contract gap</Tag>,
      },
      {
        title: 'From',
        dataIndex: 'from_status',
        key: 'from_status',
        width: 110,
        render: (value?: string) => <Tag color={statusColor(value)}>{value || 'n/a'}</Tag>,
      },
      {
        title: 'To',
        dataIndex: 'to_status',
        key: 'to_status',
        width: 110,
        render: (value?: string) => <Tag color={statusColor(value)}>{value || 'n/a'}</Tag>,
      },
      {
        title: 'Reason',
        dataIndex: 'reason',
        key: 'reason',
        ellipsis: true,
      },
      {
        title: 'Time',
        dataIndex: 'created_at',
        key: 'created_at',
        width: 180,
      },
    ],
    []
  );

  const loadFlags = useCallback(async () => {
    try {
      setFlagsLoading(true);
      setFlagsError(null);
      const [flagResponse, operationResponse] = await Promise.all([
        apiClient.get(KB_ADMIN_API.LIST_STRATEGY_FLAGS) as Promise<{ items?: StrategyFlag[] }>,
        apiClient.get(KB_ADMIN_API.LIST_STRATEGY_OPERATIONS, {
          params: { page: 1, page_size: 50 },
        }) as Promise<ListResponse<StrategyOperationLog>>,
      ]);

      const nextFlags = flagResponse.items ?? [];
      setFlags(nextFlags);
      setOverviewOperations(operationResponse.items ?? []);

      if (!selectedFlagKey && nextFlags[0]?.flag_key) {
        setSelectedFlagKey(nextFlags[0].flag_key);
      } else if (selectedFlagKey && !nextFlags.some((item) => item.flag_key === selectedFlagKey)) {
        setSelectedFlagKey(nextFlags[0]?.flag_key ?? '');
      }
    } catch (loadError) {
      setFlagsError(normalizeError(loadError, '加载策略开关失败'));
      setFlags([]);
      setOverviewOperations([]);
    } finally {
      setFlagsLoading(false);
    }
  }, [selectedFlagKey]);

  const loadDetail = useCallback(async (flagKey: string, range: MetricsRange) => {
    if (!flagKey) {
      setImpact(null);
      setGates(null);
      setVersions([]);
      setOperations([]);
      return;
    }

    setImpactLoading(true);
    setImpactError(null);
    setGateLoading(true);
    setGateError(null);
    setVersionsLoading(true);
    setVersionsError(null);
    setOperationsLoading(true);
    setOperationsError(null);

    const [impactResult, gateResult, versionResult, operationResult] = await Promise.allSettled([
      apiClient.get(KB_ADMIN_API.GET_STRATEGY_IMPACT, {
        params: { flag_key: flagKey, range },
      }) as Promise<StrategyImpact>,
      apiClient.get(KB_ADMIN_API.GET_STRATEGY_GATES, {
        params: { flag_key: flagKey },
      }) as Promise<StrategyGateSummary>,
      apiClient.get(KB_ADMIN_API.LIST_STRATEGY_VERSIONS, {
        params: { flag_key: flagKey, page: 1, page_size: 10 },
      }) as Promise<ListResponse<StrategyVersion>>,
      apiClient.get(KB_ADMIN_API.LIST_STRATEGY_OPERATIONS, {
        params: { flag_key: flagKey, page: 1, page_size: 10 },
      }) as Promise<ListResponse<StrategyOperationLog>>,
    ]);

    if (impactResult.status === 'fulfilled') {
      setImpact(impactResult.value);
    } else {
      setImpact(null);
      setImpactError(normalizeError(impactResult.reason, '加载 impact 失败'));
    }
    setImpactLoading(false);

    if (gateResult.status === 'fulfilled') {
      setGates(gateResult.value);
    } else {
      setGates(null);
      setGateError(normalizeError(gateResult.reason, '加载 gate 摘要失败'));
    }
    setGateLoading(false);

    if (versionResult.status === 'fulfilled') {
      setVersions(versionResult.value.items ?? []);
    } else {
      setVersions([]);
      setVersionsError(normalizeError(versionResult.reason, '加载版本列表失败'));
    }
    setVersionsLoading(false);

    if (operationResult.status === 'fulfilled') {
      setOperations(operationResult.value.items ?? []);
    } else {
      setOperations([]);
      setOperationsError(normalizeError(operationResult.reason, '加载操作日志失败'));
    }
    setOperationsLoading(false);
  }, []);

  useEffect(() => {
    void loadFlags();
  }, [loadFlags]);

  useEffect(() => {
    if (selectedFlagKey) {
      void loadDetail(selectedFlagKey, selectedRange);
    }
  }, [loadDetail, selectedFlagKey, selectedRange]);

  const openEditModal = () => {
    if (!selectedFlag) {
      return;
    }
    editForm.setFieldsValue({
      enabled: Boolean(selectedFlag.enabled),
      status: (selectedFlag.status || 'disabled') as StrategyFlagStatus,
      rollout_percentage: selectedFlag.rollout_percentage ?? 0,
      reason: '',
    });
    setEditOpen(true);
  };

  const submitEdit = async () => {
    if (!selectedFlag) {
      return;
    }
    try {
      const values = await editForm.validateFields();
      setActionLoading(true);
      await apiClient.patch(KB_ADMIN_API.UPDATE_STRATEGY_FLAG(selectedFlag.flag_key), {
        enabled: values.enabled,
        status: values.status,
        rollout_percentage: values.rollout_percentage,
        reason: values.reason.trim(),
      });
      messageApi.success('策略已更新');
      setEditOpen(false);
      await loadFlags();
      await loadDetail(selectedFlag.flag_key, selectedRange);
    } catch (submitError) {
      if (submitError instanceof Error && submitError.message) {
        messageApi.error(submitError.message);
      }
    } finally {
      setActionLoading(false);
    }
  };

  const openRollbackModal = (all: boolean) => {
    rollbackForm.resetFields();
    setRollbackAll(all);
    setRollbackOpen(true);
  };

  const submitRollback = async () => {
    if (!selectedFlag && !rollbackAll) {
      return;
    }
    try {
      const values = await rollbackForm.validateFields();
      setActionLoading(true);
      const payload: StrategyRollbackRequest = {
        target_version: 'phase2_baseline',
        flag_keys: rollbackAll ? undefined : [selectedFlag!.flag_key],
        reason: values.reason.trim(),
      };
      const result = (await apiClient.post(
        KB_ADMIN_API.ROLLBACK_STRATEGY,
        payload
      )) as StrategyRollbackResult;
      setLastRollbackResult(result);
      messageApi.success(`回滚完成：${result.rollback_id ?? 'rollback'}`);
      setRollbackOpen(false);
      await loadFlags();
      if (selectedFlagKey) {
        await loadDetail(selectedFlagKey, selectedRange);
      }
    } catch (rollbackError) {
      if (rollbackError instanceof Error && rollbackError.message) {
        messageApi.error(rollbackError.message);
      }
    } finally {
      setActionLoading(false);
    }
  };

  const highRiskDirectEnable =
    selectedFlag?.risk_level === 'high' &&
    !selectedFlag?.enabled &&
    editForm.getFieldValue('enabled') &&
    editForm.getFieldValue('status') !== 'shadow' &&
    editForm.getFieldValue('status') !== 'canary';

  return (
    <div className="space-y-6">
      {contextHolder}

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            策略中心
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            查看 Phase 3 高级检索策略的状态、影响、版本和最小操作日志，并在需要时安全地做灰度或回滚。
          </Paragraph>
        </div>
        <Space wrap>
          <Link href="/evaluation/runs">
            <Button>查看评测运行</Button>
          </Link>
          <Button icon={<ReloadOutlined />} onClick={() => void loadFlags()}>
            刷新
          </Button>
        </Space>
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <Statistic title="启用策略数" value={enabledCount} prefix={<SettingOutlined />} />
        </Card>
        <Card>
          <Statistic title="Canary 策略数" value={canaryCount} />
        </Card>
        <Card>
          <Statistic title="Error 策略数" value={errorCount} />
        </Card>
        <Card>
          <Statistic title="最近回滚次数" value={rollbackCount} prefix={<RollbackOutlined />} />
        </Card>
      </div>

      {flagsError ? (
        <Alert
          type="error"
          showIcon
          message={flagsError}
          action={
            <Button size="small" onClick={() => void loadFlags()}>
              重试
            </Button>
          }
        />
      ) : null}

      <div className="grid gap-6 xl:grid-cols-[minmax(440px,520px)_minmax(0,1fr)]">
        <Card title="Feature Flags">
          {flagsLoading ? (
            <div className="flex justify-center py-10">
              <Spin />
            </div>
          ) : flags.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有可展示的策略开关。" />
          ) : (
            <Table<StrategyFlag>
              rowKey="flag_key"
              className="strategy-flags-table"
              size="small"
              columns={flagColumns}
              dataSource={flags}
              pagination={false}
              scroll={{ x: 560 }}
              rowSelection={{
                type: 'radio',
                selectedRowKeys: selectedFlagKey ? [selectedFlagKey] : [],
                onChange: (selectedKeys) => setSelectedFlagKey(String(selectedKeys[0] ?? '')),
              }}
              onRow={(record) => ({
                onClick: () => setSelectedFlagKey(record.flag_key),
                style: { cursor: 'pointer' },
              })}
            />
          )}
        </Card>

        <Space direction="vertical" size="large" className="w-full">
          {!selectedFlag ? (
            <Card>
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="请选择一个策略查看详情。" />
            </Card>
          ) : (
            <>
              <Card
                title={selectedFlag.label || selectedFlag.flag_key}
                extra={
                  <Space wrap>
                    <Select
                      value={selectedRange}
                      options={impactRanges.map((value) => ({ label: value, value }))}
                      onChange={(value) => setSelectedRange(value)}
                      style={{ width: 120 }}
                    />
                    <Button onClick={openEditModal}>修改策略</Button>
                    <Button onClick={() => openRollbackModal(false)}>回滚当前策略</Button>
                    <Button danger onClick={() => openRollbackModal(true)}>
                      回滚到 Phase2 Baseline
                    </Button>
                  </Space>
                }
              >
                <Descriptions column={2} size="small" bordered>
                  <Descriptions.Item label="flag_key">{selectedFlag.flag_key}</Descriptions.Item>
                  <Descriptions.Item label="label">{renderValue(selectedFlag.label)}</Descriptions.Item>
                  <Descriptions.Item label="status">
                    <Tag color={statusColor(selectedFlag.status)}>{selectedFlag.status || 'unknown'}</Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="enabled">{renderValue(selectedFlag.enabled)}</Descriptions.Item>
                  <Descriptions.Item label="rollout_percentage">
                    {renderValue(selectedFlag.rollout_percentage)}
                  </Descriptions.Item>
                  <Descriptions.Item label="strategy_version">
                    {renderValue(selectedFlag.strategy_version)}
                  </Descriptions.Item>
                  <Descriptions.Item label="risk_level">
                    <Tag color={selectedFlag.risk_level === 'high' ? 'error' : 'default'}>
                      {selectedFlag.risk_level || 'unknown'}
                    </Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="updated_at">
                    {renderValue(selectedFlag.updated_at)}
                  </Descriptions.Item>
                </Descriptions>
              </Card>

              <div className="grid gap-4 xl:grid-cols-2">
                <Card title="Impact 分析">
                  {impactLoading ? (
                    <Spin />
                  ) : impactError ? (
                    <Alert type="error" showIcon message={impactError} />
                  ) : impact ? (
                    <Space direction="vertical" size="middle" className="w-full">
                      {impact.sample_size_too_small ? (
                        <Alert type="warning" showIcon message="样本量不足" />
                      ) : null}
                      {impact.contract_gaps?.length ? (
                        <Alert
                          type="warning"
                          showIcon
                          message="Impact 存在契约缺口"
                          description={impact.contract_gaps.join(', ')}
                        />
                      ) : null}
                      <Descriptions column={1} size="small" bordered>
                        <Descriptions.Item label="sample_size">
                          {renderValue(impact.sample_size)}
                        </Descriptions.Item>
                        <Descriptions.Item label="parent_fill_gain">
                          {renderMetric(impact.parent_fill_gain)}
                        </Descriptions.Item>
                        <Descriptions.Item label="rewrite_gain">
                          {renderMetric(impact.rewrite_gain)}
                        </Descriptions.Item>
                        <Descriptions.Item label="evidence_refusal_rate">
                          {renderMetric(impact.evidence_refusal_rate)}
                        </Descriptions.Item>
                        <Descriptions.Item label="refusal_false_positive_rate">
                          {renderMetric(impact.refusal_false_positive_rate)}
                        </Descriptions.Item>
                        <Descriptions.Item label="citation_support_score">
                          {renderMetric(impact.citation_support_score)}
                        </Descriptions.Item>
                        <Descriptions.Item label="p95_latency_delta_ms">
                          {renderMetric(impact.p95_latency_delta_ms, 0)}
                        </Descriptions.Item>
                        <Descriptions.Item label="avg_context_tokens_delta">
                          {renderMetric(impact.avg_context_tokens_delta, 0)}
                        </Descriptions.Item>
                        <Descriptions.Item label="route_contribution">
                          {impact.route_contribution ? (
                            <Space direction="vertical" size="small">
                              <Text>dense: {renderMetric(impact.route_contribution.dense)}</Text>
                              <Text>sparse: {renderMetric(impact.route_contribution.sparse)}</Text>
                            </Space>
                          ) : (
                            <Tag color="warning">Contract gap</Tag>
                          )}
                        </Descriptions.Item>
                      </Descriptions>
                    </Space>
                  ) : (
                    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有 impact 数据。" />
                  )}
                </Card>

                <Card title="Gate 摘要">
                  {gateLoading ? (
                    <Spin />
                  ) : gateError ? (
                    <Alert type="error" showIcon message={gateError} />
                  ) : gates ? (
                    <Descriptions column={1} size="small" bordered>
                      <Descriptions.Item label="gate_status">
                        <Tag color={gates.passed ? 'success' : 'warning'}>
                          {gates.gate_status || 'unknown'}
                        </Tag>
                      </Descriptions.Item>
                      <Descriptions.Item label="passed">
                        {gates.passed ? <Tag color="success">true</Tag> : <Tag color="error">false</Tag>}
                      </Descriptions.Item>
                      <Descriptions.Item label="failed_rules">
                        {gates.failed_rules?.length ? gates.failed_rules.join(', ') : <Tag color="default">none</Tag>}
                      </Descriptions.Item>
                      <Descriptions.Item label="baseline_report_id">
                        {renderValue(gates.baseline_report_id)}
                      </Descriptions.Item>
                      <Descriptions.Item label="candidate_report_id">
                        {renderValue(gates.candidate_report_id)}
                      </Descriptions.Item>
                      <Descriptions.Item label="last_eval_run_id">
                        {renderValue(gates.last_eval_run_id)}
                      </Descriptions.Item>
                    </Descriptions>
                  ) : (
                    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有 gate 数据。" />
                  )}
                </Card>
              </div>

              <div className="grid gap-4 xl:grid-cols-2">
                <Card title="版本列表">
                  {versionsLoading ? (
                    <Spin />
                  ) : versionsError ? (
                    <Alert type="error" showIcon message={versionsError} />
                  ) : versions.length === 0 ? (
                    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有版本记录。" />
                  ) : (
                    <Table<StrategyVersion>
                      rowKey="version_id"
                      size="small"
                      columns={versionColumns}
                      dataSource={versions}
                      pagination={false}
                    />
                  )}
                </Card>

                <Card title="操作日志">
                  {operationsLoading ? (
                    <Spin />
                  ) : operationsError ? (
                    <Alert type="error" showIcon message={operationsError} />
                  ) : operations.length === 0 ? (
                    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有操作日志。" />
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
              </div>

              {lastRollbackResult ? (
                <Alert
                  type="success"
                  showIcon
                  message={`最近回滚：${lastRollbackResult.rollback_id || 'rollback'}`}
                  description={
                    lastRollbackResult.changed_flags?.length
                      ? `Changed flags: ${lastRollbackResult.changed_flags
                          .map((item) => item.label || item.flag_key)
                          .join(', ')}`
                      : undefined
                  }
                />
              ) : null}
            </>
          )}
        </Space>
      </div>

      <Modal
        title="修改策略"
        open={editOpen}
        confirmLoading={actionLoading}
        onCancel={() => setEditOpen(false)}
        onOk={() => void submitEdit()}
      >
        <Space direction="vertical" size="middle" className="w-full">
          {selectedFlag?.risk_level === 'high' ? (
            <Alert
              type="warning"
              showIcon
              message="高风险策略"
              description="从 disabled 到 enabled 时建议先走 shadow 或 canary，避免直接全量开启。"
            />
          ) : null}
          {highRiskDirectEnable ? (
            <Alert
              type="error"
              showIcon
              message="当前选择会触发后端高风险校验"
              description="请优先选择 shadow 或 canary，再逐步扩大 rollout。"
            />
          ) : null}
          <Form form={editForm} layout="vertical">
            <Form.Item label="Enabled" name="enabled" rules={[{ required: true }]}>
              <Select
                options={[
                  { label: 'true', value: true },
                  { label: 'false', value: false },
                ]}
              />
            </Form.Item>
            <Form.Item label="Status" name="status" rules={[{ required: true }]}>
              <Select options={strategyStatuses.map((value) => ({ label: value, value }))} />
            </Form.Item>
            <Form.Item label="Rollout Percentage" name="rollout_percentage" rules={[{ required: true }]}>
              <InputNumber min={0} max={100} className="w-full" />
            </Form.Item>
            <Form.Item
              label="Reason"
              name="reason"
              rules={[{ required: true, message: '请填写 reason' }]}
            >
              <Input.TextArea rows={4} placeholder="说明这次策略调整的原因" />
            </Form.Item>
          </Form>
        </Space>
      </Modal>

      <Modal
        title={rollbackAll ? '回滚到 Phase2 Baseline' : '回滚当前策略'}
        open={rollbackOpen}
        confirmLoading={actionLoading}
        onCancel={() => setRollbackOpen(false)}
        onOk={() => void submitRollback()}
      >
        <Space direction="vertical" size="middle" className="w-full">
          <Alert
            type="warning"
            showIcon
            message={rollbackAll ? '将按后端冻结顺序做全量回滚' : '将仅回滚当前策略'}
            description={
              rollbackAll
                ? '影响范围：全部 Phase 3 flags，目标版本为 phase2_baseline。'
                : `影响范围：${selectedFlag?.label || selectedFlag?.flag_key || '当前策略'}。`
            }
          />
          <Form form={rollbackForm} layout="vertical">
            <Form.Item
              label="Reason"
              name="reason"
              rules={[{ required: true, message: '请填写 reason' }]}
            >
              <Input.TextArea rows={4} placeholder="说明回滚原因，例如延迟回退或质量异常" />
            </Form.Item>
          </Form>
        </Space>
      </Modal>
    </div>
  );
}
