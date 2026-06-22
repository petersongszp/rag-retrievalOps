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
import { ActionEmpty } from './ui/action-empty';
import { PageHeader } from './ui/page-header';

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
  { label: '待处理', value: 'pending' },
  { label: '处理中', value: 'processing' },
  { label: '已完成', value: 'completed' },
  { label: '失败', value: 'failed' },
  { label: '重试中', value: 'retrying' },
  { label: '已终止', value: 'dead' },
  { label: '已取消', value: 'canceled' },
];

function statusLabel(status: KBIngestJob['status']) {
  switch (status) {
    case 'pending':
      return '待处理';
    case 'processing':
      return '处理中';
    case 'completed':
      return '已完成';
    case 'failed':
      return '失败';
    case 'retrying':
      return '重试中';
    case 'dead':
      return '已终止';
    case 'canceled':
      return '已取消';
    default:
      return status;
  }
}

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
      { title: '知识库', dataIndex: 'kb_id', key: 'kb_id', width: 90 },
      { title: '文档', dataIndex: 'document_id', key: 'document_id', width: 110 },
      {
        title: '状态',
        dataIndex: 'status',
        key: 'status',
        width: 120,
        render: (value: KBIngestJob['status']) => <Tag color={statusColor(value)}>{statusLabel(value)}</Tag>,
      },
      {
        title: '错误编号',
        dataIndex: 'last_error_code',
        key: 'last_error_code',
        width: 150,
        render: (value?: string) => value || '-',
      },
      {
        title: '操作',
        dataIndex: 'operation',
        key: 'operation',
        width: 110,
        render: (value?: string) => value || '-',
      },
      {
        title: '原因',
        dataIndex: 'operation_reason',
        key: 'operation_reason',
        ellipsis: true,
        render: (value?: string) => value || '-',
      },
      {
        title: '创建时间',
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
    <div className="admin-page">
      <PageHeader
        title="入库链路追踪"
        subtitle="按知识库、状态和错误类型定位一次文档处理流程，查看任务状态、错误原因和人工操作记录。"
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
                placeholder="按错误编号过滤"
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
              查询链路
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

      <Card className="admin-section-card">
        {items.length === 0 && !isLoading ? (
          <ActionEmpty
            title="当前筛选条件下没有入库链路"
            description="可以先上传文档触发入库流程，或放宽筛选条件后重新查询。"
          />
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
            <Descriptions title="基础信息" column={1} size="small" bordered>
              <Descriptions.Item label="Job ID">{detail.job.id}</Descriptions.Item>
              <Descriptions.Item label="知识库">{detail.job.kb_id}</Descriptions.Item>
              <Descriptions.Item label="文档">{detail.job.document_id}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={statusColor(detail.job.status)}>{statusLabel(detail.job.status)}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="重试次数">{detail.job.retry_count}</Descriptions.Item>
              <Descriptions.Item label="错误编号">{detail.job.last_error_code || '-'}</Descriptions.Item>
              <Descriptions.Item label="错误详情">
                {detail.job.last_error_detail || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="操作">{detail.job.operation || '-'}</Descriptions.Item>
              <Descriptions.Item label="操作原因">
                {detail.job.operation_reason || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="操作时间">
                {detail.job.operated_at ? dayjs(detail.job.operated_at).format('YYYY-MM-DD HH:mm:ss') : '-'}
              </Descriptions.Item>
            </Descriptions>

            <Card title="操作审计记录" className="admin-section-card">
              {detail.operation_logs.length === 0 ? (
                <ActionEmpty
                  title="该任务暂无操作审计记录"
                  description="说明当前任务还没有人工介入或额外状态流转记录。"
                />
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
                    { title: '状态流转', key: 'transition', render: (_, record) => `${statusLabel(record.from_status as KBIngestJob['status'])} -> ${statusLabel(record.to_status as KBIngestJob['status'])}` },
                    { title: '原因', dataIndex: 'operation_reason', key: 'operation_reason', render: (value?: string) => value || '-' },
                  ]}
                />
              )}
            </Card>
          </Space>
        ) : (
          <ActionEmpty
            title="请选择一条入库链路查看详情"
            description="点击上方任意一行，可展开查看任务状态、错误原因和操作记录。"
          />
        )}
      </Drawer>
    </div>
  );
}
