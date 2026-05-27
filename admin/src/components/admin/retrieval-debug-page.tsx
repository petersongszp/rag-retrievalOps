'use client';

import Link from 'next/link';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import {
  CopyOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Collapse,
  Descriptions,
  Empty,
  Form,
  Input,
  List,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { KB_ADMIN_API } from '@/config/api';
import apiClient from '@/services/api/client';
import type {
  RetrievalDebugDocument,
  RetrievalDebugTrace,
  RetrievalRouteHit,
} from '@/types/kb';

const { Title, Paragraph, Text } = Typography;

type DebugDocumentRow = RetrievalDebugDocument & { key: string };

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

function renderValue(value?: string | number | boolean | null) {
  if (value === undefined || value === null || value === '') {
    return <Tag color="warning">Contract gap</Tag>;
  }
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }
  return String(value);
}

function renderStringList(items?: string[], emptyLabel = 'none') {
  if (!items?.length) {
    return <Tag color="default">{emptyLabel}</Tag>;
  }
  return items.join(', ');
}

function toDebugRows(items?: RetrievalDebugDocument[], prefix = 'doc'): DebugDocumentRow[] {
  return (items ?? []).map((item, index) => ({
    ...item,
    key: `${prefix}-${item.chunk_id ?? item.document_id ?? index}`,
  }));
}

const debugDocumentColumns: ColumnsType<DebugDocumentRow> = [
  {
    title: 'Chunk',
    dataIndex: 'chunk_id',
    key: 'chunk_id',
    width: 180,
    render: (value?: string) => value || <Tag color="warning">Contract gap</Tag>,
  },
  {
    title: 'File',
    dataIndex: 'file_name',
    key: 'file_name',
    width: 180,
    render: (value?: string) => value || <Tag color="default">unknown</Tag>,
  },
  {
    title: 'Route',
    dataIndex: 'route',
    key: 'route',
    width: 120,
    render: (value?: string) => value || <Tag color="default">n/a</Tag>,
  },
  {
    title: 'Score',
    dataIndex: 'score',
    key: 'score',
    width: 120,
    render: (value?: number) =>
      value === undefined || value === null ? <Tag color="warning">Contract gap</Tag> : value.toFixed(4),
  },
];

function StageFallback({
  title,
  description,
  gaps,
}: {
  title: string;
  description: string;
  gaps?: string[];
}) {
  return (
    <Space direction="vertical" size="middle" className="w-full">
      <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={description} />
      {gaps?.length ? (
        <Alert type="warning" showIcon message={`${title} contract gaps`} description={gaps.join(', ')} />
      ) : null}
    </Space>
  );
}

