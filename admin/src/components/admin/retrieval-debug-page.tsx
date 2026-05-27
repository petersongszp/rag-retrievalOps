'use client';

import Link from 'next/link';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { CopyOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Empty,
  Form,
  Input,
  Space,
  Spin,
  Tag,
  Typography,
  message,
} from 'antd';
import { KB_ADMIN_API } from '@/config/api';
import apiClient from '@/services/api/client';
import type { RetrievalDebugTrace } from '@/types/kb';

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

function renderValue(value?: string | number | boolean | null) {
  if (value === undefined || value === null || value === '') {
    return <Tag color="warning">契约缺口</Tag>;
  }
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false';
  }
  return String(value);
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
      setError(normalizeError(loadError, '加载调试视图失败'));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    form.setFieldsValue({ request_id: requestId });
    void loadTrace(requestId);
  }, [form, loadTrace, requestId]);

  const routeSummary = useMemo(() => {
    if (!trace?.route_hits?.length) {
      return null;
    }
    return trace.route_hits.map((item) => ({
      key: item.route,
      label: item.route,
      contribution: item.contribution ?? 0,
      latency: item.latency_ms ?? 0,
      hits: item.hits?.length ?? 0,
      error: item.error,
    }));
  }, [trace]);

  return (
    <div className="space-y-6">
      {contextHolder}

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            检索调试视图
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            先建立稳定的 debug 路由、契约与基础摘要，后续阶段再把 route、fusion、rerank、 evidence
            和 citation 细节逐块补齐。
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
            <Button type="primary">查看 Trace Logs</Button>
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
              params.toString()
                ? `/retrieval-lab/debug?${params.toString()}`
                : '/retrieval-lab/debug'
            );
          }}
        >
          <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto_auto]">
            <Form.Item
              label="Request ID"
              name="request_id"
              rules={[{ required: true, message: '请输入 request_id' }]}
            >
              <Input placeholder="输入 request_id 以加载调试详情" />
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
          <div className="flex justify-center py-10">
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
            title="请求摘要"
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
              <Descriptions.Item label="Request ID">
                {renderValue(trace.request_id)}
              </Descriptions.Item>
              <Descriptions.Item label="Original Query">
                {renderValue(trace.original_query)}
              </Descriptions.Item>
              <Descriptions.Item label="Rewritten Query">
                {renderValue(trace.rewritten_query)}
              </Descriptions.Item>
              <Descriptions.Item label="KB IDs">
                {trace.kb_ids?.length ? (
                  trace.kb_ids.join(', ')
                ) : (
                  <Tag color="warning">契约缺口</Tag>
                )}
              </Descriptions.Item>
              <Descriptions.Item label="Created At">
                {renderValue(trace.created_at)}
              </Descriptions.Item>
              <Descriptions.Item label="Debug Available">
                {trace.debug_available ? (
                  <Tag color="success">true</Tag>
                ) : (
                  <Tag color="warning">false</Tag>
                )}
              </Descriptions.Item>
              <Descriptions.Item label="Degradation">
                {trace.degradation?.enabled ? (
                  <Space wrap>
                    <Tag color="warning">{trace.degradation.reason || 'degraded'}</Tag>
                    {trace.degradation.error_code ? (
                      <Text code>{trace.degradation.error_code}</Text>
                    ) : null}
                  </Space>
                ) : (
                  <Tag color="success">none</Tag>
                )}
              </Descriptions.Item>
            </Descriptions>
          </Card>

          {trace.contract_gaps?.length ? (
            <Alert
              type="warning"
              showIcon
              message="检测到契约缺口"
              description={trace.contract_gaps.join(', ')}
            />
          ) : null}

          <div className="grid gap-4 xl:grid-cols-2">
            <Card title="Route Hits 摘要">
              {!routeSummary?.length ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description="当前没有 route hits 数据。"
                />
              ) : (
                <Space direction="vertical" size="middle" className="w-full">
                  {routeSummary.map((item) => (
                    <Card key={item.key} size="small">
                      <Descriptions column={1} size="small">
                        <Descriptions.Item label="Route">{item.label}</Descriptions.Item>
                        <Descriptions.Item label="Contribution">
                          {item.contribution}
                        </Descriptions.Item>
                        <Descriptions.Item label="Latency">{item.latency} ms</Descriptions.Item>
                        <Descriptions.Item label="Hit Count">{item.hits}</Descriptions.Item>
                        <Descriptions.Item label="Error">
                          {item.error ? (
                            <Tag color="error">{item.error}</Tag>
                          ) : (
                            <Tag color="success">none</Tag>
                          )}
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                  ))}
                </Space>
              )}
            </Card>

            <Card title="高风险决策摘要">
              <Descriptions column={1} size="small" bordered>
                <Descriptions.Item label="TopK Reason">
                  {renderValue(trace.topk_decision?.topk_decision_reason)}
                </Descriptions.Item>
                <Descriptions.Item label="Evidence Gate">
                  {renderValue(trace.evidence_gate?.evidence_gate_result)}
                </Descriptions.Item>
                <Descriptions.Item label="Refusal Reason">
                  {renderValue(trace.evidence_gate?.refusal_reason)}
                </Descriptions.Item>
                <Descriptions.Item label="Citation Supported">
                  {renderValue(trace.citation_check?.citation_supported)}
                </Descriptions.Item>
                <Descriptions.Item label="Citation Support Score">
                  {renderValue(trace.citation_check?.citation_support_score)}
                </Descriptions.Item>
                <Descriptions.Item label="Unsupported Claims">
                  {trace.citation_check?.unsupported_claims?.length ? (
                    trace.citation_check.unsupported_claims.join(', ')
                  ) : (
                    <Tag color="default">none</Tag>
                  )}
                </Descriptions.Item>
              </Descriptions>
            </Card>
          </div>
        </>
      )}
    </div>
  );
}
