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
import { ActionEmpty } from './ui/action-empty';
import { MetricCard } from './ui/metric-card';
import { PageHeader } from './ui/page-header';

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
    return <Tag color="warning">字段暂缺</Tag>;
  }
  return value.toFixed(digits);
}

function renderValue(value?: string | number | boolean | null) {
  if (value === undefined || value === null || value === '') {
    return <Tag color="warning">字段暂缺</Tag>;
  }
  if (typeof value === 'boolean') {
    return value ? '是' : '否';
  }
  return String(value);
}

function formatStrategyStatus(status?: string): string {
  switch (status) {
    case 'enabled':
      return '已启用';
    case 'disabled':
      return '已停用';
    case 'shadow':
      return '影子模式';
    case 'canary':
      return '灰度中';
    case 'rolling_back':
      return '回退中';
    case 'error':
      return '异常';
    default:
      return '未知';
  }
}

function formatRiskLevel(riskLevel?: string): string {
  switch (riskLevel) {
    case 'high':
      return '高风险';
    case 'medium':
      return '中风险';
    case 'low':
      return '低风险';
    default:
      return '未知';
  }
}

function riskTagColor(riskLevel?: string): string {
  switch (riskLevel) {
    case 'high':
      return 'error';
    case 'medium':
      return 'warning';
    case 'low':
      return 'success';
    default:
      return 'default';
  }
}

