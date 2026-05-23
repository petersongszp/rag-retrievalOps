'use client';

import { useState, useEffect, useCallback } from 'react';
import {
  Layout,
  Typography,
  Button,
  Form,
  Input,
  Table,
  Upload,
  Select,
  Space,
  Tag,
  Modal,
  message,
  Card,
  Divider,
  Tooltip,
  Badge,
  Spin,
} from 'antd';
import {
  PlusOutlined,
  UploadOutlined,
  DeleteOutlined,
  ReloadOutlined,
  SearchOutlined,
  FileTextOutlined,
  DatabaseOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  SyncOutlined,
  StopOutlined,
} from '@ant-design/icons';
import type { UploadProps, UploadFile } from 'antd';
import apiClient from '@/services/api/client';
import { KB_ADMIN_API } from '@/config/api';
import type { KnowledgeBase, KBDocument, KBIngestJob, ListResponse, RetrieveResponse, RetrieveItem } from '@/types/kb';

const { Header, Content, Sider } = Layout;
const { Title, Text } = Typography;
const { TextArea } = Input;
const { Option } = Select;

const STATUS_COLOR_MAP: Record<string, string> = {
  pending: 'default',
  processing: 'processing',
  completed: 'success',
  failed: 'error',
  retrying: 'warning',
  dead: 'error',
  canceled: 'default',
};

const STATUS_ICON_MAP: Record<string, React.ReactNode> = {
  pending: <ClockCircleOutlined />,
  processing: <SyncOutlined spin />,
  completed: <CheckCircleOutlined />,
  failed: <CloseCircleOutlined />,
  retrying: <SyncOutlined />,
  dead: <StopOutlined />,
  canceled: <StopOutlined />,
};

