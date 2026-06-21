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
import { ActionEmpty } from './ui/action-empty';
import { PageHeader } from './ui/page-header';

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
    return <Tag color="warning">暂未返回</Tag>;
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
    title: '分块编号',
    dataIndex: 'chunk_id',
    key: 'chunk_id',
    width: 180,
    render: (value?: string) => value || <Tag color="warning">暂未返回</Tag>,
  },
  {
    title: '来源文件',
    dataIndex: 'file_name',
    key: 'file_name',
    width: 180,
    render: (value?: string) => value || <Tag color="default">未知</Tag>,
  },
  {
    title: '召回路线',
    dataIndex: 'route',
    key: 'route',
    width: 120,
    render: (value?: string) => value || <Tag color="default">暂未返回</Tag>,
  },
  {
    title: '相关度',
    dataIndex: 'score',
    key: 'score',
    width: 120,
    render: (value?: number) =>
      value === undefined || value === null ? <Tag color="warning">暂未返回</Tag> : value.toFixed(4),
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
        <Alert type="warning" showIcon message={`${title} 返回信息不完整`} description={gaps.join(', ')} />
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
    <div className="admin-page">
      {contextHolder}

      <PageHeader
        title="检索链路分析"
        subtitle="按阶段查看一次检索请求的改写、召回、排序、过滤和引用检查结果。高级调试字段保留在下方明细中。"
        extra={
          <>
            <Link href="/retrieval-lab">
              <Button>返回检索调优</Button>
            </Link>
            <Link
              href={
                requestId
                  ? `/trace-logs/retrieval?request_id=${encodeURIComponent(requestId)}`
                  : '/trace-logs/retrieval'
              }
            >
              <Button type="primary">查看链路追踪</Button>
            </Link>
          </>
        }
      />

      <Card className="admin-section-card">
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
              label="请求编号"
              name="request_id"
              rules={[{ required: true, message: '请输入请求编号' }]}
            >
              <Input placeholder="输入请求编号，查看单次检索的链路分析详情" />
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
        <Card className="admin-section-card">
          <ActionEmpty
            title="请输入请求编号"
            description="可从检索调优、链路追踪或评测报告进入，查看完整检索链路分析。"
          />
        </Card>
      ) : loading ? (
        <Card className="admin-section-card">
          <div className="flex justify-center py-12">
            <Spin />
          </div>
        </Card>
      ) : !trace ? (
        <Card className="admin-section-card">
          <ActionEmpty
            title="当前没有可展示的链路数据"
            description="请确认请求编号是否正确，或稍后重新加载。"
          />
        </Card>
      ) : (
        <>
          <Card
            title="请求概览"
            extra={
              <Button
                icon={<CopyOutlined />}
                disabled={!trace.request_id}
                onClick={async () => {
                  if (!trace.request_id) {
                    messageApi.warning('当前缺少请求编号');
                    return;
                  }
                  await navigator.clipboard.writeText(trace.request_id);
                  messageApi.success('请求编号已复制');
                }}
              >
                复制请求编号
              </Button>
            }
            className="admin-section-card"
          >
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="请求编号">{renderValue(trace.request_id)}</Descriptions.Item>
              <Descriptions.Item label="原始问题">
                {renderValue(trace.original_query)}
              </Descriptions.Item>
              <Descriptions.Item label="改写后问题">
                {renderValue(trace.rewritten_query)}
              </Descriptions.Item>
              <Descriptions.Item label="知识库范围">
                {trace.kb_ids?.length ? trace.kb_ids.join(', ') : <Tag color="warning">暂未返回</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="创建时间">{renderValue(trace.created_at)}</Descriptions.Item>
              <Descriptions.Item label="链路明细状态">
                {trace.debug_available ? <Tag color="success">已返回</Tag> : <Tag color="warning">部分缺失</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="降级信息">
                {trace.degradation?.enabled ? (
                  <Space wrap>
                    <Tag color="warning">{trace.degradation.reason || '已降级'}</Tag>
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
              message="链路返回信息不完整"
              description={contractGaps.join('、')}
            />
          ) : null}

          <Collapse
            defaultActiveKey={['query', 'routes', 'topk', 'evidence', 'citation', 'final']}
            items={[
              {
                key: 'query',
                label: '查询改写',
                children: (
                  <Space direction="vertical" size="middle" className="w-full">
                    <Descriptions column={1} size="small" bordered>
                      <Descriptions.Item label="原始问题">
                        {renderValue(trace.original_query)}
                      </Descriptions.Item>
                      <Descriptions.Item label="改写后问题">
                        {renderValue(trace.rewritten_query)}
                      </Descriptions.Item>
                      <Descriptions.Item label="各路线最终查询">
                        {Object.keys(routeFinalQueries).length ? (
                          <Space direction="vertical" size="small">
                            {Object.entries(routeFinalQueries).map(([key, value]) => (
                              <Text key={key}>
                                <Text code>{key}</Text>: {value}
                              </Text>
                            ))}
                          </Space>
                        ) : (
                          <Tag color="warning">暂未返回</Tag>
                        )}
                      </Descriptions.Item>
                      <Descriptions.Item label="改写策略">
                        {renderValue(trace.rewrite_strategy)}
                      </Descriptions.Item>
                      <Descriptions.Item label="收益分层">
                        {renderValue(trace.rewrite_gain_bucket)}
                      </Descriptions.Item>
                      <Descriptions.Item label="命中术语">
                        {renderStringList(trace.term_hits)}
                      </Descriptions.Item>
                    </Descriptions>
                  </Space>
                ),
              },
              {
                key: 'routes',
                label: '召回路线',
                children: routeCards.length ? (
                  <Space direction="vertical" size="middle" className="w-full">
                    {routeCards.map((item) => (
                      <Card key={item.route} size="small" title={item.route} className="admin-section-card">
                        <Descriptions column={1} size="small" bordered>
                          <Descriptions.Item label="贡献占比">
                            {renderValue(item.contribution)}
                          </Descriptions.Item>
                          <Descriptions.Item label="耗时（毫秒）">
                            {renderValue(item.latency_ms)}
                          </Descriptions.Item>
                          <Descriptions.Item label="异常信息">{renderValue(item.error)}</Descriptions.Item>
                        </Descriptions>
                        <Table<DebugDocumentRow>
                          className="mt-4"
                          rowKey="key"
                          size="small"
                          columns={debugDocumentColumns}
                          dataSource={item.rows}
                          pagination={false}
                          locale={{ emptyText: '当前路线没有命中文档' }}
                        />
                      </Card>
                    ))}
                  </Space>
                ) : (
                  <StageFallback
                    title="召回路线"
                    description="当前没有召回路线明细。"
                    gaps={contractGaps.filter((item) => item.startsWith('route_hits'))}
                  />
                ),
              },
              {
                key: 'fusion',
                label: '结果合并与排序',
                children: (
                  <div className="grid gap-4 xl:grid-cols-2">
                    <Card size="small" title="结果合并">
                      <Descriptions column={1} size="small" bordered>
                        <Descriptions.Item label="合并前数量">
                          {trace.fusion_results?.before?.length ?? <Tag color="warning">暂未返回</Tag>}
                        </Descriptions.Item>
                        <Descriptions.Item label="合并后数量">
                          {trace.fusion_results?.after?.length ?? <Tag color="warning">暂未返回</Tag>}
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                    <Card size="small" title="去重处理">
                      <Descriptions column={1} size="small" bordered>
                        <Descriptions.Item label="去重前数量">
                          {renderValue(trace.dedupe_results?.before_count)}
                        </Descriptions.Item>
                        <Descriptions.Item label="去重后数量">
                          {renderValue(trace.dedupe_results?.after_count)}
                        </Descriptions.Item>
                        <Descriptions.Item label="移除数量">
                          {trace.dedupe_results?.removed?.length ?? <Tag color="warning">暂未返回</Tag>}
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                    <Card size="small" title="重排序">
                      <Descriptions column={1} size="small" bordered>
                        <Descriptions.Item label="重排模型">
                          {renderValue(trace.rerank_results?.rerank_model)}
                        </Descriptions.Item>
                        <Descriptions.Item label="回退状态">
                          {renderValue(trace.rerank_results?.fallback)}
                        </Descriptions.Item>
                        <Descriptions.Item label="说明">
                          {renderValue(trace.rerank_results?.reason)}
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                    <Card size="small" title="过滤处理">
                      <Descriptions column={1} size="small" bordered>
                        <Descriptions.Item label="过滤前数量">
                          {renderValue(trace.filter_results?.before_count)}
                        </Descriptions.Item>
                        <Descriptions.Item label="过滤后数量">
                          {renderValue(trace.filter_results?.after_count)}
                        </Descriptions.Item>
                        <Descriptions.Item label="截断原因">
                          {renderValue(trace.filter_results?.truncate_reason)}
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                  </div>
                ),
              },
              {
                key: 'parent-child',
                label: '上下文补全',
                children: trace.parent_child ? (
                  <Space direction="vertical" size="middle" className="w-full">
                    <Descriptions column={1} size="small" bordered>
                      <Descriptions.Item label="是否启用">
                        {renderValue(trace.parent_child.parent_child_enabled)}
                      </Descriptions.Item>
                      <Descriptions.Item label="补全策略">
                        {renderValue(trace.parent_child.parent_fill_strategy)}
                      </Descriptions.Item>
                      <Descriptions.Item label="补全文本量">
                        {renderValue(trace.parent_child.parent_fill_tokens)}
                      </Descriptions.Item>
                      <Descriptions.Item label="回退原因">
                        {renderValue(trace.parent_child.fallback_reason)}
                      </Descriptions.Item>
                    </Descriptions>
                    <div className="grid gap-4 xl:grid-cols-2">
                      <Card size="small" title="命中的子分块">
                        <Table<DebugDocumentRow>
                          rowKey="key"
                          size="small"
                          columns={debugDocumentColumns}
                          dataSource={toDebugRows(trace.parent_child.child_hits, 'child')}
                          pagination={false}
                          locale={{ emptyText: '没有命中的子分块' }}
                        />
                      </Card>
                      <Card size="small" title="补全的父级上下文">
                        <Table<DebugDocumentRow>
                          rowKey="key"
                          size="small"
                          columns={debugDocumentColumns}
                          dataSource={toDebugRows(trace.parent_child.parent_contexts, 'parent')}
                          pagination={false}
                          locale={{ emptyText: '没有补全的父级上下文' }}
                        />
                      </Card>
                    </div>
                  </Space>
                ) : (
                  <StageFallback
                    title="上下文补全"
                    description="当前没有上下文补全数据。"
                    gaps={contractGaps.filter((item) => item.startsWith('parent_child'))}
                  />
                ),
              },
              {
                key: 'topk',
                label: '召回数量决策',
                children: trace.topk_decision ? (
                  <Descriptions column={1} size="small" bordered>
                    <Descriptions.Item label="候选召回数">
                      {renderValue(trace.topk_decision.candidate_topk)}
                    </Descriptions.Item>
                    <Descriptions.Item label="最终返回数">
                      {renderValue(trace.topk_decision.final_topk)}
                    </Descriptions.Item>
                    <Descriptions.Item label="分数分布">
                      {renderValue(trace.topk_decision.score_distribution)}
                    </Descriptions.Item>
                    <Descriptions.Item label="重排间隔">
                      {renderValue(trace.topk_decision.rerank_gap)}
                    </Descriptions.Item>
                    <Descriptions.Item label="证据密度">
                      {renderValue(trace.topk_decision.evidence_density)}
                    </Descriptions.Item>
                    <Descriptions.Item label="上下文预算">
                      {renderValue(trace.topk_decision.token_budget)}
                    </Descriptions.Item>
                    <Descriptions.Item label="决策原因">
                      {renderValue(trace.topk_decision.topk_decision_reason)}
                    </Descriptions.Item>
                  </Descriptions>
                ) : (
                  <StageFallback title="召回数量决策" description="当前没有召回数量决策数据。" />
                ),
              },
              {
                key: 'evidence',
                label: '证据检查',
                children: trace.evidence_gate ? (
                  <Descriptions column={1} size="small" bordered>
                    <Descriptions.Item label="检查结果">
                      {renderValue(trace.evidence_gate.evidence_gate_result)}
                    </Descriptions.Item>
                    <Descriptions.Item label="拒答原因">
                      {renderValue(trace.evidence_gate.refusal_reason)}
                    </Descriptions.Item>
                    <Descriptions.Item label="阈值设置">
                      {trace.evidence_gate.thresholds ? (
                        <Space direction="vertical" size="small">
                          <Text>最低重排得分: {renderValue(trace.evidence_gate.thresholds.min_rerank_score)}</Text>
                          <Text>最低证据密度: {renderValue(trace.evidence_gate.thresholds.min_density)}</Text>
                          <Text>
                            最低引用覆盖率:{' '}
                            {renderValue(trace.evidence_gate.thresholds.min_citation_coverage)}
                          </Text>
                        </Space>
                      ) : (
                        <Tag color="warning">暂未返回</Tag>
                      )}
                    </Descriptions.Item>
                    <Descriptions.Item label="异常信息">
                      {renderValue(trace.evidence_gate.evidence_gate_error)}
                    </Descriptions.Item>
                    <Descriptions.Item label="模板版本">
                      {renderValue(trace.evidence_gate.refusal_template_version)}
                    </Descriptions.Item>
                  </Descriptions>
                ) : (
                  <StageFallback title="证据检查" description="当前没有证据检查数据。" />
                ),
              },
              {
                key: 'citation',
                label: '引用检查',
                children: trace.citation_check ? (
                  <Space direction="vertical" size="middle" className="w-full">
                    <Descriptions column={1} size="small" bordered>
                      <Descriptions.Item label="引用是否通过">
                        {renderValue(trace.citation_check.citation_supported)}
                      </Descriptions.Item>
                      <Descriptions.Item label="引用支持得分">
                        {renderValue(trace.citation_check.citation_support_score)}
                      </Descriptions.Item>
                      <Descriptions.Item label="未支持的陈述">
                        {renderStringList(trace.citation_check.unsupported_claims)}
                      </Descriptions.Item>
                      <Descriptions.Item label="检查版本">
                        {renderValue(trace.citation_check.citation_check_version)}
                      </Descriptions.Item>
                    </Descriptions>
                    <Alert
                      type="info"
                      showIcon
                      message="当前版本暂未返回更细粒度的引用片段对照信息"
                    />
                  </Space>
                ) : (
                  <StageFallback title="引用检查" description="当前没有引用检查数据。" />
                ),
              },
              {
                key: 'final',
                label: '最终结果',
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
                          <Text strong>结果 {index + 1}</Text>
                          <Text>{item.content}</Text>
                          <Space wrap>
                            <Tag>召回路线：{item.source?.route || '暂未返回'}</Tag>
                            <Tag>引用来源：{item.citation?.file_name || '暂未返回'}</Tag>
                            <Tag>分块编号：{item.citation?.chunk_id || '暂未返回'}</Tag>
                            <Tag>知识库：{item.citation?.kb_id ?? '暂未返回'}</Tag>
                          </Space>
                        </Space>
                      </List.Item>
                    )}
                  />
                ) : (
                  <StageFallback title="最终结果" description="当前没有最终结果数据。" />
                ),
              },
              {
                key: 'advanced',
                label: '高级信息',
                children: (
                  <Descriptions column={1} size="small" bordered>
                    <Descriptions.Item label="调试字段状态">
                      {trace.debug_available ? '已返回' : '部分缺失'}
                    </Descriptions.Item>
                    <Descriptions.Item label="契约缺口列表">
                      {contractGaps.length ? contractGaps.join('、') : '无'}
                    </Descriptions.Item>
                  </Descriptions>
                ),
              },
            ]}
          />
        </>
      )}
    </div>
  );
}
