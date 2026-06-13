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

function formatContractField(value: unknown): string {
  if (value === undefined || value === null || value === '') {
    return 'Contract gap';
  }

  return String(value);
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
      render: (value: string) => <Tag color={STATUS_COLOR_MAP[value] || 'default'}>{value}</Tag>,
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
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (value: string) => <Tag color={STATUS_COLOR_MAP[value] || 'default'}>{value}</Tag>,
    },
    {
      title: '重试次数',
      dataIndex: 'retry_count',
      key: 'retry_count',
      render: (value: number | undefined) => formatContractField(value),
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

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Title level={2} style={{ marginBottom: 8 }}>
            {base?.name ?? '知识库详情'}
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            在这里管理文档上传、入库任务和基础知识库信息。
          </Paragraph>
        </div>
        <Space wrap>
          <Button icon={<ReloadOutlined />} onClick={() => void refreshDetail()}>
            刷新
          </Button>
          <Link href="/retrieval-lab">
            <Button icon={<SearchOutlined />}>打开检索实验室</Button>
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
              {base?.status ?? '未知'}
            </Tag>
          </Text>
          <Text>Collection: {base?.vector_collection || 'Contract gap'}</Text>
          <Text>描述：{base?.description || '暂无描述。'}</Text>
          <Text>创建时间：{base?.created_at ?? '未知'}</Text>
        </Space>
      </Card>

      <Spin spinning={isLoading || isBasesLoading} indicator={<SyncOutlined spin />}>
        <Tabs
          items={[
            {
              key: 'documents',
              label: `文档（${documentsTotal}）`,
              children: (
                <Card>
                  <Table
                    rowKey="id"
                    columns={documentColumns}
                    dataSource={documents}
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
        destroyOnClose
      >
        <Space direction="vertical" className="w-full">
          <Text type="secondary">当前知识库：{base?.name ?? `#${kbId}`}</Text>
          <Upload {...uploadProps} maxCount={10}>
            <Button icon={<UploadOutlined />}>选择文件</Button>
          </Upload>
          <Text type="secondary">支持格式：PDF、MD、TXT、MARKDOWN、DOCX、HTML、HTM</Text>
        </Space>
      </Modal>
    </div>
  );
}