export default function AdminPage() {
  const [bases, setBases] = useState<KnowledgeBase[]>([]);
  const [selectedBase, setSelectedBase] = useState<KnowledgeBase | null>(null);
  const [documents, setDocuments] = useState<KBDocument[]>([]);
  const [jobs, setJobs] = useState<KBIngestJob[]>([]);
  const [loading, setLoading] = useState(false);
  const [createBaseModalVisible, setCreateBaseModalVisible] = useState(false);
  const [uploadModalVisible, setUploadModalVisible] = useState(false);
  const [testRetrieveModalVisible, setTestRetrieveModalVisible] = useState(false);
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [form] = Form.useForm();
  const [retrieveForm] = Form.useForm();
  const [retrieveResult, setRetrieveResult] = useState<RetrieveResponse | null>(null);
  const [retrieveLoading, setRetrieveLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<'documents' | 'jobs'>('documents');

  const fetchBases = useCallback(async () => {
    try {
      setLoading(true);
      const data = await apiClient.get(KB_ADMIN_API.LIST_BASES) as any;
      setBases(data.items || []);
      if (data.items && data.items.length > 0 && !selectedBase) {
        setSelectedBase(data.items[0]);
      }
    } catch (error) {
      console.error('Failed to fetch knowledge bases:', error);
      message.error('获取知识库列表失败');
    } finally {
      setLoading(false);
    }
  }, [selectedBase]);

  const fetchDocuments = useCallback(async (kbId: number) => {
    try {
      const data = await apiClient.get(KB_ADMIN_API.LIST_DOCUMENTS, {
        params: { kb_id: kbId },
      }) as any;
      setDocuments(data.items || []);
    } catch (error) {
      console.error('Failed to fetch documents:', error);
      message.error('获取文档列表失败');
    }
  }, []);

  const fetchJobs = useCallback(async () => {
    try {
      const data = await apiClient.get(KB_ADMIN_API.LIST_JOBS) as any;
      setJobs(data.items || []);
    } catch (error) {
      console.error('Failed to fetch jobs:', error);
      message.error('获取任务列表失败');
    }
  }, []);

  useEffect(() => {
    fetchBases();
  }, [fetchBases]);

  useEffect(() => {
    if (selectedBase) {
      fetchDocuments(selectedBase.id);
      fetchJobs();
    }
  }, [selectedBase, fetchDocuments, fetchJobs]);

  const handleCreateBase = async (values: { name: string; description?: string }) => {
    try {
      const newBase = await apiClient.post(KB_ADMIN_API.CREATE_BASE, values) as any;
      message.success('知识库创建成功');
      setCreateBaseModalVisible(false);
      form.resetFields();
      fetchBases();
      setSelectedBase(newBase);
    } catch (error) {
      console.error('Failed to create knowledge base:', error);
      message.error('创建知识库失败');
    }
  };

  const uploadProps: UploadProps = {
    fileList,
    onChange: ({ fileList: newFileList }) => setFileList(newFileList),
    beforeUpload: () => false,
    accept: '.pdf,.md,.txt,.markdown',
  };

  const handleUpload = async () => {
    if (!selectedBase || fileList.length === 0) {
      message.warning('请选择知识库和文件');
      return;
    }

    const formData = new FormData();
    formData.append('kb_id', selectedBase.id.toString());
    fileList.forEach((file) => {
      if (file.originFileObj) {
        formData.append('file', file.originFileObj);
      }
    });

    try {
      setLoading(true);
      await apiClient.post(KB_ADMIN_API.UPLOAD_DOCUMENT, formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });
      message.success('文档上传成功，正在处理中...');
      setUploadModalVisible(false);
      setFileList([]);
      setTimeout(() => {
        fetchDocuments(selectedBase.id);
        fetchJobs();
      }, 1000);
    } catch (error) {
      console.error('Failed to upload document:', error);
      message.error('文档上传失败');
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteDocument = async (doc: KBDocument) => {
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除文档 "${doc.file_name}" 吗？`,
      onOk: async () => {
        try {
          await apiClient.delete(KB_ADMIN_API.DELETE_DOCUMENT(doc.id));
          message.success('文档删除成功');
          if (selectedBase) {
            fetchDocuments(selectedBase.id);
          }
        } catch (error) {
          console.error('Failed to delete document:', error);
          message.error('文档删除失败');
        }
      },
    });
  };

  const handleRetryJob = async (job: KBIngestJob) => {
    try {
      await apiClient.post(KB_ADMIN_API.RETRY_JOB(job.id));
      message.success('任务已重新开始');
      fetchJobs();
      if (selectedBase) {
        fetchDocuments(selectedBase.id);
      }
    } catch (error) {
      console.error('Failed to retry job:', error);
      message.error('重试任务失败');
    }
  };

  const handleCancelJob = async (job: KBIngestJob) => {
    Modal.confirm({
      title: '确认取消',
      content: `确定要取消任务 ID: ${job.id} 吗？`,
      onOk: async () => {
        try {
          await apiClient.post(KB_ADMIN_API.CANCEL_JOB(job.id));
          message.success('任务已取消');
          fetchJobs();
        } catch (error) {
          console.error('Failed to cancel job:', error);
          message.error('取消任务失败');
        }
      },
    });
  };

  const handleTestRetrieve = async (values: { query: string; top_k?: number }) => {
    if (!selectedBase) {
      message.warning('请先选择知识库');
      return;
    }

    try {
      setRetrieveLoading(true);
      const result = await apiClient.post(KB_ADMIN_API.RETRIEVE, {
        query: values.query,
        top_k: values.top_k || 5,
        kb_id: selectedBase.id,
      }) as any;
      setRetrieveResult(result);
    } catch (error) {
      console.error('Failed to retrieve:', error);
      message.error('检索失败');
    } finally {
      setRetrieveLoading(false);
    }
  };

  const documentColumns = [
    {
      title: '文件名',
      dataIndex: 'file_name',
      key: 'file_name',
      render: (text: string, record: KBDocument) => (
        <Space>
          <FileTextOutlined />
          <span>{text}</span>
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'file_type',
      key: 'file_type',
      width: 100,
      render: (type: string) => <Tag color="blue">{type.toUpperCase()}</Tag>,
    },
    {
      title: '大小',
      dataIndex: 'file_size',
      key: 'file_size',
      width: 120,
      render: (size: number) => `${(size / 1024).toFixed(2)} KB`,
    },
    {
      title: '分块数',
      dataIndex: 'chunk_count',
      key: 'chunk_count',
      width: 100,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (status: string) => (
        <Tag color={STATUS_COLOR_MAP[status] || 'default'}>
          {STATUS_ICON_MAP[status]} {status}
        </Tag>
      ),
    },
    {
      title: '错误信息',
      dataIndex: 'error_msg',
      key: 'error_msg',
      ellipsis: true,
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
    },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (_: any, record: KBDocument) => (
        <Space>
          <Button
            type="text"
            danger
            icon={<DeleteOutlined />}
            onClick={() => handleDeleteDocument(record)}
          >
            删除
          </Button>
        </Space>
      ),
    },
  ];

  const jobColumns = [
    {
      title: '任务 ID',
      dataIndex: 'id',
      key: 'id',
      width: 100,
    },
    {
      title: '文档 ID',
      dataIndex: 'document_id',
      key: 'document_id',
      width: 100,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (status: string) => (
        <Tag color={STATUS_COLOR_MAP[status] || 'default'}>
          {STATUS_ICON_MAP[status]} {status}
        </Tag>
      ),
    },
    {
      title: '重试次数',
      dataIndex: 'retry_count',
      key: 'retry_count',
      width: 100,
    },
    {
      title: '错误信息',
      dataIndex: 'error_msg',
      key: 'error_msg',
      ellipsis: true,
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
    },
    {
      title: '操作',
      key: 'action',
      width: 150,
      render: (_: any, record: KBIngestJob) => (
        <Space>
          {(record.status === 'failed' || record.status === 'dead') && (
            <Button
              type="text"
              icon={<ReloadOutlined />}
              onClick={() => handleRetryJob(record)}
            >
              重试
            </Button>
          )}
          {record.status !== 'completed' && record.status !== 'canceled' && record.status !== 'dead' && (
            <Button
              type="text"
              danger
              icon={<StopOutlined />}
              onClick={() => handleCancelJob(record)}
            >
              取消
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <Layout className="min-h-screen">
      <Header className="bg-white border-b border-slate-200 px-6 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <DatabaseOutlined className="text-2xl text-indigo-600" />
          <Title level={4} style={{ margin: 0 }}>知识库管理后台</Title>
        </div>
        <Space>
          <Text type="secondary">端口: 3001</Text>
          <Button icon={<ReloadOutlined />} onClick={() => {
            fetchBases();
            if (selectedBase) {
              fetchDocuments(selectedBase.id);
            }
            fetchJobs();
          }}>
            刷新
          </Button>
        </Space>
      </Header>
      
      <Layout>
        <Sider width={300} className="bg-white border-r border-slate-200">
          <div className="p-4">
            <div className="flex items-center justify-between mb-4">
              <Text strong>知识库列表</Text>
              <Button
                type="primary"
                size="small"
                icon={<PlusOutlined />}
                onClick={() => setCreateBaseModalVisible(true)}
              >
                新建
              </Button>
            </div>
            <div className="space-y-2">
              {bases.map((base) => (
                <Card
                  key={base.id}
                  size="small"
                  hoverable
                  className={`cursor-pointer ${selectedBase?.id === base.id ? 'border-indigo-500 bg-indigo-50' : ''}`}
                  onClick={() => setSelectedBase(base)}
                >
                  <div className="flex items-center justify-between">
                    <Space>
                      <DatabaseOutlined className={selectedBase?.id === base.id ? 'text-indigo-600' : 'text-slate-400'} />
                      <span className="font-medium">{base.name}</span>
                    </Space>
                    <Tag color={base.status === 'active' ? 'success' : 'default'}>
                      {base.status}
                    </Tag>
                  </div>
                  {base.description && (
                    <Text type="secondary" className="text-xs mt-2 block" ellipsis>
                      {base.description}
                    </Text>
                  )}
                </Card>
              ))}
            </div>
          </div>
        </Sider>

        <Content className="p-6 bg-slate-50">
          <Spin spinning={loading}>
            {selectedBase ? (
              <div className="space-y-6">
                <div className="flex items-center justify-between">
                  <div>
                    <Title level={3} style={{ margin: 0 }}>{selectedBase.name}</Title>
                    <Text type="secondary">{selectedBase.description}</Text>
                  </div>
                  <Space>
                    <Button
                      type="primary"
                      icon={<SearchOutlined />}
                      onClick={() => setTestRetrieveModalVisible(true)}
                    >
                      测试检索
                    </Button>
                    <Button
                      type="primary"
                      icon={<UploadOutlined />}
                      onClick={() => setUploadModalVisible(true)}
                    >
                      上传文档
                    </Button>
                  </Space>
                </div>

                <Card
                  tabList={[
                    { key: 'documents', label: '文档列表' },
                    { key: 'jobs', label: '任务列表' },
                  ]}
                  activeTabKey={activeTab}
                  onTabChange={(key) => setActiveTab(key as 'documents' | 'jobs')}
                >
                  {activeTab === 'documents' ? (
                    <Table
                      columns={documentColumns}
                      dataSource={documents}
                      rowKey="id"
                      pagination={{ pageSize: 10 }}
                    />
                  ) : (
                    <Table
                      columns={jobColumns}
                      dataSource={jobs}
                      rowKey="id"
                      pagination={{ pageSize: 10 }}
                    />
                  )}
                </Card>
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center h-full py-20">
                <DatabaseOutlined className="text-6xl text-slate-300 mb-4" />
                <Title level={4} type="secondary">请选择或创建知识库</Title>
                <Button
                  type="primary"
                  size="large"
                  icon={<PlusOutlined />}
                  onClick={() => setCreateBaseModalVisible(true)}
                  className="mt-4"
                >
                  创建知识库
                </Button>
              </div>
            )}
          </Spin>
        </Content>
      </Layout>

      {/* 创建知识库模态框 */}
      <Modal
        title="创建知识库"
        open={createBaseModalVisible}
        onOk={form.submit}
        onCancel={() => setCreateBaseModalVisible(false)}
        okText="创建"
        cancelText="取消"
      >
        <Form form={form} layout="vertical" onFinish={handleCreateBase}>
          <Form.Item
            label="知识库名称"
            name="name"
            rules={[{ required: true, message: '请输入知识库名称' }]}
          >
            <Input placeholder="例如：Go 专项题库" />
          </Form.Item>
          <Form.Item
            label="描述"
            name="description"
          >
            <TextArea placeholder="可选，描述该知识库的用途" rows={3} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 上传文档模态框 */}
      <Modal
        title="上传文档"
        open={uploadModalVisible}
        onOk={handleUpload}
        onCancel={() => {
          setUploadModalVisible(false);
          setFileList([]);
        }}
        okText="上传"
        cancelText="取消"
        confirmLoading={loading}
      >
        <div className="mb-4">
          <Text type="secondary">当前知识库: {selectedBase?.name}</Text>
        </div>
        <Upload {...uploadProps} maxCount={10}>
          <Button icon={<UploadOutlined />}>选择文件</Button>
        </Upload>
        <div className="mt-4 text-sm text-slate-500">
          支持格式: PDF, MD, TXT, MARKDOWN
        </div>
      </Modal>

      {/* 测试检索模态框 */}
      <Modal
        title="测试检索"
        open={testRetrieveModalVisible}
        onCancel={() => {
          setTestRetrieveModalVisible(false);
          setRetrieveResult(null);
        }}
        footer={null}
        width={800}
      >
        <Form form={retrieveForm} layout="vertical" onFinish={handleTestRetrieve}>
          <Form.Item
            label="检索查询"
            name="query"
            rules={[{ required: true, message: '请输入检索内容' }]}
          >
            <TextArea placeholder="输入要搜索的内容" rows={3} />
          </Form.Item>
          <Form.Item
            label="返回数量"
            name="top_k"
            initialValue={5}
          >
            <Select>
              <Option value={3}>3</Option>
              <Option value={5}>5</Option>
              <Option value={10}>10</Option>
            </Select>
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={retrieveLoading} icon={<SearchOutlined />}>
              检索
            </Button>
          </Form.Item>
        </Form>

        {retrieveResult && (
          <div className="mt-6">
            <Divider>检索结果</Divider>
            <div className="space-y-4">
              {retrieveResult.items.length === 0 ? (
                <Text type="secondary">没有找到相关内容</Text>
              ) : (
                retrieveResult.items.map((item, index) => (
                  <Card key={index} size="small">
                    <div className="flex items-start justify-between mb-2">
                      <Badge count={index + 1} color="blue" />
                      <Text type="secondary">得分: {item.score.toFixed(4)}</Text>
                    </div>
                    <Text className="mb-3 block">{item.content}</Text>
                    <div className="text-xs text-slate-500">
                      <div>文档: {item.citation.file_name}</div>
                      <div>分块: {item.citation.chunk_index}</div>
                      <div>来源: {item.source.route}</div>
                    </div>
                  </Card>
                ))
              )}
            </div>
          </div>
        )}
      </Modal>
    </Layout>
  );
}
