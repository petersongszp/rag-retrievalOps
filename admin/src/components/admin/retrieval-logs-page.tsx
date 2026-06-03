'use client';

import { useEffect, useMemo, useState } from 'react';
import dayjs, { type Dayjs } from 'dayjs';
import { useRouter, useSearchParams } from 'next/navigation';
import { ReloadOutlined, SearchOutlined } from '@ant-design/icons';
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
  Select,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';
import type { KBRetrieveLog, ListResponse, RetrieveResultStatus } from '@/types/kb';
import { useKnowledgeBaseContext } from './knowledge-base-provider';

const { Paragraph, Text, Title } = Typography;

type RetrievalLogFormValues = {
  kb_id?: number;
  result_status?: RetrieveResultStatus;
  request_id?: string;
  query_keyword?: string;
  range?: [Dayjs, Dayjs];
};

type RetrievalLogFilters = {
  kb_id?: number;
  result_status?: RetrieveResultStatus;
  request_id?: string;
  query_keyword?: string;
  start_time?: string;
  end_time?: string;
};

const PAGE_SIZE = 20;

type RetrievalLogWithP3Summary = KBRetrieveLog & {
  topk_policy_version?: string;
};

const statusOptions: Array<{ label: string; value: RetrieveResultStatus }> = [
  { label: '成功', value: 'success' },
  { label: '无结果', value: 'no_result' },
  { label: '被过滤', value: 'filtered_out' },
  { label: '错误', value: 'error' },
  { label: '超时', value: 'timeout' },
];

function buildFilters(values: RetrievalLogFormValues): RetrievalLogFilters {
  return {
    kb_id: values.kb_id,
    result_status: values.result_status,
    request_id: values.request_id?.trim() || undefined,
    query_keyword: values.query_keyword?.trim() || undefined,
    start_time: values.range?.[0]?.toISOString(),
    end_time: values.range?.[1]?.toISOString(),
  };
}

function buildParams(filters: RetrievalLogFilters, page: number) {
  return {
    page,
    page_size: PAGE_SIZE,
    ...(filters.kb_id ? { kb_id: filters.kb_id } : {}),
    ...(filters.result_status ? { result_status: filters.result_status } : {}),
    ...(filters.request_id ? { request_id: filters.request_id } : {}),
    ...(filters.query_keyword ? { query_keyword: filters.query_keyword } : {}),
    ...(filters.start_time ? { start_time: filters.start_time } : {}),
    ...(filters.end_time ? { end_time: filters.end_time } : {}),
  };
}

function statusColor(status: RetrieveResultStatus): string {
  switch (status) {
    case 'success':
      return 'success';
    case 'no_result':
      return 'gold';
    case 'filtered_out':
      return 'purple';
    case 'error':
      return 'error';
    case 'timeout':
      return 'volcano';
    default:
      return 'default';
  }
}

function renderField(value: string | number | boolean | null | undefined) {
  if (value === null || value === undefined || value === '') {
    return <Tag color="warning">Contract gap</Tag>;
  }
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }
  return String(value);
}

function buildRetrievalDebugHref(requestId: string) {
  return `/retrieval-lab/debug?request_id=${encodeURIComponent(requestId)}`;
}

function hasP3Summary(detail: RetrievalLogWithP3Summary | null) {
  if (!detail) {
    return false;
  }

  return (
    detail.parent_child_enabled !== undefined ||
    Boolean(detail.topk_policy_version) ||
    Boolean(detail.evidence_gate_result) ||
    detail.citation_support_score !== undefined ||
    Boolean(detail.refusal_reason)
  );
}

