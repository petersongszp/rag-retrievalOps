'use client';

import { useEffect, useMemo, useState } from 'react';
import dayjs, { type Dayjs } from 'dayjs';
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
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';
import type {
  IngestLogDetail,
  KBIngestJob,
  ListResponse,
} from '@/types/kb';
import { useKnowledgeBaseContext } from './knowledge-base-provider';

const { Paragraph, Title } = Typography;

type IngestLogFormValues = {
  kb_id?: number;
  status?: KBIngestJob['status'];
  error_code?: string;
  range?: [Dayjs, Dayjs];
};

type IngestLogFilters = {
  kb_id?: number;
  status?: KBIngestJob['status'];
  error_code?: string;
  start_time?: string;
  end_time?: string;
};

const PAGE_SIZE = 20;

const statusOptions: Array<{ label: string; value: KBIngestJob['status'] }> = [
  { label: 'pending', value: 'pending' },
  { label: 'processing', value: 'processing' },
  { label: 'completed', value: 'completed' },
  { label: 'failed', value: 'failed' },
  { label: 'retrying', value: 'retrying' },
  { label: 'dead', value: 'dead' },
  { label: 'canceled', value: 'canceled' },
];

function buildFilters(values: IngestLogFormValues): IngestLogFilters {
  return {
    kb_id: values.kb_id,
    status: values.status,
    error_code: values.error_code || undefined,
    start_time: values.range?.[0]?.toISOString(),
    end_time: values.range?.[1]?.toISOString(),
  };
}

function statusColor(status: KBIngestJob['status']) {
  switch (status) {
    case 'completed':
      return 'success';
    case 'processing':
    case 'retrying':
      return 'processing';
    case 'failed':
    case 'dead':
      return 'error';
    case 'canceled':
      return 'default';
    default:
      return 'gold';
  }
}

