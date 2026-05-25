'use client';

import { useEffect, useMemo, useState } from 'react';
import { CopyOutlined, SearchOutlined, WarningOutlined } from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Collapse,
  Empty,
  Form,
  Input,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';
import type { RetrieveItem, RetrieveResponse } from '@/types/kb';
import { useKnowledgeBaseContext } from './knowledge-base-provider';

const { Paragraph, Text, Title } = Typography;

interface ContractGapEntry {
  field: string;
  interface: string;
  affectedPage: string;
  blocksAcceptance: boolean;
}

const FIELD_META: Record<string, Omit<ContractGapEntry, 'field'>> = {
  score: {
    interface: 'POST /admin/kb/retrieve → items[].score',
    affectedPage: '检索实验室',
    blocksAcceptance: true,
  },
  citation: {
    interface: 'POST /admin/kb/retrieve → items[].citation',
    affectedPage: '检索实验室',
    blocksAcceptance: true,
  },
  'citation.file_name': {
    interface: 'POST /admin/kb/retrieve → items[].citation.file_name',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
  'citation.chunk_index': {
    interface: 'POST /admin/kb/retrieve → items[].citation.chunk_index',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
  'citation.chunk_id': {
    interface: 'POST /admin/kb/retrieve → items[].citation.chunk_id',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
  source: {
    interface: 'POST /admin/kb/retrieve → items[].source',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
  'source.route': {
    interface: 'POST /admin/kb/retrieve → items[].source.route',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
  'source.collection': {
    interface: 'POST /admin/kb/retrieve → items[].source.collection',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
  'source.retriever_version': {
    interface: 'POST /admin/kb/retrieve → items[].source.retriever_version',
    affectedPage: '检索实验室',
    blocksAcceptance: false,
  },
};

const gapColumns: ColumnsType<ContractGapEntry> = [
  {
    title: '缺失字段',
    dataIndex: 'field',
    key: 'field',
    render: (v: string) => <Text code>{v}</Text>,
  },
  { title: '所属接口', dataIndex: 'interface', key: 'interface' },
  { title: '影响页面', dataIndex: 'affectedPage', key: 'affectedPage' },
  {
    title: '阻塞验收',
    dataIndex: 'blocksAcceptance',
    key: 'blocksAcceptance',
    render: (v: boolean) =>
      v ? <Tag color="error">是</Tag> : <Tag color="default">否</Tag>,
  },
];

function findContractGaps(item: RetrieveItem): string[] {
  const gaps: string[] = [];

  if (item.score === undefined || item.score === null) {
    gaps.push('score');
  }

  if (!item.citation) {
    gaps.push('citation');
  } else {
    if (!item.citation.file_name) gaps.push('citation.file_name');
    if (item.citation.chunk_index === undefined || item.citation.chunk_index === null)
      gaps.push('citation.chunk_index');
    if (!item.citation.chunk_id) gaps.push('citation.chunk_id');
  }

  if (!item.source) {
    gaps.push('source');
  } else {
    if (!item.source.route) gaps.push('source.route');
    if (!item.source.collection) gaps.push('source.collection');
    if (!item.source.retriever_version) gaps.push('source.retriever_version');
  }

  return gaps;
}

export function RetrievalLabPage() {
  const { selectedBase, setSelectedBaseId, bases } = useKnowledgeBaseContext();
  const [form] = Form.useForm<{ query: string; top_k: number; kb_id: number }>();
  const [result, setResult] = useState<RetrieveResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (selectedBase?.id) {
      form.setFieldsValue({ kb_id: selectedBase.id });
    }
  }, [form, selectedBase?.id]);

  const onFinish = async (values: { query: string; top_k: number; kb_id: number }) => {
    try {
      setIsLoading(true);
      setError(null);
      setSelectedBaseId(values.kb_id);
      const response = (await apiClient.post(KB_ADMIN_API.RETRIEVE, values)) as RetrieveResponse;
      setResult(response);
    } catch (err) {
      setError(err instanceof Error ? err.message : '检索请求失败');
      setResult(null);
    } finally {
      setIsLoading(false);
    }
  };

  // Deduplicated gap log across all result items
  const gapLog = useMemo<ContractGapEntry[]>(() => {
    if (!result) return [];
    const seen = new Set<string>();
    const entries: ContractGapEntry[] = [];
    for (const item of result.items) {
      for (const field of findContractGaps(item)) {
        if (!seen.has(field)) {
          seen.add(field);
          const meta = FIELD_META[field];
          if (meta) {
            entries.push({ field, ...meta });
          }
        }
      }
    }
    return entries;
  }, [result]);

  return (
    <div className="space-y-6">
      <div>
        <Title level={2} style={{ marginBottom: 8 }}>
          检索实验室
        </Title>
        <Paragraph type="secondary" style={{ marginBottom: 0 }}>
          此页面是 L1 为检索测试工作流及请求级追踪交接预留的专属路由。
        </Paragraph>
      </div>

      <Card>
        <Form
          form={form}
          layout="vertical"
          initialValues={{
            kb_id: selectedBase?.id,
            top_k: 5,
          }}
          onFinish={(values) => void onFinish(values)}
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
          <Button type="primary" htmlType="submit" icon={<SearchOutlined />} loading={isLoading}>
            运行检索测试
          </Button>
        </Form>
      </Card>

      {error ? <Alert type="error" showIcon message={error} /> : null}

      {!result ? (
        <Card>
          <Empty
            description={
              selectedBase
                ? `对"${selectedBase.name}"运行测试，检查 request_id、score、citation 和 source 字段。`
                : '请选择知识库并运行检索测试。'
            }
            image={Empty.PRESENTED_IMAGE_SIMPLE}
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
              <Button
                icon={<CopyOutlined />}
                onClick={async () => {
                  if (!result.request_id) {
                    message.warning('后端未返回 request_id');
                    return;
                  }
                  await navigator.clipboard.writeText(result.request_id);
                  message.success('request_id 已复制');
                }}
              >
                复制 request_id
              </Button>
            </div>
          </Card>

          {result.items.length === 0 ? (
            <Card>
              <Empty description="未返回检索结果。" image={Empty.PRESENTED_IMAGE_SIMPLE} />
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
                      <Tag>Chunk Index: {item.citation?.chunk_index ?? 'Contract gap'}</Tag>
                      <Tag>Chunk ID: {item.citation?.chunk_id || 'Contract gap'}</Tag>
                      <Tag>Route: {item.source?.route || 'Contract gap'}</Tag>
                      <Tag>Collection: {item.source?.collection || 'Contract gap'}</Tag>
                      <Tag>Retriever: {item.source?.retriever_version || 'Contract gap'}</Tag>
                    </Space>
                    {gaps.length > 0 ? (
                      <Alert
                        type="warning"
                        showIcon
                        message={`Contract gaps: ${gaps.join(', ')}`}
                      />
                    ) : null}
                  </Space>
                </Card>
              );
            })
          )}

          {gapLog.length > 0 && (
            <Collapse
              items={[
                {
                  key: 'gap-log',
                  label: (
                    <Space>
                      <WarningOutlined style={{ color: '#faad14' }} />
                      <Text>契约缺口记录（{gapLog.length} 项）— 用于联调交接</Text>
                    </Space>
                  ),
                  children: (
                    <Table
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
          )}
        </Space>
      )}
    </div>
  );
}
