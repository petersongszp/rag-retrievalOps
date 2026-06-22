'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import dayjs from 'dayjs';
import {
  DownloadOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  UploadOutlined,
} from '@ant-design/icons';
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
  Table,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type { UploadFile } from 'antd/es/upload/interface';
import { KB_ADMIN_API } from '@/config/api';
import apiClient from '@/services/api/client';
import type {
  CitationTarget,
  EvalCase,
  EvalCaseImportResult,
  EvalDataset,
  EvalDatasetStatus,
  EvalDatasetValidationResult,
  ListResponse,
} from '@/types/kb';
import { useKnowledgeBaseContext } from './knowledge-base-provider';

const { Title, Paragraph, Text } = Typography;
const { TextArea } = Input;

const DATASET_PAGE_SIZE = 10;
const CASE_PAGE_SIZE = 20;

type EvalCaseValidationStatus = EvalCase['validation_status'];

type DatasetFilterFormValues = {
  kb_id?: number;
  status?: EvalDatasetStatus;
  keyword?: string;
};

type CaseFilterFormValues = {
  query_type?: string;
  tag?: string;
  validation_status?: EvalCaseValidationStatus;
  keyword?: string;
};

type CreateDatasetFormValues = {
  name: string;
  description?: string;
  kb_id?: number;
};

type CreateCaseFormValues = {
  case_key: string;
  query: string;
  top_k: number;
  query_type?: string;
  tags_text?: string;
  kb_ids_text?: string;
  relevant_ids_text?: string;
  citation_targets_json?: string;
  collection?: string;
  notes?: string;
};

const datasetStatusOptions: Array<{ label: string; value: EvalDatasetStatus }> = [
  { label: '草稿', value: 'draft' },
  { label: '可运行', value: 'ready' },
  { label: '校验异常', value: 'invalid' },
  { label: '已归档', value: 'archived' },
];

