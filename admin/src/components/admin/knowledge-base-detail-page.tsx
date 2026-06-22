'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  DeleteOutlined,
  FileTextOutlined,
  ReloadOutlined,
  SearchOutlined,
  StopOutlined,
  SyncOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Empty,
  Modal,
  Space,
  Spin,
  Table,
  Tabs,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type { UploadFile, UploadProps } from 'antd';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';
import { canDeleteKB, canUploadDocument } from '@/services/auth/permissions';
import { useAuth } from '@/services/auth/store';
import type { KBDocument, KBIngestJob } from '@/types/kb';
import { useKnowledgeBaseContext } from './knowledge-base-provider';

const { Paragraph, Text, Title } = Typography;

const KNOWLEDGE_BASE_STATUS_LABELS: Record<string, string> = {
  active: '可用',
  processing: '处理中',
  building: '处理中',
  syncing: '处理中',
  failed: '异常',
  error: '异常',
  inactive: '停用',
  disabled: '停用',
};

const DOCUMENT_STATUS_LABELS: Record<string, string> = {
  pending: '待入库',
  processing: '处理中',
  completed: '已完成',
  failed: '失败',
};

const JOB_STATUS_LABELS: Record<string, string> = {
  pending: '待处理',
  processing: '处理中',
  completed: '已完成',
  failed: '失败',
  retrying: '重试中',
  dead: '已终止',
  canceled: '已取消',
};

const STATUS_COLOR_MAP: Record<string, string> = {
  pending: 'default',
  processing: 'processing',
  completed: 'success',
  failed: 'error',
  retrying: 'warning',
  dead: 'error',
  canceled: 'default',
};

const ACTIVE_STATUSES = new Set(['pending', 'processing', 'retrying']);

interface PaginatedResponse<T> {
  items?: T[];
  total?: number;
  page?: number;
  page_size?: number;
}

type JobStageKey =
  | 'parse'
  | 'chunk'
  | 'embed'
  | 'write'
  | 'completed'
  | 'failed'
  | 'pending'
  | 'processing'
  | 'retrying'
  | 'canceled';

function formatContractField(value: unknown): string {
  if (value === undefined || value === null || value === '') {
    return '暂未返回';
  }

  return String(value);
}

function getKnowledgeBaseStatusLabel(status?: string): string {
  if (!status) {
    return '未知';
  }

  return KNOWLEDGE_BASE_STATUS_LABELS[status] ?? status;
}

function getDocumentStatusLabel(status?: string): string {
  if (!status) {
    return '未知';
  }

  return DOCUMENT_STATUS_LABELS[status] ?? status;
}

function getJobStatusLabel(status?: string): string {
  if (!status) {
    return '未知';
  }

  return JOB_STATUS_LABELS[status] ?? status;
}

function normalizeJobStage(value: unknown): JobStageKey | null {
  if (typeof value !== 'string') {
    return null;
  }

  const normalized = value.toLowerCase();
  if (normalized.includes('parse')) {
    return 'parse';
  }
  if (normalized.includes('chunk') || normalized.includes('split')) {
    return 'chunk';
  }
  if (normalized.includes('embed') || normalized.includes('vector')) {
    return 'embed';
  }
  if (normalized.includes('write') || normalized.includes('milvus') || normalized.includes('index')) {
    return 'write';
  }
  if (normalized.includes('retry')) {
    return 'retrying';
  }
  if (normalized.includes('cancel')) {
    return 'canceled';
  }
  if (normalized.includes('complete') || normalized.includes('success') || normalized.includes('done')) {
    return 'completed';
  }
  if (normalized.includes('fail') || normalized.includes('dead') || normalized.includes('error')) {
    return 'failed';
  }
  if (normalized.includes('pending') || normalized.includes('queue')) {
    return 'pending';
  }
  if (normalized.includes('process') || normalized.includes('running')) {
    return 'processing';
  }

  return null;
}

function getJobStageKey(job: KBIngestJob): JobStageKey {
  const record = job as KBIngestJob & Record<string, unknown>;
  const candidates = [
    record.stage,
    record.current_stage,
    record.currentStage,
    record.last_stage,
    record.lastStage,
    record.operation,
    record.last_error_code,
    record.last_error_detail,
    record.error_msg,
  ];

  for (const candidate of candidates) {
    const stage = normalizeJobStage(candidate);
    if (stage) {
      return stage;
    }
  }

  const statusStage = normalizeJobStage(job.status);
  return statusStage ?? 'processing';
}

