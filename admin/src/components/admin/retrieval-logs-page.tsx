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
import { KB_ADMIN_API } from '@/config/api';
import apiClient from '@/services/api/client';
import type { KBRetrieveLog, ListResponse, RetrieveResultStatus } from '@/types/kb';
import { useKnowledgeBaseContext } from './knowledge-base-provider';
import { ActionEmpty } from './ui/action-empty';
import { PageHeader } from './ui/page-header';

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

type RetrievalLogWithP3Summary = KBRetrieveLog & {
  topk_policy_version?: string;
};

const PAGE_SIZE = 20;

const statusOptions: Array<{ label: string; value: RetrieveResultStatus }> = [
  { label: '成功', value: 'success' },
  { label: '无结果', value: 'no_result' },
  { label: '已过滤', value: 'filtered_out' },
  { label: '异常', value: 'error' },
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
    return <Tag color="warning">当前版本暂未返回</Tag>;
  }
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }
  return String(value);
}

function renderCacheHitTag(hit: boolean | null | undefined) {
  if (hit === undefined || hit === null) {
    return <Tag color="warning">暂未返回</Tag>;
  }
  return hit ? <Tag color="success">命中</Tag> : <Tag>未命中</Tag>;
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
        title: '问题',
        dataIndex: 'query',
        key: 'query',
        ellipsis: true,
        render: (value: string) => value || <Tag color="warning">当前版本暂未返回</Tag>,
      },
      {
        title: '请求编号',
        dataIndex: 'request_id',
        key: 'request_id',
        width: 180,
        ellipsis: true,
        render: (value: string) => <Text code>{value || '暂未返回'}</Text>,
      },
      {
        title: '知识库',
        dataIndex: 'kb_ids',
        key: 'kb_ids',
        width: 120,
        render: (value: string) => value || <Tag color="warning">暂未返回</Tag>,
      },
      {
        title: '返回上限',
        dataIndex: 'top_k',
        key: 'top_k',
        width: 90,
      },
      {
        title: '返回数量',
        dataIndex: 'final_count',
        key: 'final_count',
        width: 110,
      },
      {
        title: '耗时',
        dataIndex: 'duration_ms',
        key: 'duration_ms',
        width: 110,
        render: (value: number) => `${value ?? 0} ms`,
      },
      {
        title: '状态',
        dataIndex: 'result_status',
        key: 'result_status',
        width: 110,
        render: (value: RetrieveResultStatus) => <Tag color={statusColor(value)}>{value}</Tag>,
      },
      {
        title: '语义缓存',
        dataIndex: 'semantic_cache_hit',
        key: 'semantic_cache_hit',
        width: 140,
        render: (value: boolean | undefined) => renderCacheHitTag(value),
      },
      {
        title: '向量缓存',
        dataIndex: 'embedding_cache_hit',
        key: 'embedding_cache_hit',
        width: 140,
        render: (value: boolean | undefined) => renderCacheHitTag(value),
      },
      {
        title: '创建时间',
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
            链路分析
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
      setError(err instanceof Error ? err.message : '加载检索链路追踪失败');
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
      setDetailError(err instanceof Error ? err.message : '加载链路详情失败');
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
    <div className="admin-page">
      <PageHeader
        title="检索链路追踪"
        subtitle="按请求编号、知识库、状态和时间范围定位一次检索，并进入详情查看阶段耗时、缓存命中和异常原因。"
        extra={
          <Button icon={<ReloadOutlined />} onClick={() => void loadList(filters, page)}>
            刷新
          </Button>
        }
      />

      <Card className="admin-section-card">
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
            <Form.Item label="请求编号" name="request_id">
              <Input placeholder="输入请求编号，快速定位一次检索" />
            </Form.Item>
            <Form.Item label="问题关键词" name="query_keyword">
              <Input placeholder="按问题内容模糊查询" />
            </Form.Item>
            <Form.Item label="时间范围" name="range">
              <DatePicker.RangePicker showTime className="w-full" />
            </Form.Item>
          </div>
          <Space>
            <Button type="primary" htmlType="submit" icon={<SearchOutlined />} loading={isLoading}>
              查询链路
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

      <Card className="admin-section-card">
        {items.length === 0 && !isLoading ? (
          <ActionEmpty
            title="当前筛选条件下没有检索链路"
            description="可以直接输入请求编号定位一次检索，或放宽状态与时间范围后重新查询。"
            action={
              <Button type="link" onClick={() => router.push('/retrieval-lab')}>
                去做检索验证
              </Button>
            }
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
        title={detail?.request_id ? `链路详情 · ${detail.request_id}` : '链路详情'}
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
                message="高级链路摘要暂未完整返回"
                description="当前仍可查看基础链路信息。如需完整高级摘要，请确认日志流水线已返回上下文补全、TopK、证据检查与引用字段。"
              />
            ) : null}

            <Descriptions title="基础信息" column={1} size="small" bordered>
              <Descriptions.Item label="请求编号">{renderField(detail.request_id)}</Descriptions.Item>
              <Descriptions.Item label="知识库">{renderField(detail.kb_ids)}</Descriptions.Item>
              <Descriptions.Item label="问题">{renderField(detail.query)}</Descriptions.Item>
              <Descriptions.Item label="最终查询">{renderField(detail.final_query)}</Descriptions.Item>
              <Descriptions.Item label="过滤表达式">{renderField(detail.expr)}</Descriptions.Item>
              <Descriptions.Item label="请求 Top K">{renderField(detail.top_k)}</Descriptions.Item>
              <Descriptions.Item label="候选召回数">
                {renderField(detail.candidate_topk)}
              </Descriptions.Item>
              <Descriptions.Item label="最终返回数">{renderField(detail.final_topk)}</Descriptions.Item>
            </Descriptions>

            <Descriptions title="阶段耗时" column={1} size="small" bordered>
              <Descriptions.Item label="向量计算">{renderField(detail.embedding_ms)} ms</Descriptions.Item>
              <Descriptions.Item label="检索查询">{renderField(detail.search_ms)} ms</Descriptions.Item>
              <Descriptions.Item label="后处理">
                {renderField(detail.postprocess_ms)} ms
              </Descriptions.Item>
              <Descriptions.Item label="重排序">{renderField(detail.rerank_ms)} ms</Descriptions.Item>
              <Descriptions.Item label="总耗时">{renderField(detail.duration_ms)} ms</Descriptions.Item>
            </Descriptions>

            <Descriptions title="结果摘要" column={1} size="small" bordered>
              <Descriptions.Item label="召回路线">{renderField(detail.routes)}</Descriptions.Item>
              <Descriptions.Item label="数据集合">{renderField(detail.collection)}</Descriptions.Item>
              <Descriptions.Item label="检索版本">{renderField(detail.retriever_version)}</Descriptions.Item>
              <Descriptions.Item label="结果状态">
                <Tag color={statusColor(detail.result_status)}>{detail.result_status}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="返回数量">{renderField(detail.final_count)}</Descriptions.Item>
              <Descriptions.Item label="截断数量">
                {renderField(detail.truncated_count)}
              </Descriptions.Item>
              <Descriptions.Item label="向量命中数">{renderField(detail.dense_hits)}</Descriptions.Item>
              <Descriptions.Item label="关键词命中数">{renderField(detail.sparse_hits)}</Descriptions.Item>
              <Descriptions.Item label="向量贡献占比">
                {renderField(detail.dense_contribution)}
              </Descriptions.Item>
              <Descriptions.Item label="关键词贡献占比">
                {renderField(detail.sparse_contribution)}
              </Descriptions.Item>
            </Descriptions>

            <Descriptions title="语义缓存" column={1} size="small" bordered>
              <Descriptions.Item label="是否启用">
                {renderField(detail.semantic_cache_enabled)}
              </Descriptions.Item>
              <Descriptions.Item label="缓存状态">
                {renderCacheHitTag(detail.semantic_cache_hit)}
              </Descriptions.Item>
              <Descriptions.Item label="查询耗时">
                {renderField(detail.semantic_cache_lookup_ms)} ms
              </Descriptions.Item>
              <Descriptions.Item label="相似度">
                {detail.semantic_cache_similarity === null ||
                detail.semantic_cache_similarity === undefined ? (
                  <Tag color="warning">当前版本暂未返回</Tag>
                ) : (
                  detail.semantic_cache_similarity.toFixed(4)
                )}
              </Descriptions.Item>
              <Descriptions.Item label="命中原因">
                {renderField(detail.semantic_cache_reason)}
              </Descriptions.Item>
              <Descriptions.Item label="缓存条目">
                {renderField(detail.semantic_cache_entry_id)}
              </Descriptions.Item>
            </Descriptions>

            <Descriptions title="向量缓存" column={1} size="small" bordered>
              <Descriptions.Item label="是否启用">
                {renderField(detail.embedding_cache_enabled)}
              </Descriptions.Item>
              <Descriptions.Item label="缓存状态">
                {renderCacheHitTag(detail.embedding_cache_hit)}
              </Descriptions.Item>
              <Descriptions.Item label="查询耗时">
                {renderField(detail.embedding_cache_lookup_ms)} ms
              </Descriptions.Item>
              <Descriptions.Item label="命中原因">
                {renderField(detail.embedding_cache_reason)}
              </Descriptions.Item>
            </Descriptions>

            <Descriptions title="错误原因与高级字段" column={1} size="small" bordered>
              <Descriptions.Item label="组织编号">{renderField(detail.tenant_id)}</Descriptions.Item>
              <Descriptions.Item label="应用编号">{renderField(detail.app_id)}</Descriptions.Item>
              <Descriptions.Item label="接入密钥">{renderField(detail.api_key_id)}</Descriptions.Item>
              <Descriptions.Item label="鉴权方式">{renderField(detail.auth_type)}</Descriptions.Item>
              <Descriptions.Item label="来源接口">{renderField(detail.source_api)}</Descriptions.Item>
              <Descriptions.Item label="权限结果">
                {renderField(detail.permission_result)}
              </Descriptions.Item>
              <Descriptions.Item label="旧链路回退">{renderField(detail.is_legacy)}</Descriptions.Item>
              <Descriptions.Item label="高级链路摘要">
                <Space direction="vertical" size="middle" className="w-full">
                  <div>
                    <Text strong>上下文补全: </Text>
                    {renderField(detail.parent_child_enabled)}
                  </div>
                  <div>
                    <Text strong>TopK 策略版本: </Text>
                    {detail.topk_policy_version ? (
                      detail.topk_policy_version
                    ) : (
                      <Text type="secondary">当前版本暂未返回</Text>
                    )}
                  </div>
                  <div>
                    <Text strong>证据检查结果: </Text>
                    {renderField(detail.evidence_gate_result)}
                  </div>
                  <div>
                    <Text strong>引用支持得分: </Text>
                    {renderField(detail.citation_support_score)}
                  </div>
                  <div>
                    <Text strong>拒答原因: </Text>
                    {renderField(detail.refusal_reason)}
                  </div>
                </Space>
              </Descriptions.Item>
              <Descriptions.Item label="错误编号">{renderField(detail.error_code)}</Descriptions.Item>
              <Descriptions.Item label="错误信息">{renderField(detail.error_msg)}</Descriptions.Item>
              <Descriptions.Item label="策略版本">{renderField(detail.strategy)}</Descriptions.Item>
              <Descriptions.Item label="发布阶段">
                {renderField(detail.release_stage)}
              </Descriptions.Item>
              <Descriptions.Item label="发布说明">
                {renderField(detail.release_reason)}
              </Descriptions.Item>
              <Descriptions.Item label="无结果原因">{renderField(detail.empty_reason)}</Descriptions.Item>
              <Descriptions.Item label="记录时间">
                {renderField(dayjs(detail.created_at).format('YYYY-MM-DD HH:mm:ss'))}
              </Descriptions.Item>
            </Descriptions>
          </Space>
        ) : (
          <ActionEmpty
            title="请选择一条链路记录查看详情"
            description="点击上方任意一行，可展开查看阶段耗时、缓存命中与错误原因。"
          />
        )}
      </Drawer>
    </div>
  );
}