function formatImpactRange(range: MetricsRange): string {
  switch (range) {
    case '1h':
      return '近 1 小时';
    case '7d':
      return '近 7 天';
    default:
      return '近 24 小时';
  }
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
        render: (value?: string) => <Tag color={statusColor(value)}>{formatStrategyStatus(value)}</Tag>,
      },
      {
        title: '灰度',
        dataIndex: 'rollout_percentage',
        key: 'rollout_percentage',
        width: 90,
        render: (value?: number) => (value === undefined ? <Tag color="warning">字段暂缺</Tag> : `${value}%`),
      },
      {
        title: '风险',
        dataIndex: 'risk_level',
        key: 'risk_level',
        width: 90,
        render: (value?: string) => <Tag color={riskTagColor(value)}>{formatRiskLevel(value)}</Tag>,
      },
    ],
    []
  );

  const versionColumns = useMemo<ColumnsType<StrategyVersion>>(
    () => [
      { title: '版本 ID', dataIndex: 'version_id', key: 'version_id', ellipsis: true },
      { title: '创建人', dataIndex: 'created_by', key: 'created_by', width: 120 },
      {
        title: '门禁',
        dataIndex: 'gate_status',
        key: 'gate_status',
        width: 120,
        render: (value?: string) => <Tag color={statusColor(value)}>{value || 'pending'}</Tag>,
      },
      { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
    ],
    []
  );

  const operationColumns = useMemo<ColumnsType<StrategyOperationLog>>(
    () => [
      {
        title: '操作类型',
        dataIndex: 'operation',
        key: 'operation',
        width: 120,
        render: (value?: string) => value || <Tag color="warning">字段暂缺</Tag>,
      },
      {
        title: '原状态',
        dataIndex: 'from_status',
        key: 'from_status',
        width: 110,
        render: (value?: string) => <Tag color={statusColor(value)}>{value ? formatStrategyStatus(value) : '无'}</Tag>,
      },
      {
        title: '目标状态',
        dataIndex: 'to_status',
        key: 'to_status',
        width: 110,
        render: (value?: string) => <Tag color={statusColor(value)}>{value ? formatStrategyStatus(value) : '无'}</Tag>,
      },
      {
        title: '原因',
        dataIndex: 'reason',
        key: 'reason',
        ellipsis: true,
      },
      {
        title: '时间',
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
    <div className="admin-page">
      {contextHolder}

      <PageHeader
        title="策略管理"
        subtitle="查看检索策略的状态、影响、版本和操作记录，并在确认风险后进行灰度或回退。"
        extra={
          <>
            <Link href="/evaluation/runs">
              <Button>查看评测任务</Button>
            </Link>
            <Button icon={<ReloadOutlined />} onClick={() => void loadFlags()}>
              刷新
            </Button>
          </>
        }
      />

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          label="启用策略数"
          value={<Statistic value={enabledCount} prefix={<SettingOutlined />} />}
          helper="当前处于启用状态的策略总数"
        />
        <MetricCard
          label="小流量试用策略数"
          value={<Statistic value={canaryCount} />}
          helper="正在小范围放量观察的策略数量"
        />
        <MetricCard
          label="异常策略数"
          value={<Statistic value={errorCount} />}
          helper="需要优先检查的异常策略数量"
        />
        <MetricCard
          label="最近回退次数"
          value={<Statistic value={rollbackCount} prefix={<RollbackOutlined />} />}
          helper="近期执行过的策略回退次数"
        />
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
        <Card title="策略开关" className="admin-section-card">
          {flagsLoading ? (
            <div className="flex justify-center py-10">
              <Spin />
            </div>
          ) : flags.length === 0 ? (
            <ActionEmpty
              title="当前没有可展示的策略开关"
              description="请先确认后端是否已返回策略配置，或稍后刷新重试。"
              action={
                <Button type="primary" onClick={() => void loadFlags()}>
                  重新加载策略
                </Button>
              }
            />
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
            <Card className="admin-section-card">
              <ActionEmpty
                title="请选择一个策略查看详情"
                description="选择后可以查看影响分析、门禁摘要、版本记录和操作日志。"
                action={<Button onClick={() => void loadFlags()}>刷新策略列表</Button>}
              />
            </Card>
          ) : (
            <>
              <Card
                title={selectedFlag.label || selectedFlag.flag_key}
                extra={
                  <Space wrap>
                    <Select
                      value={selectedRange}
                      options={impactRanges.map((value) => ({ label: formatImpactRange(value), value }))}
                      onChange={(value) => setSelectedRange(value)}
                      style={{ width: 120 }}
                    />
                    <Button onClick={openEditModal}>修改策略</Button>
                    <Button onClick={() => openRollbackModal(false)}>回滚当前策略</Button>
                    <Button danger onClick={() => openRollbackModal(true)}>
                      回退到稳定检索策略
                    </Button>
                  </Space>
                }
                className="admin-section-card"
              >
                <Descriptions column={2} size="small" bordered>
                  <Descriptions.Item label="策略标识">{selectedFlag.flag_key}</Descriptions.Item>
                  <Descriptions.Item label="策略名称">{renderValue(selectedFlag.label)}</Descriptions.Item>
                  <Descriptions.Item label="当前状态">
                    <Tag color={statusColor(selectedFlag.status)}>
                      {formatStrategyStatus(selectedFlag.status)}
                    </Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="是否启用">{renderValue(selectedFlag.enabled)}</Descriptions.Item>
                  <Descriptions.Item label="灰度比例">
                    {renderValue(selectedFlag.rollout_percentage)}
                  </Descriptions.Item>
                  <Descriptions.Item label="策略版本">
                    {renderValue(selectedFlag.strategy_version)}
                  </Descriptions.Item>
                  <Descriptions.Item label="风险等级">
                    <Tag color={riskTagColor(selectedFlag.risk_level)}>
                      {formatRiskLevel(selectedFlag.risk_level)}
                    </Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="最近更新时间">
                    {renderValue(selectedFlag.updated_at)}
                  </Descriptions.Item>
                </Descriptions>
              </Card>

              <div className="grid gap-4 xl:grid-cols-2">
                <Card title="影响分析">
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
                          message="影响分析存在字段缺口"
                          description={impact.contract_gaps.join(', ')}
                        />
                      ) : null}
                      <Descriptions column={1} size="small" bordered>
                        <Descriptions.Item label="样本量">
                          {renderValue(impact.sample_size)}
                        </Descriptions.Item>
                        <Descriptions.Item label="父子召回补全收益">
                          {renderMetric(impact.parent_fill_gain)}
                        </Descriptions.Item>
                        <Descriptions.Item label="查询改写收益">
                          {renderMetric(impact.rewrite_gain)}
                        </Descriptions.Item>
                        <Descriptions.Item label="证据不足拒答率">
                          {renderMetric(impact.evidence_refusal_rate)}
                        </Descriptions.Item>
                        <Descriptions.Item label="拒答误判率">
                          {renderMetric(impact.refusal_false_positive_rate)}
                        </Descriptions.Item>
                        <Descriptions.Item label="引用支撑评分">
                          {renderMetric(impact.citation_support_score)}
                        </Descriptions.Item>
                        <Descriptions.Item label="P95 延迟变化（ms）">
                          {renderMetric(impact.p95_latency_delta_ms, 0)}
                        </Descriptions.Item>
                        <Descriptions.Item label="平均上下文 Token 变化">
                          {renderMetric(impact.avg_context_tokens_delta, 0)}
                        </Descriptions.Item>
                        <Descriptions.Item label="路由贡献">
                          {impact.route_contribution ? (
                            <Space direction="vertical" size="small">
                              <Text>稠密检索：{renderMetric(impact.route_contribution.dense)}</Text>
                              <Text>稀疏检索：{renderMetric(impact.route_contribution.sparse)}</Text>
                            </Space>
                          ) : (
                            <Tag color="warning">字段暂缺</Tag>
                          )}
                        </Descriptions.Item>
                      </Descriptions>
                    </Space>
                  ) : (
                    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有影响分析数据。" />
                  )}
                </Card>

                <Card title="门禁摘要">
                  {gateLoading ? (
                    <Spin />
                  ) : gateError ? (
                    <Alert type="error" showIcon message={gateError} />
                  ) : gates ? (
                    <Descriptions column={1} size="small" bordered>
                      <Descriptions.Item label="门禁状态">
                        <Tag color={gates.passed ? 'success' : 'warning'}>
                          {gates.gate_status || 'unknown'}
                        </Tag>
                      </Descriptions.Item>
                      <Descriptions.Item label="是否通过">
                        {gates.passed ? <Tag color="success">是</Tag> : <Tag color="error">否</Tag>}
                      </Descriptions.Item>
                      <Descriptions.Item label="未通过规则">
                        {gates.failed_rules?.length ? gates.failed_rules.join(', ') : <Tag color="default">无</Tag>}
                      </Descriptions.Item>
                      <Descriptions.Item label="基线报告 ID">
                        {renderValue(gates.baseline_report_id)}
                      </Descriptions.Item>
                      <Descriptions.Item label="候选报告 ID">
                        {renderValue(gates.candidate_report_id)}
                      </Descriptions.Item>
                      <Descriptions.Item label="最近评测运行 ID">
                        {renderValue(gates.last_eval_run_id)}
                      </Descriptions.Item>
                    </Descriptions>
                  ) : (
                    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有门禁摘要数据。" />
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
                      ? `已变更策略：${lastRollbackResult.changed_flags
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
              description="从停用直接切到启用前，建议先走影子模式或灰度发布，避免一次性全量放开。"
            />
          ) : null}
          {highRiskDirectEnable ? (
            <Alert
              type="error"
              showIcon
              message="当前选择会触发后端高风险校验"
              description="请优先选择影子模式或灰度发布，再逐步扩大灰度比例。"
            />
          ) : null}
          <Form form={editForm} layout="vertical">
            <Form.Item label="是否启用" name="enabled" rules={[{ required: true }]}>
              <Select
                options={[
                  { label: '是', value: true },
                  { label: '否', value: false },
                ]}
              />
            </Form.Item>
            <Form.Item label="发布状态" name="status" rules={[{ required: true }]}>
              <Select
                options={strategyStatuses.map((value) => ({
                  label: formatStrategyStatus(value),
                  value,
                }))}
              />
            </Form.Item>
            <Form.Item label="灰度比例" name="rollout_percentage" rules={[{ required: true }]}>
              <InputNumber min={0} max={100} className="w-full" />
            </Form.Item>
            <Form.Item
              label="调整原因"
              name="reason"
              rules={[{ required: true, message: '请填写调整原因' }]}
            >
              <Input.TextArea rows={4} placeholder="说明这次策略调整的原因" />
            </Form.Item>
          </Form>
        </Space>
      </Modal>

      <Modal
        title={rollbackAll ? '回退到 Phase 2 基线策略' : '回滚当前策略'}
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
                ? '影响范围：全部 Phase 3 策略开关，目标版本为 phase2_baseline。'
                : `影响范围：${selectedFlag?.label || selectedFlag?.flag_key || '当前策略'}。`
            }
          />
          <Form form={rollbackForm} layout="vertical">
            <Form.Item
              label="回滚原因"
              name="reason"
              rules={[{ required: true, message: '请填写回滚原因' }]}
            >
              <Input.TextArea rows={4} placeholder="说明回滚原因，例如延迟回退或质量异常" />
            </Form.Item>
          </Form>
        </Space>
      </Modal>
    </div>
  );
}
