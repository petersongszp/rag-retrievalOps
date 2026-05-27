'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  CopyOutlined,
  PlusOutlined,
  SaveOutlined,
  SearchOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Collapse,
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
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { KB_ADMIN_API } from '@/config/api';
import apiClient from '@/services/api/client';
import type { EvalDataset, ListResponse, RetrieveItem, RetrieveResponse } from '@/types/kb';
import { useKnowledgeBaseContext } from './knowledge-base-provider';

const { Title, Paragraph, Text } = Typography;

interface ContractGapEntry {
  field: string;
  interface: string;
  affectedPage: string;
  blocksAcceptance: boolean;
}

type RetrieveFormValues = {
  kb_id: number;
  query: string;
  top_k: number;
};

type SaveCaseFormValues = {
  dataset_id: number;
  case_key: string;
  query: string;
  top_k: number;
  query_type?: string;
  tags_text?: string;
  relevant_ids_text?: string;
  notes?: string;
  collection?: string;
};

const FIELD_META: Record<string, Omit<ContractGapEntry, 'field'>> = {
  score: {
    interface: 'POST /admin/kb/retrieve -> items[].score',
    affectedPage: '检索实验室',
    blocksAcceptance: true,
  },
  citation: {
    interface: 'POST /admin/kb/retrieve -> items[].citation',
    affectedPage: '检索实验室',
    blocksAcceptance: true,
  },
  'citation.file_name': {
    interface: 'POST /admin/kb/retrieve -> items[].citation.file_name',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
  'citation.chunk_index': {
    interface: 'POST /admin/kb/retrieve -> items[].citation.chunk_index',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
  'citation.chunk_id': {
    interface: 'POST /admin/kb/retrieve -> items[].citation.chunk_id',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
  source: {
    interface: 'POST /admin/kb/retrieve -> items[].source',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
  'source.route': {
    interface: 'POST /admin/kb/retrieve -> items[].source.route',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
  'source.collection': {
    interface: 'POST /admin/kb/retrieve -> items[].source.collection',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
  'source.retriever_version': {
    interface: 'POST /admin/kb/retrieve -> items[].source.retriever_version',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
};

const gapColumns: ColumnsType<ContractGapEntry> = [
  {
    title: '缺失字段',
    dataIndex: 'field',
    key: 'field',
    render: (value: string) => <Text code>{value}</Text>,
  },
  { title: '所属接口', dataIndex: 'interface', key: 'interface' },
  { title: '影响页面', dataIndex: 'affectedPage', key: 'affectedPage' },
  {
    title: '阻塞验收',
    dataIndex: 'blocksAcceptance',
    key: 'blocksAcceptance',
    render: (value: boolean) =>
      value ? <Tag color="error">是</Tag> : <Tag color="default">否</Tag>,
  },
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

function findContractGaps(item: RetrieveItem): string[] {
  const gaps: string[] = [];

  if (item.score === undefined || item.score === null) {
    gaps.push('score');
  }
  if (!item.citation) {
    gaps.push('citation');
  } else {
    if (!item.citation.file_name) {
      gaps.push('citation.file_name');
    }
    if (item.citation.chunk_index === undefined || item.citation.chunk_index === null) {
      gaps.push('citation.chunk_index');
    }
    if (!item.citation.chunk_id) {
      gaps.push('citation.chunk_id');
    }
  }

  if (!item.source) {
    gaps.push('source');
  } else {
    if (!item.source.route) {
      gaps.push('source.route');
    }
    if (!item.source.collection) {
      gaps.push('source.collection');
    }
    if (!item.source.retriever_version) {
      gaps.push('source.retriever_version');
    }
  }

  return gaps;
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

function buildRelevantIdDraft(items: RetrieveItem[]): string {
  const seen = new Set<string>();
  const chunkIds = items
    .map((item) => item.citation?.chunk_id)
    .filter((value): value is string => Boolean(value))
    .filter((value) => {
      if (seen.has(value)) {
        return false;
      }
      seen.add(value);
      return true;
    });

  return chunkIds.join('\n');
}

function buildDefaultCaseKey() {
  const stamp = new Date().toISOString().replace(/[-:TZ.]/g, '').slice(0, 14);
  return `retrieval-lab-${stamp}`;
}

export function RetrievalLabPage() {
  const router = useRouter();
  const { selectedBase, setSelectedBaseId, bases } = useKnowledgeBaseContext();
  const [messageApi, contextHolder] = message.useMessage();

  const [retrieveForm] = Form.useForm<RetrieveFormValues>();
  const [saveCaseForm] = Form.useForm<SaveCaseFormValues>();

  const [result, setResult] = useState<RetrieveResponse | null>(null);
  const [lastRetrieveValues, setLastRetrieveValues] = useState<RetrieveFormValues | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [datasets, setDatasets] = useState<EvalDataset[]>([]);
  const [datasetsLoading, setDatasetsLoading] = useState(false);
  const [datasetsError, setDatasetsError] = useState<string | null>(null);
  const [saveOpen, setSaveOpen] = useState(false);
  const [saveSubmitting, setSaveSubmitting] = useState(false);

  useEffect(() => {
    if (selectedBase?.id) {
      retrieveForm.setFieldsValue({ kb_id: selectedBase.id });
    }
  }, [retrieveForm, selectedBase?.id]);

  const gapLog = useMemo<ContractGapEntry[]>(() => {
    if (!result) {
      return [];
    }

    const seen = new Set<string>();
    const entries: ContractGapEntry[] = [];

    for (const item of result.items) {
      for (const field of findContractGaps(item)) {
        if (seen.has(field)) {
          continue;
        }
        seen.add(field);
        const meta = FIELD_META[field];
        if (meta) {
          entries.push({ field, ...meta });
        }
      }
    }

    return entries;
  }, [result]);

  const loadDatasets = useCallback(async () => {
    try {
      setDatasetsLoading(true);
      setDatasetsError(null);

      const response = (await apiClient.get(KB_ADMIN_API.LIST_EVAL_DATASETS, {
        params: { page: 1, page_size: 100 },
      })) as ListResponse<EvalDataset>;

      setDatasets(response.items ?? []);
      return response.items ?? [];
    } catch (loadError) {
      setDatasets([]);
      const nextError = normalizeError(loadError, '加载评测集失败');
      setDatasetsError(nextError);
      return [];
    } finally {
      setDatasetsLoading(false);
    }
  }, []);

  const handleRetrieve = async (values: RetrieveFormValues) => {
    try {
      setLoading(true);
      setError(null);
      setSelectedBaseId(values.kb_id);

      const response = (await apiClient.post(KB_ADMIN_API.RETRIEVE, values)) as RetrieveResponse;
      setResult(response);
      setLastRetrieveValues(values);
    } catch (err) {
      setError(normalizeError(err, '检索请求失败'));
      setResult(null);
      setLastRetrieveValues(null);
    } finally {
      setLoading(false);
    }
  };

  const openSaveModal = async () => {
    if (!result || !lastRetrieveValues) {
      return;
    }

    const items = datasets.length ? datasets : await loadDatasets();
    const firstCollection = result.items.find((item) => item.source?.collection)?.source.collection;

    saveCaseForm.setFieldsValue({
      dataset_id: items[0]?.id,
      case_key: buildDefaultCaseKey(),
      query: lastRetrieveValues.query,
      top_k: lastRetrieveValues.top_k,
      query_type: undefined,
      tags_text: undefined,
      relevant_ids_text: undefined,
      notes: undefined,
      collection: firstCollection,
    });
    setSaveOpen(true);
  };

  const handleSaveCase = async () => {
    try {
      const values = await saveCaseForm.validateFields();
      setSaveSubmitting(true);

      await apiClient.post(KB_ADMIN_API.CREATE_EVAL_CASE(values.dataset_id), {
        case_key: values.case_key.trim(),
        query: values.query.trim(),
        top_k: values.top_k,
        query_type: values.query_type?.trim() || undefined,
        tags: parseStringList(values.tags_text),
        relevant_ids: parseStringList(values.relevant_ids_text),
        collection: values.collection?.trim() || undefined,
        notes: values.notes?.trim() || undefined,
      });

      setSaveOpen(false);
      saveCaseForm.resetFields();
      messageApi.success('样本已保存，请前往评测集页面补齐标准答案并执行校验。');
    } catch (error) {
      messageApi.error(normalizeError(error, '保存评测样本失败'));
    } finally {
      setSaveSubmitting(false);
    }
  };

  return (
    <div className="space-y-6">
      {contextHolder}

      <div>
        <Title level={2} style={{ marginBottom: 8 }}>
          检索实验室
        </Title>
        <Paragraph type="secondary" style={{ marginBottom: 0 }}>
          运行检索测试、检查 request_id 与契约字段，并把一次检索沉淀为评测样本草稿。
        </Paragraph>
      </div>

      <Card>
        <Form
          form={retrieveForm}
          layout="vertical"
          initialValues={{ kb_id: selectedBase?.id, top_k: 5 }}
          onFinish={(values) => void handleRetrieve(values)}
        >
          <Form.Item
            label="知识库"
            name="kb_id"
            rules={[{ required: true, message: '请选择知识库' }]}
          >
            <Select
              options={bases.map((base) => ({ label: base.name, value: base.id }))}
              placeholder="选择知识库"
            />
          </Form.Item>
          <Form.Item
            label="查询"
            name="query"
            rules={[{ required: true, message: '请输入检索查询内容' }]}
          >
            <Input.TextArea rows={4} placeholder="输入针对当前知识库的检索问题" />
          </Form.Item>
          <Form.Item label="Top K" name="top_k">
            <Select
              options={[
                { label: '3', value: 3 },
                { label: '5', value: 5 },
                { label: '10', value: 10 },
              ]}
            />
          </Form.Item>
          <Button type="primary" htmlType="submit" icon={<SearchOutlined />} loading={loading}>
            运行检索测试
          </Button>
        </Form>
      </Card>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      {!result ? (
        <Card>
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              selectedBase
                ? `对 ${selectedBase.name} 运行测试，检查 request_id、score、citation 与 source 字段。`
                : '请选择知识库并运行检索测试。'
            }
          />
        </Card>
      ) : (
        <Space direction="vertical" size="large" className="w-full">
          <Card>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <Space direction="vertical" size={4}>
                <Text strong>Request ID</Text>
                <Text code>{result.request_id || 'Contract gap: request_id'}</Text>
              </Space>
              <Space wrap>
                <Button
                  icon={<CopyOutlined />}
                  onClick={async () => {
                    if (!result.request_id) {
                      messageApi.warning('后端未返回 request_id');
                      return;
                    }
                    await navigator.clipboard.writeText(result.request_id);
                    messageApi.success('request_id 已复制');
                  }}
                >
                  复制 request_id
                </Button>
                <Button
                  disabled={!result.request_id}
                  onClick={() => {
                    if (!result.request_id) {
                      return;
                    }
                    router.push(
                      `/trace-logs/retrieval?request_id=${encodeURIComponent(result.request_id)}`
                    );
                  }}
                >
                  查看 Trace
                </Button>
                <Button type="primary" icon={<SaveOutlined />} onClick={() => void openSaveModal()}>
                  保存为评测样本
                </Button>
              </Space>
            </div>
          </Card>

          {result.items.length === 0 ? (
            <Card>
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="未返回检索结果。" />
            </Card>
          ) : (
            result.items.map((item, index) => {
              const gaps = findContractGaps(item);

              return (
                <Card key={`${item.citation?.chunk_id ?? 'row'}-${index}`}>
                  <Space direction="vertical" size={12} className="w-full">
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <Tag color="blue">结果 {index + 1}</Tag>
                      <Text>Score: {item.score ?? 'Contract gap'}</Text>
                    </div>
                    <Text>{item.content}</Text>
                    <Space wrap>
                      <Tag>文件：{item.citation?.file_name || 'Contract gap'}</Tag>
                      <Tag>Chunk Index：{item.citation?.chunk_index ?? 'Contract gap'}</Tag>
                      <Tag>Chunk ID：{item.citation?.chunk_id || 'Contract gap'}</Tag>
                      <Tag>Route：{item.source?.route || 'Contract gap'}</Tag>
                      <Tag>Collection：{item.source?.collection || 'Contract gap'}</Tag>
                      <Tag>Retriever：{item.source?.retriever_version || 'Contract gap'}</Tag>
                    </Space>
                    {gaps.length > 0 ? (
                      <Alert type="warning" showIcon message={`Contract gaps: ${gaps.join(', ')}`} />
                    ) : null}
                  </Space>
                </Card>
              );
            })
          )}

          {gapLog.length > 0 ? (
            <Collapse
              items={[
                {
                  key: 'gap-log',
                  label: (
                    <Space>
                      <WarningOutlined style={{ color: '#faad14' }} />
                      <Text>契约缺口记录（{gapLog.length} 项）</Text>
                    </Space>
                  ),
                  children: (
                    <Table<ContractGapEntry>
                      rowKey="field"
                      size="small"
                      columns={gapColumns}
                      dataSource={gapLog}
                      pagination={false}
                    />
                  ),
                },
              ]}
            />
          ) : null}
        </Space>
      )}

      <Modal
        title="保存为评测样本"
        open={saveOpen}
        width={720}
        destroyOnClose
        confirmLoading={saveSubmitting}
        onCancel={() => {
          setSaveOpen(false);
          saveCaseForm.resetFields();
        }}
        onOk={() => void handleSaveCase()}
      >
        <Space direction="vertical" size="large" className="w-full">
          {datasetsError ? <Alert type="error" showIcon message={datasetsError} /> : null}
          {datasets.length === 0 && !datasetsLoading ? (
            <Alert
              type="warning"
              showIcon
              message="还没有可用评测集"
              description={
                <Space>
                  <Text>请先创建评测集，再把检索结果沉淀为样本。</Text>
                  <Link href="/evaluation/datasets">
                    <Button size="small" type="link" icon={<PlusOutlined />}>
                      去创建评测集
                    </Button>
                  </Link>
                </Space>
              }
            />
          ) : null}

          <Form form={saveCaseForm} layout="vertical" preserve={false}>
            <Form.Item
              label="评测集"
              name="dataset_id"
              rules={[{ required: true, message: '请选择评测集' }]}
            >
              <Select
                showSearch
                loading={datasetsLoading}
                optionFilterProp="label"
                placeholder="选择要保存到的评测集"
                options={datasets.map((dataset) => ({
                  label: `${dataset.name} (#${dataset.id}) - ${dataset.status}`,
                  value: dataset.id,
                }))}
              />
            </Form.Item>

            <div className="grid gap-4 md:grid-cols-2">
              <Form.Item
                label="Case Key"
                name="case_key"
                rules={[{ required: true, message: '请输入 case_key' }]}
              >
                <Input placeholder="例如 retrieval-lab-20260526183000" />
              </Form.Item>
              <Form.Item
                label="Top K"
                name="top_k"
                rules={[{ required: true, message: '请输入 top_k' }]}
              >
                <InputNumber min={1} precision={0} className="w-full" />
              </Form.Item>
            </div>

            <Form.Item
              label="Query"
              name="query"
              rules={[{ required: true, message: '请输入 query' }]}
            >
              <Input.TextArea rows={3} />
            </Form.Item>

            <div className="grid gap-4 md:grid-cols-2">
              <Form.Item label="Query Type" name="query_type">
                <Input placeholder="例如 factual / multi-hop" />
              </Form.Item>
              <Form.Item label="Collection" name="collection">
                <Input placeholder="可选，默认取当前检索结果的 collection" />
              </Form.Item>
            </div>

            <Form.Item label="Tags" name="tags_text">
              <Input.TextArea rows={2} placeholder="逗号或换行分隔标签" />
            </Form.Item>

            <Form.Item label="Relevant IDs 草稿" name="relevant_ids_text">
              <Input.TextArea
                rows={4}
                placeholder="可留空。只有你确认后才应当作为 golden answer 使用。"
              />
            </Form.Item>

            <Button
              className="mb-4"
              onClick={() => {
                if (!result) {
                  return;
                }
                saveCaseForm.setFieldValue('relevant_ids_text', buildRelevantIdDraft(result.items));
                messageApi.success('已把当前结果中的 chunk_id 复制到 relevant_ids 草稿区');
              }}
            >
              复制当前结果 chunk_id 到 relevant_ids 草稿
            </Button>

            <Form.Item label="Notes" name="notes">
              <Input.TextArea
                rows={4}
                placeholder="这里保存的是评测样本草稿，后续仍需补齐标准答案并执行校验。"
              />
            </Form.Item>
          </Form>
        </Space>
      </Modal>
    </div>
  );
}