export function RetrievalLogsPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { bases, selectedBase } = useKnowledgeBaseContext();
  const [form] = Form.useForm<RetrievalLogFormValues>();
  const [filters, setFilters] = useState<RetrievalLogFilters>({});
  const [items, setItems] = useState<KBRetrieveLog[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [detail, setDetail] = useState<RetrievalLogWithP3Summary | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

  const columns = useMemo<ColumnsType<KBRetrieveLog>>(
    () => [
      {
        title: 'Query',
        dataIndex: 'query',
        key: 'query',
        ellipsis: true,
        render: (value: string) => value || <Tag color="warning">Contract gap</Tag>,
      },
      {
        title: 'Request ID',
        dataIndex: 'request_id',
        key: 'request_id',
        width: 180,
        ellipsis: true,
        render: (value: string) => <Text code>{value || 'Contract gap'}</Text>,
      },
      {
        title: 'KB',
        dataIndex: 'kb_ids',
        key: 'kb_ids',
        width: 120,
        render: (value: string) => value || <Tag color="warning">Contract gap</Tag>,
      },
      {
        title: 'Top K',
        dataIndex: 'top_k',
        key: 'top_k',
        width: 90,
      },
      {
        title: 'Final',
        dataIndex: 'final_count',
        key: 'final_count',
        width: 90,
      },
      {
        title: 'Duration',
        dataIndex: 'duration_ms',
        key: 'duration_ms',
        width: 110,
        render: (value: number) => `${value ?? 0} ms`,
      },
      {
        title: 'Status',
        dataIndex: 'result_status',
        key: 'result_status',
        width: 110,
        render: (value: RetrieveResultStatus) => <Tag color={statusColor(value)}>{value}</Tag>,
      },
      {
        title: 'Created',
        dataIndex: 'created_at',
        key: 'created_at',
        width: 190,
        render: (value: string) => dayjs(value).format('YYYY-MM-DD HH:mm:ss'),
      },
      {
        title: '操作',
        key: 'actions',
        width: 120,
        render: (_, record) => (
          <Button
            type="link"
            size="small"
            disabled={!record.request_id}
            onClick={(event) => {
              event.stopPropagation();
              if (!record.request_id) {
                return;
              }
              router.push(buildRetrievalDebugHref(record.request_id));
            }}
          >
            调试视图
          </Button>
        ),
      },
    ],
    [router]
  );

  const loadList = async (nextFilters: RetrievalLogFilters, nextPage: number) => {
    try {
      setIsLoading(true);
      setError(null);
      const data = (await apiClient.get(KB_ADMIN_API.LIST_RETRIEVE_AUDIT_LOGS, {
        params: buildParams(nextFilters, nextPage),
      })) as ListResponse<KBRetrieveLog>;
      setItems(data.items ?? []);
      setTotal(data.total ?? 0);
      setPage(data.page ?? nextPage);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载检索日志失败');
      setItems([]);
      setTotal(0);
    } finally {
      setIsLoading(false);
    }
  };

  const openDetail = async (requestId: string) => {
    try {
      setDetailOpen(true);
      setDetailLoading(true);
      setDetailError(null);
      const data = (await apiClient.get(
        KB_ADMIN_API.GET_RETRIEVE_AUDIT_LOG(requestId)
      )) as RetrievalLogWithP3Summary;
      setDetail(data);
    } catch (err) {
      setDetailError(err instanceof Error ? err.message : '加载检索详情失败');
      setDetail(null);
    } finally {
      setDetailLoading(false);
    }
  };

  useEffect(() => {
    const requestId = searchParams.get('request_id') ?? undefined;
    const kbIdParam = searchParams.get('kb_id');
    const statusParam = searchParams.get('result_status') as RetrieveResultStatus | null;
    const nextFilters: RetrievalLogFilters = {
      request_id: requestId,
      kb_id: kbIdParam ? Number(kbIdParam) : selectedBase?.id,
      result_status: statusParam ?? undefined,
    };
    setFilters(nextFilters);
    form.setFieldsValue({
      kb_id: nextFilters.kb_id,
      request_id: nextFilters.request_id,
      result_status: nextFilters.result_status,
      query_keyword: undefined,
      range: undefined,
    });
    void loadList(nextFilters, 1);
  }, [form, searchParams, selectedBase?.id]);

  const syncUrl = (nextFilters: RetrievalLogFilters) => {
    const params = new URLSearchParams();
    if (nextFilters.request_id) {
      params.set('request_id', nextFilters.request_id);
    }
    if (nextFilters.kb_id) {
      params.set('kb_id', String(nextFilters.kb_id));
    }
    if (nextFilters.result_status) {
      params.set('result_status', nextFilters.result_status);
    }
    router.replace(
      params.toString() ? `/trace-logs/retrieval?${params.toString()}` : '/trace-logs/retrieval'
    );
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            检索日志
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            查看结构化检索日志，按 request_id、知识库、状态和时间范围筛选，并下钻单次 trace 详情。
          </Paragraph>
        </div>
        <Button icon={<ReloadOutlined />} onClick={() => void loadList(filters, page)}>
          刷新
        </Button>
      </div>

      <Card>
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) => {
            const nextFilters = buildFilters(values);
            setFilters(nextFilters);
            syncUrl(nextFilters);
            void loadList(nextFilters, 1);
          }}
        >
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
            <Form.Item label="知识库" name="kb_id">
              <Select
                allowClear
                placeholder="全部知识库"
                options={bases.map((base) => ({ label: base.name, value: base.id }))}
              />
            </Form.Item>
            <Form.Item label="状态" name="result_status">
              <Select allowClear placeholder="全部状态" options={statusOptions} />
            </Form.Item>
            <Form.Item label="Request ID" name="request_id">
              <Input placeholder="精确查找 request_id" />
            </Form.Item>
            <Form.Item label="Query 关键词" name="query_keyword">
              <Input placeholder="模糊搜索 query" />
            </Form.Item>
            <Form.Item label="时间范围" name="range">
              <DatePicker.RangePicker showTime className="w-full" />
            </Form.Item>
          </div>
          <Space>
            <Button type="primary" htmlType="submit" icon={<SearchOutlined />} loading={isLoading}>
              查询日志
            </Button>
            <Button
              onClick={() => {
                form.resetFields();
                const nextFilters = { kb_id: selectedBase?.id } as RetrievalLogFilters;
                setFilters(nextFilters);
                syncUrl(nextFilters);
                void loadList(nextFilters, 1);
              }}
            >
              重置
            </Button>
          </Space>
        </Form>
      </Card>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      <Card>
        {items.length === 0 && !isLoading ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="当前筛选条件下没有检索日志"
          />
        ) : (
          <Table<KBRetrieveLog>
            rowKey="request_id"
            loading={isLoading}
            columns={columns}
            dataSource={items}
            pagination={{
              current: page,
              pageSize: PAGE_SIZE,
              total,
              onChange: (nextPage) => void loadList(filters, nextPage),
            }}
            onRow={(record) => ({
              onClick: () => void openDetail(record.request_id),
              style: { cursor: 'pointer' },
            })}
          />
        )}
      </Card>

      <Drawer
        title={detail?.request_id ? `Trace 详情 · ${detail.request_id}` : 'Trace 详情'}
        width={560}
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
      >
        {detailLoading ? (
          <div className="flex justify-center py-10">
            <Spin />
          </div>
        ) : detailError ? (
          <Alert type="error" showIcon message={detailError} />
        ) : detail ? (
          <Space direction="vertical" size="large" className="w-full">
            {!hasP3Summary(detail) ? (
              <Alert
                type="warning"
                showIcon
                message="该请求暂无 P3 调试字段"
                description="当前仍可查看基础 trace 详情；如需完整 P3 摘要，请确认日志链路已返回 parent-child、TopK、evidence 与 citation 相关字段。"
              />
            ) : null}
            <Descriptions title="请求信息" column={1} size="small" bordered>
              <Descriptions.Item label="Request ID">{renderField(detail.request_id)}</Descriptions.Item>
              <Descriptions.Item label="KB IDs">{renderField(detail.kb_ids)}</Descriptions.Item>
              <Descriptions.Item label="Query">{renderField(detail.query)}</Descriptions.Item>
              <Descriptions.Item label="Final Query">{renderField(detail.final_query)}</Descriptions.Item>
              <Descriptions.Item label="Expr">{renderField(detail.expr)}</Descriptions.Item>
              <Descriptions.Item label="Top K">{renderField(detail.top_k)}</Descriptions.Item>
              <Descriptions.Item label="Candidate Top K">
                {renderField(detail.candidate_topk)}
              </Descriptions.Item>
              <Descriptions.Item label="Final Top K">{renderField(detail.final_topk)}</Descriptions.Item>
            </Descriptions>

            <Descriptions title="阶段耗时" column={1} size="small" bordered>
              <Descriptions.Item label="Embedding">{renderField(detail.embedding_ms)} ms</Descriptions.Item>
              <Descriptions.Item label="Search">{renderField(detail.search_ms)} ms</Descriptions.Item>
              <Descriptions.Item label="Postprocess">
                {renderField(detail.postprocess_ms)} ms
              </Descriptions.Item>
              <Descriptions.Item label="Rerank">{renderField(detail.rerank_ms)} ms</Descriptions.Item>
              <Descriptions.Item label="Duration">{renderField(detail.duration_ms)} ms</Descriptions.Item>
            </Descriptions>

            <Descriptions title="结果与路由" column={1} size="small" bordered>
              <Descriptions.Item label="Routes">{renderField(detail.routes)}</Descriptions.Item>
              <Descriptions.Item label="Collection">{renderField(detail.collection)}</Descriptions.Item>
              <Descriptions.Item label="Retriever">{renderField(detail.retriever_version)}</Descriptions.Item>
              <Descriptions.Item label="Result Status">
                <Tag color={statusColor(detail.result_status)}>{detail.result_status}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Final Count">{renderField(detail.final_count)}</Descriptions.Item>
              <Descriptions.Item label="Truncated Count">
                {renderField(detail.truncated_count)}
              </Descriptions.Item>
              <Descriptions.Item label="Dense Hits">{renderField(detail.dense_hits)}</Descriptions.Item>
              <Descriptions.Item label="Sparse Hits">{renderField(detail.sparse_hits)}</Descriptions.Item>
              <Descriptions.Item label="Dense Contribution">
                {renderField(detail.dense_contribution)}
              </Descriptions.Item>
              <Descriptions.Item label="Sparse Contribution">
                {renderField(detail.sparse_contribution)}
              </Descriptions.Item>
            </Descriptions>

            <Descriptions title="错误与补充信息" column={1} size="small" bordered>
              <Descriptions.Item label="Tenant ID">{renderField(detail.tenant_id)}</Descriptions.Item>
              <Descriptions.Item label="App ID">{renderField(detail.app_id)}</Descriptions.Item>
              <Descriptions.Item label="API Key ID">{renderField(detail.api_key_id)}</Descriptions.Item>
              <Descriptions.Item label="Auth Type">{renderField(detail.auth_type)}</Descriptions.Item>
              <Descriptions.Item label="Source API">{renderField(detail.source_api)}</Descriptions.Item>
              <Descriptions.Item label="Permission Result">
                {renderField(detail.permission_result)}
              </Descriptions.Item>
              <Descriptions.Item label="Legacy Path">{renderField(detail.is_legacy)}</Descriptions.Item>
              <Descriptions.Item label="P3 摘要">
                <Space direction="vertical" size="middle" className="w-full">
                  <div>
                    <Text strong>parent_child_enabled: </Text>
                    {renderField(detail.parent_child_enabled)}
                  </div>
                  <div>
                    <Text strong>topk_policy_version: </Text>
                    {detail.topk_policy_version ? (
                      detail.topk_policy_version
                    ) : (
                      <Text type="secondary">当前日志未返回 topk_policy_version</Text>
                    )}
                  </div>
                  <div>
                    <Text strong>evidence_gate_result: </Text>
                    {renderField(detail.evidence_gate_result)}
                  </div>
                  <div>
                    <Text strong>citation_support_score: </Text>
                    {renderField(detail.citation_support_score)}
                  </div>
                  <div>
                    <Text strong>refusal_reason: </Text>
                    {renderField(detail.refusal_reason)}
                  </div>
                </Space>
              </Descriptions.Item>
              <Descriptions.Item label="Error Code">{renderField(detail.error_code)}</Descriptions.Item>
              <Descriptions.Item label="Error Message">{renderField(detail.error_msg)}</Descriptions.Item>
              <Descriptions.Item label="Strategy">{renderField(detail.strategy)}</Descriptions.Item>
              <Descriptions.Item label="Release Stage">
                {renderField(detail.release_stage)}
              </Descriptions.Item>
              <Descriptions.Item label="Release Reason">
                {renderField(detail.release_reason)}
              </Descriptions.Item>
              <Descriptions.Item label="Empty Reason">{renderField(detail.empty_reason)}</Descriptions.Item>
              <Descriptions.Item label="Created At">
                {renderField(dayjs(detail.created_at).format('YYYY-MM-DD HH:mm:ss'))}
              </Descriptions.Item>
            </Descriptions>
          </Space>
        ) : (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="请选择一条日志查看详情" />
        )}
      </Drawer>
    </div>
  );
}