function getJobStageLabel(job: KBIngestJob): string {
  const stage = getJobStageKey(job);

  switch (stage) {
    case 'parse':
      return '解析中';
    case 'chunk':
      return '切分中';
    case 'embed':
      return '向量化中';
    case 'write':
      return '写入中';
    case 'completed':
      return '完成';
    case 'failed':
      return '失败';
    case 'pending':
      return '待处理';
    case 'retrying':
      return '重试中';
    case 'canceled':
      return '已取消';
    default:
      return '处理中';
  }
}

function getJobFailureCode(job: KBIngestJob): string {
  if (job.last_error_code) {
    return job.last_error_code;
  }

  return `JOB-${job.id}`;
}

export function KnowledgeBaseDetailPage({ kbId }: { kbId: number }) {
  const router = useRouter();
  const { user } = useAuth();
  const {
    bases,
    selectedBase,
    isLoading: isBasesLoading,
    error,
    isPermissionDenied,
    deleteBase,
    setSelectedBaseId,
  } = useKnowledgeBaseContext();
  const [documents, setDocuments] = useState<KBDocument[]>([]);
  const [jobs, setJobs] = useState<KBIngestJob[]>([]);
  const [documentsPage, setDocumentsPage] = useState(1);
  const [documentsPageSize, setDocumentsPageSize] = useState(10);
  const [documentsTotal, setDocumentsTotal] = useState(0);
  const [jobsPage, setJobsPage] = useState(1);
  const [jobsPageSize, setJobsPageSize] = useState(10);
  const [jobsTotal, setJobsTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [uploadLoading, setUploadLoading] = useState(false);
  const [actionLoadingId, setActionLoadingId] = useState<number | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [activeTabKey, setActiveTabKey] = useState('documents');
  const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const allowUpload = canUploadDocument(user?.role);
  const allowDeleteKnowledgeBase = canDeleteKB(user?.role);

  const base = useMemo(
    () =>
      bases.find((item) => item.id === kbId) ?? (selectedBase?.id === kbId ? selectedBase : null),
    [bases, kbId, selectedBase]
  );

  const documentsPageRef = useRef(1);
  const documentsPageSizeRef = useRef(10);
  const jobsPageRef = useRef(1);
  const jobsPageSizeRef = useRef(10);
  const failedJobs = useMemo(
    () => jobs.filter((job) => job.status === 'failed' || job.status === 'dead'),
    [jobs]
  );

  const refreshJobs = useCallback(
    async (page?: number, pageSize?: number) => {
      try {
        const p = page ?? jobsPageRef.current;
        const ps = pageSize ?? jobsPageSizeRef.current;
        const jobsData = await (apiClient.get(KB_ADMIN_API.LIST_JOBS_BY_KB(kbId), {
          params: { page: p, page_size: ps },
        }) as Promise<PaginatedResponse<KBIngestJob>>);
        const nextJobs = jobsData?.items ?? [];
        const nextPage = jobsData?.page ?? p;
        const nextPageSize = jobsData?.page_size ?? ps;
        setJobs(nextJobs);
        setJobsTotal(jobsData?.total ?? 0);
        setJobsPage(nextPage);
        setJobsPageSize(nextPageSize);
        jobsPageRef.current = nextPage;
        jobsPageSizeRef.current = nextPageSize;
        return nextJobs;
      } catch {
        return null;
      }
    },
    [kbId]
  );

  const refreshDocuments = useCallback(
    async (page?: number, pageSize?: number) => {
      const p = page ?? documentsPageRef.current;
      const ps = pageSize ?? documentsPageSizeRef.current;
      try {
        const documentsData = await (apiClient.get(KB_ADMIN_API.LIST_DOCUMENTS, {
          params: { kb_id: kbId, page: p, page_size: ps },
        }) as Promise<PaginatedResponse<KBDocument>>);
        const nextPage = documentsData?.page ?? p;
        const nextPageSize = documentsData?.page_size ?? ps;
        setDocuments(documentsData?.items ?? []);
        setDocumentsTotal(documentsData?.total ?? 0);
        setDocumentsPage(nextPage);
        setDocumentsPageSize(nextPageSize);
        documentsPageRef.current = nextPage;
        documentsPageSizeRef.current = nextPageSize;
      } catch (loadError) {
        message.error(loadError instanceof Error ? loadError.message : '加载文档列表失败');
      }
    },
    [kbId]
  );

  const refreshDetail = useCallback(async () => {
    try {
      setIsLoading(true);
      await Promise.all([refreshDocuments(), refreshJobs()]);
    } catch (loadError) {
      message.error(loadError instanceof Error ? loadError.message : '加载知识库详情失败');
    } finally {
      setIsLoading(false);
    }
  }, [refreshDocuments, refreshJobs]);

  useEffect(() => {
    setSelectedBaseId(kbId);
  }, [kbId, setSelectedBaseId]);

  useEffect(() => {
    void refreshDetail();
  }, [refreshDetail]);

  useEffect(() => {
    const hasActiveJobs = jobs.some((job) => ACTIVE_STATUSES.has(job.status));

    if (!hasActiveJobs) {
      if (pollTimerRef.current) {
        clearTimeout(pollTimerRef.current);
        pollTimerRef.current = null;
      }
      return;
    }

    pollTimerRef.current = setTimeout(async () => {
      await refreshJobs();
    }, 3000);

    return () => {
      if (pollTimerRef.current) {
        clearTimeout(pollTimerRef.current);
        pollTimerRef.current = null;
      }
    };
  }, [jobs, refreshJobs]);

  const uploadProps: UploadProps = {
    multiple: true,
    fileList,
    beforeUpload: () => false,
    onChange: ({ fileList: nextFileList }) => setFileList(nextFileList),
    accept: '.pdf,.md,.txt,.markdown,.docx,.html,.htm',
  };

  const copyErrorCode = useCallback(async (job: KBIngestJob) => {
    const code = getJobFailureCode(job);

    try {
      await navigator.clipboard.writeText(code);
      if (job.last_error_code) {
        message.success(`错误编号已复制：${code}`);
      } else {
        message.success(`当前任务未返回错误编号，已复制任务编号：${code}`);
      }
    } catch {
      message.error('复制失败，请手动记录任务编号');
    }
  }, []);

  const showFailureReason = useCallback((job: KBIngestJob) => {
    Modal.info({
      title: `任务 ${job.id} 失败原因`,
      okText: '我知道了',
      width: 640,
      content: (
        <Space direction="vertical" size={12} className="w-full">
          <Text>
            当前阶段：<Tag color="error">{getJobStageLabel(job)}</Tag>
          </Text>
          <Text>
            错误编号：<Text code>{getJobFailureCode(job)}</Text>
          </Text>
          <Text>
            错误摘要：{job.error_msg || '暂无摘要，建议结合任务编号排查后端日志。'}
          </Text>
          <Text>
            补充信息：{job.last_error_detail || '暂未返回补充信息。'}
          </Text>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            建议先确认文档内容与格式是否完整，再检查解析、切分或向量服务状态，确认后可直接回到任务列表重试。
          </Paragraph>
        </Space>
      ),
    });
  }, []);

  const handleDeleteDocument = (document: KBDocument) => {
    Modal.confirm({
      title: '删除文档',
      content: `确认删除 "${document.file_name}"？此操作不可撤销。`,
      okText: '确认删除',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: async () => {
        try {
          await apiClient.delete(KB_ADMIN_API.DELETE_DOCUMENT(document.id));
          message.success('文档已删除');
          await refreshDetail();
        } catch (err) {
          message.error(err instanceof Error ? err.message : '删除文档失败');
        }
      },
    });
  };

  const handleRetryJob = (job: KBIngestJob) => {
    Modal.confirm({
      title: '重试任务',
      content: `确认重试任务 ${job.id}？`,
      okText: '确认重试',
      cancelText: '取消',
      onOk: async () => {
        try {
          setActionLoadingId(job.id);
          await apiClient.post(KB_ADMIN_API.RETRY_JOB(job.id));
          message.success(`已请求重试任务 ${job.id}`);
          await refreshDetail();
        } catch (err) {
          message.error(err instanceof Error ? err.message : `重试任务 ${job.id} 失败`);
        } finally {
          setActionLoadingId(null);
        }
      },
    });
  };

  const handleCancelJob = (job: KBIngestJob) => {
    Modal.confirm({
      title: '取消任务',
      content: `确认取消任务 ${job.id}？`,
      okText: '确认取消',
      cancelText: '返回',
      onOk: async () => {
        try {
          setActionLoadingId(job.id);
          await apiClient.post(KB_ADMIN_API.CANCEL_JOB(job.id));
          message.success(`任务 ${job.id} 已取消`);
          await refreshDetail();
        } catch (err) {
          message.error(err instanceof Error ? err.message : `取消任务 ${job.id} 失败`);
        } finally {
          setActionLoadingId(null);
        }
      },
    });
  };

  const handleUpload = async () => {
    if (fileList.length === 0) {
      message.warning('请至少选择一个文件');
      return;
    }

    const formData = new FormData();
    formData.append('kb_id', String(kbId));
    fileList.forEach((file) => {
      if (file.originFileObj) {
        formData.append('file', file.originFileObj);
      }
    });

    try {
      setUploadLoading(true);
      await apiClient.post(KB_ADMIN_API.UPLOAD_DOCUMENT, formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      message.success('上传成功，正在刷新文档和任务列表...');
      setUploadOpen(false);
      setFileList([]);
      await Promise.all([refreshDocuments(1), refreshJobs(1)]);
    } catch (uploadError) {
      message.error(uploadError instanceof Error ? uploadError.message : '上传失败');
    } finally {
      setUploadLoading(false);
    }
  };

  const documentColumns: ColumnsType<KBDocument> = [
    {
      title: '文件',
      dataIndex: 'file_name',
      key: 'file_name',
      render: (value: string) => (
        <Space>
          <FileTextOutlined />
          <span>{value}</span>
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'file_type',
      key: 'file_type',
      render: (value: string) => <Tag color="blue">{formatContractField(value).toUpperCase()}</Tag>,
    },
    {
      title: '大小',
      dataIndex: 'file_size',
      key: 'file_size',
      render: (value: number) => `${(value / 1024).toFixed(2)} KB`,
    },
    {
      title: '分块数',
      dataIndex: 'chunk_count',
      key: 'chunk_count',
      render: (value: number | undefined) => formatContractField(value),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (value: string) => (
        <Tag color={STATUS_COLOR_MAP[value] || 'default'}>{getDocumentStatusLabel(value)}</Tag>
      ),
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Button
          danger
          type="text"
          icon={<DeleteOutlined />}
          disabled={isPermissionDenied || !allowUpload}
          title={isPermissionDenied || !allowUpload ? '当前角色无权删除文档' : undefined}
          onClick={() => handleDeleteDocument(record)}
        >
          删除
        </Button>
      ),
    },
  ];

  const jobColumns: ColumnsType<KBIngestJob> = [
    { title: '任务 ID', dataIndex: 'id', key: 'id' },
    { title: '文档 ID', dataIndex: 'document_id', key: 'document_id' },
    {
      title: '当前阶段',
      key: 'stage',
      render: (_, record) => {
        const stageLabel = getJobStageLabel(record);
        const stageKey = getJobStageKey(record);
        const color =
          stageKey === 'completed'
            ? 'success'
            : stageKey === 'failed'
              ? 'error'
              : stageKey === 'canceled'
                ? 'default'
                : 'processing';

        return <Tag color={color}>{stageLabel}</Tag>;
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (value: string) => <Tag color={STATUS_COLOR_MAP[value] || 'default'}>{getJobStatusLabel(value)}</Tag>,
    },
    {
      title: '重试次数',
      dataIndex: 'retry_count',
      key: 'retry_count',
      render: (value: number | undefined) => formatContractField(value),
    },
    {
      title: '错误编号',
      key: 'error_code',
      render: (_, record) =>
        record.status === 'failed' || record.status === 'dead' ? (
          <Text code>{getJobFailureCode(record)}</Text>
        ) : (
          <Text type="secondary">-</Text>
        ),
    },
    {
      title: '错误信息',
      dataIndex: 'error_msg',
      key: 'error_msg',
      render: (value: string | undefined) => formatContractField(value),
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Space>
          {(record.status === 'failed' || record.status === 'dead') && (
            <Button type="text" onClick={() => showFailureReason(record)}>
              查看原因
            </Button>
          )}
          {(record.status === 'failed' || record.status === 'dead') && (
            <Button
              type="text"
              icon={<ReloadOutlined />}
              loading={actionLoadingId === record.id}
              disabled={isPermissionDenied || !allowUpload}
              title={isPermissionDenied || !allowUpload ? '当前角色无权重试任务' : undefined}
              onClick={() => handleRetryJob(record)}
            >
              重试
            </Button>
          )}
          {(record.status === 'failed' || record.status === 'dead') && (
            <Button type="text" onClick={() => void copyErrorCode(record)}>
              复制错误编号
            </Button>
          )}
          {!['completed', 'canceled', 'dead'].includes(record.status) && (
            <Button
              danger
              type="text"
              icon={<StopOutlined />}
              loading={actionLoadingId === record.id}
              disabled={isPermissionDenied || !allowUpload}
              title={isPermissionDenied || !allowUpload ? '当前角色无权取消任务' : undefined}
              onClick={() => handleCancelJob(record)}
            >
              取消
            </Button>
          )}
        </Space>
      ),
    },
  ];

  if (!isBasesLoading && !base) {
    return (
      <Card>
        <Empty
          description="请求的知识库不存在或暂不可用。"
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        >
          <Button type="primary" onClick={() => router.push('/knowledge-bases')}>
            返回列表
          </Button>
        </Empty>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      {error && !isPermissionDenied ? <Alert type="warning" showIcon message={error} /> : null}
      {isPermissionDenied ? (
        <Alert
          type="error"
          showIcon
          message="权限不足"
          description="当前账号无权访问知识库数据（403）。请联系管理员确认权限配置。"
        />
      ) : null}
      {failedJobs.length > 0 ? (
        <Alert
          type="warning"
          showIcon
          message={`当前有 ${failedJobs.length} 个失败的入库任务待处理`}
          description="建议先查看失败原因并记录错误编号，确认文档或服务状态后再执行重试。"
          action={
            <Button
              size="small"
              onClick={() => {
                setActiveTabKey('jobs');
              }}
            >
              前往入库任务
            </Button>
          }
        />
      ) : null}

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            {base?.name ?? '知识库详情'}
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            在这里维护文档资产、跟进入库任务，并持续校验知识库是否已经准备好服务检索问答。
          </Paragraph>
        </div>
        <Space wrap>
          <Button icon={<ReloadOutlined />} onClick={() => void refreshDetail()}>
            刷新
          </Button>
          <Link href="/retrieval-lab">
            <Button icon={<SearchOutlined />}>进入检索调优</Button>
          </Link>
          <Button
            danger
            icon={<DeleteOutlined />}
            loading={deleteLoading}
            disabled={isPermissionDenied || !allowDeleteKnowledgeBase}
            title={
              isPermissionDenied || !allowDeleteKnowledgeBase
                ? '当前角色无权删除知识库'
                : undefined
            }
            onClick={() => {
              Modal.confirm({
                title: '删除知识库',
                content: `删除 "${base?.name || `#${kbId}`}" 及其绑定的向量 collection？`,
                okText: '删除',
                cancelText: '取消',
                okButtonProps: { danger: true },
                onOk: async () => {
                  try {
                    setDeleteLoading(true);
                    await deleteBase(kbId);
                    router.push('/knowledge-bases');
                  } finally {
                    setDeleteLoading(false);
                  }
                },
              });
            }}
          >
            删除知识库
          </Button>
          <Button
            type="primary"
            icon={<UploadOutlined />}
            disabled={isPermissionDenied || !allowUpload}
            title={isPermissionDenied || !allowUpload ? '当前角色无权上传文档' : undefined}
            onClick={() => setUploadOpen(true)}
          >
            上传文档
          </Button>
        </Space>
      </div>

      <Card>
        <Space direction="vertical" size={8}>
          <Text>
            状态：{' '}
            <Tag color={base?.status === 'active' ? 'success' : 'default'}>
              {getKnowledgeBaseStatusLabel(base?.status)}
            </Tag>
          </Text>
          <Text>向量集合：{base?.vector_collection || '待系统生成'}</Text>
          <Text>描述：{base?.description || '暂无说明，建议补充知识范围和维护场景。'}</Text>
          <Text>创建时间：{base?.created_at ?? '未知'}</Text>
        </Space>
      </Card>

      <Spin spinning={isLoading || isBasesLoading} indicator={<SyncOutlined spin />}>
        <Tabs
          activeKey={activeTabKey}
          onChange={setActiveTabKey}
          items={[
            {
              key: 'documents',
              label: `文档（${documentsTotal}）`,
              children: (
                <Card>
                  <Paragraph type="secondary">
                    上传文档后，系统会依次完成解析、切分、向量化和写入，处理进度可在下方入库任务中持续跟踪。
                  </Paragraph>
                  <Table
                    rowKey="id"
                    columns={documentColumns}
                    dataSource={documents}
                    locale={{
                      emptyText: (
                        <Empty
                          description="还没有文档。先上传第一份文档，系统会自动创建对应的入库任务。"
                          image={Empty.PRESENTED_IMAGE_SIMPLE}
                        >
                          <Space>
                            <Button
                              type="primary"
                              icon={<UploadOutlined />}
                              disabled={isPermissionDenied || !allowUpload}
                              onClick={() => setUploadOpen(true)}
                            >
                              上传第一份文档
                            </Button>
                            <Button onClick={() => void refreshDocuments()}>刷新文档列表</Button>
                          </Space>
                        </Empty>
                      ),
                    }}
                    pagination={{
                      current: documentsPage,
                      pageSize: documentsPageSize,
                      total: documentsTotal,
                      showSizeChanger: true,
                      onChange: (p, ps) => {
                        setDocumentsPage(p);
                        setDocumentsPageSize(ps);
                        void refreshDocuments(p, ps);
                      },
                    }}
                  />
                </Card>
              ),
            },
            {
              key: 'jobs',
              label: `入库任务（${jobsTotal}）`,
              children: (
                <Card>
                  <Table
                    rowKey="id"
                    columns={jobColumns}
                    dataSource={jobs}
                    locale={{
                      emptyText: (
                        <Empty
                          description={
                            documentsTotal > 0
                              ? '暂时还没有入库任务。可刷新列表，或重新上传文档以触发新的处理流程。'
                              : '还没有入库任务。请先上传文档，系统会自动生成对应的入库任务。'
                          }
                          image={Empty.PRESENTED_IMAGE_SIMPLE}
                        >
                          <Space>
                            {documentsTotal > 0 ? (
                              <Button onClick={() => void refreshJobs()}>刷新任务列表</Button>
                            ) : (
                              <Button
                                type="primary"
                                icon={<UploadOutlined />}
                                disabled={isPermissionDenied || !allowUpload}
                                onClick={() => setUploadOpen(true)}
                              >
                                先上传文档
                              </Button>
                            )}
                            <Button onClick={() => setActiveTabKey('documents')}>查看文档列表</Button>
                          </Space>
                        </Empty>
                      ),
                    }}
                    pagination={{
                      current: jobsPage,
                      pageSize: jobsPageSize,
                      total: jobsTotal,
                      showSizeChanger: true,
                      onChange: (p, ps) => {
                        setJobsPage(p);
                        setJobsPageSize(ps);
                        void refreshJobs(p, ps);
                      },
                    }}
                  />
                </Card>
              ),
            },
          ]}
        />
      </Spin>

      <Modal
        title="上传文档"
        open={uploadOpen}
        okText="上传"
        cancelText="取消"
        confirmLoading={uploadLoading}
        onCancel={() => {
          if (uploadLoading) return;
          setUploadOpen(false);
          setFileList([]);
        }}
        onOk={() => void handleUpload()}
        destroyOnHidden
      >
        <Space direction="vertical" className="w-full">
          <Text type="secondary">当前知识库：{base?.name ?? `#${kbId}`}</Text>
          <Upload {...uploadProps} maxCount={10}>
            <Button icon={<UploadOutlined />}>选择文件</Button>
          </Upload>
          <Text type="secondary">
            上传后会自动进入解析、切分、向量化和写入流程。支持格式：PDF、MD、TXT、MARKDOWN、DOCX、HTML、HTM
          </Text>
        </Space>
      </Modal>
    </div>
  );
}
