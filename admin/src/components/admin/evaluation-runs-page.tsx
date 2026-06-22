'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import dayjs, { type Dayjs } from 'dayjs';
import { EyeOutlined, PlusOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  DatePicker,
  Descriptions,
  Drawer,
  Empty,
  Form,
  Input,
  Modal,
  Progress,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { KB_ADMIN_API } from '@/config/api';
import apiClient from '@/services/api/client';
import type {
  EvalDataset,
  EvalGateThresholds,
  EvalRun,
  EvalRunStatus,
  EvalStrategyProfile,
  ListResponse,
} from '@/types/kb';

const { Title, Paragraph, Text } = Typography;
const { TextArea } = Input;

const PAGE_SIZE = 10;
const DATASET_SELECT_PAGE_SIZE = 100;
const POLL_INTERVAL_MS = 4000;

type RunFilterFormValues = {
  dataset_id?: number;
  status?: EvalRunStatus;
  range?: [Dayjs, Dayjs];
};

type CreateRunFormValues = {
  dataset_id: number;
  baseline_profile: string;
  candidate_profile: string;
  profiles_json: string;
  gate_thresholds_json?: string;
};

type RunFilters = {
  dataset_id?: number;
  status?: EvalRunStatus;
  start_time?: string;
  end_time?: string;
};

const runStatusOptions: Array<{ label: string; value: EvalRunStatus }> = [
  { label: '排队中', value: 'pending' },
  { label: '运行中', value: 'running' },
  { label: '已完成', value: 'succeeded' },
  { label: '失败', value: 'failed' },
  { label: '已取消', value: 'canceled' },
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

function isFormValidationError(error: unknown): boolean {
  return Boolean(error && typeof error === 'object' && 'errorFields' in error);
}

function formatTime(value?: string): string {
  if (!value) {
    return '-';
  }
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : value;
}

function formatProgress(progress?: number): number {
  if (typeof progress !== 'number' || Number.isNaN(progress)) {
    return 0;
  }
  if (progress <= 1) {
    return Math.max(0, Math.min(100, progress * 100));
  }
  return Math.max(0, Math.min(100, progress));
}

function formatPercent(progress?: number): string {
  return `${formatProgress(progress).toFixed(0)}%`;
}

function statusColor(status: EvalRunStatus): string {
  switch (status) {
    case 'succeeded':
      return 'success';
    case 'running':
      return 'processing';
    case 'failed':
      return 'error';
    case 'canceled':
      return 'default';
    default:
      return 'gold';
  }
}

function formatRunStatus(status: EvalRunStatus): string {
  switch (status) {
    case 'pending':
      return '排队中';
    case 'running':
      return '运行中';
    case 'succeeded':
      return '已完成';
    case 'failed':
      return '失败';
    case 'canceled':
      return '已取消';
    default:
      return status;
  }
}

function buildFilters(values: RunFilterFormValues): RunFilters {
  return {
    dataset_id: values.dataset_id,
    status: values.status,
    start_time: values.range?.[0]?.toISOString(),
    end_time: values.range?.[1]?.toISOString(),
  };
}

function buildParams(filters: RunFilters, page: number) {
  return {
    page,
    page_size: PAGE_SIZE,
    ...(filters.dataset_id ? { dataset_id: filters.dataset_id } : {}),
    ...(filters.status ? { status: filters.status } : {}),
    ...(filters.start_time ? { start_time: filters.start_time } : {}),
    ...(filters.end_time ? { end_time: filters.end_time } : {}),
  };
}

function createDefaultProfiles(
  baselineProfile: string,
  candidateProfile: string
): EvalStrategyProfile[] {
  return [
    {
      name: baselineProfile,
      label: baselineProfile,
      baseline: true,
      mode: 'dense',
    },
    {
      name: candidateProfile,
      label: candidateProfile,
      candidate: true,
      mode: 'hybrid',
    },
  ];
}

function parseProfiles(
  raw: string,
  baselineProfile: string,
  candidateProfile: string
): EvalStrategyProfile[] {
  if (!raw.trim()) {
    return createDefaultProfiles(baselineProfile, candidateProfile);
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    throw new Error('profiles JSON 解析失败');
  }

  if (!Array.isArray(parsed) || parsed.length === 0) {
    throw new Error('profiles 必须是非空 JSON 数组');
  }

  return parsed as EvalStrategyProfile[];
}

function parseGateThresholds(raw?: string): EvalGateThresholds | undefined {
  if (!raw?.trim()) {
    return undefined;
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    throw new Error('gate_thresholds JSON 解析失败');
  }

  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('gate_thresholds 必须是 JSON 对象');
  }

  return parsed as EvalGateThresholds;
}

export function EvaluationRunsPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [messageApi, contextHolder] = message.useMessage();

  const [filterForm] = Form.useForm<RunFilterFormValues>();
  const [createForm] = Form.useForm<CreateRunFormValues>();

  const [filters, setFilters] = useState<RunFilters>({});
  const [runs, setRuns] = useState<EvalRun[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [runsLoading, setRunsLoading] = useState(false);
  const [runsError, setRunsError] = useState<string | null>(null);

  const [datasets, setDatasets] = useState<EvalDataset[]>([]);
  const [datasetsLoading, setDatasetsLoading] = useState(false);
  const [datasetsError, setDatasetsError] = useState<string | null>(null);

  const [createOpen, setCreateOpen] = useState(false);
  const [createSubmitting, setCreateSubmitting] = useState(false);

  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [detail, setDetail] = useState<EvalRun | null>(null);

  const [highlightRunId, setHighlightRunId] = useState<string | null>(null);

  const datasetNameMap = useMemo(
    () => new Map(datasets.map((dataset) => [dataset.id, dataset.name])),
    [datasets]
  );

  const loadDatasets = useCallback(async () => {
    try {
      setDatasetsLoading(true);
      setDatasetsError(null);

      const response = (await apiClient.get(KB_ADMIN_API.LIST_EVAL_DATASETS, {
        params: { page: 1, page_size: DATASET_SELECT_PAGE_SIZE },
      })) as ListResponse<EvalDataset>;

      setDatasets(response.items ?? []);
    } catch (error) {
      setDatasets([]);
      setDatasetsError(normalizeError(error, '加载评测集选项失败'));
    } finally {
      setDatasetsLoading(false);
    }
  }, []);

  const loadRuns = useCallback(async (nextFilters: RunFilters, nextPage: number) => {
    try {
      setRunsLoading(true);
      setRunsError(null);

      const response = (await apiClient.get(KB_ADMIN_API.LIST_EVAL_RUNS, {
        params: buildParams(nextFilters, nextPage),
      })) as ListResponse<EvalRun>;

      setRuns(response.items ?? []);
      setTotal(response.total ?? 0);
      setPage(response.page ?? nextPage);
    } catch (error) {
      setRuns([]);
      setTotal(0);
      setRunsError(normalizeError(error, '加载评测运行失败'));
    } finally {
      setRunsLoading(false);
    }
  }, []);

  const loadRunDetail = useCallback(async (runId: string, openDrawer = true) => {
    try {
      if (openDrawer) {
        setDetailOpen(true);
      }
      setDetailLoading(true);
      setDetailError(null);

      const response = (await apiClient.get(KB_ADMIN_API.GET_EVAL_RUN(runId))) as EvalRun;
      setDetail(response);
    } catch (error) {
      setDetail(null);
      setDetailError(normalizeError(error, '加载运行详情失败'));
    } finally {
      setDetailLoading(false);
    }
  }, []);

  useEffect(() => {
    const datasetId = Number(searchParams.get('dataset_id') ?? '');
    const status = searchParams.get('status') as EvalRunStatus | null;
    const nextFilters: RunFilters = {
      dataset_id: Number.isFinite(datasetId) && datasetId > 0 ? datasetId : undefined,
      status: status ?? undefined,
    };

    filterForm.setFieldsValue({
      dataset_id: nextFilters.dataset_id,
      status: nextFilters.status,
      range: undefined,
    });
    setFilters(nextFilters);

    void loadDatasets();
    void loadRuns(nextFilters, 1);
  }, [filterForm, loadDatasets, loadRuns, searchParams]);

  useEffect(() => {
    const hasActiveRun = runs.some((run) => run.status === 'pending' || run.status === 'running');
    if (!hasActiveRun) {
      return undefined;
    }

    const timer = window.setTimeout(() => {
      void loadRuns(filters, page);
      if (detail?.run_id && (detail.status === 'pending' || detail.status === 'running')) {
        void loadRunDetail(detail.run_id, false);
      }
    }, POLL_INTERVAL_MS);

    return () => window.clearTimeout(timer);
  }, [detail, filters, loadRunDetail, loadRuns, page, runs]);

  const columns = useMemo<ColumnsType<EvalRun>>(
    () => [
      {
        title: 'Run ID',
        dataIndex: 'run_id',
        key: 'run_id',
        width: 220,
        render: (value: string) => <Text code>{value}</Text>,
      },
      {
        title: '评测集',
        dataIndex: 'dataset_id',
        key: 'dataset_id',
        width: 180,
        render: (value: number) => (
          <Space direction="vertical" size={0}>
            <Text>{datasetNameMap.get(value) ?? `#${value}`}</Text>
            <Text type="secondary">dataset_id: {value}</Text>
          </Space>
        ),
      },
      {
        title: '基线策略',
        dataIndex: 'baseline_profile',
        key: 'baseline_profile',
        width: 180,
        ellipsis: true,
      },
      {
        title: '候选策略',
        dataIndex: 'candidate_profile',
        key: 'candidate_profile',
        width: 180,
        ellipsis: true,
      },
      {
        title: '状态',
        dataIndex: 'status',
        key: 'status',
        width: 140,
        render: (value: EvalRunStatus, record) =>
          value === 'running' ? (
            <Space direction="vertical" size={4} className="w-full">
              <Tag color={statusColor(value)}>{formatRunStatus(value)}</Tag>
              <Progress percent={Number(formatProgress(record.progress).toFixed(0))} size="small" status="active" />
            </Space>
          ) : (
            <Tag color={statusColor(value)}>{formatRunStatus(value)}</Tag>
          ),
      },
      {
        title: '进度',
        key: 'progress',
        width: 150,
        render: (_, record) => `${record.case_finished}/${record.case_total} (${formatPercent(record.progress)})`,
      },
      {
        title: '开始时间',
        dataIndex: 'started_at',
        key: 'started_at',
        width: 180,
        render: (value?: string) => formatTime(value),
      },
      {
        title: '结束时间',
        dataIndex: 'finished_at',
        key: 'finished_at',
        width: 180,
        render: (value?: string) => formatTime(value),
      },
      {
        title: '操作',
        key: 'actions',
        width: 220,
        render: (_, record) => (
          <Space wrap onClick={(event) => event.stopPropagation()}>
            {record.status === 'succeeded' ? (
              <Link href={`/evaluation/reports/${record.run_id}`}>
                <Button size="small" type="primary">
                  查看报告
                </Button>
              </Link>
            ) : null}
            <Button size="small" icon={<EyeOutlined />} onClick={() => void loadRunDetail(record.run_id)}>
              查看详情
            </Button>
          </Space>
        ),
      },
    ],
    [datasetNameMap, loadRunDetail]
  );

  const handleCreateRun = async () => {
    try {
      const values = await createForm.validateFields();
      const profiles = parseProfiles(
        values.profiles_json,
        values.baseline_profile,
        values.candidate_profile
      );
      const gateThresholds = parseGateThresholds(values.gate_thresholds_json);

      setCreateSubmitting(true);

      const created = (await apiClient.post(KB_ADMIN_API.CREATE_EVAL_RUN, {
        dataset_id: values.dataset_id,
        baseline_profile: values.baseline_profile,
        candidate_profile: values.candidate_profile,
        profiles,
        ...(gateThresholds ? { gate_thresholds: gateThresholds } : {}),
      })) as EvalRun;

      setCreateOpen(false);
      createForm.resetFields();
      setHighlightRunId(created.run_id);

      const nextFilters = { ...filters, dataset_id: values.dataset_id };
      setFilters(nextFilters);
      router.replace(`/evaluation/runs?dataset_id=${values.dataset_id}`);
      filterForm.setFieldsValue({
        dataset_id: values.dataset_id,
        status: filters.status,
        range: undefined,
      });

      messageApi.success(`评测运行已创建：${created.run_id}`);
      await loadRuns(nextFilters, 1);
      await loadRunDetail(created.run_id);
    } catch (error) {
      if (isFormValidationError(error)) {
        return;
      }
      messageApi.error(normalizeError(error, '创建评测运行失败'));
    } finally {
      setCreateSubmitting(false);
    }
  };

  return (
    <div className="space-y-6">
      {contextHolder}

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            评测运行
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            触发离线评测任务、轮询运行状态、查看错误信息，并在成功后进入完整报告页面。
          </Paragraph>
        </div>
        <Space wrap>
          <Button icon={<ReloadOutlined />} onClick={() => void loadRuns(filters, page)}>
            刷新
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setCreateOpen(true);
              const datasetId = filters.dataset_id ?? datasets[0]?.id;
              if (datasetId) {
                createForm.setFieldValue('dataset_id', datasetId);
              }
            }}
          >
            新建运行
          </Button>
        </Space>
      </div>

      <Card>
        <Form
          form={filterForm}
          layout="vertical"
          onFinish={(values) => {
            const nextFilters = buildFilters(values);
            setFilters(nextFilters);

            const params = new URLSearchParams();
            if (nextFilters.dataset_id) {
              params.set('dataset_id', String(nextFilters.dataset_id));
            }
            if (nextFilters.status) {
              params.set('status', nextFilters.status);
            }

            router.replace(params.toString() ? `/evaluation/runs?${params.toString()}` : '/evaluation/runs');
            void loadRuns(nextFilters, 1);
          }}
        >
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <Form.Item label="评测集" name="dataset_id">
              <Select
                allowClear
                showSearch
                placeholder="全部评测集"
                loading={datasetsLoading}
                optionFilterProp="label"
                options={datasets.map((dataset) => ({
                  label: `${dataset.name} (#${dataset.id})`,
                  value: dataset.id,
                }))}
              />
            </Form.Item>
            <Form.Item label="状态" name="status">
              <Select allowClear placeholder="全部状态" options={runStatusOptions} />
            </Form.Item>
            <Form.Item label="时间范围" name="range" className="xl:col-span-2">
              <DatePicker.RangePicker showTime className="w-full" />
            </Form.Item>
          </div>
          <Space>
            <Button type="primary" htmlType="submit" icon={<SearchOutlined />} loading={runsLoading}>
              查询运行
            </Button>
            <Button
              onClick={() => {
                filterForm.resetFields();
                const nextFilters = {} as RunFilters;
                setFilters(nextFilters);
                router.replace('/evaluation/runs');
                void loadRuns(nextFilters, 1);
              }}
            >
              重置
            </Button>
          </Space>
        </Form>
      </Card>

      {datasetsError ? <Alert type="warning" showIcon message={datasetsError} /> : null}
      {runsError ? <Alert type="error" showIcon message={runsError} /> : null}

      <Card title="运行列表">
        {runs.length === 0 && !runsLoading ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="当前筛选条件下还没有评测运行。先选定评测集，再创建第一条运行。"
          >
            <Space wrap>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                新建运行
              </Button>
              <Link href="/evaluation/datasets">
                <Button>去准备评测集</Button>
              </Link>
            </Space>
          </Empty>
        ) : (
          <Table<EvalRun>
            rowKey="run_id"
            loading={runsLoading}
            columns={columns}
            dataSource={runs}
            pagination={{
              current: page,
              pageSize: PAGE_SIZE,
              total,
              onChange: (nextPage) => void loadRuns(filters, nextPage),
            }}
            rowClassName={(record) => (record.run_id === highlightRunId ? 'bg-slate-50' : '')}
            onRow={(record) => ({
              onClick: () => void loadRunDetail(record.run_id),
              style: { cursor: 'pointer' },
            })}
          />
        )}
      </Card>

      <Modal
        title="新建评测运行"
        open={createOpen}
        width={780}
        destroyOnClose
        confirmLoading={createSubmitting}
        onCancel={() => {
          setCreateOpen(false);
          createForm.resetFields();
        }}
        onOk={() => void handleCreateRun()}
      >
        <Space direction="vertical" size="large" className="w-full">
          <Alert
            type="info"
            showIcon
            message="运行参数"
            description="profiles 和 gate_thresholds 只做 JSON 语法校验。策略是否合法、门禁是否通过以及任务执行结果由后端负责。"
          />

          <Form
            form={createForm}
            layout="vertical"
            preserve={false}
            initialValues={{
              dataset_id: filters.dataset_id ?? datasets[0]?.id,
              baseline_profile: 'dense_only',
              candidate_profile: 'hybrid',
              profiles_json: JSON.stringify(createDefaultProfiles('dense_only', 'hybrid'), null, 2),
              gate_thresholds_json: JSON.stringify(
                {
                  min_recall_delta: 0,
                  max_p95_latency_regression_ratio: 0.2,
                },
                null,
                2
              ),
            }}
          >
            <Form.Item
              label="评测集"
              name="dataset_id"
              rules={[{ required: true, message: '请选择评测集' }]}
            >
              <Select
                showSearch
                optionFilterProp="label"
                loading={datasetsLoading}
                placeholder="选择一个 ready 评测集"
                options={datasets.map((dataset) => ({
                  label: `${dataset.name} (#${dataset.id}) - ${dataset.status}`,
                  value: dataset.id,
                }))}
              />
            </Form.Item>

            <div className="grid gap-4 md:grid-cols-2">
              <Form.Item
                label="基线策略 Profile"
                name="baseline_profile"
                rules={[{ required: true, message: '请输入基线策略 profile' }]}
              >
                <Input placeholder="例如 dense_only" />
              </Form.Item>
              <Form.Item
                label="候选策略 Profile"
                name="candidate_profile"
                rules={[{ required: true, message: '请输入候选策略 profile' }]}
              >
                <Input placeholder="例如 hybrid_rewrite_dynamic_topk" />
              </Form.Item>
            </div>

            <Form.Item
              label="策略 Profiles JSON"
              name="profiles_json"
              rules={[{ required: true, message: '请输入策略 profiles JSON' }]}
            >
              <TextArea rows={12} placeholder='例如: [{"name":"dense_only","baseline":true,"mode":"dense"}]' />
            </Form.Item>

            <Form.Item label="门禁阈值 JSON" name="gate_thresholds_json">
              <TextArea
                rows={8}
                placeholder='例如: {"min_recall_delta":0.08,"max_p95_latency_regression_ratio":0.2}'
              />
            </Form.Item>
          </Form>
        </Space>
      </Modal>

      <Drawer
        title={detail?.run_id ? `运行详情 · ${detail.run_id}` : '运行详情'}
        width={620}
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
      >
        {detailLoading ? (
          <div className="py-10 text-center">加载中...</div>
        ) : detailError ? (
          <Alert type="error" showIcon message={detailError} />
        ) : detail ? (
          <Space direction="vertical" size="large" className="w-full">
            <Descriptions title="运行信息" column={1} size="small" bordered>
              <Descriptions.Item label="运行 ID">{detail.run_id}</Descriptions.Item>
              <Descriptions.Item label="评测集">
                {datasetNameMap.get(detail.dataset_id) ?? `#${detail.dataset_id}`}
              </Descriptions.Item>
              <Descriptions.Item label="基线策略">{detail.baseline_profile}</Descriptions.Item>
              <Descriptions.Item label="候选策略">{detail.candidate_profile}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={statusColor(detail.status)}>{formatRunStatus(detail.status)}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="进度">
                <Space direction="vertical" size={4} className="w-full">
                  <Text>{`${detail.case_finished}/${detail.case_total} (${formatPercent(detail.progress)})`}</Text>
                  <Progress
                    percent={Number(formatProgress(detail.progress).toFixed(0))}
                    status={
                      detail.status === 'failed'
                        ? 'exception'
                        : detail.status === 'succeeded'
                          ? 'success'
                          : 'active'
                    }
                  />
                </Space>
              </Descriptions.Item>
              <Descriptions.Item label="开始时间">{formatTime(detail.started_at)}</Descriptions.Item>
              <Descriptions.Item label="结束时间">{formatTime(detail.finished_at)}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{formatTime(detail.created_at)}</Descriptions.Item>
              <Descriptions.Item label="错误信息">
                {detail.error_msg || <Text type="secondary">-</Text>}
              </Descriptions.Item>
              <Descriptions.Item label="报告路径">
                {detail.report_path ? <Text code>{detail.report_path}</Text> : <Tag color="warning">字段暂缺</Tag>}
              </Descriptions.Item>
            </Descriptions>

            <Card title="门禁阈值" size="small">
              {detail.gate_thresholds && Object.keys(detail.gate_thresholds).length > 0 ? (
                <pre className="mb-0 whitespace-pre-wrap rounded bg-slate-50 p-3 text-xs">
                  {JSON.stringify(detail.gate_thresholds, null, 2)}
                </pre>
              ) : (
                <Text type="secondary">未配置门禁阈值</Text>
              )}
            </Card>

            <Card title="策略配置 Profiles" size="small">
              {detail.profiles?.length ? (
                <pre className="mb-0 whitespace-pre-wrap rounded bg-slate-50 p-3 text-xs">
                  {JSON.stringify(detail.profiles, null, 2)}
                </pre>
              ) : (
                <Alert
                  type="warning"
                  showIcon
                  message="策略配置字段暂缺"
                  description="后端暂未返回 profiles，当前无法展示完整策略配置。"
                />
              )}
            </Card>

            {detail.status === 'succeeded' ? (
              <Link href={`/evaluation/reports/${detail.run_id}`}>
                <Button type="primary" block>
                  进入报告页
                </Button>
              </Link>
            ) : null}
          </Space>
        ) : (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="请选择一条运行记录查看详情" />
        )}
      </Drawer>
    </div>
  );
}