export function RetrievalDebugPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [messageApi, contextHolder] = message.useMessage();
  const [form] = Form.useForm<{ request_id: string }>();

  const requestId = searchParams.get('request_id')?.trim() || '';
  const [trace, setTrace] = useState<RetrievalDebugTrace | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadTrace = useCallback(async (nextRequestId: string) => {
    const trimmed = nextRequestId.trim();
    if (!trimmed) {
      setTrace(null);
      setError(null);
      return;
    }

    try {
      setLoading(true);
      setError(null);
      const data = (await apiClient.get(
        KB_ADMIN_API.GET_RETRIEVE_DEBUG_TRACE(trimmed)
      )) as RetrievalDebugTrace;
      setTrace(data);
    } catch (loadError) {
      setTrace(null);
      setError(normalizeError(loadError, 'Failed to load debug trace'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    form.setFieldsValue({ request_id: requestId });
    void loadTrace(requestId);
  }, [form, loadTrace, requestId]);

  const routeHits = trace?.route_hits ?? [];
  const finalResults = trace?.final_results ?? [];
  const contractGaps = trace?.contract_gaps ?? [];
  const routeFinalQueries = trace?.route_final_queries ?? {};

  const routeCards = useMemo(
    () =>
      routeHits.map((item: RetrievalRouteHit) => ({
        ...item,
        rows: toDebugRows(item.hits, `route-${item.route}`),
      })),
    [routeHits]
  );

  return (
    <div className="space-y-6">
      {contextHolder}

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            检索调试视图
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            把一次检索请求拆成可读的阶段剖面，优先展示风险最高的 query rewrite、topk、evidence 与 citation 决策。
          </Paragraph>
        </div>
        <Space wrap>
          <Link href="/retrieval-lab">
            <Button>返回检索实验室</Button>
          </Link>
          <Link
            href={
              requestId
                ? `/trace-logs/retrieval?request_id=${encodeURIComponent(requestId)}`
                : '/trace-logs/retrieval'
            }
          >
            <Button type="primary">打开原始 Trace Logs</Button>
          </Link>
        </Space>
      </div>

      <Card>
        <Form
          form={form}
          layout="vertical"
          onFinish={(values) => {
            const trimmed = values.request_id?.trim() || '';
            const params = new URLSearchParams(searchParams.toString());
            if (trimmed) {
              params.set('request_id', trimmed);
            } else {
              params.delete('request_id');
            }
            router.replace(
              params.toString() ? `/retrieval-lab/debug?${params.toString()}` : '/retrieval-lab/debug'
            );
          }}
        >
          <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto_auto]">
            <Form.Item
              label="Request ID"
              name="request_id"
              rules={[{ required: true, message: '请输入 request_id' }]}
            >
              <Input placeholder="输入 request_id 以查询单次检索调试详情" />
            </Form.Item>
            <Form.Item label=" ">
              <Button type="primary" htmlType="submit" icon={<SearchOutlined />}>
                查询
              </Button>
            </Form.Item>
            <Form.Item label=" ">
              <Button icon={<ReloadOutlined />} onClick={() => void loadTrace(requestId)}>
                刷新
              </Button>
            </Form.Item>
          </div>
        </Form>
      </Card>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      {!requestId ? (
        <Card>
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="请输入 request_id，或从检索实验室、检索日志、评测报告跳转进入。"
          />
        </Card>
      ) : loading ? (
        <Card>
          <div className="flex justify-center py-12">
            <Spin />
          </div>
        </Card>
      ) : !trace ? (
        <Card>
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前没有可展示的调试数据。" />
        </Card>
      ) : (
        <>
          <Card
            title="请求顶栏"
            extra={
              <Button
                icon={<CopyOutlined />}
                disabled={!trace.request_id}
                onClick={async () => {
                  if (!trace.request_id) {
                    messageApi.warning('当前缺少 request_id');
                    return;
                  }
                  await navigator.clipboard.writeText(trace.request_id);
                  messageApi.success('request_id 已复制');
                }}
              >
                复制 request_id
              </Button>
            }
          >
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="request_id">{renderValue(trace.request_id)}</Descriptions.Item>
              <Descriptions.Item label="original_query">
                {renderValue(trace.original_query)}
              </Descriptions.Item>
              <Descriptions.Item label="rewritten_query">
                {renderValue(trace.rewritten_query)}
              </Descriptions.Item>
              <Descriptions.Item label="kb_ids">
                {trace.kb_ids?.length ? trace.kb_ids.join(', ') : <Tag color="warning">Contract gap</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="created_at">{renderValue(trace.created_at)}</Descriptions.Item>
              <Descriptions.Item label="debug_available">
                {trace.debug_available ? <Tag color="success">true</Tag> : <Tag color="warning">false</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="degradation">
                {trace.degradation?.enabled ? (
                  <Space wrap>
                    <Tag color="warning">{trace.degradation.reason || 'degraded'}</Tag>
                    {trace.degradation.error_code ? <Text code>{trace.degradation.error_code}</Text> : null}
                    {trace.degradation.fallback_strategy ? (
                      <Tag>{trace.degradation.fallback_strategy}</Tag>
                    ) : null}
                  </Space>
                ) : (
                  <Tag color="success">none</Tag>
                )}
              </Descriptions.Item>
            </Descriptions>
          </Card>

          {contractGaps.length ? (
            <Alert
              type="warning"
              showIcon
              message="集中契约缺口"
              description={contractGaps.join(', ')}
            />
          ) : null}

          <Collapse
            defaultActiveKey={['query', 'routes', 'topk', 'evidence', 'citation']}
            items={[
              {
                key: 'query',
                label: 'Query Rewrite',
                children: (
                  <Space direction="vertical" size="middle" className="w-full">
                    <Descriptions column={1} size="small" bordered>
                      <Descriptions.Item label="original query">
                        {renderValue(trace.original_query)}
                      </Descriptions.Item>
                      <Descriptions.Item label="rewritten query">
                        {renderValue(trace.rewritten_query)}
                      </Descriptions.Item>
                      <Descriptions.Item label="route final queries">
                        {Object.keys(routeFinalQueries).length ? (
                          <Space direction="vertical" size="small">
                            {Object.entries(routeFinalQueries).map(([key, value]) => (
                              <Text key={key}>
                                <Text code>{key}</Text>: {value}
                              </Text>
                            ))}
                          </Space>
                        ) : (
                          <Tag color="warning">Contract gap</Tag>
                        )}
                      </Descriptions.Item>
                      <Descriptions.Item label="term_hits / rewrite_strategy / rewrite_gain_bucket">
                        <Tag color="warning">Contract gap</Tag>
                      </Descriptions.Item>
                    </Descriptions>
                  </Space>
                ),
              },
              {
                key: 'routes',
                label: 'Route Hits',
                children: routeCards.length ? (
                  <Space direction="vertical" size="middle" className="w-full">
                    {routeCards.map((item) => (
                      <Card key={item.route} size="small" title={item.route}>
                        <Descriptions column={1} size="small" bordered>
                          <Descriptions.Item label="contribution">
                            {renderValue(item.contribution)}
                          </Descriptions.Item>
                          <Descriptions.Item label="latency_ms">
                            {renderValue(item.latency_ms)}
                          </Descriptions.Item>
                          <Descriptions.Item label="error">{renderValue(item.error)}</Descriptions.Item>
                        </Descriptions>
                        <Table<DebugDocumentRow>
                          className="mt-4"
                          rowKey="key"
                          size="small"
                          columns={debugDocumentColumns}
                          dataSource={item.rows}
                          pagination={false}
                          locale={{ emptyText: '当前 route 没有 top hits' }}
                        />
                      </Card>
                    ))}
                  </Space>
                ) : (
                  <StageFallback
                    title="Route Hits"
                    description="当前没有 route hits 数据。"
                    gaps={contractGaps.filter((item) => item.startsWith('route_hits'))}
                  />
                ),
              },
              {
                key: 'fusion',
                label: 'Fusion / Dedupe / Rerank / Filter',
                children: (
                  <div className="grid gap-4 xl:grid-cols-2">
                    <Card size="small" title="Fusion">
                      <Descriptions column={1} size="small" bordered>
                        <Descriptions.Item label="before">
                          {trace.fusion_results?.before?.length ?? <Tag color="warning">Contract gap</Tag>}
                        </Descriptions.Item>
                        <Descriptions.Item label="after">
                          {trace.fusion_results?.after?.length ?? <Tag color="warning">Contract gap</Tag>}
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                    <Card size="small" title="Dedupe">
                      <Descriptions column={1} size="small" bordered>
                        <Descriptions.Item label="before_count">
                          {renderValue(trace.dedupe_results?.before_count)}
                        </Descriptions.Item>
                        <Descriptions.Item label="after_count">
                          {renderValue(trace.dedupe_results?.after_count)}
                        </Descriptions.Item>
                        <Descriptions.Item label="removed">
                          {trace.dedupe_results?.removed?.length ?? <Tag color="warning">Contract gap</Tag>}
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                    <Card size="small" title="Rerank">
                      <Descriptions column={1} size="small" bordered>
                        <Descriptions.Item label="rerank_model">
                          {renderValue(trace.rerank_results?.rerank_model)}
                        </Descriptions.Item>
                        <Descriptions.Item label="fallback">
                          {renderValue(trace.rerank_results?.fallback)}
                        </Descriptions.Item>
                        <Descriptions.Item label="reason">
                          {renderValue(trace.rerank_results?.reason)}
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                    <Card size="small" title="Filter">
                      <Descriptions column={1} size="small" bordered>
                        <Descriptions.Item label="before_count">
                          {renderValue(trace.filter_results?.before_count)}
                        </Descriptions.Item>
                        <Descriptions.Item label="after_count">
                          {renderValue(trace.filter_results?.after_count)}
                        </Descriptions.Item>
                        <Descriptions.Item label="truncate_reason">
                          {renderValue(trace.filter_results?.truncate_reason)}
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                  </div>
                ),
              },
              {
                key: 'parent-child',
                label: 'Parent-Child',
                children: trace.parent_child ? (
                  <Space direction="vertical" size="middle" className="w-full">
                    <Descriptions column={1} size="small" bordered>
                      <Descriptions.Item label="parent_child_enabled">
                        {renderValue(trace.parent_child.parent_child_enabled)}
                      </Descriptions.Item>
                      <Descriptions.Item label="parent_fill_strategy">
                        {renderValue(trace.parent_child.parent_fill_strategy)}
                      </Descriptions.Item>
                      <Descriptions.Item label="parent_fill_tokens">
                        {renderValue(trace.parent_child.parent_fill_tokens)}
                      </Descriptions.Item>
                      <Descriptions.Item label="fallback_reason">
                        {renderValue(trace.parent_child.fallback_reason)}
                      </Descriptions.Item>
                    </Descriptions>
                    <div className="grid gap-4 xl:grid-cols-2">
                      <Card size="small" title="Child Hits">
                        <Table<DebugDocumentRow>
                          rowKey="key"
                          size="small"
                          columns={debugDocumentColumns}
                          dataSource={toDebugRows(trace.parent_child.child_hits, 'child')}
                          pagination={false}
                          locale={{ emptyText: '没有 child hits' }}
                        />
                      </Card>
                      <Card size="small" title="Parent Contexts">
                        <Table<DebugDocumentRow>
                          rowKey="key"
                          size="small"
                          columns={debugDocumentColumns}
                          dataSource={toDebugRows(trace.parent_child.parent_contexts, 'parent')}
                          pagination={false}
                          locale={{ emptyText: '没有 parent contexts' }}
                        />
                      </Card>
                    </div>
                  </Space>
                ) : (
                  <StageFallback
                    title="Parent-Child"
                    description="当前没有 parent-child 数据。"
                    gaps={contractGaps.filter((item) => item.startsWith('parent_child'))}
                  />
                ),
              },
              {
                key: 'topk',
                label: 'TopK Decision',
                children: trace.topk_decision ? (
                  <Descriptions column={1} size="small" bordered>
                    <Descriptions.Item label="candidate_topk">
                      {renderValue(trace.topk_decision.candidate_topk)}
                    </Descriptions.Item>
                    <Descriptions.Item label="final_topk">
                      {renderValue(trace.topk_decision.final_topk)}
                    </Descriptions.Item>
                    <Descriptions.Item label="score_distribution">
                      {renderValue(trace.topk_decision.score_distribution)}
                    </Descriptions.Item>
                    <Descriptions.Item label="rerank_gap">
                      {renderValue(trace.topk_decision.rerank_gap)}
                    </Descriptions.Item>
                    <Descriptions.Item label="evidence_density">
                      {renderValue(trace.topk_decision.evidence_density)}
                    </Descriptions.Item>
                    <Descriptions.Item label="token_budget">
                      {renderValue(trace.topk_decision.token_budget)}
                    </Descriptions.Item>
                    <Descriptions.Item label="topk_decision_reason">
                      {renderValue(trace.topk_decision.topk_decision_reason)}
                    </Descriptions.Item>
                  </Descriptions>
                ) : (
                  <StageFallback title="TopK Decision" description="当前没有 topk 决策数据。" />
                ),
              },
              {
                key: 'evidence',
                label: 'Evidence Gate',
                children: trace.evidence_gate ? (
                  <Descriptions column={1} size="small" bordered>
                    <Descriptions.Item label="evidence_gate_result">
                      {renderValue(trace.evidence_gate.evidence_gate_result)}
                    </Descriptions.Item>
                    <Descriptions.Item label="refusal_reason">
                      {renderValue(trace.evidence_gate.refusal_reason)}
                    </Descriptions.Item>
                    <Descriptions.Item label="thresholds">
                      {trace.evidence_gate.thresholds ? (
                        <Space direction="vertical" size="small">
                          <Text>min_rerank_score: {renderValue(trace.evidence_gate.thresholds.min_rerank_score)}</Text>
                          <Text>min_density: {renderValue(trace.evidence_gate.thresholds.min_density)}</Text>
                          <Text>
                            min_citation_coverage:{' '}
                            {renderValue(trace.evidence_gate.thresholds.min_citation_coverage)}
                          </Text>
                        </Space>
                      ) : (
                        <Tag color="warning">Contract gap</Tag>
                      )}
                    </Descriptions.Item>
                    <Descriptions.Item label="evidence_gate_error">
                      {renderValue(trace.evidence_gate.evidence_gate_error)}
                    </Descriptions.Item>
                    <Descriptions.Item label="refusal_template_version">
                      {renderValue(trace.evidence_gate.refusal_template_version)}
                    </Descriptions.Item>
                  </Descriptions>
                ) : (
                  <StageFallback title="Evidence Gate" description="当前没有 evidence gate 数据。" />
                ),
              },
              {
                key: 'citation',
                label: 'Citation Consistency',
                children: trace.citation_check ? (
                  <Space direction="vertical" size="middle" className="w-full">
                    <Descriptions column={1} size="small" bordered>
                      <Descriptions.Item label="citation_supported">
                        {renderValue(trace.citation_check.citation_supported)}
                      </Descriptions.Item>
                      <Descriptions.Item label="citation_support_score">
                        {renderValue(trace.citation_check.citation_support_score)}
                      </Descriptions.Item>
                      <Descriptions.Item label="unsupported_claims">
                        {renderStringList(trace.citation_check.unsupported_claims)}
                      </Descriptions.Item>
                      <Descriptions.Item label="citation_check_version">
                        {renderValue(trace.citation_check.citation_check_version)}
                      </Descriptions.Item>
                    </Descriptions>
                    <Alert
                      type="info"
                      showIcon
                      message="当前后端尚未返回 citation snippets 与 child/parent 对照专用结构"
                    />
                  </Space>
                ) : (
                  <StageFallback title="Citation Consistency" description="当前没有 citation consistency 数据。" />
                ),
              },
              {
                key: 'final',
                label: 'Final Results',
                children: finalResults.length ? (
                  <List
                    itemLayout="vertical"
                    dataSource={finalResults}
                    renderItem={(item, index) => (
                      <List.Item
                        key={`${item.citation?.chunk_id ?? 'result'}-${index}`}
                        extra={
                          requestId ? (
                            <Link href={`/trace-logs/retrieval?request_id=${encodeURIComponent(requestId)}`}>
                              <Button size="small">跳转原始详情</Button>
                            </Link>
                          ) : null
                        }
                      >
                        <Space direction="vertical" size="small" className="w-full">
                          <Text strong>Result {index + 1}</Text>
                          <Text>{item.content}</Text>
                          <Space wrap>
                            <Tag>route: {item.source?.route || 'Contract gap'}</Tag>
                            <Tag>file: {item.citation?.file_name || 'Contract gap'}</Tag>
                            <Tag>chunk: {item.citation?.chunk_id || 'Contract gap'}</Tag>
                            <Tag>kb: {item.citation?.kb_id ?? 'Contract gap'}</Tag>
                          </Space>
                        </Space>
                      </List.Item>
                    )}
                  />
                ) : (
                  <StageFallback title="Final Results" description="当前没有 final results 数据。" />
                ),
              },
            ]}
          />
        </>
      )}
    </div>
  );
}