export function IngestLogsPage() {
  const { bases, selectedBase } = useKnowledgeBaseContext();
  const [form] = Form.useForm<IngestLogFormValues>();
  const [filters, setFilters] = useState<IngestLogFilters>({});
  const [items, setItems] = useState<KBIngestJob[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [detail, setDetail] = useState<IngestLogDetail | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

  const columns = useMemo<ColumnsType<KBIngestJob>>(
    () => [
      { title: 'Job ID', dataIndex: 'id', key: 'id', width: 100 },
      { title: 'KB', dataIndex: 'kb_id', key: 'kb_id', width: 90 },
      { title: 'Document', dataIndex: 'document_id', key: 'document_id', width: 110 },
      {
        title: 'Status',
        dataIndex: 'status',
        key: 'status',
        width: 120,
        render: (value: KBIngestJob['status']) => <Tag color={statusColor(value)}>{value}</Tag>,
      },
      {
        title: 'Error Code',
        dataIndex: 'last_error_code',
        key: 'last_error_code',
        width: 150,
        render: (value?: string) => value || '-',
      },
      {
        title: 'Operation',
        dataIndex: 'operation',
        key: 'operation',
        width: 110,
        render: (value?: string) => value || '-',
      },
      {
        title: 'Reason',
        dataIndex: 'operation_reason',
        key: 'operation_reason',
        ellipsis: true,
        render: (value?: string) => value || '-',
      },
      {
        title: 'Created',
        dataIndex: 'created_at',
        key: 'created_at',
        width: 180,
        render: (value: string) => dayjs(value).format('YYYY-MM-DD HH:mm:ss'),
      },
    ],
    []
  );

  const loadList = async (nextFilters: IngestLogFilters, nextPage: number) => {
    try {
      setIsLoading(true);
      setError(null);
      const data = (await apiClient.get(KB_ADMIN_API.LIST_INGEST_LOGS, {
        params: {
          page: nextPage,
          page_size: PAGE_SIZE,
          ...(nextFilters.kb_id ? { kb_id: nextFilters.kb_id } : {}),
          ...(nextFilters.status ? { status: nextFilters.status } : {}),
          ...(nextFilters.error_code ? { error_code: nextFilters.error_code } : {}),
          ...(nextFilters.start_time ? { start_time: nextFilters.start_time } : {}),
          ...(nextFilters.end_time ? { end_time: nextFilters.end_time } : {}),
        },
      })) as ListResponse<KBIngestJob>;
      setItems(data.items ?? []);
      setPage(data.page ?? nextPage);
      setTotal(data.total ?? 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载入库日志失败');
      setItems([]);
      setTotal(0);
    } finally {
      setIsLoading(false);
    }
  };

  const openDetail = async (jobId: number) => {
    try {
      setDetailOpen(true);
      setDetailLoading(true);
      setDetailError(null);
      const data = (await apiClient.get(KB_ADMIN_API.GET_INGEST_LOG_DETAIL(jobId))) as IngestLogDetail;
      setDetail(data);
    } catch (err) {
      setDetailError(err instanceof Error ? err.message : '加载入库详情失败');
      setDetail(null);
    } finally {
      setDetailLoading(false);
    }
  };

  useEffect(() => {
    const nextFilters = { kb_id: selectedBase?.id } as IngestLogFilters;
    setFilters(nextFilters);
    form.setFieldsValue({ kb_id: selectedBase?.id, status: undefined, error_code: undefined, range: undefined });
    void loadList(nextFilters, 1);
  }, [form, selectedBase?.id]);

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            入库日志
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            查看入库任务状态机和操作审计记录，定位失败原因与人工操作轨迹。
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
            void loadList(nextFilters, 1);
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
              <Select allowClear placeholder="全部状态" options={statusOptions} />
            </Form.Item>
            <Form.Item label="错误类型" name="error_code">
              <Select
                allowClear
                placeholder="按 error_code 过滤"
                options={[
                  { label: 'embedding_failed', value: 'embedding_failed' },
                  { label: 'milvus_error', value: 'milvus_error' },
                  { label: 'canceled_by_operator', value: 'canceled_by_operator' },
                ]}
              />
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
                const nextFilters = { kb_id: selectedBase?.id } as IngestLogFilters;
                setFilters(nextFilters);
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
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前筛选条件下没有入库日志" />
        ) : (
          <Table<KBIngestJob>
            rowKey="id"
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
              onClick: () => void openDetail(record.id),
              style: { cursor: 'pointer' },
            })}
          />
        )}
      </Card>

      <Drawer
        title={detail?.job ? `入库详情 · Job ${detail.job.id}` : '入库详情'}
        width={620}
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
      >
        {detailLoading ? (
          <div className="py-10 text-center">加载中...</div>
        ) : detailError ? (
          <Alert type="error" showIcon message={detailError} />
        ) : detail?.job ? (
          <Space direction="vertical" size="large" className="w-full">
            <Descriptions title="任务信息" column={1} size="small" bordered>
              <Descriptions.Item label="Job ID">{detail.job.id}</Descriptions.Item>
              <Descriptions.Item label="KB ID">{detail.job.kb_id}</Descriptions.Item>
              <Descriptions.Item label="Document ID">{detail.job.document_id}</Descriptions.Item>
              <Descriptions.Item label="Status">
                <Tag color={statusColor(detail.job.status)}>{detail.job.status}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Retry Count">{detail.job.retry_count}</Descriptions.Item>
              <Descriptions.Item label="Error Code">{detail.job.last_error_code || '-'}</Descriptions.Item>
              <Descriptions.Item label="Error Detail">
                {detail.job.last_error_detail || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="Operation">{detail.job.operation || '-'}</Descriptions.Item>
              <Descriptions.Item label="Operation Reason">
                {detail.job.operation_reason || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="Operated At">
                {detail.job.operated_at ? dayjs(detail.job.operated_at).format('YYYY-MM-DD HH:mm:ss') : '-'}
              </Descriptions.Item>
            </Descriptions>

            <Card title="操作审计记录">
              {detail.operation_logs.length === 0 ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="该任务暂无操作审计记录" />
              ) : (
                <Table
                  rowKey="id"
                  pagination={false}
                  size="small"
                  dataSource={detail.operation_logs}
                  columns={[
                    { title: '时间', dataIndex: 'created_at', key: 'created_at', render: (value: string) => dayjs(value).format('YYYY-MM-DD HH:mm:ss') },
                    { title: '操作人', dataIndex: 'operator_id', key: 'operator_id' },
                    { title: '操作', dataIndex: 'operation', key: 'operation' },
                    { title: '状态流转', key: 'transition', render: (_, record) => `${record.from_status} -> ${record.to_status}` },
                    { title: '原因', dataIndex: 'operation_reason', key: 'operation_reason', render: (value?: string) => value || '-' },
                  ]}
                />
              )}
            </Card>
          </Space>
        ) : (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="请选择一条入库日志查看详情" />
        )}
      </Drawer>
    </div>
  );
}