const validationStatusOptions: Array<{ label: string; value: EvalCaseValidationStatus }> = [
  { label: '未检查', value: 'unchecked' },
  { label: '有效', value: 'valid' },
  { label: '无效', value: 'invalid' },
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

function datasetStatusColor(status: EvalDatasetStatus): string {
  switch (status) {
    case 'ready':
      return 'success';
    case 'invalid':
      return 'error';
    case 'archived':
      return 'default';
    default:
      return 'gold';
  }
}

function validationStatusColor(status: EvalCaseValidationStatus): string {
  switch (status) {
    case 'valid':
      return 'success';
    case 'invalid':
      return 'error';
    default:
      return 'gold';
  }
}

function formatDatasetStatus(status: EvalDatasetStatus): string {
  switch (status) {
    case 'ready':
      return '可运行';
    case 'invalid':
      return '校验异常';
    case 'archived':
      return '已归档';
    default:
      return '草稿';
  }
}

function formatValidationStatus(status: EvalCaseValidationStatus): string {
  switch (status) {
    case 'valid':
      return '有效';
    case 'invalid':
      return '无效';
    default:
      return '未检查';
  }
}

function parseStringList(raw?: string): string[] {
  if (!raw) {
    return [];
  }

  return raw
    .split(/[\r\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseNumericList(raw?: string): number[] {
  return parseStringList(raw).map((item) => {
    const value = Number(item);
    if (!Number.isInteger(value) || value <= 0) {
      throw new Error(`kb_ids 中包含非法值：${item}`);
    }
    return value;
  });
}

function parseCitationTargets(raw?: string): CitationTarget[] {
  if (!raw?.trim()) {
    return [];
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    throw new Error('citation_targets JSON 解析失败');
  }

  if (!Array.isArray(parsed)) {
    throw new Error('citation_targets 必须是 JSON 数组');
  }

  return parsed.map((item, index) => {
    if (!item || typeof item !== 'object') {
      throw new Error(`citation_targets[${index}] 不是对象`);
    }

    const target = item as Record<string, unknown>;
    const documentId = target.document_id;
    const chunkId = target.chunk_id;
    const fileName = target.file_name;

    if (
      documentId !== undefined &&
      (!Number.isInteger(Number(documentId)) || Number(documentId) < 0)
    ) {
      throw new Error(`citation_targets[${index}].document_id 非法`);
    }
    if (chunkId !== undefined && typeof chunkId !== 'string') {
      throw new Error(`citation_targets[${index}].chunk_id 必须是字符串`);
    }
    if (fileName !== undefined && typeof fileName !== 'string') {
      throw new Error(`citation_targets[${index}].file_name 必须是字符串`);
    }

    return {
      document_id: documentId !== undefined ? Number(documentId) : undefined,
      chunk_id: typeof chunkId === 'string' ? chunkId : undefined,
      file_name: typeof fileName === 'string' ? fileName : undefined,
    };
  });
}

function buildDatasetParams(filters: DatasetFilterFormValues, page: number) {
  return {
    page,
    page_size: DATASET_PAGE_SIZE,
    ...(filters.kb_id ? { kb_id: filters.kb_id } : {}),
    ...(filters.status ? { status: filters.status } : {}),
    ...(filters.keyword?.trim() ? { keyword: filters.keyword.trim() } : {}),
  };
}

function buildCaseParams(filters: CaseFilterFormValues, page: number) {
  return {
    page,
    page_size: CASE_PAGE_SIZE,
    ...(filters.query_type?.trim() ? { query_type: filters.query_type.trim() } : {}),
    ...(filters.tag?.trim() ? { tag: filters.tag.trim() } : {}),
    ...(filters.validation_status ? { validation_status: filters.validation_status } : {}),
    ...(filters.keyword?.trim() ? { keyword: filters.keyword.trim() } : {}),
  };
}

function downloadDataset(datasetId: number) {
  if (typeof window === 'undefined') {
    return;
  }

  window.open(KB_ADMIN_API.EXPORT_EVAL_CASES(datasetId), '_blank', 'noopener,noreferrer');
}

export function EvaluationDatasetsPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { bases, selectedBase } = useKnowledgeBaseContext();
  const [messageApi, contextHolder] = message.useMessage();

  const [datasetFilterForm] = Form.useForm<DatasetFilterFormValues>();
  const [caseFilterForm] = Form.useForm<CaseFilterFormValues>();
  const [createDatasetForm] = Form.useForm<CreateDatasetFormValues>();
  const [createCaseForm] = Form.useForm<CreateCaseFormValues>();

  const [datasetFilters, setDatasetFilters] = useState<DatasetFilterFormValues>({});
  const [datasets, setDatasets] = useState<EvalDataset[]>([]);
  const [datasetPage, setDatasetPage] = useState(1);
  const [datasetTotal, setDatasetTotal] = useState(0);
  const [datasetsLoading, setDatasetsLoading] = useState(false);
  const [datasetError, setDatasetError] = useState<string | null>(null);

  const [selectedDatasetId, setSelectedDatasetId] = useState<number | null>(null);

  const [caseFilters, setCaseFilters] = useState<CaseFilterFormValues>({});
  const [cases, setCases] = useState<EvalCase[]>([]);
  const [casePage, setCasePage] = useState(1);
  const [caseTotal, setCaseTotal] = useState(0);
  const [casesLoading, setCasesLoading] = useState(false);
  const [caseError, setCaseError] = useState<string | null>(null);

  const [createDatasetOpen, setCreateDatasetOpen] = useState(false);
  const [createDatasetSubmitting, setCreateDatasetSubmitting] = useState(false);
  const [createCaseOpen, setCreateCaseOpen] = useState(false);
  const [createCaseSubmitting, setCreateCaseSubmitting] = useState(false);

  const [importOpen, setImportOpen] = useState(false);
  const [importSubmitting, setImportSubmitting] = useState(false);
  const [importFileList, setImportFileList] = useState<UploadFile[]>([]);
  const [importResult, setImportResult] = useState<EvalCaseImportResult | null>(null);

  const [validatingDatasetId, setValidatingDatasetId] = useState<number | null>(null);
  const [lastValidation, setLastValidation] = useState<EvalDatasetValidationResult | null>(null);

  const selectedDataset = useMemo(
    () => datasets.find((dataset) => dataset.id === selectedDatasetId) ?? null,
    [datasets, selectedDatasetId]
  );

  const loadDatasets = useCallback(
    async (filters: DatasetFilterFormValues, page: number) => {
      try {
        setDatasetsLoading(true);
        setDatasetError(null);

        const response = (await apiClient.get(KB_ADMIN_API.LIST_EVAL_DATASETS, {
          params: buildDatasetParams(filters, page),
        })) as ListResponse<EvalDataset>;

        const items = response.items ?? [];
        setDatasets(items);
        setDatasetTotal(response.total ?? 0);
        setDatasetPage(response.page ?? page);

        const queryDatasetId = Number(searchParams.get('dataset_id') ?? '');
        setSelectedDatasetId((previous) => {
          if (queryDatasetId && items.some((item) => item.id === queryDatasetId)) {
            return queryDatasetId;
          }
          if (previous && items.some((item) => item.id === previous)) {
            return previous;
          }
          return items[0]?.id ?? null;
        });
      } catch (error) {
        setDatasets([]);
        setDatasetTotal(0);
        setDatasetError(normalizeError(error, '加载评测集失败'));
        setSelectedDatasetId(null);
      } finally {
        setDatasetsLoading(false);
      }
    },
    [searchParams]
  );

  const loadCases = useCallback(
    async (datasetId: number, filters: CaseFilterFormValues, page: number) => {
      try {
        setCasesLoading(true);
        setCaseError(null);

        const response = (await apiClient.get(KB_ADMIN_API.LIST_EVAL_CASES(datasetId), {
          params: buildCaseParams(filters, page),
        })) as ListResponse<EvalCase>;

        setCases(response.items ?? []);
        setCaseTotal(response.total ?? 0);
        setCasePage(response.page ?? page);
      } catch (error) {
        setCases([]);
        setCaseTotal(0);
        setCaseError(normalizeError(error, '加载样本失败'));
      } finally {
        setCasesLoading(false);
      }
    },
    []
  );

  const updateSelectedDatasetInUrl = useCallback(
    (datasetId: number) => {
      const params = new URLSearchParams(searchParams.toString());
      params.set('dataset_id', String(datasetId));
      router.replace(`/evaluation/datasets?${params.toString()}`);
    },
    [router, searchParams]
  );

  const handleSelectDataset = useCallback(
    (datasetId: number) => {
      setSelectedDatasetId(datasetId);
      updateSelectedDatasetInUrl(datasetId);
    },
    [updateSelectedDatasetInUrl]
  );

  const handleValidateDataset = useCallback(
    async (dataset: EvalDataset) => {
      try {
        setValidatingDatasetId(dataset.id);
        const result = (await apiClient.post(
          KB_ADMIN_API.VALIDATE_EVAL_DATASET(dataset.id)
        )) as EvalDatasetValidationResult;

        setLastValidation(result);
        messageApi.success(`评测集 ${dataset.name} 校验完成`);
        await loadDatasets(datasetFilters, datasetPage);

        if (selectedDatasetId === dataset.id) {
          await loadCases(dataset.id, caseFilters, 1);
        }
      } catch (error) {
        messageApi.error(normalizeError(error, '评测集校验失败'));
      } finally {
        setValidatingDatasetId(null);
      }
    },
    [caseFilters, datasetFilters, datasetPage, loadCases, loadDatasets, messageApi, selectedDatasetId]
  );

  useEffect(() => {
    const nextFilters = { kb_id: selectedBase?.id } as DatasetFilterFormValues;
    datasetFilterForm.setFieldsValue(nextFilters);
    setDatasetFilters(nextFilters);
    void loadDatasets(nextFilters, 1);
  }, [datasetFilterForm, loadDatasets, selectedBase?.id]);

  useEffect(() => {
    if (!selectedDatasetId) {
      setCases([]);
      setCaseTotal(0);
      return;
    }

    void loadCases(selectedDatasetId, caseFilters, 1);
  }, [caseFilters, loadCases, selectedDatasetId]);

  const datasetColumns = useMemo<ColumnsType<EvalDataset>>(
    () => [
      {
        title: '评测集',
        key: 'name',
        render: (_, record) => (
          <Space direction="vertical" size={2}>
            <Button
              type="link"
              className="!h-auto !p-0 text-left"
              onClick={() => handleSelectDataset(record.id)}
            >
              {record.name}
            </Button>
            <Text type="secondary">{record.description || '暂无描述'}</Text>
          </Space>
        ),
      },
      {
        title: '知识库',
        dataIndex: 'kb_id',
        key: 'kb_id',
        width: 180,
        render: (value?: number) => {
          if (!value) {
            return <Text type="secondary">未绑定</Text>;
          }
          const base = bases.find((item) => item.id === value);
          return (
            <Space direction="vertical" size={0}>
              <Text>{base?.name ?? `#${value}`}</Text>
              {base ? <Text type="secondary">#{value}</Text> : null}
            </Space>
          );
        },
      },
      {
        title: '样本数',
        dataIndex: 'case_count',
        key: 'case_count',
        width: 100,
      },
      {
        title: '状态',
        dataIndex: 'status',
        key: 'status',
        width: 120,
        render: (value: EvalDatasetStatus) => (
          <Tag color={datasetStatusColor(value)}>{formatDatasetStatus(value)}</Tag>
        ),
      },
      {
        title: '更新时间',
        dataIndex: 'updated_at',
        key: 'updated_at',
        width: 180,
        render: (value: string) => formatTime(value),
      },
      {
        title: '操作',
        key: 'actions',
        width: 320,
        render: (_, record) => (
          <Space wrap onClick={(event) => event.stopPropagation()}>
            <Button size="small" onClick={() => handleSelectDataset(record.id)}>
              查看样本
            </Button>
            <Button
              size="small"
              loading={validatingDatasetId === record.id}
              onClick={() => void handleValidateDataset(record)}
            >
              校验
            </Button>
            <Button
              size="small"
              icon={<DownloadOutlined />}
              onClick={() => downloadDataset(record.id)}
            >
              导出
            </Button>
            <Link href={`/evaluation/runs?dataset_id=${record.id}`}>
              <Button
                size="small"
                type="primary"
                ghost
                icon={<PlayCircleOutlined />}
                disabled={record.status !== 'ready'}
              >
                创建运行
              </Button>
            </Link>
          </Space>
        ),
      },
    ],
    [bases, handleSelectDataset, handleValidateDataset, validatingDatasetId]
  );

  const caseColumns = useMemo<ColumnsType<EvalCase>>(
    () => [
      {
        title: 'Case Key',
        dataIndex: 'case_key',
        key: 'case_key',
        width: 180,
        render: (value: string) => <Text code>{value}</Text>,
      },
      {
        title: '查询',
        dataIndex: 'query',
        key: 'query',
        ellipsis: true,
      },
      {
        title: '查询类型',
        dataIndex: 'query_type',
        key: 'query_type',
        width: 140,
        render: (value?: string) => value || <Text type="secondary">-</Text>,
      },
      {
        title: 'Tags',
        dataIndex: 'tags',
        key: 'tags',
        width: 220,
        render: (value?: string[]) =>
          value?.length ? (
            <Space size={[4, 4]} wrap>
              {value.map((tag) => (
                <Tag key={tag}>{tag}</Tag>
              ))}
            </Space>
          ) : (
            <Text type="secondary">-</Text>
          ),
      },
      {
        title: 'Top K',
        dataIndex: 'top_k',
        key: 'top_k',
        width: 90,
      },
      {
        title: 'Relevant IDs',
        key: 'relevant_ids_count',
        width: 120,
        render: (_, record) => record.relevant_ids.length,
      },
      {
        title: 'Citation Targets',
        key: 'citation_targets_count',
        width: 140,
        render: (_, record) => record.citation_targets.length,
      },
      {
        title: '校验状态',
        dataIndex: 'validation_status',
        key: 'validation_status',
        width: 130,
        render: (value: EvalCaseValidationStatus) => (
          <Tag color={validationStatusColor(value)}>{formatValidationStatus(value)}</Tag>
        ),
      },
      {
        title: '更新时间',
        dataIndex: 'updated_at',
        key: 'updated_at',
        width: 180,
        render: (value?: string) => formatTime(value),
      },
    ],
    []
  );

  const handleCreateDataset = async () => {
    try {
      const values = await createDatasetForm.validateFields();
      setCreateDatasetSubmitting(true);

      const created = (await apiClient.post(KB_ADMIN_API.CREATE_EVAL_DATASET, {
        name: values.name.trim(),
        description: values.description?.trim() || undefined,
        kb_id: values.kb_id,
      })) as EvalDataset;

      setCreateDatasetOpen(false);
      createDatasetForm.resetFields();
      messageApi.success(`评测集 ${created.name} 已创建`);
      await loadDatasets(datasetFilters, 1);
      handleSelectDataset(created.id);
    } catch (error) {
      if (isFormValidationError(error)) {
        return;
      }
      messageApi.error(normalizeError(error, '创建评测集失败'));
    } finally {
      setCreateDatasetSubmitting(false);
    }
  };

  const handleCreateCase = async () => {
    if (!selectedDataset) {
      return;
    }

    try {
      const values = await createCaseForm.validateFields();
      setCreateCaseSubmitting(true);

      const payload = {
        case_key: values.case_key.trim(),
        query: values.query.trim(),
        top_k: values.top_k,
        query_type: values.query_type?.trim() || undefined,
        tags: parseStringList(values.tags_text),
        kb_ids: parseNumericList(values.kb_ids_text),
        relevant_ids: parseStringList(values.relevant_ids_text),
        citation_targets: parseCitationTargets(values.citation_targets_json),
        collection: values.collection?.trim() || undefined,
        notes: values.notes?.trim() || undefined,
      };

      await apiClient.post(KB_ADMIN_API.CREATE_EVAL_CASE(selectedDataset.id), payload);
      setCreateCaseOpen(false);
      createCaseForm.resetFields();
      messageApi.success(`样本 ${payload.case_key} 已添加到 ${selectedDataset.name}`);
      await loadDatasets(datasetFilters, datasetPage);
      await loadCases(selectedDataset.id, caseFilters, 1);
    } catch (error) {
      if (isFormValidationError(error)) {
        return;
      }
      messageApi.error(normalizeError(error, '新增样本失败'));
    } finally {
      setCreateCaseSubmitting(false);
    }
  };

  const handleImportCases = async () => {
    if (!selectedDataset) {
      return;
    }

    const currentFile = importFileList[0]?.originFileObj;
    if (!currentFile) {
      messageApi.warning('请先选择一个 JSON 文件');
      return;
    }

    try {
      setImportSubmitting(true);
      const formData = new FormData();
      formData.append('file', currentFile);

      const result = (await apiClient.post(
        KB_ADMIN_API.IMPORT_EVAL_CASES(selectedDataset.id),
        formData
      )) as EvalCaseImportResult;

      setImportResult(result);
      messageApi.success(`导入完成：成功 ${result.imported} 条，失败 ${result.failed} 条`);
      await loadDatasets(datasetFilters, datasetPage);
      await loadCases(selectedDataset.id, caseFilters, 1);
    } catch (error) {
      messageApi.error(normalizeError(error, '导入样本失败'));
    } finally {
      setImportSubmitting(false);
    }
  };

  return (
    <div className="space-y-6">
      {contextHolder}

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            评测集
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            管理离线评测数据集、样本导入导出与校验状态，为后续评测运行准备可回归的资产。
          </Paragraph>
        </div>
        <Space wrap>
          <Button icon={<ReloadOutlined />} onClick={() => void loadDatasets(datasetFilters, datasetPage)}>
            刷新
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateDatasetOpen(true)}>
            新建评测集
          </Button>
        </Space>
      </div>

      <Card>
        <Form
          form={datasetFilterForm}
          layout="vertical"
          onFinish={(values) => {
            setDatasetFilters(values);
            void loadDatasets(values, 1);
          }}
        >
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <Form.Item label="知识库" name="kb_id">
              <Select
                allowClear
                placeholder="全部知识库"
                options={bases.map((base) => ({ label: base.name, value: base.id }))}
              />
            </Form.Item>
            <Form.Item label="状态" name="status">
              <Select allowClear placeholder="全部状态" options={datasetStatusOptions} />
            </Form.Item>
            <Form.Item label="关键词" name="keyword" className="xl:col-span-2">
              <Input placeholder="按名称或描述搜索" />
            </Form.Item>
          </div>
          <Space>
            <Button type="primary" htmlType="submit" icon={<SearchOutlined />} loading={datasetsLoading}>
              查询评测集
            </Button>
            <Button
              onClick={() => {
                const nextFilters = { kb_id: selectedBase?.id } as DatasetFilterFormValues;
                datasetFilterForm.resetFields();
                datasetFilterForm.setFieldsValue(nextFilters);
                setDatasetFilters(nextFilters);
                void loadDatasets(nextFilters, 1);
              }}
            >
              重置
            </Button>
          </Space>
        </Form>
      </Card>

      {datasetError ? <Alert type="error" showIcon message={datasetError} /> : null}

      <Card title="评测集列表">
        {datasets.length === 0 && !datasetsLoading ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="当前筛选条件下还没有评测集。先创建一个评测集，再导入样本。"
          >
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateDatasetOpen(true)}>
              新建评测集
            </Button>
          </Empty>
        ) : (
          <Table<EvalDataset>
            rowKey="id"
            loading={datasetsLoading}
            columns={datasetColumns}
            dataSource={datasets}
            pagination={{
              current: datasetPage,
              pageSize: DATASET_PAGE_SIZE,
              total: datasetTotal,
              onChange: (page) => void loadDatasets(datasetFilters, page),
            }}
            rowClassName={(record) => (record.id === selectedDatasetId ? 'bg-slate-50' : '')}
            onRow={(record) => ({
              onClick: () => handleSelectDataset(record.id),
              style: { cursor: 'pointer' },
            })}
          />
        )}
      </Card>

      <Card
        id="evaluation-cases-section"
        title={`样本管理 · ${selectedDataset?.name ?? '未选择评测集'}`}
        extra={
          selectedDataset ? (
            <Space wrap>
              <Button icon={<PlusOutlined />} onClick={() => setCreateCaseOpen(true)}>
                新增样本
              </Button>
              <Button icon={<UploadOutlined />} onClick={() => setImportOpen(true)}>
                导入 JSON
              </Button>
              <Button
                loading={validatingDatasetId === selectedDataset.id}
                onClick={() => void handleValidateDataset(selectedDataset)}
              >
                校验评测集
              </Button>
              <Button icon={<DownloadOutlined />} onClick={() => downloadDataset(selectedDataset.id)}>
                导出 JSON
              </Button>
              <Link href={`/evaluation/runs?dataset_id=${selectedDataset.id}`}>
                <Button
                  type="primary"
                  icon={<PlayCircleOutlined />}
                  disabled={selectedDataset.status !== 'ready'}
                >
                  创建运行
                </Button>
              </Link>
            </Space>
          ) : null
        }
      >
        {!selectedDataset ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="先从上面的列表中选择一个评测集，再继续管理样本。"
          >
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateDatasetOpen(true)}>
              先创建评测集
            </Button>
          </Empty>
        ) : (
          <Space direction="vertical" size="large" className="w-full">
            <Descriptions bordered column={2} size="small">
              <Descriptions.Item label="评测集名称">{selectedDataset.name}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={datasetStatusColor(selectedDataset.status)}>
                  {formatDatasetStatus(selectedDataset.status)}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="知识库">
                {selectedDataset.kb_id
                  ? bases.find((base) => base.id === selectedDataset.kb_id)?.name ?? `#${selectedDataset.kb_id}`
                  : '未绑定'}
              </Descriptions.Item>
              <Descriptions.Item label="样本数">{selectedDataset.case_count}</Descriptions.Item>
              <Descriptions.Item label="描述" span={2}>
                {selectedDataset.description || '暂无描述'}
              </Descriptions.Item>
            </Descriptions>

            {lastValidation?.dataset_id === selectedDataset.id ? (
              <Alert
                type={lastValidation.status === 'ready' ? 'success' : 'warning'}
                showIcon
                message={`最近一次校验结果：${formatDatasetStatus(lastValidation.status)}`}
                description={
                  <Space wrap size="large">
                    <Text>总样本 {lastValidation.case_count}</Text>
                    <Text>有效 {lastValidation.valid_count}</Text>
                    <Text>无效 {lastValidation.invalid_count}</Text>
                    <Text>未检查 {lastValidation.unchecked_count}</Text>
                  </Space>
                }
              />
            ) : null}

            <Form
              form={caseFilterForm}
              layout="vertical"
              onFinish={(values) => {
                setCaseFilters(values);
                void loadCases(selectedDataset.id, values, 1);
              }}
            >
              <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                <Form.Item label="查询类型" name="query_type">
                  <Input placeholder="例如 factual / multi-hop" />
                </Form.Item>
                <Form.Item label="标签" name="tag">
                  <Input placeholder="按单个标签筛选" />
                </Form.Item>
                <Form.Item label="校验状态" name="validation_status">
                  <Select allowClear placeholder="全部状态" options={validationStatusOptions} />
                </Form.Item>
                <Form.Item label="关键词" name="keyword">
                  <Input placeholder="匹配 case_key / query / notes" />
                </Form.Item>
              </div>
              <Space>
                <Button type="primary" htmlType="submit" icon={<SearchOutlined />} loading={casesLoading}>
                  查询样本
                </Button>
                <Button
                  onClick={() => {
                    caseFilterForm.resetFields();
                    const nextFilters = {} as CaseFilterFormValues;
                    setCaseFilters(nextFilters);
                    void loadCases(selectedDataset.id, nextFilters, 1);
                  }}
                >
                  重置
                </Button>
              </Space>
            </Form>

            {caseError ? <Alert type="error" showIcon message={caseError} /> : null}

            {cases.length === 0 && !casesLoading ? (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="当前评测集下还没有样本。先手动新增，或直接导入一批 JSON 样本。"
              >
                <Space wrap>
                  <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateCaseOpen(true)}>
                    新增样本
                  </Button>
                  <Button icon={<UploadOutlined />} onClick={() => setImportOpen(true)}>
                    导入 JSON
                  </Button>
                </Space>
              </Empty>
            ) : (
              <Table<EvalCase>
                rowKey="id"
                loading={casesLoading}
                columns={caseColumns}
                dataSource={cases}
                expandable={{
                  rowExpandable: (record) =>
                    (record.validation_errors?.length ?? 0) > 0 ||
                    record.relevant_ids.length > 0 ||
                    record.citation_targets.length > 0 ||
                    Boolean(record.notes) ||
                    (record.kb_ids?.length ?? 0) > 0,
                  expandedRowRender: (record) => (
                    <Space direction="vertical" size="middle" className="w-full">
                      {(record.validation_errors?.length ?? 0) > 0 ? (
                        <Alert
                          type="error"
                          showIcon
                          message="校验错误"
                          description={
                            <ul className="mb-0 pl-5">
                              {record.validation_errors?.map((item) => (
                                <li key={item}>{item}</li>
                              ))}
                            </ul>
                          }
                        />
                      ) : null}
                      <Descriptions bordered column={1} size="small">
                        <Descriptions.Item label="Relevant IDs">
                          {record.relevant_ids.length ? record.relevant_ids.join('\n') : '-'}
                        </Descriptions.Item>
                        <Descriptions.Item label="Citation Targets">
                          {record.citation_targets.length ? (
                            <pre className="mb-0 whitespace-pre-wrap rounded bg-slate-50 p-3 text-xs">
                              {JSON.stringify(record.citation_targets, null, 2)}
                            </pre>
                          ) : (
                            '-'
                          )}
                        </Descriptions.Item>
                        <Descriptions.Item label="KB IDs">
                          {record.kb_ids?.length ? record.kb_ids.join(', ') : '-'}
                        </Descriptions.Item>
                        <Descriptions.Item label="Collection">
                          {record.collection || '-'}
                        </Descriptions.Item>
                        <Descriptions.Item label="Notes">{record.notes || '-'}</Descriptions.Item>
                      </Descriptions>
                    </Space>
                  ),
                }}
                pagination={{
                  current: casePage,
                  pageSize: CASE_PAGE_SIZE,
                  total: caseTotal,
                  onChange: (page) => void loadCases(selectedDataset.id, caseFilters, page),
                }}
              />
            )}
          </Space>
        )}
      </Card>

      <Modal
        title="新建评测集"
        open={createDatasetOpen}
        destroyOnClose
        confirmLoading={createDatasetSubmitting}
        onCancel={() => {
          setCreateDatasetOpen(false);
          createDatasetForm.resetFields();
        }}
        onOk={() => void handleCreateDataset()}
      >
        <Form form={createDatasetForm} layout="vertical" preserve={false}>
          <Form.Item label="名称" name="name" rules={[{ required: true, message: '请输入评测集名称' }]}>
            <Input placeholder="例如 phase2-core-regression" />
          </Form.Item>
          <Form.Item label="描述" name="description">
            <TextArea rows={3} placeholder="补充评测集用途、范围或维护说明" />
          </Form.Item>
          <Form.Item label="绑定知识库" name="kb_id">
            <Select
              allowClear
              placeholder="可选，绑定后校验会检查样本 kb_ids"
              options={bases.map((base) => ({ label: base.name, value: base.id }))}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={selectedDataset ? `新增样本 · ${selectedDataset.name}` : '新增样本'}
        open={createCaseOpen}
        width={760}
        destroyOnClose
        confirmLoading={createCaseSubmitting}
        onCancel={() => {
          setCreateCaseOpen(false);
          createCaseForm.resetFields();
        }}
        onOk={() => void handleCreateCase()}
      >
        <Form form={createCaseForm} layout="vertical" preserve={false} initialValues={{ top_k: 5 }}>
          <div className="grid gap-4 md:grid-cols-2">
            <Form.Item label="Case Key" name="case_key" rules={[{ required: true, message: '请输入 case_key' }]}>
              <Input placeholder="例如 faq-login-001" />
            </Form.Item>
            <Form.Item label="Top K" name="top_k" rules={[{ required: true, message: '请输入 top_k' }]}>
              <InputNumber min={1} precision={0} className="w-full" />
            </Form.Item>
          </div>

          <Form.Item label="查询" name="query" rules={[{ required: true, message: '请输入查询内容' }]}>
            <TextArea rows={3} placeholder="输入用于检索回归的查询" />
          </Form.Item>

          <div className="grid gap-4 md:grid-cols-2">
            <Form.Item label="查询类型" name="query_type">
              <Input placeholder="例如 factual / multi-hop" />
            </Form.Item>
            <Form.Item label="Collection" name="collection">
              <Input placeholder="可选 collection 名称" />
            </Form.Item>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <Form.Item label="Tags" name="tags_text">
              <TextArea rows={3} placeholder="逗号或换行分隔标签，例如：core, faq" />
            </Form.Item>
            <Form.Item label="KB IDs" name="kb_ids_text">
              <TextArea rows={3} placeholder="逗号或换行分隔知识库 ID，例如：1,2" />
            </Form.Item>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <Form.Item label="Relevant IDs" name="relevant_ids_text">
              <TextArea rows={5} placeholder="一行一个 ID，也支持逗号分隔" />
            </Form.Item>
            <Form.Item label="Citation Targets JSON" name="citation_targets_json">
              <TextArea
                rows={5}
                placeholder='例如: [{"document_id":1,"chunk_id":"chunk-1","file_name":"faq.md"}]'
              />
            </Form.Item>
          </div>

          <Form.Item label="Notes" name="notes">
            <TextArea rows={4} placeholder="补充样本背景、预期或人工标注说明" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={selectedDataset ? `导入样本 · ${selectedDataset.name}` : '导入样本'}
        open={importOpen}
        destroyOnClose
        confirmLoading={importSubmitting}
        onCancel={() => {
          setImportOpen(false);
          setImportFileList([]);
          setImportResult(null);
        }}
        onOk={() => void handleImportCases()}
      >
        <Space direction="vertical" size="large" className="w-full">
          <Alert
            type="info"
            showIcon
            message="导入格式"
            description="上传与 backend/scripts/evaluation/dataset.json 兼容的 JSON 数组。case_key 将取自每条记录的 id 字段。"
          />
          <Upload
            accept=".json,application/json"
            beforeUpload={(file) => {
              setImportFileList([file]);
              return false;
            }}
            onRemove={() => setImportFileList([])}
            fileList={importFileList}
            maxCount={1}
          >
            <Button icon={<UploadOutlined />}>选择 JSON 文件</Button>
          </Upload>

          {importResult ? (
            <Alert
              type={importResult.failed > 0 ? 'warning' : 'success'}
              showIcon
              message={`导入结果：成功 ${importResult.imported} 条，失败 ${importResult.failed} 条`}
              description={
                importResult.errors.length ? (
                  <ul className="mb-0 pl-5">
                    {importResult.errors.map((item) => (
                      <li key={`${item.index}-${item.case_key ?? 'unknown'}`}>
                        第 {item.index + 1} 条{item.case_key ? `（${item.case_key}）` : ''}：{item.message}
                      </li>
                    ))}
                  </ul>
                ) : null
              }
            />
          ) : null}
        </Space>
      </Modal>
    </div>
  );
}
